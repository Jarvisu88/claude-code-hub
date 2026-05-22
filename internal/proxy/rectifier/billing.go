package rectifier

import (
	"regexp"
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
)

// billingHeaderPattern matches system-message lines that try to inject
// the x-anthropic-billing-header header into the request.
var billingHeaderPattern = regexp.MustCompile(`(?i)^\s*x-anthropic-billing-header\s*:`)

// billingHeaderEnabled returns true when the billing header rectifier
// is enabled via system settings. Defaults to true when settings are nil.
func billingHeaderEnabled(settings *model.SystemSettings) bool {
	if settings == nil {
		return true
	}
	return settings.EnableBillingHeaderRectifier
}

// RectifyBillingHeader removes system-message content blocks that attempt
// to inject billing headers. This prevents clients from manipulating
// the billing-source attribution.
func RectifyBillingHeader(body map[string]any) Result {
	result := Result{
		Name:    "billing_header",
		Details: map[string]any{},
	}

	system, exists := body["system"]
	if !exists || system == nil {
		return result
	}

	switch typed := system.(type) {
	case string:
		if billingHeaderPattern.MatchString(typed) {
			delete(body, "system")
			result.Applied = true
			result.Details["removed_count"] = 1
			result.Details["extracted_values"] = []string{strings.TrimSpace(typed)}
		}

	case []any:
		filtered := make([]any, 0, len(typed))
		extracted := make([]string, 0)

		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok {
				filtered = append(filtered, item)
				continue
			}
			blockType, _ := block["type"].(string)
			text, _ := block["text"].(string)
			if blockType == "text" && billingHeaderPattern.MatchString(text) {
				extracted = append(extracted, strings.TrimSpace(text))
				continue
			}
			filtered = append(filtered, item)
		}

		if len(extracted) > 0 {
			body["system"] = filtered
			result.Applied = true
			result.Details["removed_count"] = len(extracted)
			result.Details["extracted_values"] = extracted
		}
	}

	return result
}

// responseInputEnabled returns true when the response-input rectifier is
// enabled via system settings. Defaults to true when settings are nil.
func responseInputEnabled(settings *model.SystemSettings) bool {
	if settings == nil {
		return true
	}
	return settings.EnableResponseInputRectifier
}

// RectifyResponseInput normalizes the "input" field in a Codex/Response API
// request body so that it is always an array of messages. Handles the case
// where the client sends a plain string or a single message object.
func RectifyResponseInput(body map[string]any) Result {
	result := Result{
		Name:    "response_input",
		Details: map[string]any{},
	}

	input, exists := body["input"]
	if !exists {
		result.Details["action"] = "passthrough"
		result.Details["original_type"] = "absent"
		return result
	}

	switch typed := input.(type) {
	case []any:
		result.Details["action"] = "passthrough"
		result.Details["original_type"] = "array"

	case string:
		if typed == "" {
			body["input"] = []any{}
			result.Applied = true
			result.Details["action"] = "empty_string_to_empty_array"
			result.Details["original_type"] = "string"
		} else {
			body["input"] = []any{
				map[string]any{
					"role": "user",
					"content": []any{
						map[string]any{
							"type": "input_text",
							"text": typed,
						},
					},
				},
			}
			result.Applied = true
			result.Details["action"] = "string_to_array"
			result.Details["original_type"] = "string"
		}

	case map[string]any:
		if _, hasRole := typed["role"]; hasRole {
			body["input"] = []any{typed}
			result.Applied = true
			result.Details["action"] = "object_to_array"
			result.Details["original_type"] = "object"
		} else if _, hasType := typed["type"]; hasType {
			body["input"] = []any{typed}
			result.Applied = true
			result.Details["action"] = "object_to_array"
			result.Details["original_type"] = "object"
		} else {
			result.Details["action"] = "passthrough"
			result.Details["original_type"] = "object"
		}

	default:
		result.Details["action"] = "passthrough"
		result.Details["original_type"] = "other"
	}

	return result
}
