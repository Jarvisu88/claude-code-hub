package rectifier

import (
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
)

// thinkingSignatureEnabled returns true when the thinking signature rectifier
// is enabled via system settings. Defaults to true when settings are nil.
func thinkingSignatureEnabled(settings *model.SystemSettings) bool {
	if settings == nil {
		return true
	}
	return settings.EnableThinkingSignatureRectifier
}

// RectifyThinkingSignature removes thinking blocks, redacted_thinking blocks,
// and signature fields from assistant messages so that the upstream provider
// does not reject the request due to stale or invalid thinking signatures.
func RectifyThinkingSignature(body map[string]any) Result {
	result := Result{
		Name:    "thinking_signature",
		Details: map[string]any{},
	}

	messages, ok := body["messages"].([]any)
	if !ok || len(messages) == 0 {
		return result
	}

	removedThinking := 0
	removedRedacted := 0
	removedSignatures := 0

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

			blockType, _ := block["type"].(string)
			switch blockType {
			case "thinking":
				removedThinking++
				contentChanged = true
				continue
			case "redacted_thinking":
				removedRedacted++
				contentChanged = true
				continue
			}

			if _, exists := block["signature"]; exists {
				delete(block, "signature")
				removedSignatures++
				contentChanged = true
			}
			next = append(next, block)
		}

		if contentChanged {
			msg["content"] = next
			result.Applied = true
		}
	}

	result.Details["removed_thinking_blocks"] = removedThinking
	result.Details["removed_redacted_thinking"] = removedRedacted
	result.Details["removed_signatures"] = removedSignatures
	return result
}

// DetectThinkingSignatureTrigger checks whether an error message from the
// upstream provider indicates a thinking-signature related issue that could
// be resolved by applying the thinking signature rectifier to the next retry.
func DetectThinkingSignatureTrigger(errorMessage string) string {
	lower := strings.ToLower(strings.TrimSpace(errorMessage))
	if lower == "" {
		return ""
	}

	// "messages: ... must start with a thinking block"
	if strings.Contains(lower, "must start with a thinking block") ||
		(strings.Contains(lower, "expected `thinking`") && strings.Contains(lower, "tool_use")) {
		return "assistant_message_must_start_with_thinking"
	}

	// Invalid signature in thinking block
	if strings.Contains(lower, "invalid") && strings.Contains(lower, "signature") &&
		strings.Contains(lower, "thinking") && strings.Contains(lower, "block") {
		return "invalid_signature_in_thinking_block"
	}
	if strings.Contains(lower, "signature") && strings.Contains(lower, "field required") {
		return "invalid_signature_in_thinking_block"
	}
	if strings.Contains(lower, "signature") && strings.Contains(lower, "extra inputs are not permitted") {
		return "invalid_signature_in_thinking_block"
	}
	if (strings.Contains(lower, "thinking") || strings.Contains(lower, "redacted_thinking")) &&
		strings.Contains(lower, "cannot be modified") {
		return "invalid_signature_in_thinking_block"
	}

	// Generic invalid request patterns
	if strings.Contains(lower, "illegal request") || strings.Contains(lower, "invalid request") ||
		strings.Contains(lower, "非法请求") {
		return "invalid_request"
	}

	return ""
}
