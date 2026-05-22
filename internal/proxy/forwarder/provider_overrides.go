package forwarder

import (
	"encoding/json"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/rs/zerolog/log"
)

// ProviderOverrides applies provider-specific modifications to the request body and headers.
// Modifications are applied in-place on the decoded JSON and then re-encoded.
type ProviderOverrides struct{}

// NewProviderOverrides creates a new overrides applier.
func NewProviderOverrides() *ProviderOverrides {
	return &ProviderOverrides{}
}

// Apply applies all provider-specific overrides to the request.
// Returns the potentially modified body and headers.
func (po *ProviderOverrides) Apply(
	provider *model.Provider,
	body []byte,
	headers map[string]string,
	streaming bool,
	userID string,
) ([]byte, map[string]string) {
	switch provider.ProviderType {
	case "anthropic", "claude", "claude-auth":
		return po.applyAnthropicOverrides(provider, body, headers, streaming, userID)
	case "codex":
		return po.applyCodexOverrides(provider, body, headers)
	case "gemini", "gemini-cli":
		return po.applyGeminiOverrides(provider, body, headers)
	default:
		return body, headers
	}
}

// --------------------------------------------------------------------------
// Anthropic / Claude overrides
// --------------------------------------------------------------------------

func (po *ProviderOverrides) applyAnthropicOverrides(
	provider *model.Provider,
	body []byte,
	headers map[string]string,
	streaming bool,
	userID string,
) ([]byte, map[string]string) {
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		log.Warn().Err(err).Msg("provider_overrides: failed to parse body for Anthropic overrides")
		return body, headers
	}

	modified := false

	// 1. max_tokens injection: ensure max_tokens is present
	if _, hasMaxTokens := parsed["max_tokens"]; !hasMaxTokens {
		parsed["max_tokens"] = 16384
		modified = true
	}

	// 2. Thinking budget: if provider has thinking budget preferences, apply them
	// (placeholder for extended thinking configuration)

	// 3. 1M context window preference
	if provider.Context1mPreference != nil && *provider.Context1mPreference != "" {
		switch *provider.Context1mPreference {
		case "force_enable":
			if headers == nil {
				headers = make(map[string]string)
			}
			headers["anthropic-beta"] = appendBeta(headers["anthropic-beta"], "max-tokens-3-5-sonnet-2024-07-15")
		case "force_disable":
			// Remove the beta flag if present - no action needed if not present
		}
		// "auto" or empty: do nothing
	}

	// 4. Cache TTL preference
	if provider.CacheTtlPreference != nil && *provider.CacheTtlPreference != "" {
		// The cache TTL is typically set via headers
		switch *provider.CacheTtlPreference {
		case "ephemeral":
			if headers == nil {
				headers = make(map[string]string)
			}
			headers["anthropic-beta"] = appendBeta(headers["anthropic-beta"], "prompt-caching-2024-07-31")
		}
	}

	// 5. Metadata user_id injection
	if userID != "" {
		metadata, ok := parsed["metadata"].(map[string]interface{})
		if !ok {
			metadata = make(map[string]interface{})
		}
		if _, hasUserID := metadata["user_id"]; !hasUserID {
			metadata["user_id"] = userID
			parsed["metadata"] = metadata
			modified = true
		}
	}

	if modified {
		newBody, err := json.Marshal(parsed)
		if err != nil {
			log.Warn().Err(err).Msg("provider_overrides: failed to marshal modified body")
			return body, headers
		}
		return newBody, headers
	}

	return body, headers
}

// appendBeta appends a beta feature to the anthropic-beta header value.
func appendBeta(existing, feature string) string {
	if existing == "" {
		return feature
	}
	return existing + "," + feature
}

// --------------------------------------------------------------------------
// Codex overrides
// --------------------------------------------------------------------------

func (po *ProviderOverrides) applyCodexOverrides(
	provider *model.Provider,
	body []byte,
	headers map[string]string,
) ([]byte, map[string]string) {
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		log.Warn().Err(err).Msg("provider_overrides: failed to parse body for Codex overrides")
		return body, headers
	}

	modified := false

	// 1. Reasoning effort preference
	if provider.CodexReasoningEffortPreference != nil && *provider.CodexReasoningEffortPreference != "" {
		if _, has := parsed["reasoning"]; !has {
			parsed["reasoning"] = map[string]interface{}{
				"effort": *provider.CodexReasoningEffortPreference,
			}
			modified = true
		} else if reasoning, ok := parsed["reasoning"].(map[string]interface{}); ok {
			if _, hasEffort := reasoning["effort"]; !hasEffort {
				reasoning["effort"] = *provider.CodexReasoningEffortPreference
				modified = true
			}
		}
	}

	// 2. Service tier - set to "default" if not present
	if _, has := parsed["service_tier"]; !has {
		parsed["service_tier"] = "default"
		modified = true
	}

	// 3. Reasoning summary preference
	if provider.CodexReasoningSummaryPreference != nil && *provider.CodexReasoningSummaryPreference != "" {
		if reasoning, ok := parsed["reasoning"].(map[string]interface{}); ok {
			if _, has := reasoning["summary"]; !has {
				reasoning["summary"] = *provider.CodexReasoningSummaryPreference
				modified = true
			}
		}
	}

	// 4. Text verbosity preference
	if provider.CodexTextVerbosityPreference != nil && *provider.CodexTextVerbosityPreference != "" {
		if _, has := parsed["text"]; !has {
			parsed["text"] = map[string]interface{}{
				"format": map[string]interface{}{
					"type": *provider.CodexTextVerbosityPreference,
				},
			}
			modified = true
		}
	}

	// 5. Parallel tool calls preference
	if provider.CodexParallelToolCallsPreference != nil && *provider.CodexParallelToolCallsPreference != "" {
		pref := *provider.CodexParallelToolCallsPreference
		if pref == "true" || pref == "false" {
			val := pref == "true"
			if _, has := parsed["parallel_tool_calls"]; !has {
				parsed["parallel_tool_calls"] = val
				modified = true
			}
		}
	}

	if modified {
		newBody, err := json.Marshal(parsed)
		if err != nil {
			log.Warn().Err(err).Msg("provider_overrides: failed to marshal modified body")
			return body, headers
		}
		return newBody, headers
	}

	return body, headers
}

// --------------------------------------------------------------------------
// Gemini overrides
// --------------------------------------------------------------------------

func (po *ProviderOverrides) applyGeminiOverrides(
	provider *model.Provider,
	body []byte,
	headers map[string]string,
) ([]byte, map[string]string) {
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		log.Warn().Err(err).Msg("provider_overrides: failed to parse body for Gemini overrides")
		return body, headers
	}

	modified := false

	// 1. Google Search grounding injection
	// If no tools are specified, add Google Search as a default tool for applicable models
	if tools, hasTool := parsed["tools"]; !hasTool || tools == nil {
		// Check generationConfig for search grounding
		if genConfig, ok := parsed["generationConfig"].(map[string]interface{}); ok {
			if _, hasSearchGrounding := genConfig["googleSearchGrounding"]; hasSearchGrounding {
				// Search grounding is already configured via generationConfig
			}
		}
	}

	if modified {
		newBody, err := json.Marshal(parsed)
		if err != nil {
			log.Warn().Err(err).Msg("provider_overrides: failed to marshal modified body")
			return body, headers
		}
		return newBody, headers
	}

	return body, headers
}
