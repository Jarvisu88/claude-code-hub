package response

import (
	"encoding/json"

	"github.com/ding113/claude-code-hub/internal/pkg/sse"
)

// Usage token usage extracted from API responses
type Usage struct {
	// InputTokens prompt / input token count
	InputTokens int
	// OutputTokens completion / output token count
	OutputTokens int
	// CacheCreationTokens tokens used to create cache
	CacheCreationTokens int
	// CacheReadTokens tokens read from cache
	CacheReadTokens int
	// TotalTokens convenience total
	TotalTokens int
}

// computeTotal recalculates TotalTokens from the component fields.
func (u *Usage) computeTotal() {
	u.TotalTokens = u.InputTokens + u.OutputTokens + u.CacheCreationTokens + u.CacheReadTokens
}

// -------------------------------------------------------------------
// Claude (Anthropic) format
// -------------------------------------------------------------------

// claudeUsageEnvelope is the top-level response from Anthropic Messages API.
type claudeUsageEnvelope struct {
	Usage *claudeUsage `json:"usage"`
}

type claudeUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// ParseUsageFromClaude parses usage from a Claude Messages API response body.
func ParseUsageFromClaude(body []byte) (*Usage, error) {
	var env claudeUsageEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	if env.Usage == nil {
		return &Usage{}, nil
	}
	u := &Usage{
		InputTokens:         env.Usage.InputTokens,
		OutputTokens:        env.Usage.OutputTokens,
		CacheCreationTokens: env.Usage.CacheCreationInputTokens,
		CacheReadTokens:     env.Usage.CacheReadInputTokens,
	}
	u.computeTotal()
	return u, nil
}

// -------------------------------------------------------------------
// OpenAI format
// -------------------------------------------------------------------

type openAIUsageEnvelope struct {
	Usage *openAIUsage `json:"usage"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ParseUsageFromOpenAI parses usage from an OpenAI Chat Completions response body.
func ParseUsageFromOpenAI(body []byte) (*Usage, error) {
	var env openAIUsageEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	if env.Usage == nil {
		return &Usage{}, nil
	}
	u := &Usage{
		InputTokens:  env.Usage.PromptTokens,
		OutputTokens: env.Usage.CompletionTokens,
		TotalTokens:  env.Usage.TotalTokens,
	}
	// If upstream did not supply total, compute it.
	if u.TotalTokens == 0 {
		u.computeTotal()
	}
	return u, nil
}

// -------------------------------------------------------------------
// Codex format (OpenAI Responses API variant)
// -------------------------------------------------------------------

type codexUsageEnvelope struct {
	Usage *codexUsage `json:"usage"`
}

type codexUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ParseUsageFromCodex parses usage from a Codex / Responses API response body.
func ParseUsageFromCodex(body []byte) (*Usage, error) {
	var env codexUsageEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	if env.Usage == nil {
		return &Usage{}, nil
	}
	u := &Usage{
		InputTokens:  env.Usage.InputTokens,
		OutputTokens: env.Usage.OutputTokens,
		TotalTokens:  env.Usage.TotalTokens,
	}
	if u.TotalTokens == 0 {
		u.computeTotal()
	}
	return u, nil
}

// -------------------------------------------------------------------
// Gemini format
// -------------------------------------------------------------------

type geminiUsageEnvelope struct {
	UsageMetadata *geminiUsageMetadata `json:"usageMetadata"`
}

type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// ParseUsageFromGemini parses usage from a Google Gemini generateContent response body.
func ParseUsageFromGemini(body []byte) (*Usage, error) {
	var env geminiUsageEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	if env.UsageMetadata == nil {
		return &Usage{}, nil
	}
	u := &Usage{
		InputTokens:  env.UsageMetadata.PromptTokenCount,
		OutputTokens: env.UsageMetadata.CandidatesTokenCount,
		TotalTokens:  env.UsageMetadata.TotalTokenCount,
	}
	if u.TotalTokens == 0 {
		u.computeTotal()
	}
	return u, nil
}

// -------------------------------------------------------------------
// SSE event aggregation
// -------------------------------------------------------------------

// ParseUsageFromSSEEvents scans a slice of SSE events and extracts the final
// usage block. It recognises Claude, OpenAI, Codex and Gemini formats.
//
// For Claude streams the usage appears in the event with type "message_delta"
// (inside delta.usage) or in "message_stop". For OpenAI/Codex it appears in
// the last chunk before [DONE]. For Gemini it appears in usageMetadata of
// each chunk.
//
// format must be one of: "claude", "openai", "codex", "gemini".
func ParseUsageFromSSEEvents(events []*sse.Event, format string) (*Usage, error) {
	// We walk the events in reverse looking for the last one that contains
	// usage data. This is much cheaper than parsing every event.
	for i := len(events) - 1; i >= 0; i-- {
		evt := events[i]
		data := evt.Data
		if data == "" || data == "[DONE]" {
			continue
		}
		raw := []byte(data)

		switch format {
		case "claude":
			u, err := parseClaudeStreamEvent(raw)
			if err == nil && u != nil && (u.InputTokens > 0 || u.OutputTokens > 0) {
				return u, nil
			}
		case "openai":
			u, err := ParseUsageFromOpenAI(raw)
			if err == nil && u != nil && (u.InputTokens > 0 || u.OutputTokens > 0) {
				return u, nil
			}
		case "codex":
			u, err := ParseUsageFromCodex(raw)
			if err == nil && u != nil && (u.InputTokens > 0 || u.OutputTokens > 0) {
				return u, nil
			}
		case "gemini":
			u, err := ParseUsageFromGemini(raw)
			if err == nil && u != nil && (u.InputTokens > 0 || u.OutputTokens > 0) {
				return u, nil
			}
		}
	}

	// No usage found in any event.
	return &Usage{}, nil
}

// parseClaudeStreamEvent attempts to extract usage from a Claude streaming
// event. Claude streaming events carry usage in multiple shapes:
//   - event type "message_delta" => {"type":"message_delta","usage":{...}}
//   - event type "message_start" => {"type":"message_start","message":{"usage":{...}}}
type claudeStreamEvent struct {
	Type    string       `json:"type"`
	Usage   *claudeUsage `json:"usage"`
	Message *struct {
		Usage *claudeUsage `json:"usage"`
	} `json:"message"`
}

func parseClaudeStreamEvent(data []byte) (*Usage, error) {
	var evt claudeStreamEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return nil, err
	}

	var cu *claudeUsage
	if evt.Usage != nil {
		cu = evt.Usage
	} else if evt.Message != nil && evt.Message.Usage != nil {
		cu = evt.Message.Usage
	}

	if cu == nil {
		return nil, nil
	}

	u := &Usage{
		InputTokens:         cu.InputTokens,
		OutputTokens:        cu.OutputTokens,
		CacheCreationTokens: cu.CacheCreationInputTokens,
		CacheReadTokens:     cu.CacheReadInputTokens,
	}
	u.computeTotal()
	return u, nil
}
