package v1

import (
	"context"
	"regexp"
	"strings"
)

type responseInputRectifierAction string

const (
	responseInputRectifierActionStringToArray     responseInputRectifierAction = "string_to_array"
	responseInputRectifierActionObjectToArray     responseInputRectifierAction = "object_to_array"
	responseInputRectifierActionEmptyStringToList responseInputRectifierAction = "empty_string_to_empty_array"
	responseInputRectifierActionPassthrough       responseInputRectifierAction = "passthrough"
)

type responseInputRectifierResult struct {
	Applied      bool
	Action       responseInputRectifierAction
	OriginalType string
}

func rectifyResponseInput(message map[string]any) responseInputRectifierResult {
	input, exists := message["input"]
	if !exists {
		return responseInputRectifierResult{Applied: false, Action: responseInputRectifierActionPassthrough, OriginalType: "other"}
	}

	switch typed := input.(type) {
	case []any:
		return responseInputRectifierResult{Applied: false, Action: responseInputRectifierActionPassthrough, OriginalType: "array"}
	case string:
		if typed == "" {
			message["input"] = []any{}
			return responseInputRectifierResult{Applied: true, Action: responseInputRectifierActionEmptyStringToList, OriginalType: "string"}
		}
		message["input"] = []any{
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
		return responseInputRectifierResult{Applied: true, Action: responseInputRectifierActionStringToArray, OriginalType: "string"}
	case map[string]any:
		if _, hasRole := typed["role"]; hasRole {
			message["input"] = []any{typed}
			return responseInputRectifierResult{Applied: true, Action: responseInputRectifierActionObjectToArray, OriginalType: "object"}
		}
		if _, hasType := typed["type"]; hasType {
			message["input"] = []any{typed}
			return responseInputRectifierResult{Applied: true, Action: responseInputRectifierActionObjectToArray, OriginalType: "object"}
		}
	}

	return responseInputRectifierResult{Applied: false, Action: responseInputRectifierActionPassthrough, OriginalType: "other"}
}

type billingHeaderRectifierResult struct {
	Applied         bool
	RemovedCount    int
	ExtractedValues []string
}

var billingHeaderPattern = regexp.MustCompile(`(?i)^\s*x-anthropic-billing-header\s*:`)

func rectifyBillingHeader(message map[string]any) billingHeaderRectifierResult {
	system, exists := message["system"]
	if !exists || system == nil {
		return billingHeaderRectifierResult{}
	}

	switch typed := system.(type) {
	case string:
		if billingHeaderPattern.MatchString(typed) {
			delete(message, "system")
			return billingHeaderRectifierResult{
				Applied:         true,
				RemovedCount:    1,
				ExtractedValues: []string{strings.TrimSpace(typed)},
			}
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
			message["system"] = filtered
			return billingHeaderRectifierResult{
				Applied:         true,
				RemovedCount:    len(extracted),
				ExtractedValues: extracted,
			}
		}
	}

	return billingHeaderRectifierResult{}
}

func (h *Handler) maybeRectifyRequestBody(requestBody map[string]any, endpointKind proxyEndpointKind) bool {
	if h == nil || h.settings == nil || requestBody == nil {
		return false
	}

	settings, err := h.settings.Get(context.Background())
	if err != nil || settings == nil {
		return false
	}

	changed := false
	if endpointKind == proxyEndpointResponses && settings.EnableResponseInputRectifier {
		result := rectifyResponseInput(requestBody)
		changed = changed || result.Applied
	}

	if (endpointKind == proxyEndpointMessages || endpointKind == proxyEndpointMessagesCount) && settings.EnableBillingHeaderRectifier {
		result := rectifyBillingHeader(requestBody)
		changed = changed || result.Applied
	}
	return changed
}
