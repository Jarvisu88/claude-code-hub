package converter

import (
	"encoding/json"
	"testing"
)

// TestNewConverter 测试创建转换器
func TestNewConverter(t *testing.T) {
	tests := []struct {
		name         string
		clientType   ClientType
		providerType ProviderType
		wantType     string
	}{
		{
			name:         "Claude to Claude (passthrough)",
			clientType:   ClientTypeClaude,
			providerType: ProviderTypeClaude,
			wantType:     "*converter.PassthroughConverter",
		},
		{
			name:         "Claude to OpenAI",
			clientType:   ClientTypeClaude,
			providerType: ProviderTypeOpenAI,
			wantType:     "*converter.ClaudeToOpenAIConverter",
		},
		{
			name:         "OpenAI to Claude",
			clientType:   ClientTypeOpenAI,
			providerType: ProviderTypeClaude,
			wantType:     "*converter.OpenAIToClaudeConverter",
		},
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

// TestPassthroughConverter 测试透传转换器
func TestPassthroughConverter(t *testing.T) {
	converter := &PassthroughConverter{}

	input := map[string]interface{}{
		"model": "claude-opus-4",
		"messages": []interface{}{
			map[string]interface{}{
				"role":    "user",
				"content": "Hello",
			},
		},
	}

	// 测试请求转换
	output, err := converter.ConvertRequest(input)
	if err != nil {
		t.Fatalf("ConvertRequest() error = %v", err)
	}

	if output["model"] != "claude-opus-4" {
		t.Errorf("Expected model = claude-opus-4, got %v", output["model"])
	}

	// 测试响应转换
	response := map[string]interface{}{
		"id":   "msg_123",
		"type": "message",
	}

	output, err = converter.ConvertResponse(response)
	if err != nil {
		t.Fatalf("ConvertResponse() error = %v", err)
	}

	if output["id"] != "msg_123" {
		t.Errorf("Expected id = msg_123, got %v", output["id"])
	}

	// 测试流式转换
	chunk := []byte("data: {\"type\":\"content_block_delta\"}\n\n")
	outputChunk, err := converter.ConvertStreamChunk(chunk)
	if err != nil {
		t.Fatalf("ConvertStreamChunk() error = %v", err)
	}

	if string(outputChunk) != string(chunk) {
		t.Errorf("Expected chunk to be unchanged")
	}
}

// TestClaudeToOpenAIConverter_ConvertRequest 测试 Claude 到 OpenAI 请求转换
func TestClaudeToOpenAIConverter_ConvertRequest(t *testing.T) {
	converter := &ClaudeToOpenAIConverter{}

	input := map[string]interface{}{
		"model": "claude-opus-4",
		"messages": []interface{}{
			map[string]interface{}{
				"role":    "user",
				"content": "Hello, how are you?",
			},
		},
		"max_tokens":  1024,
		"temperature": 0.7,
		"stream":      true,
		"system":      "You are a helpful assistant.",
	}

	output, err := converter.ConvertRequest(input)
	if err != nil {
		t.Fatalf("ConvertRequest() error = %v", err)
	}

	// 检查模型
	if output["model"] != "claude-opus-4" {
		t.Errorf("Expected model = claude-opus-4, got %v", output["model"])
	}

	// 检查 max_tokens
	if output["max_tokens"] != 1024 {
		t.Errorf("Expected max_tokens = 1024, got %v", output["max_tokens"])
	}

	// 检查 temperature
	if output["temperature"] != 0.7 {
		t.Errorf("Expected temperature = 0.7, got %v", output["temperature"])
	}

	// 检查 stream
	if output["stream"] != true {
		t.Errorf("Expected stream = true, got %v", output["stream"])
	}

	// 检查消息（应该包含系统消息）
	messages, ok := output["messages"].([]interface{})
	if !ok {
		t.Fatal("Expected messages to be array")
	}

	if len(messages) != 2 {
		t.Errorf("Expected 2 messages (system + user), got %d", len(messages))
	}

	// 检查第一条消息是系统消息
	firstMsg, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected first message to be map")
	}

	if firstMsg["role"] != "system" {
		t.Errorf("Expected first message role = system, got %v", firstMsg["role"])
	}

	if firstMsg["content"] != "You are a helpful assistant." {
		t.Errorf("Expected system content, got %v", firstMsg["content"])
	}
}

// TestClaudeToOpenAIConverter_ConvertResponse 测试 Claude 到 OpenAI 响应转换
func TestClaudeToOpenAIConverter_ConvertResponse(t *testing.T) {
	converter := &ClaudeToOpenAIConverter{}

	input := map[string]interface{}{
		"id":   "msg_123",
		"type": "message",
		"role": "assistant",
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": "Hello! I'm doing well, thank you.",
			},
		},
		"model":       "claude-opus-4",
		"stop_reason": "end_turn",
		"usage": map[string]interface{}{
			"input_tokens":  10,
			"output_tokens": 20,
		},
	}

	output, err := converter.ConvertResponse(input)
	if err != nil {
		t.Fatalf("ConvertResponse() error = %v", err)
	}

	// 检查 ID
	if output["id"] != "msg_123" {
		t.Errorf("Expected id = msg_123, got %v", output["id"])
	}

	// 检查 object
	if output["object"] != "chat.completion" {
		t.Errorf("Expected object = chat.completion, got %v", output["object"])
	}

	// 检查 choices
	choices, ok := output["choices"].([]interface{})
	if !ok {
		t.Fatal("Expected choices to be array")
	}

	if len(choices) != 1 {
		t.Errorf("Expected 1 choice, got %d", len(choices))
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected choice to be map")
	}

	// 检查 message
	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected message to be map")
	}

	if message["role"] != "assistant" {
		t.Errorf("Expected role = assistant, got %v", message["role"])
	}

	if message["content"] != "Hello! I'm doing well, thank you." {
		t.Errorf("Expected content, got %v", message["content"])
	}

	// 检查 finish_reason
	if choice["finish_reason"] != "end_turn" {
		t.Errorf("Expected finish_reason = end_turn, got %v", choice["finish_reason"])
	}

	// 检查 usage
	usage, ok := output["usage"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected usage to be map")
	}

	if usage["prompt_tokens"] != 10 {
		t.Errorf("Expected prompt_tokens = 10, got %v", usage["prompt_tokens"])
	}

	if usage["completion_tokens"] != 20 {
		t.Errorf("Expected completion_tokens = 20, got %v", usage["completion_tokens"])
	}
}

// TestOpenAIToClaudeConverter_ConvertRequest 测试 OpenAI 到 Claude 请求转换
func TestOpenAIToClaudeConverter_ConvertRequest(t *testing.T) {
	converter := &OpenAIToClaudeConverter{}

	input := map[string]interface{}{
		"model": "gpt-4",
		"messages": []interface{}{
			map[string]interface{}{
				"role":    "system",
				"content": "You are a helpful assistant.",
			},
			map[string]interface{}{
				"role":    "user",
				"content": "Hello!",
			},
		},
		"max_tokens":  1024,
		"temperature": 0.7,
		"stream":      true,
	}

	output, err := converter.ConvertRequest(input)
	if err != nil {
		t.Fatalf("ConvertRequest() error = %v", err)
	}

	// 检查模型
	if output["model"] != "gpt-4" {
		t.Errorf("Expected model = gpt-4, got %v", output["model"])
	}

	// 检查系统提示
	if output["system"] != "You are a helpful assistant." {
		t.Errorf("Expected system prompt, got %v", output["system"])
	}

	// 检查消息（不应该包含系统消息）
	messages, ok := output["messages"].([]interface{})
	if !ok {
		t.Fatal("Expected messages to be array")
	}

	if len(messages) != 1 {
		t.Errorf("Expected 1 message (user only), got %d", len(messages))
	}

	// 检查第一条消息是用户消息
	firstMsg, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected first message to be map")
	}

	if firstMsg["role"] != "user" {
		t.Errorf("Expected first message role = user, got %v", firstMsg["role"])
	}
}

// TestClaudeToOpenAIConverter_ConvertStreamChunk 测试流式转换
func TestClaudeToOpenAIConverter_ConvertStreamChunk(t *testing.T) {
	converter := &ClaudeToOpenAIConverter{}

	// 测试内容增量
	claudeChunk := map[string]interface{}{
		"type": "content_block_delta",
		"delta": map[string]interface{}{
			"type": "text_delta",
			"text": "Hello",
		},
	}

	claudeData, _ := json.Marshal(claudeChunk)
	input := []byte("data: " + string(claudeData) + "\n\n")

	output, err := converter.ConvertStreamChunk(input)
	if err != nil {
		t.Fatalf("ConvertStreamChunk() error = %v", err)
	}

	// 应该转换为 OpenAI 格式
	if len(output) == 0 {
		t.Error("Expected non-empty output")
	}

	// 测试结束标记
	input = []byte("data: [DONE]\n\n")
	output, err = converter.ConvertStreamChunk(input)
	if err != nil {
		t.Fatalf("ConvertStreamChunk() error = %v", err)
	}

	// 应该转换为 Claude 的结束事件
	if !contains(string(output), "message_stop") {
		t.Error("Expected message_stop event")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
