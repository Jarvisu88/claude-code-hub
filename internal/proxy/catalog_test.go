package proxy

import (
	"testing"
)

func TestNormalizeEndpointPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/v1/messages", "/v1/messages"},
		{"/v1/Messages/", "/v1/messages"},
		{"/v1/messages?foo=bar", "/v1/messages"},
		{"/V1/CHAT/COMPLETIONS/", "/v1/chat/completions"},
		{"/", "/"},
		{"", ""},
		{"/v1/models/gpt-4?version=2", "/v1/models/gpt-4"},
	}
	for _, tt := range tests {
		got := NormalizeEndpointPath(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeEndpointPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMatchEndpoint_ClaudeMessages(t *testing.T) {
	fam := MatchEndpoint("POST", "/v1/messages")
	if fam == nil {
		t.Fatal("expected match for /v1/messages")
	}
	if fam.ID != "claude-messages" {
		t.Errorf("got ID = %q, want %q", fam.ID, "claude-messages")
	}
	if fam.Surface != SurfaceClaude {
		t.Errorf("got Surface = %q, want %q", fam.Surface, SurfaceClaude)
	}
	if fam.AccountingTier != AccountingRequired {
		t.Errorf("got AccountingTier = %q, want %q", fam.AccountingTier, AccountingRequired)
	}
}

func TestMatchEndpoint_ClaudeCountTokens(t *testing.T) {
	fam := MatchEndpoint("POST", "/v1/messages/count_tokens")
	if fam == nil {
		t.Fatal("expected match for /v1/messages/count_tokens")
	}
	if fam.ID != "claude-count-tokens" {
		t.Errorf("got ID = %q, want %q", fam.ID, "claude-count-tokens")
	}
	if !fam.RawPassthrough {
		t.Error("expected RawPassthrough = true")
	}
}

func TestMatchEndpoint_ResponseCompact(t *testing.T) {
	fam := MatchEndpoint("POST", "/v1/responses/compact")
	if fam == nil {
		t.Fatal("expected match for /v1/responses/compact")
	}
	if fam.ID != "response-compact" {
		t.Errorf("got ID = %q, want %q", fam.ID, "response-compact")
	}
}

func TestMatchEndpoint_ResponseExecution(t *testing.T) {
	fam := MatchEndpoint("POST", "/v1/responses")
	if fam == nil {
		t.Fatal("expected match for /v1/responses")
	}
	if fam.ID != "response-execution" {
		t.Errorf("got ID = %q, want %q", fam.ID, "response-execution")
	}
	if fam.AccountingTier != AccountingRequired {
		t.Errorf("got AccountingTier = %q, want %q", fam.AccountingTier, AccountingRequired)
	}
}

func TestMatchEndpoint_ResponseResources(t *testing.T) {
	fam := MatchEndpoint("GET", "/v1/responses/resp_abc123")
	if fam == nil {
		t.Fatal("expected match for /v1/responses/resp_abc123")
	}
	if fam.ID != "response-resources" {
		t.Errorf("got ID = %q, want %q", fam.ID, "response-resources")
	}
}

func TestMatchEndpoint_OpenAIChatCompletions(t *testing.T) {
	fam := MatchEndpoint("POST", "/v1/chat/completions")
	if fam == nil {
		t.Fatal("expected match for /v1/chat/completions")
	}
	if fam.ID != "openai-chat-completions" {
		t.Errorf("got ID = %q, want %q", fam.ID, "openai-chat-completions")
	}
	if !fam.ModelRequired {
		t.Error("expected ModelRequired = true")
	}
}

func TestMatchEndpoint_OpenAIChatCompletionsResources(t *testing.T) {
	fam := MatchEndpoint("GET", "/v1/chat/completions/some-id")
	if fam == nil {
		t.Fatal("expected match for /v1/chat/completions/some-id")
	}
	if fam.ID != "openai-chat-completions-resources" {
		t.Errorf("got ID = %q, want %q", fam.ID, "openai-chat-completions-resources")
	}
}

func TestMatchEndpoint_OpenAIModels(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/v1/models", true},
		{"/v1/models/gpt-4", true},
		{"/v1/models/gpt-4/sub", false},
	}
	for _, tt := range tests {
		fam := MatchEndpoint("GET", tt.path)
		got := fam != nil && fam.ID == "openai-models"
		if got != tt.want {
			t.Errorf("MatchEndpoint(GET, %q) openai-models = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestMatchEndpoint_GeminiGenerateContent(t *testing.T) {
	tests := []struct {
		path string
		id   string
	}{
		{"/v1beta/models/gemini-pro:generateContent", "gemini-generate-content"},
		{"/v1/models/gemini-pro:generateContent", "gemini-generate-content"},
		{"/v1/publishers/google/models/gemini-pro:generateContent", "gemini-generate-content"},
		{"/v1beta/models/gemini-pro:streamGenerateContent", "gemini-stream-generate-content"},
		{"/v1beta/models/gemini-pro:countTokens", "gemini-count-tokens"},
		{"/v1beta/models/gemini-pro:embedContent", "gemini-embed-content"},
		{"/v1beta/models/gemini-pro:batchGenerateContent", "gemini-batch-generate-content"},
		{"/v1beta/models/gemini-pro:batchEmbedContents", "gemini-batch-embed-contents"},
		{"/v1beta/models/gemini-pro:asyncBatchEmbedContent", "gemini-async-batch-embed-content"},
	}
	for _, tt := range tests {
		fam := MatchEndpoint("POST", tt.path)
		if fam == nil {
			t.Errorf("MatchEndpoint(POST, %q) = nil, want ID=%q", tt.path, tt.id)
			continue
		}
		if fam.ID != tt.id {
			t.Errorf("MatchEndpoint(POST, %q).ID = %q, want %q", tt.path, fam.ID, tt.id)
		}
	}
}

func TestMatchEndpoint_GeminiPredict(t *testing.T) {
	fam := MatchEndpoint("POST", "/v1beta/models/gemini-pro:predict")
	if fam == nil {
		t.Fatal("expected match for gemini predict")
	}
	if fam.ID != "gemini-predict" {
		t.Errorf("got ID = %q, want %q", fam.ID, "gemini-predict")
	}

	fam2 := MatchEndpoint("POST", "/v1/publishers/google/models/gemini-pro:predictLongRunning")
	if fam2 == nil {
		t.Fatal("expected match for gemini predictLongRunning")
	}
	if fam2.ID != "gemini-predict-long-running" {
		t.Errorf("got ID = %q, want %q", fam2.ID, "gemini-predict-long-running")
	}
}

func TestMatchEndpoint_GeminiPredictNotOnV1Models(t *testing.T) {
	// /v1/models/ prefix should NOT support :predict (only standard prefixes do)
	fam := MatchEndpoint("POST", "/v1/models/gemini-pro:predict")
	// This should NOT match gemini-predict because /v1/models/ is not in geminiPredictPrefixes
	if fam != nil && fam.ID == "gemini-predict" {
		t.Errorf("v1/models/ should not support :predict, but matched %q", fam.ID)
	}
}

func TestMatchEndpoint_GeminiFiles(t *testing.T) {
	fam := MatchEndpoint("POST", "/v1beta/files")
	if fam == nil {
		t.Fatal("expected match")
	}
	if fam.ID != "gemini-files" {
		t.Errorf("got ID = %q, want %q", fam.ID, "gemini-files")
	}
}

func TestMatchEndpoint_GeminiModelsResource(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/v1beta/models", true},
		{"/v1beta/models/gemini-pro", true},
		{"/v1/publishers/google/models/gemini-pro", true},
	}
	for _, tt := range tests {
		fam := MatchEndpoint("GET", tt.path)
		got := fam != nil && fam.ID == "gemini-models-resource"
		if got != tt.want {
			t.Errorf("MatchEndpoint(GET, %q) gemini-models-resource = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestMatchEndpoint_GeminiCLI(t *testing.T) {
	fam := MatchEndpoint("POST", "/v1internal/models/gemini-pro:generateContent")
	if fam == nil {
		t.Fatal("expected match")
	}
	if fam.ID != "gemini-cli-generate-content" {
		t.Errorf("got ID = %q, want %q", fam.ID, "gemini-cli-generate-content")
	}
	if fam.Surface != SurfaceGeminiCLI {
		t.Errorf("got Surface = %q, want %q", fam.Surface, SurfaceGeminiCLI)
	}
}

func TestMatchEndpoint_OpenAIExtraEndpoints(t *testing.T) {
	endpoints := []struct {
		path string
		id   string
	}{
		{"/v1/completions", "openai-completions"},
		{"/v1/embeddings", "openai-embeddings"},
		{"/v1/moderations", "openai-moderations"},
		{"/v1/audio/speech", "openai-audio-generation"},
		{"/v1/audio/transcriptions", "openai-audio-transcription"},
		{"/v1/audio/translations", "openai-audio-transcription"},
		{"/v1/audio/voice_consents", "openai-audio-resources"},
		{"/v1/audio/voices", "openai-audio-resources"},
		{"/v1/images/generations", "openai-images"},
		{"/v1/files", "openai-files"},
		{"/v1/uploads/abc", "openai-uploads"},
		{"/v1/batches", "openai-batches"},
		{"/v1/fine_tuning/jobs", "openai-fine-tuning"},
		{"/v1/evals", "openai-evals"},
		{"/v1/assistants", "openai-assistants"},
		{"/v1/threads", "openai-threads"},
		{"/v1/conversations", "openai-conversations"},
		{"/v1/vector_stores", "openai-vector-stores"},
		{"/v1/containers", "openai-containers"},
		{"/v1/realtime/sessions", "openai-realtime-http"},
		{"/v1/videos", "openai-videos"},
		{"/v1/skills", "openai-skills"},
		{"/v1/chatkit", "openai-chatkit"},
	}
	for _, tt := range endpoints {
		fam := MatchEndpoint("GET", tt.path)
		if fam == nil {
			t.Errorf("MatchEndpoint(GET, %q) = nil, want ID=%q", tt.path, tt.id)
			continue
		}
		if fam.ID != tt.id {
			t.Errorf("MatchEndpoint(GET, %q).ID = %q, want %q", tt.path, fam.ID, tt.id)
		}
	}
}

func TestMatchEndpoint_Unknown(t *testing.T) {
	fam := MatchEndpoint("GET", "/v1/unknown/endpoint")
	if fam != nil {
		t.Errorf("expected nil for unknown endpoint, got %q", fam.ID)
	}
}

func TestMatchEndpoint_TrailingSlashNormalization(t *testing.T) {
	fam := MatchEndpoint("POST", "/v1/messages/")
	if fam == nil {
		t.Fatal("expected match for /v1/messages/ (trailing slash)")
	}
	if fam.ID != "claude-messages" {
		t.Errorf("got ID = %q, want %q", fam.ID, "claude-messages")
	}
}

func TestMatchEndpoint_CaseInsensitive(t *testing.T) {
	fam := MatchEndpoint("POST", "/V1/MESSAGES")
	if fam == nil {
		t.Fatal("expected case-insensitive match for /V1/MESSAGES")
	}
	if fam.ID != "claude-messages" {
		t.Errorf("got ID = %q, want %q", fam.ID, "claude-messages")
	}
}

func TestMatchEndpoint_GeminiInvalidPaths(t *testing.T) {
	invalid := []string{
		"/v1beta/models/:generateContent",     // no model
		"/v1beta/models/a:b:c",                // multiple colons in model
		"/v1beta/models/a/b:generateContent",  // slash in model
		"/v1beta/models/gemini-pro:",          // no action
		"/v1beta/models/gemini-pro:unknownAction",
	}
	for _, path := range invalid {
		fam := MatchEndpoint("POST", path)
		isGeminiGeneration := fam != nil && (fam.ID == "gemini-generate-content" || fam.ID == "gemini-stream-generate-content")
		if isGeminiGeneration {
			t.Errorf("path %q should NOT match gemini generation endpoints, got %q", path, fam.ID)
		}
	}
}

func TestDetectSurface(t *testing.T) {
	tests := []struct {
		path string
		want Surface
	}{
		{"/v1/messages", SurfaceClaude},
		{"/v1/chat/completions", SurfaceOpenAI},
		{"/v1/responses", SurfaceCodex},
		{"/v1beta/models/gemini-pro:generateContent", SurfaceGemini},
		{"/v1internal/models/gemini-pro:generateContent", SurfaceGeminiCLI},
		{"/v1/unknown", ""},
	}
	for _, tt := range tests {
		got := DetectSurface(tt.path)
		if got != tt.want {
			t.Errorf("DetectSurface(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestIsKnownEndpoint(t *testing.T) {
	if !IsKnownEndpoint("/v1/messages") {
		t.Error("expected /v1/messages to be known")
	}
	if IsKnownEndpoint("/v1/completely/unknown") {
		t.Error("expected /v1/completely/unknown to be unknown")
	}
}

func TestIsGeminiGenerationEndpoint(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/v1beta/models/gemini-pro:generateContent", true},
		{"/v1beta/models/gemini-pro:streamGenerateContent", true},
		{"/v1beta/models/gemini-pro:countTokens", false},
		{"/v1/messages", false},
	}
	for _, tt := range tests {
		got := IsGeminiGenerationEndpoint(tt.path)
		if got != tt.want {
			t.Errorf("IsGeminiGenerationEndpoint(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestListEndpointFamilies_Count(t *testing.T) {
	families := ListEndpointFamilies()
	// We expect at least 35 endpoint families as per the catalog spec
	if len(families) < 35 {
		t.Errorf("expected at least 35 endpoint families, got %d", len(families))
	}
}

func TestMatchEndpoint_OrderingPrecedence(t *testing.T) {
	// /v1/responses/compact must match response-compact (not response-execution or response-resources)
	fam := MatchEndpoint("POST", "/v1/responses/compact")
	if fam == nil || fam.ID != "response-compact" {
		id := ""
		if fam != nil {
			id = fam.ID
		}
		t.Errorf("expected response-compact, got %q", id)
	}

	// /v1/chat/completions must match openai-chat-completions (not openai-chat-completions-resources)
	fam2 := MatchEndpoint("POST", "/v1/chat/completions")
	if fam2 == nil || fam2.ID != "openai-chat-completions" {
		id := ""
		if fam2 != nil {
			id = fam2.ID
		}
		t.Errorf("expected openai-chat-completions, got %q", id)
	}
}
