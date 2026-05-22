package converter

// ClaudeToCodexConverter Claude API 到 Codex API 的转换器
type ClaudeToCodexConverter struct {
	ClaudeToOpenAIConverter
}

// ClaudeToGeminiConverter Claude API 到 Gemini API 的转换器
type ClaudeToGeminiConverter struct {
	ClaudeToOpenAIConverter
}

// OpenAIToCodexConverter OpenAI API 到 Codex API 的转换器
type OpenAIToCodexConverter struct {
	PassthroughConverter
}

// OpenAIToGeminiConverter OpenAI API 到 Gemini API 的转换器
type OpenAIToGeminiConverter struct {
	PassthroughConverter
}

// CodexToClaudeConverter Codex API 到 Claude API 的转换器
type CodexToClaudeConverter struct {
	OpenAIToClaudeConverter
}

// CodexToOpenAIConverter Codex API 到 OpenAI API 的转换器
type CodexToOpenAIConverter struct {
	PassthroughConverter
}

// CodexToGeminiConverter Codex API 到 Gemini API 的转换器
type CodexToGeminiConverter struct {
	PassthroughConverter
}

// GeminiToClaudeConverter Gemini API 到 Claude API 的转换器
type GeminiToClaudeConverter struct {
	OpenAIToClaudeConverter
}

// GeminiToOpenAIConverter Gemini API 到 OpenAI API 的转换器
type GeminiToOpenAIConverter struct {
	PassthroughConverter
}

// GeminiToCodexConverter Gemini API 到 Codex API 的转换器
type GeminiToCodexConverter struct {
	PassthroughConverter
}
