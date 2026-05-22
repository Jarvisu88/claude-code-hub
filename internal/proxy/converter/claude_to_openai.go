package converter

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ClaudeToOpenAIConverter Claude API -> OpenAI API converter
type ClaudeToOpenAIConverter struct{}

// ConvertRequest converts a Claude Messages API request to OpenAI Chat Completions format
func (c *ClaudeToOpenAIConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})

	// Model passthrough
	if model := getStringField(input, "model"); model != "" {
		output["model"] = model
	}

	// Convert messages
	if messages := getArrayField(input, "messages"); messages != nil {
		convertedMessages, err := c.convertMessages(messages)
		if err != nil {
			return nil, fmt.Errorf("failed to convert messages: %w", err)
		}
		output["messages"] = convertedMessages
	}

	// Convert system prompt: Claude top-level "system" -> OpenAI system message prepended
	if system := input["system"]; system != nil {
		systemMessage := c.convertSystemPrompt(system)
		if systemMessage != nil {
			if messages, ok := output["messages"].([]interface{}); ok {
				output["messages"] = append([]interface{}{systemMessage}, messages...)
			} else {
				output["messages"] = []interface{}{systemMessage}
			}
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
	// top_k is Claude-only, OpenAI does not support it

	if stream := getBoolField(input, "stream"); stream {
		output["stream"] = true
	}

	// Stop sequences
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

	// Convert tools: Claude tools -> OpenAI function tools
	if tools := getArrayField(input, "tools"); tools != nil {
		convertedTools := c.convertTools(tools)
		if len(convertedTools) > 0 {
			output["tools"] = convertedTools
		}
	}

	// Metadata -> user
	if metadata := getMapField(input, "metadata"); metadata != nil {
		if userID, ok := metadata["user_id"].(string); ok {
			output["user"] = userID
		}
	}

	return output, nil
}

// ConvertResponse converts a Claude Messages API response to OpenAI Chat Completions format.
// Direction: upstream Claude response -> client expecting OpenAI format.
func (c *ClaudeToOpenAIConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})

	if id := getStringField(response, "id"); id != "" {
		output["id"] = id
	}
	output["object"] = "chat.completion"
	output["type"] = "message"
	output["role"] = "assistant"

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
					"id":   getStringField(block, "id"),
					"type": "function",
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

	// Build the message
	message := map[string]interface{}{
		"role":    "assistant",
		"content": textContent,
	}

	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}

	// Map thinking to reasoning_content (OpenAI o-series convention)
	if thinkingContent != "" {
		message["reasoning_content"] = thinkingContent
	}

	// Convert stop_reason -> finish_reason
	choice := map[string]interface{}{
		"index":   0,
		"message": message,
	}
	if stopReason := getStringField(response, "stop_reason"); stopReason != "" {
		choice["finish_reason"] = claudeStopToOpenAIFinish(stopReason)
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

// ConvertStreamChunk converts a Claude SSE event into an OpenAI-compatible SSE chunk.
// Input: raw SSE bytes (may contain "event:" and "data:" lines)
// Output: OpenAI "data: {...}\n\n" line
func (c *ClaudeToOpenAIConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	chunkStr := string(chunk)

	// Handle plain "data: ..." format (no event: prefix)
	if !strings.Contains(chunkStr, "event:") && strings.HasPrefix(strings.TrimSpace(chunkStr), "data: ") {
		dataStr := strings.TrimPrefix(strings.TrimSpace(chunkStr), "data: ")
		dataStr = strings.TrimSpace(dataStr)
		if dataStr == "[DONE]" {
			return []byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"), nil
		}
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
			return chunk, nil
		}
		return c.convertClaudeEventToOpenAI("", data)
	}

	// Parse event: and data: lines
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

	return c.convertClaudeEventToOpenAI(eventType, data)
}

func (c *ClaudeToOpenAIConverter) convertClaudeEventToOpenAI(eventType string, data map[string]interface{}) ([]byte, error) {
	// If eventType is empty, infer from data "type" field
	if eventType == "" {
		eventType = getStringField(data, "type")
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
			argsJSON := "{}"
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
										"arguments": argsJSON,
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
				// Map Claude thinking delta to reasoning_content delta
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
							"finish_reason": claudeStopToOpenAIFinish(stopReason),
						},
					},
				}
			}
			// Streaming usage in message_delta
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
		// Unknown event, passthrough
		return []byte(fmt.Sprintf("data: %s\n\n", mustMarshal(data))), nil
	}

	if openaiEvent == nil {
		return nil, nil
	}

	return []byte(fmt.Sprintf("data: %s\n\n", mustMarshal(openaiEvent))), nil
}

// convertMessages converts Claude messages array to OpenAI messages
func (c *ClaudeToOpenAIConverter) convertMessages(messages []interface{}) ([]interface{}, error) {
	result := make([]interface{}, 0, len(messages))

	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}

		role := getStringField(msgMap, "role")

		if content := msgMap["content"]; content != nil {
			switch v := content.(type) {
			case string:
				result = append(result, map[string]interface{}{
					"role":    role,
					"content": v,
				})
			case []interface{}:
				converted, toolCalls, err := c.convertContentBlocks(v)
				if err != nil {
					return nil, err
				}
				outMsg := map[string]interface{}{
					"role": role,
				}
				if converted != nil {
					outMsg["content"] = converted
				}
				if len(toolCalls) > 0 {
					outMsg["tool_calls"] = toolCalls
				}
				result = append(result, outMsg)
			}
		}
	}

	return result, nil
}

// convertContentBlocks converts Claude content block array.
// Returns (content for OpenAI, tool_calls for assistant messages, error).
func (c *ClaudeToOpenAIConverter) convertContentBlocks(content []interface{}) (interface{}, []interface{}, error) {
	// Classify blocks
	var textParts []interface{}
	var imageParts []interface{}
	var toolCalls []interface{}
	var toolResults []interface{}
	toolCallIdx := 0

	for _, item := range content {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		blockType := getStringField(block, "type")
		switch blockType {
		case "text":
			textParts = append(textParts, map[string]interface{}{
				"type": "text",
				"text": getStringField(block, "text"),
			})
		case "image":
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
				imageParts = append(imageParts, imageURL)
			}
		case "tool_use":
			tc := map[string]interface{}{
				"id":    getStringField(block, "id"),
				"type":  "function",
				"index": toolCallIdx,
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
			toolCallIdx++
		case "tool_result":
			// Claude tool_result -> OpenAI tool message
			toolResults = append(toolResults, map[string]interface{}{
				"role":         "tool",
				"tool_call_id": getStringField(block, "tool_use_id"),
				"content":      c.extractToolResultContent(block),
			})
		case "thinking":
			// Preserve as metadata but skip for OpenAI content
		}
	}

	// If we have tool_results, they need to be separate messages in OpenAI format,
	// but since we're converting a single message's content, we'll embed them.
	// The caller handles message-level structure.

	// Simple case: only text blocks
	if len(imageParts) == 0 && len(toolCalls) == 0 && len(toolResults) == 0 {
		if len(textParts) == 1 {
			return textParts[0].(map[string]interface{})["text"], nil, nil
		}
		if len(textParts) > 1 {
			return textParts, nil, nil
		}
		return "", nil, nil
	}

	// Multimodal: combine text + images
	combined := make([]interface{}, 0, len(textParts)+len(imageParts))
	combined = append(combined, textParts...)
	combined = append(combined, imageParts...)

	if len(combined) == 0 && len(toolCalls) > 0 {
		return nil, toolCalls, nil
	}

	return combined, toolCalls, nil
}

// extractToolResultContent extracts the content string from a Claude tool_result block
func (c *ClaudeToOpenAIConverter) extractToolResultContent(block map[string]interface{}) string {
	if content := block["content"]; content != nil {
		switch v := content.(type) {
		case string:
			return v
		case []interface{}:
			var texts []string
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					if getStringField(m, "type") == "text" {
						texts = append(texts, getStringField(m, "text"))
					}
				}
			}
			return strings.Join(texts, "")
		}
	}
	return ""
}

// convertSystemPrompt converts Claude system prompt to an OpenAI system message
func (c *ClaudeToOpenAIConverter) convertSystemPrompt(system interface{}) map[string]interface{} {
	switch v := system.(type) {
	case string:
		return map[string]interface{}{
			"role":    "system",
			"content": v,
		}
	case []interface{}:
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

// convertTools converts Claude tool definitions to OpenAI function tool definitions
func (c *ClaudeToOpenAIConverter) convertTools(tools []interface{}) []interface{} {
	result := make([]interface{}, 0, len(tools))
	for _, tool := range tools {
		t, ok := tool.(map[string]interface{})
		if !ok {
			continue
		}
		fn := map[string]interface{}{
			"name": getStringField(t, "name"),
		}
		if desc := getStringField(t, "description"); desc != "" {
			fn["description"] = desc
		}
		if schema := getMapField(t, "input_schema"); schema != nil {
			fn["parameters"] = schema
		}
		result = append(result, map[string]interface{}{
			"type":     "function",
			"function": fn,
		})
	}
	return result
}

// claudeStopToOpenAIFinish maps Claude stop_reason to OpenAI finish_reason
func claudeStopToOpenAIFinish(reason string) string {
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
		return reason
	}
}
