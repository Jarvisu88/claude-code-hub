package v1

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
)

type thinkingSignatureRectifierTrigger string

const (
	thinkingSignatureTriggerInvalidSignature thinkingSignatureRectifierTrigger = "invalid_signature_in_thinking_block"
	thinkingSignatureTriggerMissingThinking  thinkingSignatureRectifierTrigger = "assistant_message_must_start_with_thinking"
	thinkingSignatureTriggerInvalidRequest   thinkingSignatureRectifierTrigger = "invalid_request"
)

type thinkingSignatureRectifierResult struct {
	Applied                     bool
	RemovedThinkingBlocks       int
	RemovedRedactedThinking     int
	RemovedSignatureFieldsCount int
}

func detectThinkingSignatureRectifierTrigger(errorMessage string) thinkingSignatureRectifierTrigger {
	lower := strings.ToLower(strings.TrimSpace(errorMessage))
	if lower == "" {
		return ""
	}
	if strings.Contains(lower, "must start with a thinking block") ||
		(strings.Contains(lower, "expected `thinking`") && strings.Contains(lower, "tool_use")) {
		return thinkingSignatureTriggerMissingThinking
	}
	if strings.Contains(lower, "invalid") && strings.Contains(lower, "signature") && strings.Contains(lower, "thinking") && strings.Contains(lower, "block") {
		return thinkingSignatureTriggerInvalidSignature
	}
	if strings.Contains(lower, "signature") && strings.Contains(lower, "field required") {
		return thinkingSignatureTriggerInvalidSignature
	}
	if strings.Contains(lower, "signature") && strings.Contains(lower, "extra inputs are not permitted") {
		return thinkingSignatureTriggerInvalidSignature
	}
	if (strings.Contains(lower, "thinking") || strings.Contains(lower, "redacted_thinking")) && strings.Contains(lower, "cannot be modified") {
		return thinkingSignatureTriggerInvalidSignature
	}
	if strings.Contains(lower, "illegal request") || strings.Contains(lower, "invalid request") || strings.Contains(lower, "非法请求") {
		return thinkingSignatureTriggerInvalidRequest
	}
	return ""
}

func rectifyAnthropicRequestMessage(message map[string]any) thinkingSignatureRectifierResult {
	result := thinkingSignatureRectifierResult{}
	messages, ok := message["messages"].([]any)
	if !ok {
		return result
	}
	for _, raw := range messages {
		msg, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		content, ok := msg["content"].([]any)
		if !ok {
			continue
		}
		next := make([]any, 0, len(content))
		contentChanged := false
		for _, blockRaw := range content {
			block, ok := blockRaw.(map[string]any)
			if !ok {
				next = append(next, blockRaw)
				continue
			}
			switch block["type"] {
			case "thinking":
				result.RemovedThinkingBlocks++
				contentChanged = true
				continue
			case "redacted_thinking":
				result.RemovedRedactedThinking++
				contentChanged = true
				continue
			}
			if _, exists := block["signature"]; exists {
				delete(block, "signature")
				result.RemovedSignatureFieldsCount++
				contentChanged = true
			}
			next = append(next, block)
		}
		if contentChanged {
			msg["content"] = next
			result.Applied = true
		}
	}
	return result
}

type thinkingBudgetRectifierResult struct {
	Applied bool
}

func detectThinkingBudgetRectifierTrigger(errorMessage string) bool {
	lower := strings.ToLower(strings.TrimSpace(errorMessage))
	if lower == "" {
		return false
	}
	hasBudgetRef := strings.Contains(lower, "budget_tokens") || strings.Contains(lower, "budget tokens")
	hasThinkingRef := strings.Contains(lower, "thinking")
	hasConstraint := strings.Contains(lower, "greater than or equal to 1024") || strings.Contains(lower, ">= 1024") || (strings.Contains(lower, "1024") && strings.Contains(lower, "input should be"))
	return hasBudgetRef && hasThinkingRef && hasConstraint
}

func rectifyThinkingBudget(message map[string]any) thinkingBudgetRectifierResult {
	thinking, ok := message["thinking"].(map[string]any)
	if !ok {
		thinking = map[string]any{}
		message["thinking"] = thinking
	}
	if currentType, ok := thinking["type"].(string); ok && currentType == "adaptive" {
		return thinkingBudgetRectifierResult{Applied: false}
	}
	beforeMaxTokens, _ := message["max_tokens"].(float64)
	thinking["type"] = "enabled"
	thinking["budget_tokens"] = 32000
	if beforeMaxTokens == 0 || beforeMaxTokens < 32001 {
		message["max_tokens"] = 64000
	}
	return thinkingBudgetRectifierResult{Applied: true}
}

func extractUpstreamErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return strings.TrimSpace(string(body))
	}
	if message := nestedErrorMessage(payload); message != "" {
		return message
	}
	return strings.TrimSpace(string(body))
}

func nestedErrorMessage(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if raw, ok := payload["error"]; ok {
		switch typed := raw.(type) {
		case string:
			return strings.TrimSpace(typed)
		case map[string]any:
			if message, ok := typed["message"].(string); ok {
				return strings.TrimSpace(message)
			}
		}
	}
	if message, ok := payload["message"].(string); ok {
		return strings.TrimSpace(message)
	}
	return ""
}

func (h *Handler) currentProxySystemSettings() *model.SystemSettings {
	if h == nil || h.settings == nil {
		return nil
	}
	settings, err := h.settings.Get(context.Background())
	if err != nil {
		return nil
	}
	return settings
}

func thinkingSignatureRectifierEnabled(settings *model.SystemSettings) bool {
	if settings == nil {
		return true
	}
	return settings.EnableThinkingSignatureRectifier
}

func thinkingBudgetRectifierEnabled(settings *model.SystemSettings) bool {
	if settings == nil {
		return true
	}
	return settings.EnableThinkingBudgetRectifier
}
