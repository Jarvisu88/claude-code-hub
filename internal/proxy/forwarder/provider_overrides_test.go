package forwarder

import (
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestProviderOverrides_Anthropic_MaxTokens(t *testing.T) {
	po := NewProviderOverrides()
	p := &model.Provider{ProviderType: "anthropic"}

	// Body without max_tokens
	body := []byte(`{"model":"claude-3","messages":[]}`)
	newBody, _ := po.Apply(p, body, nil, false, "")

	assert.Contains(t, string(newBody), `"max_tokens"`)
}

func TestProviderOverrides_Anthropic_MaxTokensPresent(t *testing.T) {
	po := NewProviderOverrides()
	p := &model.Provider{ProviderType: "claude"}

	// Body already has max_tokens - should not override
	body := []byte(`{"model":"claude-3","max_tokens":8192}`)
	newBody, _ := po.Apply(p, body, nil, false, "")

	assert.Contains(t, string(newBody), `"max_tokens":8192`)
}

func TestProviderOverrides_Anthropic_UserID(t *testing.T) {
	po := NewProviderOverrides()
	p := &model.Provider{ProviderType: "anthropic"}

	body := []byte(`{"model":"claude-3"}`)
	newBody, _ := po.Apply(p, body, nil, false, "user-123")

	assert.Contains(t, string(newBody), `"user_id":"user-123"`)
	assert.Contains(t, string(newBody), `"metadata"`)
}

func TestProviderOverrides_Anthropic_UserID_Existing(t *testing.T) {
	po := NewProviderOverrides()
	p := &model.Provider{ProviderType: "anthropic"}

	// Body already has metadata.user_id
	body := []byte(`{"model":"claude-3","metadata":{"user_id":"existing-user"}}`)
	newBody, _ := po.Apply(p, body, nil, false, "new-user")

	// Should not override existing user_id
	assert.Contains(t, string(newBody), `"user_id":"existing-user"`)
}

func TestProviderOverrides_Anthropic_Context1m(t *testing.T) {
	po := NewProviderOverrides()
	pref := "force_enable"
	p := &model.Provider{
		ProviderType:        "anthropic",
		Context1mPreference: &pref,
	}

	body := []byte(`{"model":"claude-3"}`)
	_, headers := po.Apply(p, body, nil, false, "")

	assert.Contains(t, headers["anthropic-beta"], "max-tokens-3-5-sonnet-2024-07-15")
}

func TestProviderOverrides_Anthropic_CacheTTL(t *testing.T) {
	po := NewProviderOverrides()
	pref := "ephemeral"
	p := &model.Provider{
		ProviderType:       "anthropic",
		CacheTtlPreference: &pref,
	}

	body := []byte(`{"model":"claude-3"}`)
	_, headers := po.Apply(p, body, nil, false, "")

	assert.Contains(t, headers["anthropic-beta"], "prompt-caching-2024-07-31")
}

func TestProviderOverrides_Codex_ReasoningEffort(t *testing.T) {
	po := NewProviderOverrides()
	pref := "high"
	p := &model.Provider{
		ProviderType:                   "codex",
		CodexReasoningEffortPreference: &pref,
	}

	body := []byte(`{"model":"codex-1"}`)
	newBody, _ := po.Apply(p, body, nil, false, "")

	assert.Contains(t, string(newBody), `"reasoning"`)
	assert.Contains(t, string(newBody), `"effort":"high"`)
}

func TestProviderOverrides_Codex_ServiceTier(t *testing.T) {
	po := NewProviderOverrides()
	p := &model.Provider{ProviderType: "codex"}

	body := []byte(`{"model":"codex-1"}`)
	newBody, _ := po.Apply(p, body, nil, false, "")

	assert.Contains(t, string(newBody), `"service_tier":"default"`)
}

func TestProviderOverrides_Codex_ServiceTierPresent(t *testing.T) {
	po := NewProviderOverrides()
	p := &model.Provider{ProviderType: "codex"}

	body := []byte(`{"model":"codex-1","service_tier":"flex"}`)
	newBody, _ := po.Apply(p, body, nil, false, "")

	// Should not override existing
	assert.Contains(t, string(newBody), `"service_tier":"flex"`)
}

func TestProviderOverrides_Codex_ParallelToolCalls(t *testing.T) {
	po := NewProviderOverrides()
	pref := "true"
	p := &model.Provider{
		ProviderType:                     "codex",
		CodexParallelToolCallsPreference: &pref,
	}

	body := []byte(`{"model":"codex-1"}`)
	newBody, _ := po.Apply(p, body, nil, false, "")

	assert.Contains(t, string(newBody), `"parallel_tool_calls":true`)
}

func TestProviderOverrides_Gemini_NoModification(t *testing.T) {
	po := NewProviderOverrides()
	p := &model.Provider{ProviderType: "gemini"}

	inputHeaders := map[string]string{"Content-Type": "application/json"}
	body := []byte(`{"model":"gemini-pro","contents":[]}`)
	newBody, headers := po.Apply(p, body, inputHeaders, false, "")

	// Gemini currently makes minimal modifications - body and headers pass through
	assert.NotNil(t, newBody)
	assert.Equal(t, inputHeaders, headers)
}

func TestProviderOverrides_Unknown_PassThrough(t *testing.T) {
	po := NewProviderOverrides()
	p := &model.Provider{ProviderType: "openai-compatible"}

	body := []byte(`{"model":"gpt-4"}`)
	headers := map[string]string{"Authorization": "Bearer key"}

	newBody, newHeaders := po.Apply(p, body, headers, false, "")

	assert.Equal(t, string(body), string(newBody))
	assert.Equal(t, headers, newHeaders)
}

func TestProviderOverrides_InvalidJSON(t *testing.T) {
	po := NewProviderOverrides()
	p := &model.Provider{ProviderType: "anthropic"}

	body := []byte(`not json`)
	newBody, _ := po.Apply(p, body, nil, false, "")

	// Should return original body on parse failure
	assert.Equal(t, string(body), string(newBody))
}

func TestAppendBeta(t *testing.T) {
	assert.Equal(t, "feature1", appendBeta("", "feature1"))
	assert.Equal(t, "existing,feature2", appendBeta("existing", "feature2"))
}
