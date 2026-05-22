package rectifier

import (
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------
// Thinking Signature Rectifier
// ---------------------------------------------------------------

func TestRectifyThinkingSignature_RemovesThinkingBlocks(t *testing.T) {
	body := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "thinking", "thinking": "..."},
					map[string]any{"type": "text", "text": "hello"},
				},
			},
		},
	}

	result := RectifyThinkingSignature(body)
	assert.True(t, result.Applied)
	assert.Equal(t, "thinking_signature", result.Name)
	assert.Equal(t, 1, result.Details["removed_thinking_blocks"])
	assert.Equal(t, 0, result.Details["removed_redacted_thinking"])

	// Verify the thinking block was removed.
	messages := body["messages"].([]any)
	msg := messages[0].(map[string]any)
	content := msg["content"].([]any)
	assert.Len(t, content, 1)
	assert.Equal(t, "text", content[0].(map[string]any)["type"])
}

func TestRectifyThinkingSignature_RemovesRedactedThinking(t *testing.T) {
	body := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "redacted_thinking", "data": "..."},
					map[string]any{"type": "text", "text": "hello"},
				},
			},
		},
	}

	result := RectifyThinkingSignature(body)
	assert.True(t, result.Applied)
	assert.Equal(t, 1, result.Details["removed_redacted_thinking"])
}

func TestRectifyThinkingSignature_RemovesSignatureField(t *testing.T) {
	body := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "text", "text": "hello", "signature": "abc123"},
				},
			},
		},
	}

	result := RectifyThinkingSignature(body)
	assert.True(t, result.Applied)
	assert.Equal(t, 1, result.Details["removed_signatures"])

	// Verify the signature field was removed from the block.
	messages := body["messages"].([]any)
	msg := messages[0].(map[string]any)
	content := msg["content"].([]any)
	block := content[0].(map[string]any)
	_, exists := block["signature"]
	assert.False(t, exists)
}

func TestRectifyThinkingSignature_NoMessages(t *testing.T) {
	body := map[string]any{"model": "claude-3-opus"}
	result := RectifyThinkingSignature(body)
	assert.False(t, result.Applied)
}

func TestRectifyThinkingSignature_NoThinkingContent(t *testing.T) {
	body := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "hello"},
				},
			},
		},
	}

	result := RectifyThinkingSignature(body)
	assert.False(t, result.Applied)
}

func TestRectifyThinkingSignature_MultipleMessages(t *testing.T) {
	body := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "thinking", "thinking": "..."},
					map[string]any{"type": "text", "text": "first"},
				},
			},
			map[string]any{
				"role":    "user",
				"content": "next question",
			},
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "redacted_thinking", "data": "..."},
					map[string]any{"type": "text", "text": "second", "signature": "sig"},
				},
			},
		},
	}

	result := RectifyThinkingSignature(body)
	assert.True(t, result.Applied)
	assert.Equal(t, 1, result.Details["removed_thinking_blocks"])
	assert.Equal(t, 1, result.Details["removed_redacted_thinking"])
	assert.Equal(t, 1, result.Details["removed_signatures"])
}

// ---------------------------------------------------------------
// Thinking Signature Trigger Detection
// ---------------------------------------------------------------

func TestDetectThinkingSignatureTrigger(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"must start with thinking block",
			"messages: assistant response must start with a thinking block",
			"assistant_message_must_start_with_thinking",
		},
		{
			"expected thinking with tool_use",
			"expected `thinking` not tool_use",
			"assistant_message_must_start_with_thinking",
		},
		{
			"invalid signature in thinking block",
			"invalid signature in thinking block detected",
			"invalid_signature_in_thinking_block",
		},
		{
			"signature field required",
			"signature field required for content block",
			"invalid_signature_in_thinking_block",
		},
		{
			"signature extra inputs not permitted",
			"signature: extra inputs are not permitted",
			"invalid_signature_in_thinking_block",
		},
		{
			"thinking cannot be modified",
			"thinking content cannot be modified",
			"invalid_signature_in_thinking_block",
		},
		{
			"redacted_thinking cannot be modified",
			"redacted_thinking cannot be modified after creation",
			"invalid_signature_in_thinking_block",
		},
		{
			"invalid request (EN)",
			"invalid request due to bad params",
			"invalid_request",
		},
		{
			"invalid request (CN)",
			"非法请求",
			"invalid_request",
		},
		{
			"empty string",
			"",
			"",
		},
		{
			"unrelated error",
			"model not found",
			"",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, DetectThinkingSignatureTrigger(tc.input))
		})
	}
}

// ---------------------------------------------------------------
// Thinking Budget Rectifier
// ---------------------------------------------------------------

func TestRectifyThinkingBudget_SetsDefaults(t *testing.T) {
	body := map[string]any{
		"thinking": map[string]any{
			"type": "enabled",
		},
		"max_tokens": float64(4096),
	}

	result := RectifyThinkingBudget(body)
	assert.True(t, result.Applied)
	assert.Equal(t, "thinking_budget", result.Name)

	thinking := body["thinking"].(map[string]any)
	assert.Equal(t, "enabled", thinking["type"])
	assert.Equal(t, float64(DefaultThinkingBudget), thinking["budget_tokens"])
	assert.Equal(t, float64(DefaultMaxTokensForThinking), body["max_tokens"])
}

func TestRectifyThinkingBudget_RespectsAdaptive(t *testing.T) {
	body := map[string]any{
		"thinking": map[string]any{
			"type": "adaptive",
		},
	}

	result := RectifyThinkingBudget(body)
	assert.False(t, result.Applied)
	assert.Equal(t, "adaptive", body["thinking"].(map[string]any)["type"])
}

func TestRectifyThinkingBudget_SkipsValidBudget(t *testing.T) {
	body := map[string]any{
		"thinking": map[string]any{
			"type":          "enabled",
			"budget_tokens": float64(8192),
		},
		"max_tokens": float64(100000),
	}

	result := RectifyThinkingBudget(body)
	assert.False(t, result.Applied)
	// Should not have changed anything.
	assert.Equal(t, float64(8192), body["thinking"].(map[string]any)["budget_tokens"])
}

func TestRectifyThinkingBudget_NoThinkingBlock(t *testing.T) {
	body := map[string]any{
		"model":      "claude-3-opus",
		"max_tokens": float64(4096),
	}

	result := RectifyThinkingBudget(body)
	assert.False(t, result.Applied)
}

func TestRectifyThinkingBudget_LargeMaxTokensPreserved(t *testing.T) {
	body := map[string]any{
		"thinking": map[string]any{
			"type": "enabled",
		},
		"max_tokens": float64(200000),
	}

	result := RectifyThinkingBudget(body)
	assert.True(t, result.Applied)
	// max_tokens should NOT be overridden since it is already larger.
	assert.Equal(t, float64(200000), body["max_tokens"])
}

// ---------------------------------------------------------------
// Thinking Budget Trigger Detection
// ---------------------------------------------------------------

func TestDetectThinkingBudgetTrigger(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			"budget_tokens too low",
			"thinking budget_tokens: Input should be greater than or equal to 1024",
			true,
		},
		{
			"budget tokens >= 1024",
			"thinking: budget tokens must be >= 1024",
			true,
		},
		{
			"unrelated error",
			"model not found",
			false,
		},
		{
			"empty",
			"",
			false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, DetectThinkingBudgetTrigger(tc.input))
		})
	}
}

// ---------------------------------------------------------------
// Billing Header Rectifier
// ---------------------------------------------------------------

func TestRectifyBillingHeader_RemovesStringSystem(t *testing.T) {
	body := map[string]any{
		"system": "x-anthropic-billing-header: some-value",
	}

	result := RectifyBillingHeader(body)
	assert.True(t, result.Applied)
	assert.Equal(t, "billing_header", result.Name)
	assert.Equal(t, 1, result.Details["removed_count"])

	_, exists := body["system"]
	assert.False(t, exists)
}

func TestRectifyBillingHeader_RemovesBlocksFromArray(t *testing.T) {
	body := map[string]any{
		"system": []any{
			map[string]any{"type": "text", "text": "You are a helpful assistant."},
			map[string]any{"type": "text", "text": "X-Anthropic-Billing-Header: fake"},
			map[string]any{"type": "text", "text": "More instructions."},
		},
	}

	result := RectifyBillingHeader(body)
	assert.True(t, result.Applied)
	assert.Equal(t, 1, result.Details["removed_count"])

	system := body["system"].([]any)
	assert.Len(t, system, 2)
}

func TestRectifyBillingHeader_NoMatch(t *testing.T) {
	body := map[string]any{
		"system": "You are a helpful assistant.",
	}

	result := RectifyBillingHeader(body)
	assert.False(t, result.Applied)
}

func TestRectifyBillingHeader_NoSystem(t *testing.T) {
	body := map[string]any{
		"model": "claude-3-opus",
	}

	result := RectifyBillingHeader(body)
	assert.False(t, result.Applied)
}

// ---------------------------------------------------------------
// Response Input Rectifier
// ---------------------------------------------------------------

func TestRectifyResponseInput_StringToArray(t *testing.T) {
	body := map[string]any{
		"input": "Hello, world!",
	}

	result := RectifyResponseInput(body)
	assert.True(t, result.Applied)
	assert.Equal(t, "response_input", result.Name)
	assert.Equal(t, "string_to_array", result.Details["action"])
	assert.Equal(t, "string", result.Details["original_type"])

	input, ok := body["input"].([]any)
	require.True(t, ok)
	assert.Len(t, input, 1)
}

func TestRectifyResponseInput_EmptyStringToEmptyArray(t *testing.T) {
	body := map[string]any{
		"input": "",
	}

	result := RectifyResponseInput(body)
	assert.True(t, result.Applied)
	assert.Equal(t, "empty_string_to_empty_array", result.Details["action"])

	input := body["input"].([]any)
	assert.Len(t, input, 0)
}

func TestRectifyResponseInput_ObjectWithRole(t *testing.T) {
	body := map[string]any{
		"input": map[string]any{"role": "user", "content": "Hi"},
	}

	result := RectifyResponseInput(body)
	assert.True(t, result.Applied)
	assert.Equal(t, "object_to_array", result.Details["action"])

	input := body["input"].([]any)
	assert.Len(t, input, 1)
}

func TestRectifyResponseInput_ArrayPassthrough(t *testing.T) {
	body := map[string]any{
		"input": []any{
			map[string]any{"role": "user", "content": "Hi"},
		},
	}

	result := RectifyResponseInput(body)
	assert.False(t, result.Applied)
	assert.Equal(t, "passthrough", result.Details["action"])
}

func TestRectifyResponseInput_AbsentInput(t *testing.T) {
	body := map[string]any{
		"model": "claude-3-opus",
	}

	result := RectifyResponseInput(body)
	assert.False(t, result.Applied)
	assert.Equal(t, "absent", result.Details["original_type"])
}

// ---------------------------------------------------------------
// Chain
// ---------------------------------------------------------------

func TestChain_AppliesAllEnabled(t *testing.T) {
	body := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "thinking", "thinking": "..."},
					map[string]any{"type": "text", "text": "hello"},
				},
			},
		},
		"thinking": map[string]any{
			"type": "enabled",
		},
		"max_tokens": float64(4096),
		"system":     "x-anthropic-billing-header: fake",
	}

	chain := NewChain(nil) // nil settings = all enabled by default
	results, anyApplied := chain.Apply(body, SurfaceClaude)

	assert.True(t, anyApplied)
	assert.Len(t, results, 3) // thinking_signature + thinking_budget + billing_header
}

func TestChain_RespectsDisabledSettings(t *testing.T) {
	settings := &model.SystemSettings{
		EnableThinkingSignatureRectifier: false,
		EnableThinkingBudgetRectifier:    false,
		EnableBillingHeaderRectifier:     false,
		EnableResponseInputRectifier:     false,
	}

	body := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "thinking", "thinking": "..."},
				},
			},
		},
		"system": "x-anthropic-billing-header: fake",
	}

	chain := NewChain(&staticSettings{settings: settings})
	results, anyApplied := chain.Apply(body, SurfaceClaude)

	assert.False(t, anyApplied)
	assert.Len(t, results, 0)

	// Verify the thinking block was NOT removed.
	messages := body["messages"].([]any)
	msg := messages[0].(map[string]any)
	content := msg["content"].([]any)
	assert.Len(t, content, 1)
	assert.Equal(t, "thinking", content[0].(map[string]any)["type"])
}

func TestChain_CodexSurface(t *testing.T) {
	body := map[string]any{
		"input": "Hello",
	}

	chain := NewChain(nil)
	results, anyApplied := chain.Apply(body, SurfaceCodex)

	assert.True(t, anyApplied)
	assert.Len(t, results, 1)
	assert.Equal(t, "response_input", results[0].Name)
}

func TestChain_NilBody(t *testing.T) {
	chain := NewChain(nil)
	results, anyApplied := chain.Apply(nil, SurfaceClaude)
	assert.False(t, anyApplied)
	assert.Nil(t, results)
}

// staticSettings is a test helper that returns static system settings.
type staticSettings struct {
	settings *model.SystemSettings
}

func (s *staticSettings) Get() (*model.SystemSettings, error) {
	return s.settings, nil
}
