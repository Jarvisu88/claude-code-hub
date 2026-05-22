package converter

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Converter defines the bidirectional format converter interface.
// Every converter pair handles request conversion, response conversion,
// and streaming chunk conversion between two API formats.
type Converter interface {
	// ConvertRequest converts a request from the client format to the provider format.
	ConvertRequest(input map[string]interface{}) (map[string]interface{}, error)

	// ConvertResponse converts a response from the provider format to the client format.
	ConvertResponse(response map[string]interface{}) (map[string]interface{}, error)

	// ConvertStreamChunk converts a streaming SSE chunk between formats.
	ConvertStreamChunk(chunk []byte) ([]byte, error)
}

// ProviderType is the upstream provider API format.
type ProviderType string

const (
	ProviderTypeClaude    ProviderType = "claude"
	ProviderTypeOpenAI    ProviderType = "openai"
	ProviderTypeCodex     ProviderType = "codex"
	ProviderTypeGemini    ProviderType = "gemini"
	ProviderTypeGeminiCLI ProviderType = "gemini-cli"
)

// ClientType is the downstream client API format.
type ClientType string

const (
	ClientTypeClaude    ClientType = "claude"
	ClientTypeOpenAI    ClientType = "openai"
	ClientTypeCodex     ClientType = "codex"
	ClientTypeGemini    ClientType = "gemini"
	ClientTypeGeminiCLI ClientType = "gemini-cli"
)

// NewConverter returns a Converter that translates between clientType and providerType.
// If clientType == providerType, a PassthroughConverter is returned.
func NewConverter(clientType ClientType, providerType ProviderType) Converter {
	// Same format -> passthrough
	if string(clientType) == string(providerType) {
		return &PassthroughConverter{}
	}

	switch clientType {
	case ClientTypeClaude:
		switch providerType {
		case ProviderTypeOpenAI:
			return &ClaudeToOpenAIConverter{}
		case ProviderTypeCodex:
			return &ClaudeToCodexConverter{}
		case ProviderTypeGemini:
			return &ClaudeToGeminiConverter{}
		case ProviderTypeGeminiCLI:
			return &ClaudeToGeminiCLIConverter{}
		}

	case ClientTypeOpenAI:
		switch providerType {
		case ProviderTypeClaude:
			return &OpenAIToClaudeConverter{}
		case ProviderTypeCodex:
			return &OpenAIToCodexConverter{}
		case ProviderTypeGemini:
			return &OpenAIToGeminiConverter{}
		case ProviderTypeGeminiCLI:
			return &OpenAIToGeminiCLIConverter{}
		}

	case ClientTypeCodex:
		switch providerType {
		case ProviderTypeClaude:
			return &CodexToClaudeConverter{}
		case ProviderTypeOpenAI:
			return &CodexToOpenAIConverter{}
		case ProviderTypeGemini:
			return &CodexToGeminiConverter{}
		case ProviderTypeGeminiCLI:
			return &CodexToGeminiCLIConverter{}
		}

	case ClientTypeGemini:
		switch providerType {
		case ProviderTypeClaude:
			return &GeminiToClaudeConverter{}
		case ProviderTypeOpenAI:
			return &GeminiToOpenAIConverter{}
		case ProviderTypeCodex:
			return &GeminiToCodexConverter{}
		case ProviderTypeGeminiCLI:
			// Gemini -> GeminiCLI is mostly passthrough with envelope
			return &GeminiToGeminiCLIConverter{}
		}

	case ClientTypeGeminiCLI:
		switch providerType {
		case ProviderTypeClaude:
			return &GeminiCLIToClaudeConverter{}
		case ProviderTypeOpenAI:
			return &GeminiCLIToOpenAIConverter{}
		case ProviderTypeCodex:
			return &GeminiCLIToCodexConverter{}
		case ProviderTypeGemini:
			// GeminiCLI -> Gemini is unwrap envelope
			return &GeminiCLIToGeminiConverter{}
		}
	}

	// Fallback to passthrough
	return &PassthroughConverter{}
}

// SSEEvent represents a Server-Sent Event for streaming conversion.
type SSEEvent struct {
	Event string // event type (may be empty)
	Data  []byte // data payload
}

// ConvertStreamEvent converts a single SSE event between two formats.
// This is a convenience function that creates a converter and delegates.
func ConvertStreamEvent(event SSEEvent, fromFormat, toFormat string) (SSEEvent, error) {
	conv := NewConverter(ClientType(fromFormat), ProviderType(toFormat))

	// Reconstruct SSE bytes
	var raw []byte
	if event.Event != "" {
		raw = []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event.Event, event.Data))
	} else {
		raw = []byte(fmt.Sprintf("data: %s\n\n", event.Data))
	}

	converted, err := conv.ConvertStreamChunk(raw)
	if err != nil {
		return SSEEvent{}, err
	}

	// Parse the result back into an SSEEvent
	return parseSSEEvent(converted), nil
}

// parseSSEEvent parses raw SSE bytes into an SSEEvent struct
func parseSSEEvent(raw []byte) SSEEvent {
	if raw == nil {
		return SSEEvent{}
	}
	result := SSEEvent{Data: raw}
	// Simple extraction of event type if present
	str := string(raw)
	if len(str) > 7 && str[:7] == "event: " {
		for i := 7; i < len(str); i++ {
			if str[i] == '\n' {
				result.Event = str[7:i]
				break
			}
		}
	}
	return result
}

// PassthroughConverter performs no conversion; data passes through unchanged.
type PassthroughConverter struct{}

func (c *PassthroughConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	return input, nil
}

func (c *PassthroughConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	return response, nil
}

func (c *PassthroughConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	return chunk, nil
}

// GeminiToGeminiCLIConverter wraps standard Gemini format with CLI envelope
type GeminiToGeminiCLIConverter struct{}

func (c *GeminiToGeminiCLIConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	return wrapCLIRequest(input), nil
}

func (c *GeminiToGeminiCLIConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	return unwrapCLIResponse(response), nil
}

func (c *GeminiToGeminiCLIConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	return chunk, nil
}

// GeminiCLIToGeminiConverter unwraps CLI envelope to standard Gemini format
type GeminiCLIToGeminiConverter struct{}

func (c *GeminiCLIToGeminiConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	return unwrapCLIRequest(input), nil
}

func (c *GeminiCLIToGeminiConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	return wrapCLIResponse(response), nil
}

func (c *GeminiCLIToGeminiConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	return chunk, nil
}

// ClaudeToCodexConverter converts Claude requests to Codex format
type ClaudeToCodexConverter struct{}

func (c *ClaudeToCodexConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	// Claude -> OpenAI -> Codex
	openai, err := (&ClaudeToOpenAIConverter{}).ConvertRequest(input)
	if err != nil {
		return nil, err
	}
	return openaiRequestToCodex(openai)
}

func (c *ClaudeToCodexConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	return codexResponseToClaude(response)
}

func (c *ClaudeToCodexConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	return convertCodexStreamToClaude(chunk)
}

// OpenAIToCodexConverter converts OpenAI requests to Codex format
type OpenAIToCodexConverter struct{}

func (c *OpenAIToCodexConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	return openaiRequestToCodex(input)
}

func (c *OpenAIToCodexConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	return codexResponseToOpenAI(response)
}

func (c *OpenAIToCodexConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	return convertCodexStreamToOpenAI(chunk)
}

// GeminiToCodexConverter converts Gemini requests to Codex format
type GeminiToCodexConverter struct{}

func (c *GeminiToCodexConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	claude, err := (&GeminiToClaudeConverter{}).ConvertRequest(input)
	if err != nil {
		return nil, err
	}
	openai, err := (&ClaudeToOpenAIConverter{}).ConvertRequest(claude)
	if err != nil {
		return nil, err
	}
	return openaiRequestToCodex(openai)
}

func (c *GeminiToCodexConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	openai, err := codexResponseToOpenAI(response)
	if err != nil {
		return nil, err
	}
	return openaiResponseToGemini(openai)
}

func (c *GeminiToCodexConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	return chunk, nil
}

// ===== Helper functions =====

// getStringField extracts a string field from a map
func getStringField(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// getIntField extracts an integer field from a map (handles float64 from JSON)
func getIntField(m map[string]interface{}, key string) int {
	if m == nil {
		return 0
	}
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		case json.Number:
			if i, err := val.Int64(); err == nil {
				return int(i)
			}
		}
	}
	return 0
}

// getBoolField extracts a boolean field from a map
func getBoolField(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// getArrayField extracts an array field from a map
func getArrayField(m map[string]interface{}, key string) []interface{} {
	if m == nil {
		return nil
	}
	if v, ok := m[key]; ok {
		if arr, ok := v.([]interface{}); ok {
			return arr
		}
	}
	return nil
}

// getMapField extracts an object field from a map
func getMapField(m map[string]interface{}, key string) map[string]interface{} {
	if m == nil {
		return nil
	}
	if v, ok := m[key]; ok {
		if obj, ok := v.(map[string]interface{}); ok {
			return obj
		}
	}
	return nil
}

// mustMarshal marshals to JSON, returning "{}" on error
func mustMarshal(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// ===== Reverse direction helpers for Codex provider =====

// openaiRequestToCodex converts OpenAI Chat Completions request to Codex Response API request
func openaiRequestToCodex(input map[string]interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})

	if model := getStringField(input, "model"); model != "" {
		output["model"] = model
	}

	// Convert messages -> input items
	var inputItems []interface{}
	if messages := getArrayField(input, "messages"); messages != nil {
		for _, msg := range messages {
			m, ok := msg.(map[string]interface{})
			if !ok {
				continue
			}
			role := getStringField(m, "role")
			if role == "system" {
				// System messages -> instructions
				if existing := getStringField(output, "instructions"); existing != "" {
					output["instructions"] = existing + "\n" + getStringField(m, "content")
				} else {
					output["instructions"] = getStringField(m, "content")
				}
				continue
			}

			item := map[string]interface{}{
				"type": "message",
				"role": role,
			}

			if content := m["content"]; content != nil {
				switch v := content.(type) {
				case string:
					item["content"] = []interface{}{
						map[string]interface{}{
							"type": "input_text",
							"text": v,
						},
					}
				default:
					item["content"] = v
				}
			}

			inputItems = append(inputItems, item)
		}
	}

	output["input"] = inputItems

	if maxTokens := getIntField(input, "max_tokens"); maxTokens > 0 {
		output["max_output_tokens"] = maxTokens
	}
	if temp, ok := input["temperature"].(float64); ok {
		output["temperature"] = temp
	}
	if stream := getBoolField(input, "stream"); stream {
		output["stream"] = true
	}

	return output, nil
}

// codexResponseToClaude converts Codex Response API response to Claude Messages API format
func codexResponseToClaude(response map[string]interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})

	output["id"] = getStringField(response, "id")
	output["type"] = "message"
	output["role"] = "assistant"

	if model := getStringField(response, "model"); model != "" {
		output["model"] = model
	}

	// Convert output items -> Claude content blocks
	var contentBlocks []interface{}
	if outputItems := getArrayField(response, "output"); outputItems != nil {
		for _, item := range outputItems {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			itemType := getStringField(itemMap, "type")
			switch itemType {
			case "message":
				if contentParts := getArrayField(itemMap, "content"); contentParts != nil {
					for _, part := range contentParts {
						pm, ok := part.(map[string]interface{})
						if !ok {
							continue
						}
						if getStringField(pm, "type") == "output_text" {
							contentBlocks = append(contentBlocks, map[string]interface{}{
								"type": "text",
								"text": getStringField(pm, "text"),
							})
						}
					}
				}
			case "function_call":
				var inputArgs interface{}
				if argsStr := getStringField(itemMap, "arguments"); argsStr != "" {
					if err := json.Unmarshal([]byte(argsStr), &inputArgs); err != nil {
						inputArgs = map[string]interface{}{}
					}
				}
				contentBlocks = append(contentBlocks, map[string]interface{}{
					"type":  "tool_use",
					"id":    getStringField(itemMap, "call_id"),
					"name":  getStringField(itemMap, "name"),
					"input": inputArgs,
				})
			}
		}
	}

	if contentBlocks == nil {
		contentBlocks = []interface{}{}
	}
	output["content"] = contentBlocks

	// Status -> stop_reason
	status := getStringField(response, "status")
	switch status {
	case "completed":
		output["stop_reason"] = "end_turn"
	case "incomplete":
		output["stop_reason"] = "max_tokens"
	default:
		output["stop_reason"] = "end_turn"
	}

	// Usage
	if usage := getMapField(response, "usage"); usage != nil {
		output["usage"] = map[string]interface{}{
			"input_tokens":  getIntField(usage, "input_tokens"),
			"output_tokens": getIntField(usage, "output_tokens"),
		}
	}

	return output, nil
}

// codexResponseToOpenAI converts Codex Response API response to OpenAI Chat Completions format
func codexResponseToOpenAI(response map[string]interface{}) (map[string]interface{}, error) {
	// Codex -> Claude -> OpenAI
	claude, err := codexResponseToClaude(response)
	if err != nil {
		return nil, err
	}
	return (&OpenAIToClaudeConverter{}).ConvertResponse(claude)
}

// convertCodexStreamToClaude converts Codex SSE events to Claude SSE events
func convertCodexStreamToClaude(chunk []byte) ([]byte, error) {
	chunkStr := strings.TrimSpace(string(chunk))

	var eventType string
	var dataStr string
	for _, line := range strings.Split(chunkStr, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			dataStr = strings.TrimPrefix(line, "data: ")
		}
	}

	if dataStr == "" {
		return chunk, nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return chunk, nil
	}

	switch eventType {
	case "response.output_text.delta":
		delta := getStringField(data, "delta")
		if delta != "" {
			evt := map[string]interface{}{
				"type": "content_block_delta",
				"delta": map[string]interface{}{
					"type": "text_delta",
					"text": delta,
				},
			}
			return []byte(fmt.Sprintf("event: content_block_delta\ndata: %s\n\n", mustMarshal(evt))), nil
		}
	case "response.completed":
		return []byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"), nil
	}

	return chunk, nil
}

// convertCodexStreamToOpenAI converts Codex SSE events to OpenAI SSE chunks
func convertCodexStreamToOpenAI(chunk []byte) ([]byte, error) {
	chunkStr := strings.TrimSpace(string(chunk))

	var eventType string
	var dataStr string
	for _, line := range strings.Split(chunkStr, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			dataStr = strings.TrimPrefix(line, "data: ")
		}
	}

	if dataStr == "" {
		return chunk, nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return chunk, nil
	}

	switch eventType {
	case "response.output_text.delta":
		delta := getStringField(data, "delta")
		if delta != "" {
			evt := map[string]interface{}{
				"id":      "chatcmpl-" + generateID(),
				"object":  "chat.completion.chunk",
				"created": getCurrentTimestamp(),
				"choices": []interface{}{
					map[string]interface{}{
						"index": 0,
						"delta": map[string]interface{}{
							"content": delta,
						},
						"finish_reason": nil,
					},
				},
			}
			return []byte(fmt.Sprintf("data: %s\n\n", mustMarshal(evt))), nil
		}
	case "response.completed":
		return []byte("data: [DONE]\n\n"), nil
	}

	return chunk, nil
}
