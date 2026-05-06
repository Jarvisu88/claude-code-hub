package v1

import "testing"

func TestDetectThinkingSignatureRectifierTrigger(t *testing.T) {
	trigger := detectThinkingSignatureRectifierTrigger("thinking.signature: Field required")
	if trigger != thinkingSignatureTriggerInvalidSignature {
		t.Fatalf("expected invalid signature trigger, got %q", trigger)
	}
}

func TestRectifyAnthropicRequestMessage(t *testing.T) {
	message := map[string]any{
		"messages": []any{
			map[string]any{
				"content": []any{
					map[string]any{"type": "thinking", "text": "x"},
					map[string]any{"type": "text", "text": "hello", "signature": "abc"},
				},
			},
		},
	}
	result := rectifyAnthropicRequestMessage(message)
	if !result.Applied || result.RemovedThinkingBlocks != 1 || result.RemovedSignatureFieldsCount != 1 {
		t.Fatalf("unexpected rectifier result: %+v", result)
	}
}

func TestDetectThinkingBudgetRectifierTrigger(t *testing.T) {
	if !detectThinkingBudgetRectifierTrigger("thinking.enabled.budget_tokens: Input should be greater than or equal to 1024") {
		t.Fatal("expected budget rectifier trigger")
	}
}

func TestRectifyThinkingBudget(t *testing.T) {
	message := map[string]any{
		"thinking":   map[string]any{"type": "enabled", "budget_tokens": 100},
		"max_tokens": 1000.0,
	}
	result := rectifyThinkingBudget(message)
	if !result.Applied {
		t.Fatal("expected budget rectifier to apply")
	}
	thinking := message["thinking"].(map[string]any)
	if thinking["budget_tokens"] != 32000 || message["max_tokens"] != 64000 {
		t.Fatalf("unexpected rectified message: %#v", message)
	}
}
