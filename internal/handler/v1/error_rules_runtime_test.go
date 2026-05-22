package v1

import (
	"context"
	"strings"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
)

type fakeProxyErrorRuleStore struct {
	items []*model.ErrorRule
	err   error
}

func (f fakeProxyErrorRuleStore) ListActive(_ context.Context) ([]*model.ErrorRule, error) {
	return f.items, f.err
}

func TestEvaluateUpstreamErrorDecisionInfersFake200SystemError(t *testing.T) {
	handler := &Handler{}
	decision, err := handler.evaluateUpstreamErrorDecision(context.Background(), 200, []byte(`{"error":{"message":"service unavailable"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.StatusCode != 503 {
		t.Fatalf("expected inferred 503, got %d", decision.StatusCode)
	}
	if decision.FallbackReason != "system_error" {
		t.Fatalf("expected system_error fallback, got %q", decision.FallbackReason)
	}
	if !strings.Contains(strings.ToLower(decision.ErrorMessage), "service unavailable") {
		t.Fatalf("expected extracted error message, got %q", decision.ErrorMessage)
	}
}

func TestEvaluateUpstreamErrorDecisionAppliesMatchingErrorRuleOverride(t *testing.T) {
	handler := &Handler{
		errorRules: fakeProxyErrorRuleStore{items: []*model.ErrorRule{{
			Pattern:            "quota exceeded",
			MatchType:          "contains",
			Category:           "quota_limit",
			OverrideStatusCode: intPointer(429),
			OverrideResponse: map[string]any{
				"error": map[string]any{
					"type":    "rate_limit_error",
					"code":    "quota_exceeded",
					"message": "quota exceeded",
				},
			},
			IsEnabled: true,
		}}},
	}
	decision, err := handler.evaluateUpstreamErrorDecision(context.Background(), 500, []byte(`{"error":{"message":"quota exceeded upstream"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.StatusCode != 429 {
		t.Fatalf("expected overridden 429, got %d", decision.StatusCode)
	}
	if decision.FallbackReason != "" {
		t.Fatalf("expected non-fallback override, got %q", decision.FallbackReason)
	}
	if !strings.Contains(string(decision.ResponseBody), `"quota_exceeded"`) {
		t.Fatalf("expected rewritten response body, got %s", string(decision.ResponseBody))
	}
	if !strings.Contains(decision.ErrorMessage, "quota exceeded") {
		t.Fatalf("expected rewritten error message, got %q", decision.ErrorMessage)
	}
}
