package response

import (
	"testing"

	"github.com/ding113/claude-code-hub/internal/pkg/sse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------
// ParseUsageFromClaude
// ---------------------------------------------------------------

func TestParseUsageFromClaude_FullUsage(t *testing.T) {
	body := []byte(`{
		"id": "msg_123",
		"type": "message",
		"role": "assistant",
		"content": [{"type":"text","text":"hello"}],
		"usage": {
			"input_tokens": 100,
			"output_tokens": 50,
			"cache_creation_input_tokens": 10,
			"cache_read_input_tokens": 20
		}
	}`)

	u, err := ParseUsageFromClaude(body)
	require.NoError(t, err)
	assert.Equal(t, 100, u.InputTokens)
	assert.Equal(t, 50, u.OutputTokens)
	assert.Equal(t, 10, u.CacheCreationTokens)
	assert.Equal(t, 20, u.CacheReadTokens)
	assert.Equal(t, 180, u.TotalTokens) // 100+50+10+20
}

func TestParseUsageFromClaude_NoUsageField(t *testing.T) {
	body := []byte(`{"id":"msg_123","type":"message"}`)

	u, err := ParseUsageFromClaude(body)
	require.NoError(t, err)
	assert.Equal(t, 0, u.InputTokens)
	assert.Equal(t, 0, u.OutputTokens)
	assert.Equal(t, 0, u.TotalTokens)
}

func TestParseUsageFromClaude_InvalidJSON(t *testing.T) {
	_, err := ParseUsageFromClaude([]byte(`{invalid`))
	assert.Error(t, err)
}

func TestParseUsageFromClaude_NoCacheTokens(t *testing.T) {
	body := []byte(`{
		"usage": {
			"input_tokens": 200,
			"output_tokens": 80
		}
	}`)

	u, err := ParseUsageFromClaude(body)
	require.NoError(t, err)
	assert.Equal(t, 200, u.InputTokens)
	assert.Equal(t, 80, u.OutputTokens)
	assert.Equal(t, 0, u.CacheCreationTokens)
	assert.Equal(t, 0, u.CacheReadTokens)
	assert.Equal(t, 280, u.TotalTokens)
}

// ---------------------------------------------------------------
// ParseUsageFromOpenAI
// ---------------------------------------------------------------

func TestParseUsageFromOpenAI_FullUsage(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-abc",
		"object": "chat.completion",
		"usage": {
			"prompt_tokens": 150,
			"completion_tokens": 75,
			"total_tokens": 225
		}
	}`)

	u, err := ParseUsageFromOpenAI(body)
	require.NoError(t, err)
	assert.Equal(t, 150, u.InputTokens)
	assert.Equal(t, 75, u.OutputTokens)
	assert.Equal(t, 225, u.TotalTokens)
}

func TestParseUsageFromOpenAI_NoUsageField(t *testing.T) {
	body := []byte(`{"id":"chatcmpl-abc","object":"chat.completion"}`)

	u, err := ParseUsageFromOpenAI(body)
	require.NoError(t, err)
	assert.Equal(t, 0, u.InputTokens)
	assert.Equal(t, 0, u.OutputTokens)
}

func TestParseUsageFromOpenAI_ComputedTotal(t *testing.T) {
	body := []byte(`{
		"usage": {
			"prompt_tokens": 100,
			"completion_tokens": 50,
			"total_tokens": 0
		}
	}`)

	u, err := ParseUsageFromOpenAI(body)
	require.NoError(t, err)
	assert.Equal(t, 150, u.TotalTokens) // computed because upstream sent 0
}

func TestParseUsageFromOpenAI_InvalidJSON(t *testing.T) {
	_, err := ParseUsageFromOpenAI([]byte(`not json`))
	assert.Error(t, err)
}

// ---------------------------------------------------------------
// ParseUsageFromCodex
// ---------------------------------------------------------------

func TestParseUsageFromCodex_FullUsage(t *testing.T) {
	body := []byte(`{
		"id": "resp_abc",
		"object": "response",
		"usage": {
			"input_tokens": 300,
			"output_tokens": 120,
			"total_tokens": 420
		}
	}`)

	u, err := ParseUsageFromCodex(body)
	require.NoError(t, err)
	assert.Equal(t, 300, u.InputTokens)
	assert.Equal(t, 120, u.OutputTokens)
	assert.Equal(t, 420, u.TotalTokens)
}

func TestParseUsageFromCodex_NoUsage(t *testing.T) {
	body := []byte(`{"id":"resp_abc"}`)
	u, err := ParseUsageFromCodex(body)
	require.NoError(t, err)
	assert.Equal(t, 0, u.InputTokens)
}

func TestParseUsageFromCodex_InvalidJSON(t *testing.T) {
	_, err := ParseUsageFromCodex([]byte(`{bad`))
	assert.Error(t, err)
}

// ---------------------------------------------------------------
// ParseUsageFromGemini
// ---------------------------------------------------------------

func TestParseUsageFromGemini_FullUsage(t *testing.T) {
	body := []byte(`{
		"candidates": [{"content":{"parts":[{"text":"hi"}]}}],
		"usageMetadata": {
			"promptTokenCount": 500,
			"candidatesTokenCount": 200,
			"totalTokenCount": 700
		}
	}`)

	u, err := ParseUsageFromGemini(body)
	require.NoError(t, err)
	assert.Equal(t, 500, u.InputTokens)
	assert.Equal(t, 200, u.OutputTokens)
	assert.Equal(t, 700, u.TotalTokens)
}

func TestParseUsageFromGemini_NoMetadata(t *testing.T) {
	body := []byte(`{"candidates":[]}`)
	u, err := ParseUsageFromGemini(body)
	require.NoError(t, err)
	assert.Equal(t, 0, u.InputTokens)
}

func TestParseUsageFromGemini_InvalidJSON(t *testing.T) {
	_, err := ParseUsageFromGemini([]byte(`xxx`))
	assert.Error(t, err)
}

// ---------------------------------------------------------------
// ParseUsageFromSSEEvents
// ---------------------------------------------------------------

func TestParseUsageFromSSEEvents_Claude(t *testing.T) {
	events := []*sse.Event{
		{Event: "message_start", Data: `{"type":"message_start","message":{"usage":{"input_tokens":10,"output_tokens":0}}}`},
		{Event: "content_block_delta", Data: `{"type":"content_block_delta","delta":{"text":"hello"}}`},
		{Event: "message_delta", Data: `{"type":"message_delta","usage":{"input_tokens":10,"output_tokens":25,"cache_creation_input_tokens":5}}`},
		{Event: "message_stop", Data: `{"type":"message_stop"}`},
	}

	u, err := ParseUsageFromSSEEvents(events, "claude")
	require.NoError(t, err)
	assert.Equal(t, 10, u.InputTokens)
	assert.Equal(t, 25, u.OutputTokens)
	assert.Equal(t, 5, u.CacheCreationTokens)
}

func TestParseUsageFromSSEEvents_OpenAI(t *testing.T) {
	events := []*sse.Event{
		{Data: `{"choices":[{"delta":{"content":"hi"}}]}`},
		{Data: `{"choices":[{"delta":{"content":" there"}}],"usage":{"prompt_tokens":50,"completion_tokens":20,"total_tokens":70}}`},
		{Data: `[DONE]`},
	}

	u, err := ParseUsageFromSSEEvents(events, "openai")
	require.NoError(t, err)
	assert.Equal(t, 50, u.InputTokens)
	assert.Equal(t, 20, u.OutputTokens)
	assert.Equal(t, 70, u.TotalTokens)
}

func TestParseUsageFromSSEEvents_Codex(t *testing.T) {
	events := []*sse.Event{
		{Data: `{"type":"response.output_item.added"}`},
		{Data: `{"type":"response.completed","usage":{"input_tokens":100,"output_tokens":40,"total_tokens":140}}`},
		{Data: `[DONE]`},
	}

	u, err := ParseUsageFromSSEEvents(events, "codex")
	require.NoError(t, err)
	assert.Equal(t, 100, u.InputTokens)
	assert.Equal(t, 40, u.OutputTokens)
}

func TestParseUsageFromSSEEvents_Gemini(t *testing.T) {
	events := []*sse.Event{
		{Data: `{"candidates":[],"usageMetadata":{"promptTokenCount":80,"candidatesTokenCount":30,"totalTokenCount":110}}`},
	}

	u, err := ParseUsageFromSSEEvents(events, "gemini")
	require.NoError(t, err)
	assert.Equal(t, 80, u.InputTokens)
	assert.Equal(t, 30, u.OutputTokens)
}

func TestParseUsageFromSSEEvents_NoUsageFound(t *testing.T) {
	events := []*sse.Event{
		{Data: `{"choices":[{"delta":{"content":"hi"}}]}`},
		{Data: `[DONE]`},
	}

	u, err := ParseUsageFromSSEEvents(events, "openai")
	require.NoError(t, err)
	assert.Equal(t, 0, u.InputTokens)
	assert.Equal(t, 0, u.OutputTokens)
}

func TestParseUsageFromSSEEvents_EmptyEvents(t *testing.T) {
	u, err := ParseUsageFromSSEEvents(nil, "claude")
	require.NoError(t, err)
	assert.Equal(t, 0, u.InputTokens)
}

// ---------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------

func TestParseClaudeStreamEvent_MessageStart(t *testing.T) {
	data := []byte(`{"type":"message_start","message":{"usage":{"input_tokens":42,"output_tokens":0}}}`)
	u, err := parseClaudeStreamEvent(data)
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, 42, u.InputTokens)
}

func TestParseClaudeStreamEvent_NoUsage(t *testing.T) {
	data := []byte(`{"type":"content_block_delta","delta":{"text":"word"}}`)
	u, err := parseClaudeStreamEvent(data)
	require.NoError(t, err)
	assert.Nil(t, u) // no usage block
}

func TestComputeTotal(t *testing.T) {
	u := &Usage{InputTokens: 10, OutputTokens: 5, CacheCreationTokens: 3, CacheReadTokens: 2}
	u.computeTotal()
	assert.Equal(t, 20, u.TotalTokens)
}
