package converter

import (
	"encoding/json"
)

// Converter 格式转换器接口
// 负责在不同 AI API 格式之间进行转换
type Converter interface {
	// ConvertRequest 转换请求格式
	// 将输入格式转换为目标供应商的格式
	ConvertRequest(input map[string]interface{}) (map[string]interface{}, error)

	// ConvertResponse 转换响应格式
	// 将供应商响应转换为标准格式
	ConvertResponse(response map[string]interface{}) (map[string]interface{}, error)

	// ConvertStreamChunk 转换流式响应块
	// 将供应商的 SSE 事件转换为标准格式
	ConvertStreamChunk(chunk []byte) ([]byte, error)
}

// ProviderType 供应商类型
type ProviderType string

const (
	// ProviderTypeClaude Claude API (Anthropic)
	ProviderTypeClaude ProviderType = "claude"

	// ProviderTypeOpenAI OpenAI 兼容 API
	ProviderTypeOpenAI ProviderType = "openai"

	// ProviderTypeCodex Codex API
	ProviderTypeCodex ProviderType = "codex"

	// ProviderTypeGemini Gemini API
	ProviderTypeGemini ProviderType = "gemini"
)

// ClientType 客户端类型
type ClientType string

const (
	// ClientTypeClaude Claude 客户端
	ClientTypeClaude ClientType = "claude"

	// ClientTypeOpenAI OpenAI 客户端
	ClientTypeOpenAI ClientType = "openai"

	// ClientTypeCodex Codex 客户端
	ClientTypeCodex ClientType = "codex"

	// ClientTypeGemini Gemini 客户端
	ClientTypeGemini ClientType = "gemini"
)

// NewConverter 创建转换器
// clientType: 客户端类型（请求格式）
// providerType: 供应商类型（目标格式）
func NewConverter(clientType ClientType, providerType ProviderType) Converter {
	// 如果客户端和供应商类型相同，使用透传转换器
	if string(clientType) == string(providerType) {
		return &PassthroughConverter{}
	}

	// 根据客户端和供应商类型选择转换器
	switch clientType {
	case ClientTypeClaude:
		switch providerType {
		case ProviderTypeOpenAI:
			return &ClaudeToOpenAIConverter{}
		case ProviderTypeCodex:
			return &ClaudeToCodexConverter{}
		case ProviderTypeGemini:
			return &ClaudeToGeminiConverter{}
		}
	case ClientTypeOpenAI:
		switch providerType {
		case ProviderTypeClaude:
			return &OpenAIToClaudeConverter{}
		case ProviderTypeCodex:
			return &OpenAIToCodexConverter{}
		case ProviderTypeGemini:
			return &OpenAIToGeminiConverter{}
		}
	case ClientTypeCodex:
		switch providerType {
		case ProviderTypeClaude:
			return &CodexToClaudeConverter{}
		case ProviderTypeOpenAI:
			return &CodexToOpenAIConverter{}
		case ProviderTypeGemini:
			return &CodexToGeminiConverter{}
		}
	case ClientTypeGemini:
		switch providerType {
		case ProviderTypeClaude:
			return &GeminiToClaudeConverter{}
		case ProviderTypeOpenAI:
			return &GeminiToOpenAIConverter{}
		case ProviderTypeCodex:
			return &GeminiToCodexConverter{}
		}
	}

	// 默认使用透传转换器
	return &PassthroughConverter{}
}

// PassthroughConverter 透传转换器
// 不做任何转换，直接透传
type PassthroughConverter struct{}

// ConvertRequest 透传请求
func (c *PassthroughConverter) ConvertRequest(input map[string]interface{}) (map[string]interface{}, error) {
	return input, nil
}

// ConvertResponse 透传响应
func (c *PassthroughConverter) ConvertResponse(response map[string]interface{}) (map[string]interface{}, error) {
	return response, nil
}

// ConvertStreamChunk 透传流式响应块
func (c *PassthroughConverter) ConvertStreamChunk(chunk []byte) ([]byte, error) {
	return chunk, nil
}

// Helper functions

// getStringField 获取字符串字段
func getStringField(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// getIntField 获取整数字段
func getIntField(m map[string]interface{}, key string) int {
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

// getBoolField 获取布尔字段
func getBoolField(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// getArrayField 获取数组字段
func getArrayField(m map[string]interface{}, key string) []interface{} {
	if v, ok := m[key]; ok {
		if arr, ok := v.([]interface{}); ok {
			return arr
		}
	}
	return nil
}

// getMapField 获取对象字段
func getMapField(m map[string]interface{}, key string) map[string]interface{} {
	if v, ok := m[key]; ok {
		if obj, ok := v.(map[string]interface{}); ok {
			return obj
		}
	}
	return nil
}
