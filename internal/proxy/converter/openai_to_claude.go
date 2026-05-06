package converter

import (
	"encoding/json"
	"fmt"
	"strings"
)

// OpenAIToClaudeConverter OpenAI API 到 Claude API 的转换器
type OpenAIToClaudeConverter struct{}

// ConvertRequest 将 OpenAI 请求转换为 Claude 格式
func (c *OpenAIToClaudeConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})

	// 转换模型名称
	if model := getStringField(input, "model"); model != "" {
		output["model"] = model
	}

	// 转换消息
	if messages := getArrayField(input, "messages"); messages != nil {
		convertedMessages, systemPrompt, err := c.convertMessages(messages)
		if err != nil {
			return nil, fmt.Errorf("failed to convert messages: %w", err)
		}
		output["messages"] = convertedMessages

		// 如果有系统提示，单独设置
		if systemPrompt != "" {
			output["system"] = systemPrompt
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

	// 转换流式标志
	if stream := getBoolField(input, "stream"); stream {
		output["stream"] = true
	}

	// 转换停止序列
	if stop := input["stop"]; stop != nil {
		var stopSequences []string
		switch v := stop.(type) {
		case string:
			stopSequences = []string{v}
		case []interface{}:
			for _, s := range v {
				if str, ok := s.(string); ok {
					stopSequences = append(stopSequences, str)
				}
			}
		}
		if len(stopSequences) > 0 {
			output["stop_sequences"] = stopSequences
		}
	}

	// 转换用户 ID
	if user := getStringField(input, "user"); user != "" {
		output["metadata"] = map[string]interface{}{
			"user_id": user,
		}
	}

	return output, nil
}

// ConvertResponse 将 Claude 响应转换为 OpenAI 格式
func (c *OpenAIToClaudeConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})

	// 转换 ID
	if id := getStringField(response, "id"); id != "" {
		output["id"] = id
	} else {
		output["id"] = "chatcmpl-" + generateID()
	}

	// 设置对象类型
	output["object"] = "chat.completion"

	// 转换时间戳
	output["created"] = getCurrentTimestamp()

	// 转换模型
	if model := getStringField(response, "model"); model != "" {
		output["model"] = model
	}

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
		"finish_reason": c.convertStopReason(getStringField(response, "stop_reason")),
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

	return output, nil
}

// ConvertStreamChunk 将 Claude 流式响应块转换为 OpenAI 格式
func (c *OpenAIToClaudeConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	// 解析 SSE 数据
	chunkStr := string(chunk)
	lines := strings.Split(chunkStr, "\n")

	var eventType string
	var dataStr string

	for _, line := range lines {
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

	// 解析 JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return chunk, nil
	}

	// 根据事件类型转换
	var openaiEvent map[string]interface{}

	switch eventType {
	case "message_start":
		// 消息开始
		openaiEvent = map[string]interface{}{
			"id":      getStringField(data, "id"),
			"object":  "chat.completion.chunk",
			"created": getCurrentTimestamp(),
			"model":   getStringField(getMapField(data, "message"), "model"),
			"choices": []interface{}{
				map[string]interface{}{
					"index": 0,
					"delta": map[string]interface{}{
						"role": "assistant",
					},
					"finish_reason": nil,
				},
			},
		}

	case "content_block_delta":
		// 内容增量
		if delta := getMapField(data, "delta"); delta != nil {
			if text := getStringField(delta, "text"); text != "" {
				openaiEvent = map[string]interface{}{
					"id":      "chatcmpl-" + generateID(),
					"object":  "chat.completion.chunk",
					"created": getCurrentTimestamp(),
					"model":   "",
					"choices": []interface{}{
						map[string]interface{}{
							"index": 0,
							"delta": map[string]interface{}{
								"content": text,
							},
							"finish_reason": nil,
						},
					},
				}
			}
		}

	case "message_delta":
		// 消息增量（包含停止原因）
		if delta := getMapField(data, "delta"); delta != nil {
			if stopReason := getStringField(delta, "stop_reason"); stopReason != "" {
				openaiEvent = map[string]interface{}{
					"id":      "chatcmpl-" + generateID(),
					"object":  "chat.completion.chunk",
					"created": getCurrentTimestamp(),
					"model":   "",
					"choices": []interface{}{
						map[string]interface{}{
							"index":         0,
							"delta":         map[string]interface{}{},
							"finish_reason": c.convertStopReason(stopReason),
						},
					},
				}
			}
		}

	case "message_stop":
		// 消息结束
		return []byte("data: [DONE]\n\n"), nil

	default:
		return chunk, nil
	}

	if openaiEvent == nil {
		return chunk, nil
	}

	// 序列化为 JSON
	openaiData, err := json.Marshal(openaiEvent)
	if err != nil {
		return chunk, nil
	}

	return []byte(fmt.Sprintf("data: %s\n\n", openaiData)), nil
}

// convertMessages 转换消息列表，提取系统提示
func (c *OpenAIToClaudeConverter) convertMessages(messages []interface{}) ([]interface{}, string, error) {
	result := make([]interface{}, 0, len(messages))
	var systemPrompt string

	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}

		role := getStringField(msgMap, "role")

		// 提取系统消息
		if role == "system" {
			if content := msgMap["content"]; content != nil {
				if str, ok := content.(string); ok {
					if systemPrompt != "" {
						systemPrompt += "\n"
					}
					systemPrompt += str
				}
			}
			continue
		}

		// 转换用户和助手消息
		convertedMsg := make(map[string]interface{})
		convertedMsg["role"] = role

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
					return nil, "", err
				}
				convertedMsg["content"] = convertedContent
			}
		}

		result = append(result, convertedMsg)
	}

	return result, systemPrompt, nil
}

// convertContent 转换多模态内容
func (c *OpenAIToClaudeConverter) convertContent(content []interface{}) (interface{}, error) {
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

	// 多个块或包含图片，转换为 Claude 格式
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
		case "image_url":
			// OpenAI 的图片格式转换为 Claude 格式
			if imageURL := getMapField(block, "image_url"); imageURL != nil {
				url := getStringField(imageURL, "url")
				if strings.HasPrefix(url, "data:") {
					// Base64 图片
					parts := strings.SplitN(url, ",", 2)
					if len(parts) == 2 {
						mediaType := strings.TrimPrefix(strings.Split(parts[0], ";")[0], "data:")
						result = append(result, map[string]interface{}{
							"type": "image",
							"source": map[string]interface{}{
								"type":       "base64",
								"media_type": mediaType,
								"data":       parts[1],
							},
						})
					}
				} else {
					// URL 图片
					result = append(result, map[string]interface{}{
						"type": "image",
						"source": map[string]interface{}{
							"type": "url",
							"url":  url,
						},
					})
				}
			}
		}
	}

	return result, nil
}

// convertStopReason 转换停止原因
func (c *OpenAIToClaudeConverter) convertStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "content_filter"
	default:
		return "stop"
	}
}

// Helper functions

// generateID 生成随机 ID
func generateID() string {
	// 简单实现，实际应该使用更好的 ID 生成器
	return fmt.Sprintf("%d", getCurrentTimestamp())
}

// getCurrentTimestamp 获取当前时间戳
func getCurrentTimestamp() int64 {
	return 1704067200 // 示例时间戳，实际应该使用 time.Now().Unix()
}
