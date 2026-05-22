package v1

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/gin-gonic/gin"
)

type fakeProxyRequestFilterStore struct {
	items []*model.RequestFilter
	err   error
}

func (f fakeProxyRequestFilterStore) ListActive(_ context.Context) ([]*model.RequestFilter, error) {
	return f.items, f.err
}

type fakeProxySensitiveWordStore struct {
	items []*model.SensitiveWord
	err   error
}

func (f fakeProxySensitiveWordStore) ListActive(_ context.Context) ([]*model.SensitiveWord, error) {
	return f.items, f.err
}

func TestApplyGlobalRequestFiltersRemovesHeader(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
	c.Request.Header.Set("x-test", "remove-me")

	handler := &Handler{
		requestFilters: fakeProxyRequestFilterStore{items: []*model.RequestFilter{{
			Name:        "remove-header",
			Scope:       "header",
			Action:      "remove",
			Target:      "x-test",
			BindingType: "global",
			IsEnabled:   true,
		}}},
	}
	if _, err := handler.applyGlobalRequestFilters(c, map[string]any{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := c.Request.Header.Get("x-test"); got != "" {
		t.Fatalf("expected header to be removed, got %q", got)
	}
}

func TestApplyProviderRequestFiltersSetsBodyPath(t *testing.T) {
	body := map[string]any{"input": map[string]any{"message": "hello"}}
	handler := &Handler{
		requestFilters: fakeProxyRequestFilterStore{items: []*model.RequestFilter{{
			Name:           "set-body",
			Scope:          "body",
			Action:         "set",
			Target:         "input.message",
			Replacement:    "updated",
			BindingType:    "providers",
			ProviderIds:    []int{9},
			ExecutionPhase: "guard",
			IsEnabled:      true,
		}}},
	}
	if _, err := handler.applyProviderRequestFilters(nil, body, &model.Provider{ID: 9}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := getBodyPath(body, "input.message"); got != "updated" {
		t.Fatalf("expected body path to be updated, got %#v", got)
	}
}

func TestEnsureNoSensitiveWordsBlocksMatchedRequest(t *testing.T) {
	handler := &Handler{
		sensitiveWords: fakeProxySensitiveWordStore{items: []*model.SensitiveWord{{
			Word:      "blocked-term",
			MatchType: "contains",
			IsEnabled: true,
		}}},
	}
	err := handler.ensureNoSensitiveWords(map[string]any{"messages": []any{map[string]any{"content": "hello blocked-term"}}})
	if err == nil {
		t.Fatal("expected sensitive word match to be blocked")
	}
}

func TestEnsureNoSensitiveWordsExactMatchUsesExtractedText(t *testing.T) {
	handler := &Handler{
		sensitiveWords: fakeProxySensitiveWordStore{items: []*model.SensitiveWord{{
			Word:      "hello",
			MatchType: "exact",
			IsEnabled: true,
		}}},
	}
	if err := handler.ensureNoSensitiveWords(map[string]any{"messages": []any{map[string]any{"content": " Hello "}}}); err == nil {
		t.Fatal("expected exact match to be blocked")
	}
}

func TestPolicyResourcesFailClosedOnStoreError(t *testing.T) {
	handler := &Handler{
		requestFilters: fakeProxyRequestFilterStore{err: errors.New("db down")},
		sensitiveWords: fakeProxySensitiveWordStore{err: errors.New("cache down")},
	}
	if _, err := handler.applyGlobalRequestFilters(nil, map[string]any{}); err == nil {
		t.Fatal("expected request filter store error to fail closed")
	}
	if err := handler.ensureNoSensitiveWords(map[string]any{"content": "hello"}); err == nil {
		t.Fatal("expected sensitive word store error to fail closed")
	}
}

func TestApplyRequestFilterRejectsUnsupportedAdvancedModes(t *testing.T) {
	_, err := applyRequestFilter(nil, map[string]any{}, &model.RequestFilter{
		Name:           "advanced",
		Scope:          "body",
		Action:         "set",
		Target:         "input",
		RuleMode:       "advanced",
		ExecutionPhase: "final",
		IsEnabled:      true,
	})
	if err == nil {
		t.Fatal("expected unsupported runtime filter to be rejected")
	}
}
