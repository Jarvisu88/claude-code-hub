package converter

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CodexConverter converts between Codex Response API format and Claude/OpenAI formats.
//
// Codex Response API key differences:
//   - Request uses "input" (string or array of items) instead of "messages"
//   - Response uses "output" array instead of "choices"
//   - Streaming events: response.created, response.output_item.added,
//     response.content_part.added, response.output_text.delta,
//     response.output_text.done, response.output_item.done, response.completed

// --- Codex <-> Claude ---

// CodexToClaudeConverter converts Codex requests to Claude format and Claude responses back to Codex
type CodexToClaudeConverter struct{}

// ConvertRequest converts a Codex Response API request into a Claude Messages API request
func (c *CodexToClaudeConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})

	if model := getStringField(input, "model"); model != "" {
		output["model"] = model
	}

	// Convert input items -> Claude messages + system
	messages, systemPrompt, err := c.convertInputToMessages(input)
	if err != nil {
		return nil, fmt.Errorf("failed to convert codex input: %w", err)
	}
	if len(messages) > 0 {
		output["messages"] = messages
	}
	if systemPrompt != "" {
		output["system"] = systemPrompt
	}

	// Instructions -> system prompt (append)
	if instructions := getStringField(input, "instructions"); instructions != "" {
		if existing := getStringField(output, "system"); existing != "" {
			output["system"] = existing + "\n" + instructions
		} else {
			output["system"] = instructions
		}
	}

	// Parameters
	if maxTokens := getIntField(input, "max_output_tokens"); maxTokens > 0 {
		output["max_tokens"] = maxTokens
	} else if maxTokens := getIntField(input, "max_tokens"); maxTokens > 0 {
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

	// Convert tools
	if tools := getArrayField(input, "tools"); tools != nil {
		convertedTools := convertCodexToolsToClaude(tools)
		if len(convertedTools) > 0 {
			output["tools"] = convertedTools
		}
	}

	return output, nil
}

// ConvertResponse converts a Claude Messages API response to Codex Response API format
func (c *CodexToClaudeConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	return claudeResponseToCodex(response)
}

// ConvertStreamChunk converts Claude SSE events to Codex SSE events
func (c *CodexToClaudeConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	return convertClaudeStreamToCodex(chunk)
}

// --- Codex <-> OpenAI ---

// CodexToOpenAIConverter converts Codex requests to OpenAI format and OpenAI responses back to Codex
type CodexToOpenAIConverter struct{}

// ConvertRequest converts Codex Response API request to OpenAI Chat Completions request
func (c *CodexToOpenAIConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})

	if model := getStringField(input, "model"); model != "" {
		output["model"] = model
	}

	messages, systemPrompt, err := (&CodexToClaudeConverter{}).convertInputToMessages(input)
	if err != nil {
		return nil, err
	}

	// For OpenAI, system goes as first message
	var openaiMessages []interface{}
	if systemPrompt != "" {
		openaiMessages = append(openaiMessages, map[string]interface{}{
			"role":    "system",
			"content": systemPrompt,
		})
	}
	if instructions := getStringField(input, "instructions"); instructions != "" {
		openaiMessages = append(openaiMessages, map[string]interface{}{
			"role":    "system",
			"content": instructions,
		})
	}

	// Convert Claude-style messages to OpenAI style
	for _, msg := range messages {
		m, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		role := getStringField(m, "role")
		content := m["content"]

		// Claude content blocks -> OpenAI
		if arr, ok := content.([]interface{}); ok {
			// Check for tool_result blocks (need to become tool messages)
			for _, item := range arr {
				block, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				if getStringField(block, "type") == "tool_result" {
					openaiMessages = append(openaiMessages, map[string]interface{}{
						"role":         "tool",
						"tool_call_id": getStringField(block, "tool_use_id"),
						"content":      getStringField(block, "content"),
					})
				}
			}
			// Extract text content
			var texts []string
			for _, item := range arr {
				block, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				if getStringField(block, "type") == "text" {
					texts = append(texts, getStringField(block, "text"))
				}
			}
			if len(texts) > 0 {
				openaiMessages = append(openaiMessages, map[string]interface{}{
					"role":    role,
					"content": strings.Join(texts, ""),
				})
			}
		} else {
			openaiMessages = append(openaiMessages, m)
		}
	}

	output["messages"] = openaiMessages

	if maxTokens := getIntField(input, "max_output_tokens"); maxTokens > 0 {
		output["max_tokens"] = maxTokens
	}
	if temperature, ok := input["temperature"].(float64); ok {
		output["temperature"] = temperature
	}
	if stream := getBoolField(input, "stream"); stream {
		output["stream"] = true
	}

	return output, nil
}

// ConvertResponse converts an OpenAI Chat Completions response to Codex Response API format
func (c *CodexToOpenAIConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	return openaiResponseToCodex(response)
}

// ConvertStreamChunk converts OpenAI SSE chunks to Codex SSE events
func (c *CodexToOpenAIConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	return convertOpenAIStreamToCodex(chunk)
}

// --- Codex <-> Gemini ---

// CodexToGeminiConverter converts Codex requests to Gemini format
type CodexToGeminiConverter struct{}

// ConvertRequest converts Codex to Gemini via intermediate Claude conversion
func (c *CodexToGeminiConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	// Codex -> Claude -> Gemini
	claude, err := (&CodexToClaudeConverter{}).ConvertRequest(input)
	if err != nil {
		return nil, err
	}
	return (&ClaudeToGeminiConverter{}).ConvertRequest(claude)
}

// ConvertResponse converts Gemini response to Codex via intermediate Claude
func (c *CodexToGeminiConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	// Gemini -> Claude -> Codex
	claude, err := (&GeminiToClaudeConverter{}).ConvertResponse(response)
	if err != nil {
		return nil, err
	}
	return claudeResponseToCodex(claude)
}

// ConvertStreamChunk converts Gemini stream chunks to Codex events
func (c *CodexToGeminiConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	// Passthrough for now; Gemini streaming to Codex is rarely used
	return chunk, nil
}

// ===== Shared Codex conversion helpers =====

// convertInputToMessages converts Codex "input" field to Claude messages.
// Returns (messages, systemPrompt, error).
func (c *CodexToClaudeConverter) convertInputToMessages(input map[string]interface{}) ([]interface{}, string, error) {
	var messages []interface{}
	var systemPrompt string

	rawInput := input["input"]
	if rawInput == nil {
		return messages, systemPrompt, nil
	}

	switch v := rawInput.(type) {
	case string:
		// Simple string input -> single user message
		messages = append(messages, map[string]interface{}{
			"role":    "user",
			"content": v,
		})
	case []interface{}:
		// Array of items
		for _, item := range v {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			itemType := getStringField(itemMap, "type")
			role := getStringField(itemMap, "role")

			switch itemType {
			case "message":
				if role == "system" {
					// Extract system content
					if content := itemMap["content"]; content != nil {
						text := extractTextFromCodexContent(content)
						if text != "" {
							if systemPrompt != "" {
								systemPrompt += "\n"
							}
							systemPrompt += text
						}
					}
				} else {
					msg := map[string]interface{}{
						"role": role,
					}
					if content := itemMap["content"]; content != nil {
						msg["content"] = convertCodexContentToClaude(content)
					}
					messages = append(messages, msg)
				}
			case "function_call_output":
				// Tool result
				messages = append(messages, map[string]interface{}{
					"role": "user",
					"content": []interface{}{
						map[string]interface{}{
							"type":        "tool_result",
							"tool_use_id": getStringField(itemMap, "call_id"),
							"content":     getStringField(itemMap, "output"),
						},
					},
				})
			default:
				// Generic item, try to use as user message
				if role != "" {
					msg := map[string]interface{}{
						"role": role,
					}
					if content := itemMap["content"]; content != nil {
						msg["content"] = convertCodexContentToClaude(content)
					}
					messages = append(messages, msg)
				}
			}
		}
	}

	return messages, systemPrompt, nil
}

// extractTextFromCodexContent extracts text from Codex content (string or array of parts)
func extractTextFromCodexContent(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var texts []string
		for _, part := range v {
			if m, ok := part.(map[string]interface{}); ok {
				if getStringField(m, "type") == "input_text" || getStringField(m, "type") == "output_text" || getStringField(m, "type") == "text" {
					texts = append(texts, getStringField(m, "text"))
				}
			}
		}
		return strings.Join(texts, "")
	}
	return ""
}

// convertCodexContentToClaude converts Codex content parts to Claude content format
func convertCodexContentToClaude(content interface{}) interface{} {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		result := make([]interface{}, 0, len(v))
		for _, part := range v {
			m, ok := part.(map[string]interface{})
			if !ok {
				continue
			}
			partType := getStringField(m, "type")
			switch partType {
			case "input_text", "output_text", "text":
				result = append(result, map[string]interface{}{
					"type": "text",
					"text": getStringField(m, "text"),
				})
			case "input_image":
				if source := getMapField(m, "source"); source != nil {
					result = append(result, map[string]interface{}{
						"type": "image",
						"source": map[string]interface{}{
							"type":       "base64",
							"media_type": getStringField(source, "media_type"),
							"data":       getStringField(source, "data"),
						},
					})
				} else if url := getStringField(m, "image_url"); url != "" {
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
		if len(result) == 0 {
			return ""
		}
		return result
	}
	return ""
}

// convertCodexToolsToClaude converts Codex tool definitions to Claude format
func convertCodexToolsToClaude(tools []interface{}) []interface{} {
	result := make([]interface{}, 0, len(tools))
	for _, tool := range tools {
		t, ok := tool.(map[string]interface{})
		if !ok {
			continue
		}
		toolType := getStringField(t, "type")
		if toolType == "function" {
			claudeTool := map[string]interface{}{
				"name": getStringField(t, "name"),
			}
			if desc := getStringField(t, "description"); desc != "" {
				claudeTool["description"] = desc
			}
			if params := getMapField(t, "parameters"); params != nil {
				claudeTool["input_schema"] = params
			}
			result = append(result, claudeTool)
		}
	}
	return result
}

// claudeResponseToCodex converts a Claude response to Codex format
func claudeResponseToCodex(response map[string]interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})

	output["id"] = getStringField(response, "id")
	output["object"] = "response"
	output["created_at"] = getCurrentTimestamp()

	if model := getStringField(response, "model"); model != "" {
		output["model"] = model
	}

	// Build output items from Claude content blocks
	var outputItems []interface{}
	itemIdx := 0

	if contentArray := getArrayField(response, "content"); contentArray != nil {
		for _, item := range contentArray {
			block, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			blockType := getStringField(block, "type")
			switch blockType {
			case "text":
				outputItems = append(outputItems, map[string]interface{}{
					"type":    "message",
					"id":      fmt.Sprintf("msg_%d", itemIdx),
					"role":    "assistant",
					"status":  "completed",
					"content": []interface{}{
						map[string]interface{}{
							"type": "output_text",
							"text": getStringField(block, "text"),
						},
					},
				})
				itemIdx++
			case "tool_use":
				argsStr := "{}"
				if args := block["input"]; args != nil {
					if b, err := json.Marshal(args); err == nil {
						argsStr = string(b)
					}
				}
				outputItems = append(outputItems, map[string]interface{}{
					"type":      "function_call",
					"id":        getStringField(block, "id"),
					"call_id":   getStringField(block, "id"),
					"name":      getStringField(block, "name"),
					"arguments": argsStr,
					"status":    "completed",
				})
				itemIdx++
			}
		}
	}

	output["output"] = outputItems

	// Convert stop_reason -> status
	stopReason := getStringField(response, "stop_reason")
	switch stopReason {
	case "end_turn":
		output["status"] = "completed"
	case "max_tokens":
		output["status"] = "incomplete"
		output["incomplete_details"] = map[string]interface{}{
			"reason": "max_output_tokens",
		}
	case "tool_use":
		output["status"] = "completed"
	default:
		output["status"] = "completed"
	}

	// Usage
	if usage := getMapField(response, "usage"); usage != nil {
		output["usage"] = map[string]interface{}{
			"input_tokens":  getIntField(usage, "input_tokens"),
			"output_tokens": getIntField(usage, "output_tokens"),
			"total_tokens":  getIntField(usage, "input_tokens") + getIntField(usage, "output_tokens"),
		}
	}

	return output, nil
}

// openaiResponseToCodex converts an OpenAI response to Codex format
func openaiResponseToCodex(response map[string]interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})

	output["id"] = getStringField(response, "id")
	output["object"] = "response"
	output["created_at"] = getCurrentTimestamp()

	if model := getStringField(response, "model"); model != "" {
		output["model"] = model
	}

	var outputItems []interface{}

	if choices := getArrayField(response, "choices"); choices != nil && len(choices) > 0 {
		choice, ok := choices[0].(map[string]interface{})
		if ok {
			msg := getMapField(choice, "message")
			if msg != nil {
				content := getStringField(msg, "content")
				if content != "" {
					outputItems = append(outputItems, map[string]interface{}{
						"type":   "message",
						"id":     "msg_0",
						"role":   "assistant",
						"status": "completed",
						"content": []interface{}{
							map[string]interface{}{
								"type": "output_text",
								"text": content,
							},
						},
					})
				}

				// Tool calls
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
						outputItems = append(outputItems, map[string]interface{}{
							"type":      "function_call",
							"id":        getStringField(tcMap, "id"),
							"call_id":   getStringField(tcMap, "id"),
							"name":      getStringField(fn, "name"),
							"arguments": getStringField(fn, "arguments"),
							"status":    "completed",
						})
					}
				}
			}

			finishReason := getStringField(choice, "finish_reason")
			switch finishReason {
			case "stop":
				output["status"] = "completed"
			case "length":
				output["status"] = "incomplete"
				output["incomplete_details"] = map[string]interface{}{
					"reason": "max_output_tokens",
				}
			case "tool_calls":
				output["status"] = "completed"
			default:
				output["status"] = "completed"
			}
		}
	}

	output["output"] = outputItems

	// Usage
	if usage := getMapField(response, "usage"); usage != nil {
		output["usage"] = map[string]interface{}{
			"input_tokens":  getIntField(usage, "prompt_tokens"),
			"output_tokens": getIntField(usage, "completion_tokens"),
			"total_tokens":  getIntField(usage, "total_tokens"),
		}
	}

	return output, nil
}

// convertClaudeStreamToCodex converts Claude SSE events to Codex-style SSE events
func convertClaudeStreamToCodex(chunk []byte) ([]byte, error) {
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

	var events []map[string]interface{}

	switch eventType {
	case "message_start":
		events = append(events, map[string]interface{}{
			"type": "response.created",
			"response": map[string]interface{}{
				"id":     getStringField(getMapField(data, "message"), "id"),
				"status": "in_progress",
				"output": []interface{}{},
			},
		})

	case "content_block_start":
		cb := getMapField(data, "content_block")
		if cb != nil {
			idx := getIntField(data, "index")
			cbType := getStringField(cb, "type")
			if cbType == "text" {
				events = append(events, map[string]interface{}{
					"type":          "response.output_item.added",
					"output_index":  idx,
					"item": map[string]interface{}{
						"type":   "message",
						"id":     fmt.Sprintf("msg_%d", idx),
						"role":   "assistant",
						"status": "in_progress",
						"content": []interface{}{},
					},
				})
			} else if cbType == "tool_use" {
				events = append(events, map[string]interface{}{
					"type":          "response.output_item.added",
					"output_index":  idx,
					"item": map[string]interface{}{
						"type":      "function_call",
						"id":        getStringField(cb, "id"),
						"call_id":   getStringField(cb, "id"),
						"name":      getStringField(cb, "name"),
						"arguments": "",
						"status":    "in_progress",
					},
				})
			}
		}

	case "content_block_delta":
		delta := getMapField(data, "delta")
		if delta != nil {
			deltaType := getStringField(delta, "type")
			idx := getIntField(data, "index")
			switch deltaType {
			case "text_delta":
				events = append(events, map[string]interface{}{
					"type":          "response.output_text.delta",
					"output_index":  idx,
					"content_index": 0,
					"delta":         getStringField(delta, "text"),
				})
			case "input_json_delta":
				events = append(events, map[string]interface{}{
					"type":          "response.function_call_arguments.delta",
					"output_index":  idx,
					"delta":         getStringField(delta, "partial_json"),
				})
			}
		}

	case "content_block_stop":
		idx := getIntField(data, "index")
		events = append(events, map[string]interface{}{
			"type":         "response.output_item.done",
			"output_index": idx,
		})

	case "message_stop":
		events = append(events, map[string]interface{}{
			"type": "response.completed",
		})

	case "message_delta":
		// Usage or stop reason in delta
		delta := getMapField(data, "delta")
		if delta != nil && getStringField(delta, "stop_reason") != "" {
			// No specific Codex event for this; will be captured in response.completed
		}

	default:
		return chunk, nil
	}

	if len(events) == 0 {
		return chunk, nil
	}

	var buf strings.Builder
	for _, evt := range events {
		evtJSON := mustMarshal(evt)
		buf.WriteString(fmt.Sprintf("event: %s\ndata: %s\n\n", getStringField(evt, "type"), evtJSON))
	}
	return []byte(buf.String()), nil
}

// convertOpenAIStreamToCodex converts OpenAI SSE chunks to Codex events
func convertOpenAIStreamToCodex(chunk []byte) ([]byte, error) {
	chunkStr := strings.TrimSpace(string(chunk))
	if !strings.HasPrefix(chunkStr, "data: ") {
		return chunk, nil
	}

	dataStr := strings.TrimPrefix(chunkStr, "data: ")
	if dataStr == "[DONE]" {
		evt := map[string]interface{}{
			"type": "response.completed",
		}
		return []byte(fmt.Sprintf("event: response.completed\ndata: %s\n\n", mustMarshal(evt))), nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return chunk, nil
	}

	choices := getArrayField(data, "choices")
	if choices == nil || len(choices) == 0 {
		return chunk, nil
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return chunk, nil
	}

	delta := getMapField(choice, "delta")
	if delta == nil {
		return chunk, nil
	}

	var events []map[string]interface{}

	if content := getStringField(delta, "content"); content != "" {
		events = append(events, map[string]interface{}{
			"type":          "response.output_text.delta",
			"output_index":  0,
			"content_index": 0,
			"delta":         content,
		})
	}

	if finishReason := getStringField(choice, "finish_reason"); finishReason != "" {
		events = append(events, map[string]interface{}{
			"type":         "response.output_item.done",
			"output_index": 0,
		})
	}

	if len(events) == 0 {
		return chunk, nil
	}

	var buf strings.Builder
	for _, evt := range events {
		evtJSON := mustMarshal(evt)
		buf.WriteString(fmt.Sprintf("event: %s\ndata: %s\n\n", getStringField(evt, "type"), evtJSON))
	}
	return []byte(buf.String()), nil
}
