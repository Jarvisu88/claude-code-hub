package rectifier

import (
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
)

const (
	// DefaultThinkingBudget is the default budget_tokens value when the
	// rectifier needs to inject or replace thinking configuration.
	DefaultThinkingBudget = 32000

	// DefaultMaxTokensForThinking is the minimum max_tokens required when
	// thinking is enabled with the default budget.
	DefaultMaxTokensForThinking = 64000

	// MinThinkingBudget is the Anthropic-mandated minimum.
	MinThinkingBudget float64 = 1024
)

// thinkingBudgetEnabled returns true when the thinking budget rectifier
// is enabled via system settings. Defaults to true when settings are nil.
func thinkingBudgetEnabled(settings *model.SystemSettings) bool {
	if settings == nil {
		return true
	}
	return settings.EnableThinkingBudgetRectifier
}

// RectifyThinkingBudget ensures that the request has a valid thinking budget
// configuration. If the thinking type is "adaptive" it is left untouched;
// otherwise the rectifier sets type=enabled with a sensible budget and ensures
// max_tokens is large enough.
func RectifyThinkingBudget(body map[string]any) Result {
	result := Result{
		Name:    "thinking_budget",
		Details: map[string]any{},
	}

	thinking, ok := body["thinking"].(map[string]any)
	if !ok {
		// No thinking block present -- nothing to rectify.
		return result
	}

	// If the client explicitly chose adaptive thinking, respect that.
	if currentType, ok := thinking["type"].(string); ok && currentType == "adaptive" {
		return result
	}

	// Check current budget.
	currentBudget := float64(0)
	if b, ok := thinking["budget_tokens"].(float64); ok {
		currentBudget = b
	}

	// If a valid budget is already set (>= minimum), leave it alone.
	if currentBudget >= MinThinkingBudget {
		return result
	}

	// Set a reasonable default.
	result.Details["original_type"], _ = thinking["type"].(string)
	result.Details["original_budget"] = currentBudget

	thinking["type"] = "enabled"
	thinking["budget_tokens"] = float64(DefaultThinkingBudget)

	beforeMaxTokens, _ := body["max_tokens"].(float64)
	if beforeMaxTokens == 0 || beforeMaxTokens < float64(DefaultThinkingBudget)+1 {
		result.Details["original_max_tokens"] = beforeMaxTokens
		body["max_tokens"] = float64(DefaultMaxTokensForThinking)
	}

	result.Applied = true
	return result
}

// DetectThinkingBudgetTrigger checks whether an error message from the upstream
// provider indicates a thinking-budget issue (e.g. budget_tokens too low).
func DetectThinkingBudgetTrigger(errorMessage string) bool {
	lower := strings.ToLower(strings.TrimSpace(errorMessage))
	if lower == "" {
		return false
	}
	hasBudgetRef := strings.Contains(lower, "budget_tokens") || strings.Contains(lower, "budget tokens")
	hasThinkingRef := strings.Contains(lower, "thinking")
	hasConstraint := strings.Contains(lower, "greater than or equal to 1024") ||
		strings.Contains(lower, ">= 1024") ||
		(strings.Contains(lower, "1024") && strings.Contains(lower, "input should be"))
	return hasBudgetRef && hasThinkingRef && hasConstraint
}
