package converter

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ClaudeToOpenAIConverter Claude API 到 OpenAI API 的转换器
type ClaudeToOpenAIConverter struct{}

// ConvertRequest 将 Claude 请求转换为 OpenAI 格式
func (c *ClaudeToOpenAIConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})

	// 转换模型名称
	if model := getStringField(input, "model"); model != "" {
		output["model"] = model
	}

	// 转换消息
	if messages := getArrayField(input, "messages"); messages != nil {
		convertedMessages, err := c.convertMessages(messages)
		if err != nil {
			return nil, fmt.Errorf("failed to convert messages: %w", err)
		}
		output["messages"] = convertedMessages
	}

	// 转换系统提示
	if system := input["system"]; system != nil {
		// Claude 的 system 可以是字符串或数组
		// OpenAI 需要将其作为第一条 system 消息
		systemMessage := c.convertSystemPrompt(system)
		if systemMessage != nil {
			if messages, ok := output["messages"].([]interface{}); ok {
				output["messages"] = append([]interface{}{systemMessage}, messages...)
			} else {
				output["messages"] = []interface{}{systemMessage}
			}
		}
	}

	// 转换参数
	if maxTokens := getIntField(input, "max_tokens"); maxTokens > 0 {
		output["max_tokens"] = maxTokens
	}

	if temperature, ok := input["temperature"].(float64); ok {
		output["temperature"] = temperature
	}

	if topP, ok := input["top_p"].(float64); ok {
		output["top_p"] = topP
	}

	if topK := getIntField(input, "top_k"); topK > 0 {
		// OpenAI 不支持 top_k，忽略
	}

	// 转换流式标志
	if stream := getBoolField(input, "stream"); stream {
		output["stream"] = true
	}

	// 转换停止序列
	if stopSequences := getArrayField(input, "stop_sequences"); stopSequences != nil {
		stops := make([]string, 0, len(stopSequences))
		for _, s := range stopSequences {
			if str, ok := s.(string); ok {
				stops = append(stops, str)
			}
		}
		if len(stops) > 0 {
			output["stop"] = stops
		}
	}

	// 转换元数据（OpenAI 不支持，但保留在 user 字段中）
	if metadata := getMapField(input, "metadata"); metadata != nil {
		if userID, ok := metadata["user_id"].(string); ok {
			output["user"] = userID
		}
	}

	return output, nil
}

// ConvertResponse 将 OpenAI 响应转换为 Claude 格式
func (c *ClaudeToOpenAIConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})

	// 转换 ID
	if id := getStringField(response, "id"); id != "" {
		output["id"] = id
	}

	// 设置对象类型
	output["object"] = "chat.completion"

	// 转换类型
	output["type"] = "message"

	// 转换角色
	output["role"] = "assistant"

	// 转换内容
	var content string
	if contentArray := getArrayField(response, "content"); contentArray != nil {
		for _, item := range contentArray {
			if block, ok := item.(map[string]interface{}); ok {
				if blockType := getStringField(block, "type"); blockType == "text" {
					if text := getStringField(block, "text"); text != "" {
						content += text
					}
				}
			}
		}
	}

	// 构建 choices
	choice := map[string]interface{}{
		"index": 0,
		"message": map[string]interface{}{
			"role":    "assistant",
			"content": content,
		},
	}

	// 转换停止原因
	if stopReason := getStringField(response, "stop_reason"); stopReason != "" {
		choice["finish_reason"] = c.convertStopReason(stopReason)
	}

	output["choices"] = []interface{}{choice}

	// 转换使用统计
	if usage := getMapField(response, "usage"); usage != nil {
		output["usage"] = map[string]interface{}{
			"prompt_tokens":     getIntField(usage, "input_tokens"),
			"completion_tokens": getIntField(usage, "output_tokens"),
			"total_tokens":      getIntField(usage, "input_tokens") + getIntField(usage, "output_tokens"),
		}
	}

	// 转换模型
	if model := getStringField(response, "model"); model != "" {
		output["model"] = model
	}

	return output, nil
}

// ConvertStreamChunk 将 OpenAI 流式响应块转换为 Claude 格式
func (c *ClaudeToOpenAIConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	// 解析 SSE 数据
	chunkStr := string(chunk)
	if !strings.HasPrefix(chunkStr, "data: ") {
		return chunk, nil
	}

	dataStr := strings.TrimPrefix(chunkStr, "data: ")
	dataStr = strings.TrimSpace(dataStr)

	if dataStr == "[DONE]" {
		// OpenAI 的结束标记
		return []byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"), nil
	}

	// 解析 JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return chunk, nil // 无法解析，返回原始数据
	}

	// 转换为 Claude 格式
	claudeEvent := make(map[string]interface{})

	if choices := getArrayField(data, "choices"); choices != nil && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if delta := getMapField(choice, "delta"); delta != nil {
				if content := getStringField(delta, "content"); content != "" {
					claudeEvent["type"] = "content_block_delta"
					claudeEvent["delta"] = map[string]interface{}{
						"type": "text_delta",
						"text": content,
					}
				}
			}

			if finishReason := getStringField(choice, "finish_reason"); finishReason != "" {
				claudeEvent["type"] = "message_delta"
				claudeEvent["delta"] = map[string]interface{}{
					"stop_reason": c.convertStopReason(finishReason),
				}
			}
		}
	}

	if len(claudeEvent) == 0 {
		return chunk, nil
	}

	// 序列化为 JSON
	claudeData, err := json.Marshal(claudeEvent)
	if err != nil {
		return chunk, nil
	}

	return []byte(fmt.Sprintf("data: %s\n\n", claudeData)), nil
}

// convertMessages 转换消息列表
func (c *ClaudeToOpenAIConverter) convertMessages(messages []interface{}) ([]interface{}, error) {
	result := make([]interface{}, 0, len(messages))

	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}

		convertedMsg := make(map[string]interface{})

		// 转换角色
		if role := getStringField(msgMap, "role"); role != "" {
			convertedMsg["role"] = role
		}

		// 转换内容
		if content := msgMap["content"]; content != nil {
			switch v := content.(type) {
			case string:
				// 简单字符串内容
				convertedMsg["content"] = v
			case []interface{}:
				// 多模态内容
				convertedContent, err := c.convertContent(v)
				if err != nil {
					return nil, err
				}
				convertedMsg["content"] = convertedContent
			}
		}

		result = append(result, convertedMsg)
	}

	return result, nil
}

// convertContent 转换多模态内容
func (c *ClaudeToOpenAIConverter) convertContent(content []interface{}) (interface{}, error) {
	// 如果只有一个文本块，返回字符串
	if len(content) == 1 {
		if block, ok := content[0].(map[string]interface{}); ok {
			if blockType := getStringField(block, "type"); blockType == "text" {
				if text := getStringField(block, "text"); text != "" {
					return text, nil
				}
			}
		}
	}

	// 多个块或包含图片，转换为 OpenAI 格式
	result := make([]interface{}, 0, len(content))

	for _, item := range content {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		blockType := getStringField(block, "type")
		switch blockType {
		case "text":
			result = append(result, map[string]interface{}{
				"type": "text",
				"text": getStringField(block, "text"),
			})
		case "image":
			// Claude 的图片格式转换为 OpenAI 格式
			if source := getMapField(block, "source"); source != nil {
				imageURL := map[string]interface{}{
					"type": "image_url",
				}

				sourceType := getStringField(source, "type")
				if sourceType == "base64" {
					mediaType := getStringField(source, "media_type")
					data := getStringField(source, "data")
					imageURL["image_url"] = map[string]interface{}{
						"url": fmt.Sprintf("data:%s;base64,%s", mediaType, data),
					}
				} else if sourceType == "url" {
					imageURL["image_url"] = map[string]interface{}{
						"url": getStringField(source, "url"),
					}
				}

				result = append(result, imageURL)
			}
		}
	}

	return result, nil
}

// convertSystemPrompt 转换系统提示
func (c *ClaudeToOpenAIConverter) convertSystemPrompt(system interface{}) map[string]interface{} {
	switch v := system.(type) {
	case string:
		return map[string]interface{}{
			"role":    "system",
			"content": v,
		}
	case []interface{}:
		// Claude 的系统提示可以是数组
		var texts []string
		for _, item := range v {
			if block, ok := item.(map[string]interface{}); ok {
				if text := getStringField(block, "text"); text != "" {
					texts = append(texts, text)
				}
			}
		}
		if len(texts) > 0 {
			return map[string]interface{}{
				"role":    "system",
				"content": strings.Join(texts, "\n"),
			}
		}
	}
	return nil
}

// convertStopReason 转换停止原因
func (c *ClaudeToOpenAIConverter) convertStopReason(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "content_filter":
		return "stop_sequence"
	default:
		return reason
	}
}
