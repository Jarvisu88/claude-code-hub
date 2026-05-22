package v1

import (
	"context"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
)

type rectifierSettingsStore struct {
	settings *model.SystemSettings
}

func (s rectifierSettingsStore) Get(_ context.Context) (*model.SystemSettings, error) {
	return s.settings, nil
}

func TestRectifyResponseInputString(t *testing.T) {
	message := map[string]any{"input": "hello"}
	result := rectifyResponseInput(message)
	if !result.Applied || result.Action != responseInputRectifierActionStringToArray {
		t.Fatalf("expected string to array rectification, got %+v", result)
	}
	input, ok := message["input"].([]any)
	if !ok || len(input) != 1 {
		t.Fatalf("expected single array item, got %#v", message["input"])
	}
}

func TestRectifyResponseInputObject(t *testing.T) {
	message := map[string]any{"input": map[string]any{"role": "user", "content": []any{}}}
	result := rectifyResponseInput(message)
	if !result.Applied || result.Action != responseInputRectifierActionObjectToArray {
		t.Fatalf("expected object to array rectification, got %+v", result)
	}
	input, ok := message["input"].([]any)
	if !ok || len(input) != 1 {
		t.Fatalf("expected single array item, got %#v", message["input"])
	}
}

func TestRectifyBillingHeaderArray(t *testing.T) {
	message := map[string]any{
		"system": []any{
			map[string]any{"type": "text", "text": "x-anthropic-billing-header: abc"},
			map[string]any{"type": "text", "text": "keep me"},
		},
	}
	result := rectifyBillingHeader(message)
	if !result.Applied || result.RemovedCount != 1 {
		t.Fatalf("expected one billing header to be removed, got %+v", result)
	}
	system, ok := message["system"].([]any)
	if !ok || len(system) != 1 {
		t.Fatalf("expected one remaining system block, got %#v", message["system"])
	}
}

func TestMaybeRectifyRequestBodyRespectsSettings(t *testing.T) {
	handler := &Handler{
		settings: rectifierSettingsStore{settings: &model.SystemSettings{
			EnableResponseInputRectifier: true,
			EnableBillingHeaderRectifier: true,
		}},
	}

	responseBody := map[string]any{"input": "hello"}
	handler.maybeRectifyRequestBody(responseBody, proxyEndpointResponses)
	if _, ok := responseBody["input"].([]any); !ok {
		t.Fatalf("expected responses body input to be rectified, got %#v", responseBody["input"])
	}

	messageBody := map[string]any{
		"system": []any{map[string]any{"type": "text", "text": "x-anthropic-billing-header: abc"}},
	}
	handler.maybeRectifyRequestBody(messageBody, proxyEndpointMessages)
	if got, ok := messageBody["system"].([]any); !ok || len(got) != 0 {
		t.Fatalf("expected billing header system block to be removed, got %#v", messageBody["system"])
	}
}
