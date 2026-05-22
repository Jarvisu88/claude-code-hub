package converter

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GeminiCLIConverter handles the Gemini CLI internal format.
//
// Gemini CLI uses /v1internal/ endpoints and wraps the standard Gemini request
// in a {request: {...}} envelope. The systemInstruction may require role: "user".
// Response may be wrapped in {response: {...}}.

// --- GeminiCLI <-> Claude ---

// GeminiCLIToClaudeConverter converts Gemini CLI requests to Claude format
type GeminiCLIToClaudeConverter struct{}

// ConvertRequest unwraps the CLI envelope and delegates to standard Gemini->Claude conversion
func (c *GeminiCLIToClaudeConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	inner := unwrapCLIRequest(input)
	return (&GeminiToClaudeConverter{}).ConvertRequest(inner)
}

// ConvertResponse converts Claude response to Gemini CLI format (wrapped)
func (c *GeminiCLIToClaudeConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	geminiResp, err := claudeResponseToGemini(response)
	if err != nil {
		return nil, err
	}
	return wrapCLIResponse(geminiResp), nil
}

// ConvertStreamChunk converts Claude SSE to Gemini CLI SSE
func (c *GeminiCLIToClaudeConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	return convertClaudeStreamToGeminiCLI(chunk)
}

// --- GeminiCLI <-> OpenAI ---

// GeminiCLIToOpenAIConverter converts Gemini CLI requests to OpenAI format
type GeminiCLIToOpenAIConverter struct{}

// ConvertRequest unwraps CLI envelope, then Gemini->OpenAI
func (c *GeminiCLIToOpenAIConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	inner := unwrapCLIRequest(input)
	return (&GeminiToOpenAIConverter{}).ConvertRequest(inner)
}

// ConvertResponse converts OpenAI response to Gemini CLI format
func (c *GeminiCLIToOpenAIConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	geminiResp, err := openaiResponseToGemini(response)
	if err != nil {
		return nil, err
	}
	return wrapCLIResponse(geminiResp), nil
}

// ConvertStreamChunk converts OpenAI SSE to Gemini CLI SSE
func (c *GeminiCLIToOpenAIConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	return convertOpenAIStreamToGeminiCLI(chunk)
}

// --- GeminiCLI <-> Codex ---

// GeminiCLIToCodexConverter converts Gemini CLI requests to Codex format
type GeminiCLIToCodexConverter struct{}

// ConvertRequest converts Gemini CLI -> Claude -> Codex
func (c *GeminiCLIToCodexConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	inner := unwrapCLIRequest(input)
	claude, err := (&GeminiToClaudeConverter{}).ConvertRequest(inner)
	if err != nil {
		return nil, err
	}
	return (&ClaudeToCodexConverter{}).ConvertRequest(claude)
}

// ConvertResponse converts Codex response to Gemini CLI format
func (c *GeminiCLIToCodexConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	// Codex -> OpenAI-like intermediate -> Gemini
	geminiResp, err := openaiResponseToGemini(response)
	if err != nil {
		return nil, err
	}
	return wrapCLIResponse(geminiResp), nil
}

// ConvertStreamChunk passthrough for CLI<->Codex streaming
func (c *GeminiCLIToCodexConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	return chunk, nil
}

// --- Claude -> GeminiCLI (Claude client, GeminiCLI provider) ---

// ClaudeToGeminiCLIConverter converts Claude requests to Gemini CLI format
type ClaudeToGeminiCLIConverter struct{}

// ConvertRequest converts Claude -> Gemini -> CLI wrap
func (c *ClaudeToGeminiCLIConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	geminiReq, err := (&ClaudeToGeminiConverter{}).ConvertRequest(input)
	if err != nil {
		return nil, err
	}
	return wrapCLIRequest(geminiReq), nil
}

// ConvertResponse unwraps CLI response and converts to Claude format
func (c *ClaudeToGeminiCLIConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	inner := unwrapCLIResponse(response)
	return geminiResponseToClaude(inner)
}

// ConvertStreamChunk converts Gemini CLI SSE to Claude SSE
func (c *ClaudeToGeminiCLIConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	return convertGeminiCLIStreamToClaude(chunk)
}

// --- OpenAI -> GeminiCLI ---

// OpenAIToGeminiCLIConverter converts OpenAI requests to Gemini CLI format
type OpenAIToGeminiCLIConverter struct{}

// ConvertRequest converts OpenAI -> Gemini -> CLI wrap
func (c *OpenAIToGeminiCLIConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	geminiReq, err := (&OpenAIToGeminiConverter{}).ConvertRequest(input)
	if err != nil {
		return nil, err
	}
	return wrapCLIRequest(geminiReq), nil
}

// ConvertResponse unwraps CLI response and converts to OpenAI format
func (c *OpenAIToGeminiCLIConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	inner := unwrapCLIResponse(response)
	claude, err := geminiResponseToClaude(inner)
	if err != nil {
		return nil, err
	}
	return (&OpenAIToClaudeConverter{}).ConvertResponse(claude)
}

// ConvertStreamChunk converts Gemini CLI SSE to OpenAI SSE
func (c *OpenAIToGeminiCLIConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	return convertGeminiCLIStreamToOpenAI(chunk)
}

// --- Codex -> GeminiCLI ---

// CodexToGeminiCLIConverter converts Codex requests to Gemini CLI format
type CodexToGeminiCLIConverter struct{}

// ConvertRequest converts Codex -> Claude -> Gemini -> CLI wrap
func (c *CodexToGeminiCLIConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	claude, err := (&CodexToClaudeConverter{}).ConvertRequest(input)
	if err != nil {
		return nil, err
	}
	geminiReq, err := (&ClaudeToGeminiConverter{}).ConvertRequest(claude)
	if err != nil {
		return nil, err
	}
	return wrapCLIRequest(geminiReq), nil
}

// ConvertResponse unwraps CLI response and converts to Codex format
func (c *CodexToGeminiCLIConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	inner := unwrapCLIResponse(response)
	claude, err := geminiResponseToClaude(inner)
	if err != nil {
		return nil, err
	}
	return claudeResponseToCodex(claude)
}

// ConvertStreamChunk passthrough
func (c *CodexToGeminiCLIConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	return chunk, nil
}

// ===== CLI envelope helpers =====

// unwrapCLIRequest extracts the inner Gemini request from CLI {request: {...}} envelope
func unwrapCLIRequest(input map[string]interface{}) map[string]interface{} {
	if inner := getMapField(input, "request"); inner != nil {
		// Merge any top-level model field into the inner request
		if model := getStringField(input, "model"); model != "" {
			inner["model"] = model
		}
		return inner
	}
	return input
}

// unwrapCLIResponse extracts the inner Gemini response from CLI {response: {...}} envelope
func unwrapCLIResponse(response map[string]interface{}) map[string]interface{} {
	if inner := getMapField(response, "response"); inner != nil {
		return inner
	}
	return response
}

// wrapCLIRequest wraps a standard Gemini request in the CLI envelope
func wrapCLIRequest(geminiReq map[string]interface{}) map[string]interface{} {
	// For Gemini CLI, systemInstruction needs role: "user"
	if sysInstr := getMapField(geminiReq, "systemInstruction"); sysInstr != nil {
		sysInstr["role"] = "user"
	}
	return geminiReq
}

// wrapCLIResponse wraps a standard Gemini response in the CLI envelope
func wrapCLIResponse(geminiResp map[string]interface{}) map[string]interface{} {
	return geminiResp
}

// ===== CLI streaming helpers =====

// convertClaudeStreamToGeminiCLI converts Claude SSE to Gemini CLI streaming format
func convertClaudeStreamToGeminiCLI(chunk []byte) ([]byte, error) {
	// Same as standard Gemini streaming but wrapped
	return convertClaudeStreamToGemini(chunk)
}

// convertGeminiCLIStreamToClaude converts Gemini CLI SSE to Claude SSE
func convertGeminiCLIStreamToClaude(chunk []byte) ([]byte, error) {
	chunkStr := strings.TrimSpace(string(chunk))

	// Gemini CLI may wrap response in {response: {...}} even in streaming
	if strings.HasPrefix(chunkStr, "data: ") {
		dataStr := strings.TrimPrefix(chunkStr, "data: ")
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(dataStr), &data); err == nil {
			if wrapped := getMapField(data, "response"); wrapped != nil {
				// Unwrap and re-encode
				rewrapped := mustMarshal(wrapped)
				return convertGeminiStreamToClaude([]byte(fmt.Sprintf("data: %s\n\n", rewrapped)))
			}
		}
	}

	return convertGeminiStreamToClaude(chunk)
}

// convertGeminiCLIStreamToOpenAI converts Gemini CLI SSE to OpenAI SSE
func convertGeminiCLIStreamToOpenAI(chunk []byte) ([]byte, error) {
	chunkStr := strings.TrimSpace(string(chunk))

	if strings.HasPrefix(chunkStr, "data: ") {
		dataStr := strings.TrimPrefix(chunkStr, "data: ")
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(dataStr), &data); err == nil {
			if wrapped := getMapField(data, "response"); wrapped != nil {
				rewrapped := mustMarshal(wrapped)
				return convertGeminiStreamToOpenAI([]byte(fmt.Sprintf("data: %s\n\n", rewrapped)))
			}
		}
	}

	return convertGeminiStreamToOpenAI(chunk)
}

// convertOpenAIStreamToGeminiCLI converts OpenAI SSE to Gemini CLI format
func convertOpenAIStreamToGeminiCLI(chunk []byte) ([]byte, error) {
	return convertOpenAIStreamToGemini(chunk)
}
