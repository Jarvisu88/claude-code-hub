package converter

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// OpenAIToClaudeConverter OpenAI API -> Claude API converter
type OpenAIToClaudeConverter struct{}

// ConvertRequest converts an OpenAI Chat Completions request to Claude Messages API format
func (c *OpenAIToClaudeConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})

	// Model passthrough
	if model := getStringField(input, "model"); model != "" {
		output["model"] = model
	}

	// Convert messages (extract system messages into top-level "system")
	if messages := getArrayField(input, "messages"); messages != nil {
		convertedMessages, systemPrompt, err := c.convertMessages(messages)
		if err != nil {
			return nil, fmt.Errorf("failed to convert messages: %w", err)
		}
		output["messages"] = convertedMessages
		if systemPrompt != "" {
			output["system"] = systemPrompt
		}
	}

	// Scalar parameters
	if maxTokens := getIntField(input, "max_tokens"); maxTokens > 0 {
		output["max_tokens"] = maxTokens
	}
	if temperature, ok := input["temperature"].(float64); ok {
		output["temperature"] = temperature
	}
	if topP, ok := input["top_p"].(float64); ok {
		output["top_p"] = topP
	}

	if stream := getBoolField(input, "stream"); stream {
		output["stream"] = true
	}

	// Stop sequences
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

	// Convert tools: OpenAI function tools -> Claude tools
	if tools := getArrayField(input, "tools"); tools != nil {
		convertedTools := c.convertTools(tools)
		if len(convertedTools) > 0 {
			output["tools"] = convertedTools
		}
	}

	// User -> metadata
	if user := getStringField(input, "user"); user != "" {
		output["metadata"] = map[string]interface{}{
			"user_id": user,
		}
	}

	return output, nil
}

// ConvertResponse converts a Claude Messages API response to OpenAI Chat Completions format.
// Direction: upstream Claude response -> client expecting OpenAI format.
func (c *OpenAIToClaudeConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})

	if id := getStringField(response, "id"); id != "" {
		output["id"] = id
	} else {
		output["id"] = "chatcmpl-" + generateID()
	}
	output["object"] = "chat.completion"
	output["created"] = getCurrentTimestamp()

	if model := getStringField(response, "model"); model != "" {
		output["model"] = model
	}

	// Build content + tool_calls from Claude content blocks
	var textContent string
	var toolCalls []interface{}
	var thinkingContent string
	toolCallIndex := 0

	if contentArray := getArrayField(response, "content"); contentArray != nil {
		for _, item := range contentArray {
			block, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			blockType := getStringField(block, "type")
			switch blockType {
			case "text":
				if text := getStringField(block, "text"); text != "" {
					textContent += text
				}
			case "tool_use":
				tc := map[string]interface{}{
					"id":    getStringField(block, "id"),
					"type":  "function",
					"index": toolCallIndex,
					"function": map[string]interface{}{
						"name": getStringField(block, "name"),
					},
				}
				if args := block["input"]; args != nil {
					argsJSON, err := json.Marshal(args)
					if err == nil {
						tc["function"].(map[string]interface{})["arguments"] = string(argsJSON)
					}
				}
				toolCalls = append(toolCalls, tc)
				toolCallIndex++
			case "thinking":
				if text := getStringField(block, "thinking"); text != "" {
					thinkingContent += text
				}
			}
		}
	}

	message := map[string]interface{}{
		"role":    "assistant",
		"content": textContent,
	}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}
	if thinkingContent != "" {
		message["reasoning_content"] = thinkingContent
	}

	choice := map[string]interface{}{
		"index":         0,
		"message":       message,
		"finish_reason": openAIFinishFromClaudeStop(getStringField(response, "stop_reason")),
	}
	output["choices"] = []interface{}{choice}

	// Usage
	if usage := getMapField(response, "usage"); usage != nil {
		openaiUsage := map[string]interface{}{
			"prompt_tokens":     getIntField(usage, "input_tokens"),
			"completion_tokens": getIntField(usage, "output_tokens"),
			"total_tokens":      getIntField(usage, "input_tokens") + getIntField(usage, "output_tokens"),
		}
		if cacheRead := getIntField(usage, "cache_read_input_tokens"); cacheRead > 0 {
			openaiUsage["cache_read_input_tokens"] = cacheRead
		}
		if cacheCreate := getIntField(usage, "cache_creation_input_tokens"); cacheCreate > 0 {
			openaiUsage["cache_creation_input_tokens"] = cacheCreate
		}
		output["usage"] = openaiUsage
	}

	return output, nil
}

// ConvertStreamChunk converts Claude SSE events -> OpenAI SSE chunks.
// Input: raw SSE bytes with "event:" and "data:" lines.
func (c *OpenAIToClaudeConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
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

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return chunk, nil
	}

	var openaiEvent map[string]interface{}

	switch eventType {
	case "message_start":
		msg := getMapField(data, "message")
		openaiEvent = map[string]interface{}{
			"id":      getStringField(msg, "id"),
			"object":  "chat.completion.chunk",
			"created": getCurrentTimestamp(),
			"model":   getStringField(msg, "model"),
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

	case "content_block_start":
		cb := getMapField(data, "content_block")
		if cb != nil && getStringField(cb, "type") == "tool_use" {
			openaiEvent = map[string]interface{}{
				"id":      "chatcmpl-" + generateID(),
				"object":  "chat.completion.chunk",
				"created": getCurrentTimestamp(),
				"choices": []interface{}{
					map[string]interface{}{
						"index": 0,
						"delta": map[string]interface{}{
							"tool_calls": []interface{}{
								map[string]interface{}{
									"index": getIntField(data, "index"),
									"id":    getStringField(cb, "id"),
									"type":  "function",
									"function": map[string]interface{}{
										"name":      getStringField(cb, "name"),
										"arguments": "",
									},
								},
							},
						},
						"finish_reason": nil,
					},
				},
			}
		}

	case "content_block_delta":
		delta := getMapField(data, "delta")
		if delta != nil {
			deltaType := getStringField(delta, "type")
			switch deltaType {
			case "text_delta":
				if text := getStringField(delta, "text"); text != "" {
					openaiEvent = map[string]interface{}{
						"id":      "chatcmpl-" + generateID(),
						"object":  "chat.completion.chunk",
						"created": getCurrentTimestamp(),
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
			case "input_json_delta":
				if partial := getStringField(delta, "partial_json"); partial != "" {
					openaiEvent = map[string]interface{}{
						"id":      "chatcmpl-" + generateID(),
						"object":  "chat.completion.chunk",
						"created": getCurrentTimestamp(),
						"choices": []interface{}{
							map[string]interface{}{
								"index": 0,
								"delta": map[string]interface{}{
									"tool_calls": []interface{}{
										map[string]interface{}{
											"index": getIntField(data, "index"),
											"function": map[string]interface{}{
												"arguments": partial,
											},
										},
									},
								},
								"finish_reason": nil,
							},
						},
					}
				}
			case "thinking_delta":
				if text := getStringField(delta, "thinking"); text != "" {
					openaiEvent = map[string]interface{}{
						"id":      "chatcmpl-" + generateID(),
						"object":  "chat.completion.chunk",
						"created": getCurrentTimestamp(),
						"choices": []interface{}{
							map[string]interface{}{
								"index": 0,
								"delta": map[string]interface{}{
									"reasoning_content": text,
								},
								"finish_reason": nil,
							},
						},
					}
				}
			}
		}

	case "message_delta":
		delta := getMapField(data, "delta")
		if delta != nil {
			if stopReason := getStringField(delta, "stop_reason"); stopReason != "" {
				openaiEvent = map[string]interface{}{
					"id":      "chatcmpl-" + generateID(),
					"object":  "chat.completion.chunk",
					"created": getCurrentTimestamp(),
					"choices": []interface{}{
						map[string]interface{}{
							"index":         0,
							"delta":         map[string]interface{}{},
							"finish_reason": openAIFinishFromClaudeStop(stopReason),
						},
					},
				}
			}
			if usage := getMapField(data, "usage"); usage != nil {
				if openaiEvent == nil {
					openaiEvent = map[string]interface{}{
						"id":      "chatcmpl-" + generateID(),
						"object":  "chat.completion.chunk",
						"created": getCurrentTimestamp(),
						"choices": []interface{}{},
					}
				}
				openaiEvent["usage"] = map[string]interface{}{
					"prompt_tokens":     getIntField(usage, "input_tokens"),
					"completion_tokens": getIntField(usage, "output_tokens"),
					"total_tokens":      getIntField(usage, "input_tokens") + getIntField(usage, "output_tokens"),
				}
			}
		}

	case "message_stop":
		return []byte("data: [DONE]\n\n"), nil

	default:
		return chunk, nil
	}

	if openaiEvent == nil {
		return chunk, nil
	}

	return []byte(fmt.Sprintf("data: %s\n\n", mustMarshal(openaiEvent))), nil
}

// convertMessages extracts system prompt and converts messages
func (c *OpenAIToClaudeConverter) convertMessages(messages []interface{}) ([]interface{}, string, error) {
	result := make([]interface{}, 0, len(messages))
	var systemParts []string

	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}

		role := getStringField(msgMap, "role")

		// Extract system messages
		if role == "system" {
			if content := msgMap["content"]; content != nil {
				if str, ok := content.(string); ok {
					systemParts = append(systemParts, str)
				} else if arr, ok := content.([]interface{}); ok {
					for _, item := range arr {
						if m, ok := item.(map[string]interface{}); ok {
							if getStringField(m, "type") == "text" {
								systemParts = append(systemParts, getStringField(m, "text"))
							}
						}
					}
				}
			}
			continue
		}

		// Convert tool messages -> Claude tool_result within user message
		if role == "tool" {
			toolResult := map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": getStringField(msgMap, "tool_call_id"),
			}
			if content := msgMap["content"]; content != nil {
				if str, ok := content.(string); ok {
					toolResult["content"] = str
				}
			}
			result = append(result, map[string]interface{}{
				"role":    "user",
				"content": []interface{}{toolResult},
			})
			continue
		}

		convertedMsg := map[string]interface{}{
			"role": role,
		}

		// Handle tool_calls in assistant messages
		if role == "assistant" {
			content, toolUseBlocks := c.convertAssistantMessage(msgMap)
			if len(toolUseBlocks) > 0 || content != nil {
				blocks := make([]interface{}, 0)
				if content != nil {
					switch v := content.(type) {
					case string:
						if v != "" {
							blocks = append(blocks, map[string]interface{}{
								"type": "text",
								"text": v,
							})
						}
					case []interface{}:
						blocks = append(blocks, v...)
					}
				}
				blocks = append(blocks, toolUseBlocks...)

				// Add reasoning_content as thinking block if present
				if reasoning := getStringField(msgMap, "reasoning_content"); reasoning != "" {
					// Prepend thinking block
					thinkingBlock := map[string]interface{}{
						"type":     "thinking",
						"thinking": reasoning,
					}
					blocks = append([]interface{}{thinkingBlock}, blocks...)
				}

				convertedMsg["content"] = blocks
			} else {
				// Simple assistant text
				if content != nil {
					convertedMsg["content"] = content
				}
			}
			result = append(result, convertedMsg)
			continue
		}

		// Convert user messages content
		if content := msgMap["content"]; content != nil {
			switch v := content.(type) {
			case string:
				convertedMsg["content"] = v
			case []interface{}:
				convertedContent, err := c.convertContent(v)
				if err != nil {
					return nil, "", err
				}
				convertedMsg["content"] = convertedContent
			}
		}

		result = append(result, convertedMsg)
	}

	return result, strings.Join(systemParts, "\n"), nil
}

// convertAssistantMessage handles OpenAI assistant message with potential tool_calls
func (c *OpenAIToClaudeConverter) convertAssistantMessage(msg map[string]interface{}) (interface{}, []interface{}) {
	var content interface{}
	var toolUseBlocks []interface{}

	// Extract text content
	if raw := msg["content"]; raw != nil {
		if str, ok := raw.(string); ok {
			content = str
		} else if arr, ok := raw.([]interface{}); ok {
			content = arr
		}
	}

	// Convert tool_calls -> tool_use content blocks
	if toolCalls := getArrayField(msg, "tool_calls"); toolCalls != nil {
		for _, tc := range toolCalls {
			tcMap, ok := tc.(map[string]interface{})
			if !ok {
				continue
			}
			fn := getMapField(tcMap, "function")
			if fn == nil {
				continue
			}

			block := map[string]interface{}{
				"type": "tool_use",
				"id":   getStringField(tcMap, "id"),
				"name": getStringField(fn, "name"),
			}

			// Parse arguments JSON string into object
			if argsStr := getStringField(fn, "arguments"); argsStr != "" {
				var args interface{}
				if err := json.Unmarshal([]byte(argsStr), &args); err == nil {
					block["input"] = args
				} else {
					block["input"] = map[string]interface{}{}
				}
			} else {
				block["input"] = map[string]interface{}{}
			}

			toolUseBlocks = append(toolUseBlocks, block)
		}
	}

	return content, toolUseBlocks
}

// convertContent converts OpenAI multimodal content parts to Claude format
func (c *OpenAIToClaudeConverter) convertContent(content []interface{}) (interface{}, error) {
	// Single text block optimization
	if len(content) == 1 {
		if block, ok := content[0].(map[string]interface{}); ok {
			if blockType := getStringField(block, "type"); blockType == "text" {
				if text := getStringField(block, "text"); text != "" {
					return text, nil
				}
			}
		}
	}

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
			if imageURL := getMapField(block, "image_url"); imageURL != nil {
				url := getStringField(imageURL, "url")
				if strings.HasPrefix(url, "data:") {
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

// convertTools converts OpenAI function tool definitions to Claude tool definitions
func (c *OpenAIToClaudeConverter) convertTools(tools []interface{}) []interface{} {
	result := make([]interface{}, 0, len(tools))
	for _, tool := range tools {
		t, ok := tool.(map[string]interface{})
		if !ok {
			continue
		}
		if getStringField(t, "type") != "function" {
			continue
		}
		fn := getMapField(t, "function")
		if fn == nil {
			continue
		}
		claudeTool := map[string]interface{}{
			"name": getStringField(fn, "name"),
		}
		if desc := getStringField(fn, "description"); desc != "" {
			claudeTool["description"] = desc
		}
		if params := getMapField(fn, "parameters"); params != nil {
			claudeTool["input_schema"] = params
		}
		result = append(result, claudeTool)
	}
	return result
}

// openAIFinishFromClaudeStop maps Claude stop_reason to OpenAI finish_reason
func openAIFinishFromClaudeStop(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "stop_sequence":
		return "stop"
	default:
		if reason == "" {
			return "stop"
		}
		return reason
	}
}

// Helper functions

// generateID generates a simple numeric ID based on timestamp
func generateID() string {
	return fmt.Sprintf("%d", getCurrentTimestamp())
}

// getCurrentTimestamp returns current Unix timestamp
func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}
