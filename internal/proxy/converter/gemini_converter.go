package converter

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GeminiConverter converts between Gemini API format and Claude/OpenAI formats.
//
// Gemini API key differences:
//   - Request: contents[].parts[] instead of messages[].content
//   - Response: candidates[].content.parts[]
//   - Roles: "user" and "model" (not "assistant")
//   - System prompt: systemInstruction.parts[]
//   - Generation config: temperature, topP, topK, maxOutputTokens, stopSequences
//   - Usage: usageMetadata with promptTokenCount, candidatesTokenCount, totalTokenCount

// --- Gemini <-> Claude ---

// GeminiToClaudeConverter converts Gemini requests to Claude and Claude responses to Gemini
type GeminiToClaudeConverter struct{}

// ConvertRequest converts a Gemini request to Claude Messages API format
func (c *GeminiToClaudeConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})

	// Model may be in the URL path, not the body. Pass through if present.
	if model := getStringField(input, "model"); model != "" {
		output["model"] = model
	}

	// Convert systemInstruction -> system
	if sysInstr := getMapField(input, "systemInstruction"); sysInstr != nil {
		parts := getArrayField(sysInstr, "parts")
		var texts []string
		for _, part := range parts {
			if pm, ok := part.(map[string]interface{}); ok {
				if text := getStringField(pm, "text"); text != "" {
					texts = append(texts, text)
				}
			}
		}
		if len(texts) > 0 {
			output["system"] = strings.Join(texts, "\n")
		}
	}

	// Convert contents -> messages
	if contents := getArrayField(input, "contents"); contents != nil {
		messages, err := c.convertContentsToMessages(contents)
		if err != nil {
			return nil, fmt.Errorf("failed to convert gemini contents: %w", err)
		}
		output["messages"] = messages
	}

	// Convert generationConfig
	if config := getMapField(input, "generationConfig"); config != nil {
		if maxTokens := getIntField(config, "maxOutputTokens"); maxTokens > 0 {
			output["max_tokens"] = maxTokens
		}
		if temp, ok := config["temperature"].(float64); ok {
			output["temperature"] = temp
		}
		if topP, ok := config["topP"].(float64); ok {
			output["top_p"] = topP
		}
		if topK := getIntField(config, "topK"); topK > 0 {
			output["top_k"] = topK
		}
		if stopSeqs := getArrayField(config, "stopSequences"); stopSeqs != nil {
			var seqs []string
			for _, s := range stopSeqs {
				if str, ok := s.(string); ok {
					seqs = append(seqs, str)
				}
			}
			if len(seqs) > 0 {
				output["stop_sequences"] = seqs
			}
		}
	}

	// Convert tools: Gemini functionDeclarations -> Claude tools
	if tools := getArrayField(input, "tools"); tools != nil {
		convertedTools := convertGeminiToolsToClaude(tools)
		if len(convertedTools) > 0 {
			output["tools"] = convertedTools
		}
	}

	return output, nil
}

// ConvertResponse converts a Claude Messages API response to Gemini format
func (c *GeminiToClaudeConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	return claudeResponseToGemini(response)
}

// ConvertStreamChunk converts Claude SSE events to Gemini SSE chunks
func (c *GeminiToClaudeConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	return convertClaudeStreamToGemini(chunk)
}

// --- Gemini <-> OpenAI ---

// GeminiToOpenAIConverter converts Gemini requests to OpenAI and OpenAI responses to Gemini
type GeminiToOpenAIConverter struct{}

// ConvertRequest converts Gemini request to OpenAI Chat Completions format
func (c *GeminiToOpenAIConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	// Gemini -> Claude -> OpenAI
	claude, err := (&GeminiToClaudeConverter{}).ConvertRequest(input)
	if err != nil {
		return nil, err
	}
	return (&ClaudeToOpenAIConverter{}).ConvertRequest(claude)
}

// ConvertResponse converts OpenAI response to Gemini format
func (c *GeminiToOpenAIConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	return openaiResponseToGemini(response)
}

// ConvertStreamChunk converts OpenAI SSE chunks to Gemini-style data
func (c *GeminiToOpenAIConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	return convertOpenAIStreamToGemini(chunk)
}

// --- Claude <-> Gemini (Claude client, Gemini provider) ---

// ClaudeToGeminiConverter converts Claude requests to Gemini format
type ClaudeToGeminiConverter struct{}

// ConvertRequest converts Claude Messages API request to Gemini format
func (c *ClaudeToGeminiConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})

	if model := getStringField(input, "model"); model != "" {
		output["model"] = model
	}

	// Convert system prompt -> systemInstruction
	if system := input["system"]; system != nil {
		sysText := extractSystemText(system)
		if sysText != "" {
			output["systemInstruction"] = map[string]interface{}{
				"parts": []interface{}{
					map[string]interface{}{"text": sysText},
				},
			}
		}
	}

	// Convert messages -> contents
	if messages := getArrayField(input, "messages"); messages != nil {
		contents, err := convertMessagesToGeminiContents(messages)
		if err != nil {
			return nil, err
		}
		output["contents"] = contents
	}

	// Convert parameters -> generationConfig
	genConfig := make(map[string]interface{})
	if maxTokens := getIntField(input, "max_tokens"); maxTokens > 0 {
		genConfig["maxOutputTokens"] = maxTokens
	}
	if temp, ok := input["temperature"].(float64); ok {
		genConfig["temperature"] = temp
	}
	if topP, ok := input["top_p"].(float64); ok {
		genConfig["topP"] = topP
	}
	if topK := getIntField(input, "top_k"); topK > 0 {
		genConfig["topK"] = topK
	}
	if stopSeqs := getArrayField(input, "stop_sequences"); stopSeqs != nil {
		genConfig["stopSequences"] = stopSeqs
	}
	if len(genConfig) > 0 {
		output["generationConfig"] = genConfig
	}

	// Convert tools -> Gemini tools with functionDeclarations
	if tools := getArrayField(input, "tools"); tools != nil {
		geminiTools := convertClaudeToolsToGemini(tools)
		if len(geminiTools) > 0 {
			output["tools"] = geminiTools
		}
	}

	return output, nil
}

// ConvertResponse converts Gemini response to Claude Messages API format
func (c *ClaudeToGeminiConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	return geminiResponseToClaude(response)
}

// ConvertStreamChunk converts Gemini SSE to Claude SSE events
func (c *ClaudeToGeminiConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	return convertGeminiStreamToClaude(chunk)
}

// --- OpenAI <-> Gemini (OpenAI client, Gemini provider) ---

// OpenAIToGeminiConverter converts OpenAI requests to Gemini format
type OpenAIToGeminiConverter struct{}

// ConvertRequest converts OpenAI Chat Completions request to Gemini format
func (c *OpenAIToGeminiConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	// OpenAI -> Claude -> Gemini
	claude, err := (&OpenAIToClaudeConverter{}).ConvertRequest(input)
	if err != nil {
		return nil, err
	}
	return (&ClaudeToGeminiConverter{}).ConvertRequest(claude)
}

// ConvertResponse converts Gemini response to OpenAI format
func (c *OpenAIToGeminiConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	claude, err := geminiResponseToClaude(response)
	if err != nil {
		return nil, err
	}
	return (&OpenAIToClaudeConverter{}).ConvertResponse(claude)
}

// ConvertStreamChunk converts Gemini SSE to OpenAI SSE
func (c *OpenAIToGeminiConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	return convertGeminiStreamToOpenAI(chunk)
}

// ===== Shared Gemini conversion helpers =====

// convertContentsToMessages converts Gemini contents[] to Claude messages[]
func (c *GeminiToClaudeConverter) convertContentsToMessages(contents []interface{}) ([]interface{}, error) {
	messages := make([]interface{}, 0, len(contents))

	for _, item := range contents {
		content, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		role := getStringField(content, "role")
		// Map Gemini "model" -> Claude "assistant"
		claudeRole := role
		if role == "model" {
			claudeRole = "assistant"
		}

		parts := getArrayField(content, "parts")
		if parts == nil {
			continue
		}

		claudeContent := convertGeminiPartsToClaude(parts)
		messages = append(messages, map[string]interface{}{
			"role":    claudeRole,
			"content": claudeContent,
		})
	}

	return messages, nil
}

// convertGeminiPartsToClaude converts Gemini parts[] to Claude content
func convertGeminiPartsToClaude(parts []interface{}) interface{} {
	if len(parts) == 0 {
		return ""
	}

	// Single text-only part -> return string
	if len(parts) == 1 {
		if pm, ok := parts[0].(map[string]interface{}); ok {
			if text := getStringField(pm, "text"); text != "" && pm["inlineData"] == nil && pm["functionCall"] == nil {
				return text
			}
		}
	}

	result := make([]interface{}, 0, len(parts))
	for _, part := range parts {
		pm, ok := part.(map[string]interface{})
		if !ok {
			continue
		}

		if text := getStringField(pm, "text"); text != "" {
			result = append(result, map[string]interface{}{
				"type": "text",
				"text": text,
			})
		}

		if inlineData := getMapField(pm, "inlineData"); inlineData != nil {
			result = append(result, map[string]interface{}{
				"type": "image",
				"source": map[string]interface{}{
					"type":       "base64",
					"media_type": getStringField(inlineData, "mimeType"),
					"data":       getStringField(inlineData, "data"),
				},
			})
		}

		if fnCall := getMapField(pm, "functionCall"); fnCall != nil {
			result = append(result, map[string]interface{}{
				"type":  "tool_use",
				"id":    "toolu_" + generateID(),
				"name":  getStringField(fnCall, "name"),
				"input": fnCall["args"],
			})
		}

		if fnResp := getMapField(pm, "functionResponse"); fnResp != nil {
			respContent := ""
			if resp := fnResp["response"]; resp != nil {
				if b, err := json.Marshal(resp); err == nil {
					respContent = string(b)
				}
			}
			result = append(result, map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": "toolu_" + generateID(),
				"content":     respContent,
			})
		}
	}

	if len(result) == 0 {
		return ""
	}
	return result
}

// convertMessagesToGeminiContents converts Claude messages to Gemini contents
func convertMessagesToGeminiContents(messages []interface{}) ([]interface{}, error) {
	contents := make([]interface{}, 0, len(messages))

	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}

		role := getStringField(msgMap, "role")
		// Map Claude "assistant" -> Gemini "model"
		geminiRole := role
		if role == "assistant" {
			geminiRole = "model"
		}
		// Skip system messages (handled separately)
		if role == "system" {
			continue
		}

		parts := convertClaudeContentToGeminiParts(msgMap["content"])
		if len(parts) > 0 {
			contents = append(contents, map[string]interface{}{
				"role":  geminiRole,
				"parts": parts,
			})
		}
	}

	return contents, nil
}

// convertClaudeContentToGeminiParts converts Claude content (string or blocks) to Gemini parts
func convertClaudeContentToGeminiParts(content interface{}) []interface{} {
	if content == nil {
		return nil
	}

	switch v := content.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []interface{}{
			map[string]interface{}{"text": v},
		}
	case []interface{}:
		parts := make([]interface{}, 0, len(v))
		for _, item := range v {
			block, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			blockType := getStringField(block, "type")
			switch blockType {
			case "text":
				parts = append(parts, map[string]interface{}{
					"text": getStringField(block, "text"),
				})
			case "image":
				if source := getMapField(block, "source"); source != nil {
					parts = append(parts, map[string]interface{}{
						"inlineData": map[string]interface{}{
							"mimeType": getStringField(source, "media_type"),
							"data":     getStringField(source, "data"),
						},
					})
				}
			case "tool_use":
				parts = append(parts, map[string]interface{}{
					"functionCall": map[string]interface{}{
						"name": getStringField(block, "name"),
						"args": block["input"],
					},
				})
			case "tool_result":
				var resp interface{}
				if c := block["content"]; c != nil {
					if str, ok := c.(string); ok {
						var parsed interface{}
						if err := json.Unmarshal([]byte(str), &parsed); err == nil {
							resp = parsed
						} else {
							resp = map[string]interface{}{"result": str}
						}
					}
				}
				parts = append(parts, map[string]interface{}{
					"functionResponse": map[string]interface{}{
						"name":     getStringField(block, "name"),
						"response": resp,
					},
				})
			case "thinking":
				// Gemini does not have a thinking equivalent; skip or embed as text
			}
		}
		return parts
	}
	return nil
}

// extractSystemText extracts text from Claude system (string or array)
func extractSystemText(system interface{}) string {
	switch v := system.(type) {
	case string:
		return v
	case []interface{}:
		var texts []string
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if text := getStringField(m, "text"); text != "" {
					texts = append(texts, text)
				}
			}
		}
		return strings.Join(texts, "\n")
	}
	return ""
}

// convertGeminiToolsToClaude converts Gemini tools to Claude format
func convertGeminiToolsToClaude(tools []interface{}) []interface{} {
	result := make([]interface{}, 0)
	for _, tool := range tools {
		t, ok := tool.(map[string]interface{})
		if !ok {
			continue
		}
		funcDecls := getArrayField(t, "functionDeclarations")
		for _, fd := range funcDecls {
			fdMap, ok := fd.(map[string]interface{})
			if !ok {
				continue
			}
			claudeTool := map[string]interface{}{
				"name": getStringField(fdMap, "name"),
			}
			if desc := getStringField(fdMap, "description"); desc != "" {
				claudeTool["description"] = desc
			}
			if params := getMapField(fdMap, "parameters"); params != nil {
				claudeTool["input_schema"] = params
			}
			result = append(result, claudeTool)
		}
	}
	return result
}

// convertClaudeToolsToGemini converts Claude tools to Gemini format
func convertClaudeToolsToGemini(tools []interface{}) []interface{} {
	funcDecls := make([]interface{}, 0, len(tools))
	for _, tool := range tools {
		t, ok := tool.(map[string]interface{})
		if !ok {
			continue
		}
		fd := map[string]interface{}{
			"name": getStringField(t, "name"),
		}
		if desc := getStringField(t, "description"); desc != "" {
			fd["description"] = desc
		}
		if schema := getMapField(t, "input_schema"); schema != nil {
			fd["parameters"] = schema
		}
		funcDecls = append(funcDecls, fd)
	}
	if len(funcDecls) == 0 {
		return nil
	}
	return []interface{}{
		map[string]interface{}{
			"functionDeclarations": funcDecls,
		},
	}
}

// claudeResponseToGemini converts a Claude response to Gemini format
func claudeResponseToGemini(response map[string]interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})

	// Build candidate from Claude content blocks
	var parts []interface{}
	if contentArray := getArrayField(response, "content"); contentArray != nil {
		for _, item := range contentArray {
			block, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			blockType := getStringField(block, "type")
			switch blockType {
			case "text":
				parts = append(parts, map[string]interface{}{
					"text": getStringField(block, "text"),
				})
			case "tool_use":
				parts = append(parts, map[string]interface{}{
					"functionCall": map[string]interface{}{
						"name": getStringField(block, "name"),
						"args": block["input"],
					},
				})
			}
		}
	}

	candidate := map[string]interface{}{
		"content": map[string]interface{}{
			"role":  "model",
			"parts": parts,
		},
	}

	// Map stop_reason -> finishReason
	if stopReason := getStringField(response, "stop_reason"); stopReason != "" {
		candidate["finishReason"] = claudeStopToGeminiFinish(stopReason)
	}

	output["candidates"] = []interface{}{candidate}

	// Usage
	if usage := getMapField(response, "usage"); usage != nil {
		output["usageMetadata"] = map[string]interface{}{
			"promptTokenCount":     getIntField(usage, "input_tokens"),
			"candidatesTokenCount": getIntField(usage, "output_tokens"),
			"totalTokenCount":      getIntField(usage, "input_tokens") + getIntField(usage, "output_tokens"),
		}
	}

	return output, nil
}

// geminiResponseToClaude converts a Gemini response to Claude Messages API format
func geminiResponseToClaude(response map[string]interface{}) (map[string]interface{}, error) {
	// Handle response wrapping: some providers return {response: {...}}
	if wrapped := getMapField(response, "response"); wrapped != nil {
		response = wrapped
	}

	output := make(map[string]interface{})
	output["id"] = "msg_gemini_" + generateID()
	output["type"] = "message"
	output["role"] = "assistant"

	if model := getStringField(response, "model"); model != "" {
		output["model"] = model
	}

	// Extract content from candidates
	var contentBlocks []interface{}
	candidates := getArrayField(response, "candidates")
	if candidates != nil && len(candidates) > 0 {
		candidate, ok := candidates[0].(map[string]interface{})
		if ok {
			content := getMapField(candidate, "content")
			if content != nil {
				parts := getArrayField(content, "parts")
				converted := convertGeminiPartsToClaude(parts)
				switch v := converted.(type) {
				case []interface{}:
					contentBlocks = v
				case string:
					if v != "" {
						contentBlocks = []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": v,
							},
						}
					}
				}
			}

			// Map finishReason -> stop_reason
			if fr := getStringField(candidate, "finishReason"); fr != "" {
				output["stop_reason"] = geminiFinishToClaudeStop(fr)
			}
		}
	}

	if contentBlocks == nil {
		contentBlocks = []interface{}{}
	}

	output["content"] = contentBlocks

	// Usage
	usageData := getMapField(response, "usageMetadata")
	if usageData == nil {
		usageData = getMapField(response, "usage")
	}
	if usageData != nil {
		output["usage"] = map[string]interface{}{
			"input_tokens":  getIntField(usageData, "promptTokenCount"),
			"output_tokens": getIntField(usageData, "candidatesTokenCount"),
		}
	}

	return output, nil
}

// openaiResponseToGemini converts an OpenAI response to Gemini format
func openaiResponseToGemini(response map[string]interface{}) (map[string]interface{}, error) {
	output := make(map[string]interface{})

	var parts []interface{}
	if choices := getArrayField(response, "choices"); choices != nil && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if msg := getMapField(choice, "message"); msg != nil {
				if content := getStringField(msg, "content"); content != "" {
					parts = append(parts, map[string]interface{}{
						"text": content,
					})
				}
			}
			finishReason := getStringField(choice, "finish_reason")
			output["candidates"] = []interface{}{
				map[string]interface{}{
					"content": map[string]interface{}{
						"role":  "model",
						"parts": parts,
					},
					"finishReason": openAIFinishToGemini(finishReason),
				},
			}
		}
	}

	if usage := getMapField(response, "usage"); usage != nil {
		output["usageMetadata"] = map[string]interface{}{
			"promptTokenCount":     getIntField(usage, "prompt_tokens"),
			"candidatesTokenCount": getIntField(usage, "completion_tokens"),
			"totalTokenCount":      getIntField(usage, "total_tokens"),
		}
	}

	return output, nil
}

// Streaming helpers for Gemini

// convertClaudeStreamToGemini converts Claude SSE events to Gemini format (JSON array style)
func convertClaudeStreamToGemini(chunk []byte) ([]byte, error) {
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

	switch eventType {
	case "content_block_delta":
		delta := getMapField(data, "delta")
		if delta != nil && getStringField(delta, "type") == "text_delta" {
			text := getStringField(delta, "text")
			geminiResp := map[string]interface{}{
				"candidates": []interface{}{
					map[string]interface{}{
						"content": map[string]interface{}{
							"role": "model",
							"parts": []interface{}{
								map[string]interface{}{"text": text},
							},
						},
					},
				},
			}
			return []byte(fmt.Sprintf("data: %s\n\n", mustMarshal(geminiResp))), nil
		}

	case "message_delta":
		delta := getMapField(data, "delta")
		if delta != nil {
			if stopReason := getStringField(delta, "stop_reason"); stopReason != "" {
				geminiResp := map[string]interface{}{
					"candidates": []interface{}{
						map[string]interface{}{
							"content": map[string]interface{}{
								"role":  "model",
								"parts": []interface{}{},
							},
							"finishReason": claudeStopToGeminiFinish(stopReason),
						},
					},
				}
				if usage := getMapField(data, "usage"); usage != nil {
					geminiResp["usageMetadata"] = map[string]interface{}{
						"promptTokenCount":     getIntField(usage, "input_tokens"),
						"candidatesTokenCount": getIntField(usage, "output_tokens"),
						"totalTokenCount":      getIntField(usage, "input_tokens") + getIntField(usage, "output_tokens"),
					}
				}
				return []byte(fmt.Sprintf("data: %s\n\n", mustMarshal(geminiResp))), nil
			}
		}

	case "message_stop":
		// Gemini uses finishReason in the last data chunk, no separate stop event
		return nil, nil
	}

	return chunk, nil
}

// convertGeminiStreamToClaude converts Gemini SSE to Claude SSE events
func convertGeminiStreamToClaude(chunk []byte) ([]byte, error) {
	chunkStr := strings.TrimSpace(string(chunk))
	if !strings.HasPrefix(chunkStr, "data: ") {
		// Gemini may also send raw JSON (not SSE prefixed) in array streaming
		if strings.HasPrefix(chunkStr, "[") || strings.HasPrefix(chunkStr, "{") || strings.HasPrefix(chunkStr, ",") {
			chunkStr = strings.TrimLeft(chunkStr, "[,")
			chunkStr = strings.TrimRight(chunkStr, "]")
			chunkStr = strings.TrimSpace(chunkStr)
			if chunkStr == "" {
				return nil, nil
			}
		} else {
			return chunk, nil
		}
	} else {
		chunkStr = strings.TrimPrefix(chunkStr, "data: ")
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(chunkStr), &data); err != nil {
		return chunk, nil
	}

	candidates := getArrayField(data, "candidates")
	if candidates == nil || len(candidates) == 0 {
		return chunk, nil
	}

	candidate, ok := candidates[0].(map[string]interface{})
	if !ok {
		return chunk, nil
	}

	var events []string

	content := getMapField(candidate, "content")
	if content != nil {
		parts := getArrayField(content, "parts")
		for _, part := range parts {
			pm, ok := part.(map[string]interface{})
			if !ok {
				continue
			}
			if text := getStringField(pm, "text"); text != "" {
				evt := map[string]interface{}{
					"type": "content_block_delta",
					"delta": map[string]interface{}{
						"type": "text_delta",
						"text": text,
					},
				}
				events = append(events, fmt.Sprintf("event: content_block_delta\ndata: %s\n\n", mustMarshal(evt)))
			}
		}
	}

	if fr := getStringField(candidate, "finishReason"); fr != "" {
		evt := map[string]interface{}{
			"type": "message_delta",
			"delta": map[string]interface{}{
				"stop_reason": geminiFinishToClaudeStop(fr),
			},
		}
		events = append(events, fmt.Sprintf("event: message_delta\ndata: %s\n\n", mustMarshal(evt)))
	}

	if len(events) == 0 {
		return chunk, nil
	}

	return []byte(strings.Join(events, "")), nil
}

// convertGeminiStreamToOpenAI converts Gemini SSE to OpenAI SSE
func convertGeminiStreamToOpenAI(chunk []byte) ([]byte, error) {
	// First convert Gemini -> Claude, then Claude -> OpenAI would be complex,
	// so we do a direct conversion instead
	chunkStr := strings.TrimSpace(string(chunk))
	dataStr := chunkStr

	if strings.HasPrefix(chunkStr, "data: ") {
		dataStr = strings.TrimPrefix(chunkStr, "data: ")
	} else if strings.HasPrefix(chunkStr, "[") || strings.HasPrefix(chunkStr, "{") || strings.HasPrefix(chunkStr, ",") {
		dataStr = strings.TrimLeft(chunkStr, "[,")
		dataStr = strings.TrimRight(dataStr, "]")
		dataStr = strings.TrimSpace(dataStr)
		if dataStr == "" {
			return nil, nil
		}
	} else {
		return chunk, nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return chunk, nil
	}

	candidates := getArrayField(data, "candidates")
	if candidates == nil || len(candidates) == 0 {
		return chunk, nil
	}

	candidate, ok := candidates[0].(map[string]interface{})
	if !ok {
		return chunk, nil
	}

	content := getMapField(candidate, "content")
	if content == nil {
		return chunk, nil
	}

	parts := getArrayField(content, "parts")
	var text string
	for _, part := range parts {
		if pm, ok := part.(map[string]interface{}); ok {
			text += getStringField(pm, "text")
		}
	}

	finishReason := getStringField(candidate, "finishReason")
	var openaiFinish interface{}
	if finishReason != "" {
		openaiFinish = geminiFinishToOpenAI(finishReason)
	}

	openaiChunk := map[string]interface{}{
		"id":      "chatcmpl-" + generateID(),
		"object":  "chat.completion.chunk",
		"created": getCurrentTimestamp(),
		"choices": []interface{}{
			map[string]interface{}{
				"index": 0,
				"delta": map[string]interface{}{
					"content": text,
				},
				"finish_reason": openaiFinish,
			},
		},
	}

	// Usage in last chunk
	usageData := getMapField(data, "usageMetadata")
	if usageData != nil {
		openaiChunk["usage"] = map[string]interface{}{
			"prompt_tokens":     getIntField(usageData, "promptTokenCount"),
			"completion_tokens": getIntField(usageData, "candidatesTokenCount"),
			"total_tokens":      getIntField(usageData, "totalTokenCount"),
		}
	}

	return []byte(fmt.Sprintf("data: %s\n\n", mustMarshal(openaiChunk))), nil
}

// convertOpenAIStreamToGemini converts OpenAI SSE to Gemini format
func convertOpenAIStreamToGemini(chunk []byte) ([]byte, error) {
	chunkStr := strings.TrimSpace(string(chunk))
	if !strings.HasPrefix(chunkStr, "data: ") {
		return chunk, nil
	}

	dataStr := strings.TrimPrefix(chunkStr, "data: ")
	if dataStr == "[DONE]" {
		return nil, nil
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

	text := getStringField(delta, "content")
	geminiResp := map[string]interface{}{
		"candidates": []interface{}{
			map[string]interface{}{
				"content": map[string]interface{}{
					"role": "model",
					"parts": []interface{}{
						map[string]interface{}{"text": text},
					},
				},
			},
		},
	}

	if fr := getStringField(choice, "finish_reason"); fr != "" {
		geminiResp["candidates"].([]interface{})[0].(map[string]interface{})["finishReason"] = openAIFinishToGemini(fr)
	}

	return []byte(fmt.Sprintf("data: %s\n\n", mustMarshal(geminiResp))), nil
}

// ===== Stop/finish reason mapping =====

// claudeStopToGeminiFinish maps Claude stop_reason to Gemini finishReason
func claudeStopToGeminiFinish(reason string) string {
	switch reason {
	case "end_turn":
		return "STOP"
	case "max_tokens":
		return "MAX_TOKENS"
	case "stop_sequence":
		return "STOP"
	case "tool_use":
		return "STOP"
	default:
		return "STOP"
	}
}

// geminiFinishToClaudeStop maps Gemini finishReason to Claude stop_reason
func geminiFinishToClaudeStop(reason string) string {
	switch reason {
	case "STOP":
		return "end_turn"
	case "MAX_TOKENS":
		return "max_tokens"
	case "SAFETY":
		return "end_turn"
	default:
		return "end_turn"
	}
}

// geminiFinishToOpenAI maps Gemini finishReason to OpenAI finish_reason
func geminiFinishToOpenAI(reason string) string {
	switch reason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	default:
		return strings.ToLower(reason)
	}
}

// openAIFinishToGemini maps OpenAI finish_reason to Gemini finishReason
func openAIFinishToGemini(reason string) string {
	switch reason {
	case "stop":
		return "STOP"
	case "length":
		return "MAX_TOKENS"
	case "content_filter":
		return "SAFETY"
	case "tool_calls":
		return "STOP"
	default:
		return "STOP"
	}
}
