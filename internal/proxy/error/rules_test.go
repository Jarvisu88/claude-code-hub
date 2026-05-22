package proxyerror

import (
	"encoding/json"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------
// matchesRule
// ---------------------------------------------------------------

func TestMatchesRule_Regex(t *testing.T) {
	rule := &model.ErrorRule{
		Pattern:   `rate.limit`,
		MatchType: "regex",
	}
	assert.True(t, matchesRule(rule, "rate limit exceeded", ""))
	assert.True(t, matchesRule(rule, "", `{"error":"rate_limit reached"}`))
	assert.False(t, matchesRule(rule, "model not found", ""))
}

func TestMatchesRule_Contains(t *testing.T) {
	rule := &model.ErrorRule{
		Pattern:   "overloaded",
		MatchType: "contains",
	}
	assert.True(t, matchesRule(rule, "Server is overloaded, try again", ""))
	assert.True(t, matchesRule(rule, "", "server OVERLOADED"))
	assert.False(t, matchesRule(rule, "model not found", ""))
}

func TestMatchesRule_Exact(t *testing.T) {
	rule := &model.ErrorRule{
		Pattern:   "model not found",
		MatchType: "exact",
	}
	assert.True(t, matchesRule(rule, "Model Not Found", ""))
	assert.True(t, matchesRule(rule, "", "model not found"))
	assert.False(t, matchesRule(rule, "model not found at this time", ""))
}

func TestMatchesRule_EmptyPattern(t *testing.T) {
	rule := &model.ErrorRule{
		Pattern:   "",
		MatchType: "regex",
	}
	assert.False(t, matchesRule(rule, "anything", "anything"))
}

func TestMatchesRule_InvalidRegex(t *testing.T) {
	rule := &model.ErrorRule{
		Pattern:   "[invalid",
		MatchType: "regex",
	}
	assert.False(t, matchesRule(rule, "anything", "anything"))
}

func TestMatchesRule_DefaultMatchType(t *testing.T) {
	// Empty match type should default to regex.
	rule := &model.ErrorRule{
		Pattern:   `rate.limit`,
		MatchType: "",
	}
	assert.True(t, matchesRule(rule, "rate limit exceeded", ""))
}

func TestMatchesRule_BodyFallback(t *testing.T) {
	rule := &model.ErrorRule{
		Pattern:   "overloaded",
		MatchType: "contains",
	}
	// Error message does not match, but body does.
	assert.True(t, matchesRule(rule, "generic error", `{"error":"server overloaded"}`))
}

// ---------------------------------------------------------------
// RuleMatcher
// ---------------------------------------------------------------

func TestRuleMatcher_Match(t *testing.T) {
	loader := &staticRuleLoader{
		rules: []*model.ErrorRule{
			{
				ID:        1,
				Pattern:   "overloaded",
				MatchType: "contains",
				Category:  "system_error",
				IsEnabled: true,
				Priority:  10,
			},
			{
				ID:        2,
				Pattern:   `rate.limit`,
				MatchType: "regex",
				Category:  "rate_limit",
				IsEnabled: true,
				Priority:  20,
			},
		},
	}

	matcher := NewRuleMatcher(loader, 0)

	t.Run("matches first rule", func(t *testing.T) {
		result, err := matcher.Match("Server is overloaded", nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.Rule.ID)
	})

	t.Run("matches second rule", func(t *testing.T) {
		result, err := matcher.Match("rate limit exceeded", nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 2, result.Rule.ID)
	})

	t.Run("no match", func(t *testing.T) {
		result, err := matcher.Match("unknown error", nil)
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestRuleMatcher_MatchFromBody(t *testing.T) {
	loader := &staticRuleLoader{
		rules: []*model.ErrorRule{
			{
				ID:        1,
				Pattern:   "insufficient_quota",
				MatchType: "contains",
				Category:  "rate_limit",
				IsEnabled: true,
			},
		},
	}

	matcher := NewRuleMatcher(loader, 0)
	result, err := matcher.Match("", []byte(`{"error":{"type":"insufficient_quota","message":"Quota exceeded"}}`))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.Rule.ID)
}

func TestRuleMatcher_PriorityOrder(t *testing.T) {
	// Both rules match, but the one with lower priority number wins.
	loader := &staticRuleLoader{
		rules: []*model.ErrorRule{
			{
				ID:        1,
				Pattern:   "error",
				MatchType: "contains",
				Category:  "generic",
				IsEnabled: true,
				Priority:  1,
			},
			{
				ID:        2,
				Pattern:   "error",
				MatchType: "contains",
				Category:  "specific",
				IsEnabled: true,
				Priority:  2,
			},
		},
	}

	matcher := NewRuleMatcher(loader, 0)
	result, err := matcher.Match("some error occurred", nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	// First rule (lower priority number) should win.
	assert.Equal(t, 1, result.Rule.ID)
}

func TestRuleMatcher_SkipsDisabledRules(t *testing.T) {
	loader := &staticRuleLoader{
		rules: []*model.ErrorRule{
			{
				ID:        1,
				Pattern:   "error",
				MatchType: "contains",
				Category:  "generic",
				IsEnabled: false,
			},
		},
	}

	matcher := NewRuleMatcher(loader, 0)
	result, err := matcher.Match("some error occurred", nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestRuleMatcher_EmptyRules(t *testing.T) {
	loader := &staticRuleLoader{rules: nil}
	matcher := NewRuleMatcher(loader, 0)
	result, err := matcher.Match("some error", nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestRuleMatcher_Invalidate(t *testing.T) {
	callCount := 0
	loader := &countingRuleLoader{
		rules: []*model.ErrorRule{
			{
				ID:        1,
				Pattern:   "test",
				MatchType: "contains",
				IsEnabled: true,
			},
		},
		callCount: &callCount,
	}

	matcher := NewRuleMatcher(loader, 0) // ttl=0 means always default (30s)

	// First call loads rules.
	_, _ = matcher.Match("test", nil)
	assert.Equal(t, 1, callCount)

	// Second call should use cache.
	_, _ = matcher.Match("test", nil)
	assert.Equal(t, 1, callCount)

	// After invalidation, should reload.
	matcher.Invalidate()
	_, _ = matcher.Match("test", nil)
	assert.Equal(t, 2, callCount)
}

// ---------------------------------------------------------------
// Apply
// ---------------------------------------------------------------

func TestApply_OverrideStatusCode(t *testing.T) {
	statusOverride := 503
	rule := &model.ErrorRule{
		OverrideStatusCode: &statusOverride,
		Category:           "system_error",
	}

	result := Apply(rule, 500, []byte(`{"error":"internal"}`))
	assert.Equal(t, 503, result.StatusCode)
	assert.Equal(t, "system_error", result.FallbackReason)
}

func TestApply_OverrideResponse(t *testing.T) {
	rule := &model.ErrorRule{
		OverrideResponse: map[string]any{
			"error": map[string]any{
				"type":    "overloaded_error",
				"message": "Server is temporarily overloaded",
			},
		},
		Category: "system_error",
	}

	result := Apply(rule, 529, []byte(`{"error":"original"}`))
	assert.True(t, result.OverrideApplied)
	assert.Equal(t, "Server is temporarily overloaded", result.ErrorMessage)

	// Verify the body was replaced.
	var parsed map[string]any
	err := json.Unmarshal(result.Body, &parsed)
	require.NoError(t, err)
	errObj := parsed["error"].(map[string]any)
	assert.Equal(t, "overloaded_error", errObj["type"])
}

func TestApply_BothOverrides(t *testing.T) {
	statusOverride := 503
	rule := &model.ErrorRule{
		OverrideStatusCode: &statusOverride,
		OverrideResponse: map[string]any{
			"error": map[string]any{
				"type":    "overloaded_error",
				"message": "Temporarily unavailable",
			},
		},
		Category: "system_error",
	}

	result := Apply(rule, 529, []byte(`{"error":"original"}`))
	assert.Equal(t, 503, result.StatusCode)
	assert.True(t, result.OverrideApplied)
	assert.Equal(t, "Temporarily unavailable", result.ErrorMessage)
}

func TestApply_NilRule(t *testing.T) {
	result := Apply(nil, 500, []byte(`{"error":"internal"}`))
	assert.Equal(t, 500, result.StatusCode)
	assert.False(t, result.OverrideApplied)
	assert.Equal(t, "system_error", result.FallbackReason)
}

func TestApply_ResourceNotFoundCategory(t *testing.T) {
	rule := &model.ErrorRule{
		Category: "resource_not_found",
	}

	result := Apply(rule, 404, []byte(`{"error":"not found"}`))
	assert.Equal(t, "resource_not_found", result.FallbackReason)
}

func TestApply_DefaultFallbackReason(t *testing.T) {
	rule := &model.ErrorRule{
		Category: "custom_category",
	}

	result := Apply(rule, 400, []byte(`{"error":"bad request"}`))
	assert.Equal(t, "", result.FallbackReason) // 400 does not map to a named reason
}

// ---------------------------------------------------------------
// ExtractErrorMessage
// ---------------------------------------------------------------

func TestExtractErrorMessage_NestedError(t *testing.T) {
	body := []byte(`{"error":{"type":"invalid_request_error","message":"Invalid model"}}`)
	assert.Equal(t, "Invalid model", ExtractErrorMessage(body))
}

func TestExtractErrorMessage_StringError(t *testing.T) {
	body := []byte(`{"error":"Something went wrong"}`)
	assert.Equal(t, "Something went wrong", ExtractErrorMessage(body))
}

func TestExtractErrorMessage_TopLevelMessage(t *testing.T) {
	body := []byte(`{"message":"Rate limit exceeded"}`)
	assert.Equal(t, "Rate limit exceeded", ExtractErrorMessage(body))
}

func TestExtractErrorMessage_PlainText(t *testing.T) {
	body := []byte(`Service Unavailable`)
	assert.Equal(t, "Service Unavailable", ExtractErrorMessage(body))
}

func TestExtractErrorMessage_Empty(t *testing.T) {
	assert.Equal(t, "", ExtractErrorMessage(nil))
	assert.Equal(t, "", ExtractErrorMessage([]byte("")))
	assert.Equal(t, "", ExtractErrorMessage([]byte("   ")))
}

// ---------------------------------------------------------------
// ExtractRequestID
// ---------------------------------------------------------------

func TestExtractRequestID_TopLevel(t *testing.T) {
	body := []byte(`{"request_id":"req-123","error":"bad request"}`)
	assert.Equal(t, "req-123", ExtractRequestID(body))
}

func TestExtractRequestID_NestedInError(t *testing.T) {
	body := []byte(`{"error":{"message":"bad","request_id":"req-456"}}`)
	assert.Equal(t, "req-456", ExtractRequestID(body))
}

func TestExtractRequestID_CamelCase(t *testing.T) {
	body := []byte(`{"requestId":"req-789"}`)
	assert.Equal(t, "req-789", ExtractRequestID(body))
}

func TestExtractRequestID_Empty(t *testing.T) {
	assert.Equal(t, "", ExtractRequestID(nil))
	assert.Equal(t, "", ExtractRequestID([]byte(`{}`)))
}

// ---------------------------------------------------------------
// NormalizeCategory
// ---------------------------------------------------------------

func TestNormalizeCategory(t *testing.T) {
	assert.Equal(t, "system_error", NormalizeCategory("  System_Error  "))
	assert.Equal(t, "rate_limit", NormalizeCategory("Rate_Limit"))
	assert.Equal(t, "", NormalizeCategory(""))
}

// ---------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------

type staticRuleLoader struct {
	rules []*model.ErrorRule
}

func (l *staticRuleLoader) ListActive() ([]*model.ErrorRule, error) {
	return l.rules, nil
}

type countingRuleLoader struct {
	rules     []*model.ErrorRule
	callCount *int
}

func (l *countingRuleLoader) ListActive() ([]*model.ErrorRule, error) {
	*l.callCount++
	return l.rules, nil
}
