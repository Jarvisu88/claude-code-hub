package converter

import (
	"encoding/json"
	"strings"
	"testing"
)

// ===== Factory Tests =====

func TestNewConverter(t *testing.T) {
	tests := []struct {
		name         string
		clientType   ClientType
		providerType ProviderType
		wantType     string
	}{
		{"Claude to Claude (passthrough)", ClientTypeClaude, ProviderTypeClaude, "*converter.PassthroughConverter"},
		{"Claude to OpenAI", ClientTypeClaude, ProviderTypeOpenAI, "*converter.ClaudeToOpenAIConverter"},
		{"OpenAI to Claude", ClientTypeOpenAI, ProviderTypeClaude, "*converter.OpenAIToClaudeConverter"},
		{"Claude to Gemini", ClientTypeClaude, ProviderTypeGemini, "*converter.ClaudeToGeminiConverter"},
		{"Claude to GeminiCLI", ClientTypeClaude, ProviderTypeGeminiCLI, "*converter.ClaudeToGeminiCLIConverter"},
		{"Claude to Codex", ClientTypeClaude, ProviderTypeCodex, "*converter.ClaudeToCodexConverter"},
		{"Codex to Claude", ClientTypeCodex, ProviderTypeClaude, "*converter.CodexToClaudeConverter"},
		{"Codex to OpenAI", ClientTypeCodex, ProviderTypeOpenAI, "*converter.CodexToOpenAIConverter"},
		{"Gemini to Claude", ClientTypeGemini, ProviderTypeClaude, "*converter.GeminiToClaudeConverter"},
		{"Gemini to OpenAI", ClientTypeGemini, ProviderTypeOpenAI, "*converter.GeminiToOpenAIConverter"},
		{"GeminiCLI to Claude", ClientTypeGeminiCLI, ProviderTypeClaude, "*converter.GeminiCLIToClaudeConverter"},
		{"GeminiCLI to OpenAI", ClientTypeGeminiCLI, ProviderTypeOpenAI, "*converter.GeminiCLIToOpenAIConverter"},
		{"Gemini to GeminiCLI", ClientTypeGemini, ProviderTypeGeminiCLI, "*converter.GeminiToGeminiCLIConverter"},
		{"GeminiCLI to Gemini", ClientTypeGeminiCLI, ProviderTypeGemini, "*converter.GeminiCLIToGeminiConverter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewConverter(tt.clientType, tt.providerType)
			if converter == nil {
				t.Fatal("Expected converter to be created")
			}
		})
	}
}

// ===== Passthrough Tests =====

func TestPassthroughConverter(t *testing.T) {
	c := &PassthroughConverter{}

	input := map[string]interface{}{
		"model": "claude-opus-4",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "Hello"},
		},
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)
	assertEqual(t, output["model"], "claude-opus-4")

	response := map[string]interface{}{"id": "msg_123", "type": "message"}
	output, err = c.ConvertResponse(response)
	assertNoError(t, err)
	assertEqual(t, output["id"], "msg_123")

	chunk := []byte("data: {\"type\":\"content_block_delta\"}\n\n")
	outputChunk, err := c.ConvertStreamChunk(chunk)
	assertNoError(t, err)
	assertEqual(t, string(outputChunk), string(chunk))
}

// ===== Claude <-> OpenAI Request Tests =====

func TestClaudeToOpenAI_ConvertRequest_Basic(t *testing.T) {
	c := &ClaudeToOpenAIConverter{}

	input := map[string]interface{}{
		"model": "claude-opus-4",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "Hello, how are you?"},
		},
		"max_tokens":  1024,
		"temperature": 0.7,
		"stream":      true,
		"system":      "You are a helpful assistant.",
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)
	assertEqual(t, output["model"], "claude-opus-4")
	assertEqual(t, output["max_tokens"], 1024)
	assertEqual(t, output["temperature"], 0.7)
	assertEqual(t, output["stream"], true)

	messages := output["messages"].([]interface{})
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages (system + user), got %d", len(messages))
	}
	firstMsg := messages[0].(map[string]interface{})
	assertEqual(t, firstMsg["role"], "system")
	assertEqual(t, firstMsg["content"], "You are a helpful assistant.")
}

func TestClaudeToOpenAI_ConvertRequest_WithTools(t *testing.T) {
	c := &ClaudeToOpenAIConverter{}

	input := map[string]interface{}{
		"model": "claude-opus-4",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "What is the weather?"},
		},
		"max_tokens": 1024,
		"tools": []interface{}{
			map[string]interface{}{
				"name":        "get_weather",
				"description": "Get the weather for a location",
				"input_schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)

	tools := output["tools"].([]interface{})
	if len(tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(tools))
	}
	tool := tools[0].(map[string]interface{})
	assertEqual(t, tool["type"], "function")
	fn := tool["function"].(map[string]interface{})
	assertEqual(t, fn["name"], "get_weather")
}

func TestClaudeToOpenAI_ConvertRequest_MultimodalContent(t *testing.T) {
	c := &ClaudeToOpenAIConverter{}

	input := map[string]interface{}{
		"model": "claude-opus-4",
		"messages": []interface{}{
			map[string]interface{}{
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{"type": "text", "text": "Describe this image"},
					map[string]interface{}{
						"type": "image",
						"source": map[string]interface{}{
							"type":       "base64",
							"media_type": "image/png",
							"data":       "iVBOR...",
						},
					},
				},
			},
		},
		"max_tokens": 1024,
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)

	messages := output["messages"].([]interface{})
	msg := messages[0].(map[string]interface{})
	content := msg["content"].([]interface{})
	if len(content) != 2 {
		t.Fatalf("Expected 2 content parts, got %d", len(content))
	}
	assertEqual(t, content[0].(map[string]interface{})["type"], "text")
	assertEqual(t, content[1].(map[string]interface{})["type"], "image_url")
}

// ===== Claude <-> OpenAI Response Tests =====

func TestClaudeToOpenAI_ConvertResponse_Basic(t *testing.T) {
	c := &ClaudeToOpenAIConverter{}

	input := map[string]interface{}{
		"id":   "msg_123",
		"type": "message",
		"role": "assistant",
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": "Hello! I'm doing well."},
		},
		"model":       "claude-opus-4",
		"stop_reason": "end_turn",
		"usage": map[string]interface{}{
			"input_tokens":  10,
			"output_tokens": 20,
		},
	}

	output, err := c.ConvertResponse(input)
	assertNoError(t, err)
	assertEqual(t, output["id"], "msg_123")
	assertEqual(t, output["object"], "chat.completion")

	choices := output["choices"].([]interface{})
	choice := choices[0].(map[string]interface{})
	msg := choice["message"].(map[string]interface{})
	assertEqual(t, msg["role"], "assistant")
	assertEqual(t, msg["content"], "Hello! I'm doing well.")
	assertEqual(t, choice["finish_reason"], "stop")

	usage := output["usage"].(map[string]interface{})
	assertEqual(t, usage["prompt_tokens"], 10)
	assertEqual(t, usage["completion_tokens"], 20)
	assertEqual(t, usage["total_tokens"], 30)
}

func TestClaudeToOpenAI_ConvertResponse_WithToolUse(t *testing.T) {
	c := &ClaudeToOpenAIConverter{}

	input := map[string]interface{}{
		"id":   "msg_456",
		"type": "message",
		"role": "assistant",
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": "Let me check the weather."},
			map[string]interface{}{
				"type": "tool_use",
				"id":   "toolu_123",
				"name": "get_weather",
				"input": map[string]interface{}{
					"location": "San Francisco",
				},
			},
		},
		"stop_reason": "tool_use",
		"model":       "claude-opus-4",
	}

	output, err := c.ConvertResponse(input)
	assertNoError(t, err)

	choice := output["choices"].([]interface{})[0].(map[string]interface{})
	assertEqual(t, choice["finish_reason"], "tool_calls")

	msg := choice["message"].(map[string]interface{})
	toolCalls := msg["tool_calls"].([]interface{})
	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	tc := toolCalls[0].(map[string]interface{})
	assertEqual(t, tc["id"], "toolu_123")
	assertEqual(t, tc["type"], "function")
	fn := tc["function"].(map[string]interface{})
	assertEqual(t, fn["name"], "get_weather")
	// Arguments should be JSON string
	if _, ok := fn["arguments"].(string); !ok {
		t.Error("Expected arguments to be a JSON string")
	}
}

func TestClaudeToOpenAI_ConvertResponse_WithThinking(t *testing.T) {
	c := &ClaudeToOpenAIConverter{}

	input := map[string]interface{}{
		"id":   "msg_789",
		"type": "message",
		"role": "assistant",
		"content": []interface{}{
			map[string]interface{}{"type": "thinking", "thinking": "Let me think about this..."},
			map[string]interface{}{"type": "text", "text": "The answer is 42."},
		},
		"stop_reason": "end_turn",
		"model":       "claude-opus-4",
	}

	output, err := c.ConvertResponse(input)
	assertNoError(t, err)

	msg := output["choices"].([]interface{})[0].(map[string]interface{})["message"].(map[string]interface{})
	assertEqual(t, msg["content"], "The answer is 42.")
	assertEqual(t, msg["reasoning_content"], "Let me think about this...")
}

// ===== Stop Reason Mapping Tests =====

func TestClaudeStopToOpenAIFinish(t *testing.T) {
	tests := []struct {
		claude string
		openai string
	}{
		{"end_turn", "stop"},
		{"max_tokens", "length"},
		{"tool_use", "tool_calls"},
		{"stop_sequence", "stop"},
	}
	for _, tt := range tests {
		t.Run(tt.claude, func(t *testing.T) {
			assertEqual(t, claudeStopToOpenAIFinish(tt.claude), tt.openai)
		})
	}
}

func TestOpenAIFinishFromClaudeStop(t *testing.T) {
	tests := []struct {
		claude string
		openai string
	}{
		{"end_turn", "stop"},
		{"max_tokens", "length"},
		{"tool_use", "tool_calls"},
		{"stop_sequence", "stop"},
		{"", "stop"},
	}
	for _, tt := range tests {
		t.Run(tt.claude, func(t *testing.T) {
			assertEqual(t, openAIFinishFromClaudeStop(tt.claude), tt.openai)
		})
	}
}

// ===== OpenAI -> Claude Request Tests =====

func TestOpenAIToClaude_ConvertRequest_Basic(t *testing.T) {
	c := &OpenAIToClaudeConverter{}

	input := map[string]interface{}{
		"model": "gpt-4",
		"messages": []interface{}{
			map[string]interface{}{"role": "system", "content": "You are helpful."},
			map[string]interface{}{"role": "user", "content": "Hello!"},
		},
		"max_tokens":  1024,
		"temperature": 0.7,
		"stream":      true,
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)
	assertEqual(t, output["model"], "gpt-4")
	assertEqual(t, output["system"], "You are helpful.")

	messages := output["messages"].([]interface{})
	if len(messages) != 1 {
		t.Errorf("Expected 1 message (user only, system extracted), got %d", len(messages))
	}
	assertEqual(t, messages[0].(map[string]interface{})["role"], "user")
}

func TestOpenAIToClaude_ConvertRequest_WithToolCalls(t *testing.T) {
	c := &OpenAIToClaudeConverter{}

	input := map[string]interface{}{
		"model": "gpt-4",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "What's the weather?"},
			map[string]interface{}{
				"role":    "assistant",
				"content": "Let me check.",
				"tool_calls": []interface{}{
					map[string]interface{}{
						"id":   "call_123",
						"type": "function",
						"function": map[string]interface{}{
							"name":      "get_weather",
							"arguments": "{\"location\":\"NYC\"}",
						},
					},
				},
			},
			map[string]interface{}{
				"role":         "tool",
				"tool_call_id": "call_123",
				"content":      "Sunny, 72F",
			},
		},
		"max_tokens": 1024,
		"tools": []interface{}{
			map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "get_weather",
					"description": "Get weather",
					"parameters":  map[string]interface{}{"type": "object"},
				},
			},
		},
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)

	// Should have tools converted to Claude format
	tools := output["tools"].([]interface{})
	if len(tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(tools))
	}
	assertEqual(t, tools[0].(map[string]interface{})["name"], "get_weather")

	messages := output["messages"].([]interface{})
	if len(messages) < 3 {
		t.Fatalf("Expected at least 3 messages, got %d", len(messages))
	}

	// Check that assistant message has tool_use blocks
	assistantMsg := messages[1].(map[string]interface{})
	assistantContent := assistantMsg["content"].([]interface{})
	foundToolUse := false
	for _, block := range assistantContent {
		if m, ok := block.(map[string]interface{}); ok {
			if getStringField(m, "type") == "tool_use" {
				foundToolUse = true
				assertEqual(t, m["name"], "get_weather")
			}
		}
	}
	if !foundToolUse {
		t.Error("Expected tool_use block in assistant message")
	}

	// Check that tool message was converted to user message with tool_result
	toolMsg := messages[2].(map[string]interface{})
	assertEqual(t, toolMsg["role"], "user")
}

// ===== OpenAI -> Claude Response Tests =====

func TestOpenAIToClaude_ConvertResponse(t *testing.T) {
	c := &OpenAIToClaudeConverter{}

	input := map[string]interface{}{
		"id":   "msg_abc",
		"type": "message",
		"role": "assistant",
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": "Hello!"},
		},
		"stop_reason": "end_turn",
		"model":       "claude-opus-4",
		"usage": map[string]interface{}{
			"input_tokens":  5,
			"output_tokens": 10,
		},
	}

	output, err := c.ConvertResponse(input)
	assertNoError(t, err)
	assertEqual(t, output["object"], "chat.completion")

	choice := output["choices"].([]interface{})[0].(map[string]interface{})
	assertEqual(t, choice["finish_reason"], "stop")

	msg := choice["message"].(map[string]interface{})
	assertEqual(t, msg["content"], "Hello!")
}

// ===== Streaming Tests =====

func TestClaudeToOpenAI_ConvertStreamChunk_TextDelta(t *testing.T) {
	c := &ClaudeToOpenAIConverter{}

	claudeEvent := "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n"
	output, err := c.ConvertStreamChunk([]byte(claudeEvent))
	assertNoError(t, err)

	if !strings.HasPrefix(string(output), "data: ") {
		t.Fatalf("Expected OpenAI SSE format, got: %s", output)
	}

	dataStr := strings.TrimPrefix(strings.TrimSpace(string(output)), "data: ")
	var data map[string]interface{}
	err = json.Unmarshal([]byte(dataStr), &data)
	assertNoError(t, err)
	assertEqual(t, data["object"], "chat.completion.chunk")

	choice := data["choices"].([]interface{})[0].(map[string]interface{})
	delta := choice["delta"].(map[string]interface{})
	assertEqual(t, delta["content"], "Hello")
}

func TestClaudeToOpenAI_ConvertStreamChunk_Done(t *testing.T) {
	c := &ClaudeToOpenAIConverter{}

	input := []byte("data: [DONE]\n\n")
	output, err := c.ConvertStreamChunk(input)
	assertNoError(t, err)
	assertContains(t, string(output), "message_stop")
}

func TestOpenAIToClaude_ConvertStreamChunk_TextDelta(t *testing.T) {
	c := &OpenAIToClaudeConverter{}

	claudeEvent := "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"World\"}}\n\n"
	output, err := c.ConvertStreamChunk([]byte(claudeEvent))
	assertNoError(t, err)

	dataStr := strings.TrimPrefix(strings.TrimSpace(string(output)), "data: ")
	var data map[string]interface{}
	err = json.Unmarshal([]byte(dataStr), &data)
	assertNoError(t, err)
	assertEqual(t, data["object"], "chat.completion.chunk")
}

func TestOpenAIToClaude_ConvertStreamChunk_MessageStop(t *testing.T) {
	c := &OpenAIToClaudeConverter{}

	claudeEvent := "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	output, err := c.ConvertStreamChunk([]byte(claudeEvent))
	assertNoError(t, err)
	assertContains(t, string(output), "[DONE]")
}

func TestClaudeToOpenAI_ConvertStreamChunk_ToolCallStart(t *testing.T) {
	c := &ClaudeToOpenAIConverter{}

	event := "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"get_weather\"}}\n\n"
	output, err := c.ConvertStreamChunk([]byte(event))
	assertNoError(t, err)

	dataStr := strings.TrimPrefix(strings.TrimSpace(string(output)), "data: ")
	var data map[string]interface{}
	err = json.Unmarshal([]byte(dataStr), &data)
	assertNoError(t, err)

	choice := data["choices"].([]interface{})[0].(map[string]interface{})
	delta := choice["delta"].(map[string]interface{})
	toolCalls := delta["tool_calls"].([]interface{})
	tc := toolCalls[0].(map[string]interface{})
	assertEqual(t, tc["id"], "toolu_1")
	fn := tc["function"].(map[string]interface{})
	assertEqual(t, fn["name"], "get_weather")
}

func TestClaudeToOpenAI_ConvertStreamChunk_ThinkingDelta(t *testing.T) {
	c := &ClaudeToOpenAIConverter{}

	event := "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"Let me think...\"}}\n\n"
	output, err := c.ConvertStreamChunk([]byte(event))
	assertNoError(t, err)

	dataStr := strings.TrimPrefix(strings.TrimSpace(string(output)), "data: ")
	var data map[string]interface{}
	err = json.Unmarshal([]byte(dataStr), &data)
	assertNoError(t, err)

	choice := data["choices"].([]interface{})[0].(map[string]interface{})
	delta := choice["delta"].(map[string]interface{})
	assertEqual(t, delta["reasoning_content"], "Let me think...")
}

// ===== Codex Converter Tests =====

func TestCodexToClaude_ConvertRequest_StringInput(t *testing.T) {
	c := &CodexToClaudeConverter{}

	input := map[string]interface{}{
		"model":            "gpt-4",
		"input":            "Hello world",
		"max_output_tokens": 1024,
		"instructions":     "Be concise.",
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)
	assertEqual(t, output["model"], "gpt-4")
	assertEqual(t, output["max_tokens"], 1024)
	assertContains(t, output["system"].(string), "Be concise.")

	messages := output["messages"].([]interface{})
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	assertEqual(t, messages[0].(map[string]interface{})["content"], "Hello world")
}

func TestCodexToClaude_ConvertRequest_ArrayInput(t *testing.T) {
	c := &CodexToClaudeConverter{}

	input := map[string]interface{}{
		"model": "gpt-4",
		"input": []interface{}{
			map[string]interface{}{
				"type": "message",
				"role": "system",
				"content": []interface{}{
					map[string]interface{}{"type": "input_text", "text": "You are helpful."},
				},
			},
			map[string]interface{}{
				"type": "message",
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{"type": "input_text", "text": "Hello!"},
				},
			},
		},
		"max_output_tokens": 512,
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)
	assertEqual(t, output["system"], "You are helpful.")

	messages := output["messages"].([]interface{})
	if len(messages) != 1 {
		t.Fatalf("Expected 1 user message, got %d", len(messages))
	}
}

func TestCodexToClaude_ConvertRequest_WithFunctionCallOutput(t *testing.T) {
	c := &CodexToClaudeConverter{}

	input := map[string]interface{}{
		"model": "gpt-4",
		"input": []interface{}{
			map[string]interface{}{
				"type":    "message",
				"role":    "user",
				"content": "What's the weather?",
			},
			map[string]interface{}{
				"type":    "function_call_output",
				"call_id": "call_abc",
				"output":  "Sunny, 75F",
			},
		},
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)

	messages := output["messages"].([]interface{})
	if len(messages) < 2 {
		t.Fatalf("Expected at least 2 messages, got %d", len(messages))
	}

	// Second message should be tool_result
	toolResultMsg := messages[1].(map[string]interface{})
	content := toolResultMsg["content"].([]interface{})
	block := content[0].(map[string]interface{})
	assertEqual(t, block["type"], "tool_result")
	assertEqual(t, block["tool_use_id"], "call_abc")
}

func TestCodexToClaude_ConvertResponse(t *testing.T) {
	c := &CodexToClaudeConverter{}

	claudeResponse := map[string]interface{}{
		"id":   "msg_123",
		"type": "message",
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": "The answer is 42."},
		},
		"stop_reason": "end_turn",
		"model":       "claude-opus-4",
		"usage": map[string]interface{}{
			"input_tokens":  10,
			"output_tokens": 15,
		},
	}

	output, err := c.ConvertResponse(claudeResponse)
	assertNoError(t, err)
	assertEqual(t, output["object"], "response")
	assertEqual(t, output["status"], "completed")

	outputItems := output["output"].([]interface{})
	if len(outputItems) != 1 {
		t.Fatalf("Expected 1 output item, got %d", len(outputItems))
	}

	item := outputItems[0].(map[string]interface{})
	assertEqual(t, item["type"], "message")
	contentParts := item["content"].([]interface{})
	textPart := contentParts[0].(map[string]interface{})
	assertEqual(t, textPart["type"], "output_text")
	assertEqual(t, textPart["text"], "The answer is 42.")
}

func TestCodexToOpenAI_ConvertRequest(t *testing.T) {
	c := &CodexToOpenAIConverter{}

	input := map[string]interface{}{
		"model":            "gpt-4",
		"input":            "Hello world",
		"max_output_tokens": 1024,
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)
	assertEqual(t, output["model"], "gpt-4")

	messages := output["messages"].([]interface{})
	if len(messages) < 1 {
		t.Fatal("Expected at least 1 message")
	}
}

// ===== Gemini Converter Tests =====

func TestGeminiToClaude_ConvertRequest(t *testing.T) {
	c := &GeminiToClaudeConverter{}

	input := map[string]interface{}{
		"contents": []interface{}{
			map[string]interface{}{
				"role": "user",
				"parts": []interface{}{
					map[string]interface{}{"text": "Hello Gemini!"},
				},
			},
		},
		"systemInstruction": map[string]interface{}{
			"parts": []interface{}{
				map[string]interface{}{"text": "You are helpful."},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":    0.8,
			"maxOutputTokens": 2048,
			"topP":           0.9,
			"topK":           40,
		},
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)
	assertEqual(t, output["system"], "You are helpful.")
	assertEqual(t, output["temperature"], 0.8)
	assertEqual(t, output["max_tokens"], 2048)
	assertEqual(t, output["top_p"], 0.9)
	assertEqual(t, output["top_k"], 40)

	messages := output["messages"].([]interface{})
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	msg := messages[0].(map[string]interface{})
	assertEqual(t, msg["role"], "user")
	assertEqual(t, msg["content"], "Hello Gemini!")
}

func TestGeminiToClaude_ConvertRequest_WithFunctionCall(t *testing.T) {
	c := &GeminiToClaudeConverter{}

	input := map[string]interface{}{
		"contents": []interface{}{
			map[string]interface{}{
				"role": "user",
				"parts": []interface{}{
					map[string]interface{}{"text": "Get me weather"},
				},
			},
			map[string]interface{}{
				"role": "model",
				"parts": []interface{}{
					map[string]interface{}{
						"functionCall": map[string]interface{}{
							"name": "get_weather",
							"args": map[string]interface{}{"city": "NYC"},
						},
					},
				},
			},
		},
		"tools": []interface{}{
			map[string]interface{}{
				"functionDeclarations": []interface{}{
					map[string]interface{}{
						"name":        "get_weather",
						"description": "Get weather info",
						"parameters":  map[string]interface{}{"type": "object"},
					},
				},
			},
		},
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)

	// Tools should be converted
	tools := output["tools"].([]interface{})
	if len(tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(tools))
	}
	assertEqual(t, tools[0].(map[string]interface{})["name"], "get_weather")

	// Messages should include tool_use
	messages := output["messages"].([]interface{})
	if len(messages) < 2 {
		t.Fatalf("Expected at least 2 messages, got %d", len(messages))
	}

	assistantMsg := messages[1].(map[string]interface{})
	assertEqual(t, assistantMsg["role"], "assistant")
}

func TestClaudeToGemini_ConvertRequest(t *testing.T) {
	c := &ClaudeToGeminiConverter{}

	input := map[string]interface{}{
		"model":  "claude-opus-4",
		"system": "You are a helpful assistant.",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "Hello!"},
			map[string]interface{}{"role": "assistant", "content": "Hi there!"},
			map[string]interface{}{"role": "user", "content": "How are you?"},
		},
		"max_tokens":  1024,
		"temperature": 0.5,
		"top_p":       0.9,
		"top_k":       50,
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)

	// Check systemInstruction
	sysInstr := output["systemInstruction"].(map[string]interface{})
	parts := sysInstr["parts"].([]interface{})
	assertEqual(t, parts[0].(map[string]interface{})["text"], "You are a helpful assistant.")

	// Check contents
	contents := output["contents"].([]interface{})
	if len(contents) != 3 {
		t.Fatalf("Expected 3 contents, got %d", len(contents))
	}
	// Check role mapping: assistant -> model
	assertEqual(t, contents[1].(map[string]interface{})["role"], "model")

	// Check generationConfig
	config := output["generationConfig"].(map[string]interface{})
	assertEqual(t, config["maxOutputTokens"], 1024)
	assertEqual(t, config["temperature"], 0.5)
}

func TestGeminiToClaude_ConvertResponse(t *testing.T) {
	// GeminiToClaudeConverter: client=Gemini, provider=Claude
	// ConvertResponse: takes Claude response, returns Gemini format
	c := &GeminiToClaudeConverter{}

	claudeResponse := map[string]interface{}{
		"id":   "msg_gem",
		"type": "message",
		"role": "assistant",
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": "Hello from Claude!"},
		},
		"stop_reason": "end_turn",
		"usage": map[string]interface{}{
			"input_tokens":  15,
			"output_tokens": 10,
		},
	}

	output, err := c.ConvertResponse(claudeResponse)
	assertNoError(t, err)

	// Should be Gemini format
	candidates := output["candidates"].([]interface{})
	if len(candidates) != 1 {
		t.Fatalf("Expected 1 candidate, got %d", len(candidates))
	}
	candidate := candidates[0].(map[string]interface{})
	assertEqual(t, candidate["finishReason"], "STOP")
	content := candidate["content"].(map[string]interface{})
	assertEqual(t, content["role"], "model")
	parts := content["parts"].([]interface{})
	assertEqual(t, parts[0].(map[string]interface{})["text"], "Hello from Claude!")

	usageMetadata := output["usageMetadata"].(map[string]interface{})
	assertEqual(t, usageMetadata["promptTokenCount"], 15)
	assertEqual(t, usageMetadata["candidatesTokenCount"], 10)
}

func TestGeminiResponseToClaude(t *testing.T) {
	// Test the geminiResponseToClaude helper directly
	geminiResponse := map[string]interface{}{
		"candidates": []interface{}{
			map[string]interface{}{
				"content": map[string]interface{}{
					"role": "model",
					"parts": []interface{}{
						map[string]interface{}{"text": "Hello from Gemini!"},
					},
				},
				"finishReason": "STOP",
			},
		},
		"usageMetadata": map[string]interface{}{
			"promptTokenCount":     15,
			"candidatesTokenCount": 10,
			"totalTokenCount":      25,
		},
	}

	output, err := geminiResponseToClaude(geminiResponse)
	assertNoError(t, err)
	assertEqual(t, output["type"], "message")
	assertEqual(t, output["role"], "assistant")
	assertEqual(t, output["stop_reason"], "end_turn")

	content := output["content"].([]interface{})
	if len(content) == 0 {
		t.Fatal("Expected non-empty content")
	}
	block := content[0].(map[string]interface{})
	assertEqual(t, block["type"], "text")
	assertEqual(t, block["text"], "Hello from Gemini!")

	usage := output["usage"].(map[string]interface{})
	assertEqual(t, usage["input_tokens"], 15)
	assertEqual(t, usage["output_tokens"], 10)
}

func TestClaudeToGemini_ConvertResponse(t *testing.T) {
	// ClaudeToGeminiConverter: client=Claude, provider=Gemini
	// ConvertResponse: takes Gemini response (provider), returns Claude format (client)
	c := &ClaudeToGeminiConverter{}

	geminiResponse := map[string]interface{}{
		"candidates": []interface{}{
			map[string]interface{}{
				"content": map[string]interface{}{
					"role": "model",
					"parts": []interface{}{
						map[string]interface{}{"text": "The answer is 42."},
					},
				},
				"finishReason": "STOP",
			},
		},
		"usageMetadata": map[string]interface{}{
			"promptTokenCount":     10,
			"candidatesTokenCount": 5,
			"totalTokenCount":      15,
		},
	}

	output, err := c.ConvertResponse(geminiResponse)
	assertNoError(t, err)
	assertEqual(t, output["type"], "message")
	assertEqual(t, output["role"], "assistant")
	assertEqual(t, output["stop_reason"], "end_turn")

	content := output["content"].([]interface{})
	if len(content) != 1 {
		t.Fatalf("Expected 1 content block, got %d", len(content))
	}
	block := content[0].(map[string]interface{})
	assertEqual(t, block["type"], "text")
	assertEqual(t, block["text"], "The answer is 42.")
}

func TestClaudeResponseToGemini(t *testing.T) {
	// Test the claudeResponseToGemini helper directly
	claudeResponse := map[string]interface{}{
		"id":   "msg_123",
		"type": "message",
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": "The answer is 42."},
		},
		"stop_reason": "end_turn",
		"usage": map[string]interface{}{
			"input_tokens":  10,
			"output_tokens": 5,
		},
	}

	output, err := claudeResponseToGemini(claudeResponse)
	assertNoError(t, err)

	candidates := output["candidates"].([]interface{})
	if len(candidates) != 1 {
		t.Fatalf("Expected 1 candidate, got %d", len(candidates))
	}

	candidate := candidates[0].(map[string]interface{})
	assertEqual(t, candidate["finishReason"], "STOP")

	content := candidate["content"].(map[string]interface{})
	assertEqual(t, content["role"], "model")
	parts := content["parts"].([]interface{})
	assertEqual(t, parts[0].(map[string]interface{})["text"], "The answer is 42.")
}

// ===== Gemini Finish Reason Mapping Tests =====

func TestGeminiFinishReasonMapping(t *testing.T) {
	t.Run("Claude->Gemini", func(t *testing.T) {
		assertEqual(t, claudeStopToGeminiFinish("end_turn"), "STOP")
		assertEqual(t, claudeStopToGeminiFinish("max_tokens"), "MAX_TOKENS")
		assertEqual(t, claudeStopToGeminiFinish("stop_sequence"), "STOP")
	})

	t.Run("Gemini->Claude", func(t *testing.T) {
		assertEqual(t, geminiFinishToClaudeStop("STOP"), "end_turn")
		assertEqual(t, geminiFinishToClaudeStop("MAX_TOKENS"), "max_tokens")
		assertEqual(t, geminiFinishToClaudeStop("SAFETY"), "end_turn")
	})

	t.Run("Gemini->OpenAI", func(t *testing.T) {
		assertEqual(t, geminiFinishToOpenAI("STOP"), "stop")
		assertEqual(t, geminiFinishToOpenAI("MAX_TOKENS"), "length")
		assertEqual(t, geminiFinishToOpenAI("SAFETY"), "content_filter")
	})

	t.Run("OpenAI->Gemini", func(t *testing.T) {
		assertEqual(t, openAIFinishToGemini("stop"), "STOP")
		assertEqual(t, openAIFinishToGemini("length"), "MAX_TOKENS")
		assertEqual(t, openAIFinishToGemini("content_filter"), "SAFETY")
		assertEqual(t, openAIFinishToGemini("tool_calls"), "STOP")
	})
}

// ===== GeminiCLI Converter Tests =====

func TestGeminiCLIToClaude_ConvertRequest(t *testing.T) {
	c := &GeminiCLIToClaudeConverter{}

	input := map[string]interface{}{
		"model": "gemini-2.5-pro",
		"request": map[string]interface{}{
			"contents": []interface{}{
				map[string]interface{}{
					"role": "user",
					"parts": []interface{}{
						map[string]interface{}{"text": "Hello from CLI!"},
					},
				},
			},
			"systemInstruction": map[string]interface{}{
				"role": "user",
				"parts": []interface{}{
					map[string]interface{}{"text": "You are helpful."},
				},
			},
		},
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)
	assertEqual(t, output["system"], "You are helpful.")

	messages := output["messages"].([]interface{})
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	assertEqual(t, messages[0].(map[string]interface{})["content"], "Hello from CLI!")
}

func TestClaudeToGeminiCLI_ConvertRequest(t *testing.T) {
	c := &ClaudeToGeminiCLIConverter{}

	input := map[string]interface{}{
		"model":      "claude-opus-4",
		"system":     "Be helpful.",
		"max_tokens": 512,
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "Hello!"},
		},
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)

	// Should have systemInstruction with role: "user" for CLI
	sysInstr := output["systemInstruction"].(map[string]interface{})
	assertEqual(t, sysInstr["role"], "user")
}

func TestGeminiCLIToGemini_Unwrap(t *testing.T) {
	c := &GeminiCLIToGeminiConverter{}

	input := map[string]interface{}{
		"request": map[string]interface{}{
			"contents": []interface{}{
				map[string]interface{}{
					"role":  "user",
					"parts": []interface{}{map[string]interface{}{"text": "hi"}},
				},
			},
		},
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)

	contents := output["contents"].([]interface{})
	if len(contents) != 1 {
		t.Fatalf("Expected 1 content, got %d", len(contents))
	}
}

// ===== Round-trip Tests =====

func TestRoundTrip_ClaudeOpenAIClaude(t *testing.T) {
	// Claude request -> OpenAI -> back to Claude
	original := map[string]interface{}{
		"model": "claude-opus-4",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "Hello!"},
		},
		"system":     "Be helpful.",
		"max_tokens": 1024,
	}

	// Claude -> OpenAI
	c2o := &ClaudeToOpenAIConverter{}
	openaiReq, err := c2o.ConvertRequest(original)
	assertNoError(t, err)

	// OpenAI -> Claude
	o2c := &OpenAIToClaudeConverter{}
	claudeReq, err := o2c.ConvertRequest(openaiReq)
	assertNoError(t, err)

	// Verify key fields preserved
	assertEqual(t, claudeReq["model"], "claude-opus-4")
	assertEqual(t, claudeReq["max_tokens"], 1024)
	assertEqual(t, claudeReq["system"], "Be helpful.")

	messages := claudeReq["messages"].([]interface{})
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	assertEqual(t, messages[0].(map[string]interface{})["role"], "user")
}

func TestRoundTrip_ClaudeGeminiClaude(t *testing.T) {
	// Claude -> Gemini -> Claude
	original := map[string]interface{}{
		"model": "claude-opus-4",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "Hello!"},
			map[string]interface{}{"role": "assistant", "content": "Hi!"},
			map[string]interface{}{"role": "user", "content": "How are you?"},
		},
		"system":     "Be helpful.",
		"max_tokens": 512,
	}

	// Claude -> Gemini
	c2g := &ClaudeToGeminiConverter{}
	geminiReq, err := c2g.ConvertRequest(original)
	assertNoError(t, err)

	// Gemini -> Claude
	g2c := &GeminiToClaudeConverter{}
	claudeReq, err := g2c.ConvertRequest(geminiReq)
	assertNoError(t, err)

	assertEqual(t, claudeReq["system"], "Be helpful.")
	assertEqual(t, claudeReq["max_tokens"], 512)

	messages := claudeReq["messages"].([]interface{})
	if len(messages) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(messages))
	}
	// Verify role mapping round-trip: assistant -> model -> assistant
	assertEqual(t, messages[1].(map[string]interface{})["role"], "assistant")
}

func TestRoundTrip_ResponseConversion(t *testing.T) {
	// Claude response -> OpenAI format -> back to Claude response
	claudeResponse := map[string]interface{}{
		"id":   "msg_roundtrip",
		"type": "message",
		"role": "assistant",
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": "Round trip test."},
		},
		"stop_reason": "end_turn",
		"model":       "claude-opus-4",
		"usage": map[string]interface{}{
			"input_tokens":  10,
			"output_tokens": 15,
		},
	}

	// Claude response -> OpenAI format (via OpenAIToClaudeConverter.ConvertResponse)
	o2c := &OpenAIToClaudeConverter{}
	openaiResp, err := o2c.ConvertResponse(claudeResponse)
	assertNoError(t, err)
	assertEqual(t, openaiResp["object"], "chat.completion")

	// Verify content preserved
	choice := openaiResp["choices"].([]interface{})[0].(map[string]interface{})
	msg := choice["message"].(map[string]interface{})
	assertEqual(t, msg["content"], "Round trip test.")
	assertEqual(t, choice["finish_reason"], "stop")
}

// ===== Edge Case Tests =====

func TestConvertRequest_EmptyMessages(t *testing.T) {
	c := &ClaudeToOpenAIConverter{}

	input := map[string]interface{}{
		"model":      "claude-opus-4",
		"messages":   []interface{}{},
		"max_tokens": 1024,
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)

	messages := output["messages"].([]interface{})
	if len(messages) != 0 {
		t.Errorf("Expected 0 messages, got %d", len(messages))
	}
}

func TestConvertRequest_MissingFields(t *testing.T) {
	c := &ClaudeToOpenAIConverter{}

	// Minimal input with no optional fields
	input := map[string]interface{}{
		"model": "claude-opus-4",
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)
	assertEqual(t, output["model"], "claude-opus-4")
}

func TestConvertResponse_EmptyContent(t *testing.T) {
	c := &ClaudeToOpenAIConverter{}

	input := map[string]interface{}{
		"id":          "msg_empty",
		"type":        "message",
		"content":     []interface{}{},
		"stop_reason": "end_turn",
		"model":       "claude-opus-4",
	}

	output, err := c.ConvertResponse(input)
	assertNoError(t, err)

	choice := output["choices"].([]interface{})[0].(map[string]interface{})
	msg := choice["message"].(map[string]interface{})
	assertEqual(t, msg["content"], "")
}

func TestConvertResponse_UnknownContentType(t *testing.T) {
	c := &ClaudeToOpenAIConverter{}

	input := map[string]interface{}{
		"id":   "msg_unknown",
		"type": "message",
		"content": []interface{}{
			map[string]interface{}{"type": "unknown_type", "data": "something"},
			map[string]interface{}{"type": "text", "text": "Hello"},
		},
		"stop_reason": "end_turn",
		"model":       "claude-opus-4",
	}

	output, err := c.ConvertResponse(input)
	assertNoError(t, err)

	choice := output["choices"].([]interface{})[0].(map[string]interface{})
	msg := choice["message"].(map[string]interface{})
	// Should only pick up the text block
	assertEqual(t, msg["content"], "Hello")
}

func TestCodexConvertRequest_NilInput(t *testing.T) {
	c := &CodexToClaudeConverter{}

	input := map[string]interface{}{
		"model": "gpt-4",
		// No "input" field
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)
	assertEqual(t, output["model"], "gpt-4")
}

func TestGeminiConvertRequest_NoSystemInstruction(t *testing.T) {
	c := &GeminiToClaudeConverter{}

	input := map[string]interface{}{
		"contents": []interface{}{
			map[string]interface{}{
				"role": "user",
				"parts": []interface{}{
					map[string]interface{}{"text": "Hello!"},
				},
			},
		},
	}

	output, err := c.ConvertRequest(input)
	assertNoError(t, err)
	// system should not be set
	if _, ok := output["system"]; ok {
		t.Error("Expected no system field when systemInstruction is absent")
	}
}

func TestGeminiConvertResponse_WrappedResponse(t *testing.T) {
	c := &ClaudeToGeminiConverter{}

	// Some providers wrap Gemini response in {response: {...}}
	wrappedResponse := map[string]interface{}{
		"response": map[string]interface{}{
			"candidates": []interface{}{
				map[string]interface{}{
					"content": map[string]interface{}{
						"role": "model",
						"parts": []interface{}{
							map[string]interface{}{"text": "Wrapped response."},
						},
					},
					"finishReason": "STOP",
				},
			},
		},
	}

	output, err := c.ConvertResponse(wrappedResponse)
	assertNoError(t, err)
	assertEqual(t, output["stop_reason"], "end_turn")
}

// ===== ConvertStreamEvent Tests =====

func TestConvertStreamEvent(t *testing.T) {
	event := SSEEvent{
		Event: "content_block_delta",
		Data:  []byte(`{"type":"content_block_delta","delta":{"type":"text_delta","text":"hi"}}`),
	}

	result, err := ConvertStreamEvent(event, "claude", "openai")
	assertNoError(t, err)
	if len(result.Data) == 0 {
		t.Fatal("Expected non-empty result data")
	}
}

// ===== Codex Streaming Tests =====

func TestCodexStreamToClaude(t *testing.T) {
	input := "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"Hello\"}\n\n"
	output, err := convertCodexStreamToClaude([]byte(input))
	assertNoError(t, err)
	assertContains(t, string(output), "text_delta")
	assertContains(t, string(output), "Hello")
}

func TestCodexStreamCompleted(t *testing.T) {
	input := "event: response.completed\ndata: {\"type\":\"response.completed\"}\n\n"
	output, err := convertCodexStreamToClaude([]byte(input))
	assertNoError(t, err)
	assertContains(t, string(output), "message_stop")
}

// ===== Gemini Streaming Tests =====

func TestGeminiStreamToClaude(t *testing.T) {
	geminiChunk := map[string]interface{}{
		"candidates": []interface{}{
			map[string]interface{}{
				"content": map[string]interface{}{
					"role": "model",
					"parts": []interface{}{
						map[string]interface{}{"text": "Streaming text"},
					},
				},
			},
		},
	}
	data, _ := json.Marshal(geminiChunk)
	input := "data: " + string(data) + "\n\n"

	output, err := convertGeminiStreamToClaude([]byte(input))
	assertNoError(t, err)
	assertContains(t, string(output), "text_delta")
	assertContains(t, string(output), "Streaming text")
}

func TestGeminiStreamToOpenAI(t *testing.T) {
	geminiChunk := map[string]interface{}{
		"candidates": []interface{}{
			map[string]interface{}{
				"content": map[string]interface{}{
					"role": "model",
					"parts": []interface{}{
						map[string]interface{}{"text": "Gemini text"},
					},
				},
				"finishReason": "STOP",
			},
		},
	}
	data, _ := json.Marshal(geminiChunk)
	input := "data: " + string(data) + "\n\n"

	output, err := convertGeminiStreamToOpenAI([]byte(input))
	assertNoError(t, err)
	assertContains(t, string(output), "chat.completion.chunk")
	assertContains(t, string(output), "Gemini text")
}

// ===== Test Helpers =====

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func assertEqual(t *testing.T, got, want interface{}) {
	t.Helper()
	if got != want {
		t.Errorf("got %v (%T), want %v (%T)", got, got, want, want)
	}
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("Expected string to contain %q, got: %s", substr, s)
	}
}
