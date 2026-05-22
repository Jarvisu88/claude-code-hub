// Package proxy provides the endpoint family catalog and path matching.
//
// Every proxied request is classified into an EndpointFamily that determines
// the surface (client format), accounting tier, and passthrough behavior.
// The catalog mirrors the Node.js endpoint-family-catalog.ts.
package proxy

import (
	"regexp"
	"strings"
)

// Surface describes the client-facing API format.
type Surface string

const (
	SurfaceClaude   Surface = "claude"
	SurfaceCodex    Surface = "codex"    // also known as "response"
	SurfaceOpenAI   Surface = "openai"
	SurfaceGemini   Surface = "gemini"
	SurfaceGeminiCLI Surface = "gemini-cli"
)

// AccountingTier determines how usage is recorded.
type AccountingTier string

const (
	// AccountingRequired means this endpoint always records usage.
	AccountingRequired AccountingTier = "required_usage"
	// AccountingOptional means usage recording is best-effort.
	AccountingOptional AccountingTier = "optional_usage"
	// AccountingNone means no usage recording.
	AccountingNone AccountingTier = "none"
)

// EndpointFamily describes a known API endpoint family.
type EndpointFamily struct {
	// ID is a unique identifier, e.g. "claude-messages".
	ID string

	// Surface is the client format: claude, openai, gemini, gemini-cli, codex.
	Surface Surface

	// AccountingTier determines usage recording behavior.
	AccountingTier AccountingTier

	// ModelRequired is true if the request must contain a model field.
	ModelRequired bool

	// RawPassthrough means the request/response body is forwarded without transformation.
	RawPassthrough bool

	// match is the internal matcher function.
	match func(normalizedPath string) bool
}

// NormalizeEndpointPath strips the query string, trailing slash, and lowercases the path.
func NormalizeEndpointPath(pathname string) string {
	// Strip query string
	if idx := strings.IndexByte(pathname, '?'); idx >= 0 {
		pathname = pathname[:idx]
	}
	// Strip trailing slash (but preserve root "/")
	if len(pathname) > 1 && strings.HasSuffix(pathname, "/") {
		pathname = pathname[:len(pathname)-1]
	}
	return strings.ToLower(pathname)
}

// hasPrefix returns true if pathname == prefix or starts with prefix + "/".
func hasPrefix(pathname, prefix string) bool {
	return pathname == prefix || strings.HasPrefix(pathname, prefix+"/")
}

// --------------------------------------------------------------------------
// Gemini path matching helpers
// --------------------------------------------------------------------------

// geminiStandardPrefixes are the standard model path prefixes for Gemini.
var geminiStandardPrefixes = []string{
	"/v1beta/models/",
	"/v1/publishers/google/models/",
	"/v1/models/",
}

// geminiPredictPrefixes supports predict and predictLongRunning.
var geminiPredictPrefixes = []string{
	"/v1beta/models/",
	"/v1/publishers/google/models/",
}

// geminiCLIPrefixes are v1internal model prefixes.
var geminiCLIPrefixes = []string{
	"/v1internal/models/",
}

// matchGeminiModelAction checks if pathname matches prefix + {model}:{action}.
// The model must not contain "/" or ":", and action must be in the allowed list.
func matchGeminiModelAction(pathname, prefix string, actions []string) bool {
	if !strings.HasPrefix(pathname, prefix) {
		return false
	}
	remainder := pathname[len(prefix):]
	sepIdx := strings.IndexByte(remainder, ':')
	if sepIdx <= 0 {
		return false
	}
	model := remainder[:sepIdx]
	action := remainder[sepIdx+1:]

	if model == "" || strings.ContainsAny(model, "/:") {
		return false
	}
	if action == "" || strings.Contains(action, "/") {
		return false
	}

	for _, a := range actions {
		if action == a {
			return true
		}
	}
	return false
}

// matchGeminiOnPrefixes checks the action across multiple prefixes.
func matchGeminiOnPrefixes(pathname string, prefixes, actions []string) bool {
	for _, prefix := range prefixes {
		if matchGeminiModelAction(pathname, prefix, actions) {
			return true
		}
	}
	return false
}

// --------------------------------------------------------------------------
// Precompiled regexps
// --------------------------------------------------------------------------

var (
	openAIModelsRe       = regexp.MustCompile(`(?i)^/v1/models(/[^/:]+)?$`)
	geminiBetaModelsRe   = regexp.MustCompile(`(?i)^/v1beta/models/[^/:]+$`)
	geminiVertexModelsRe = regexp.MustCompile(`(?i)^/v1/publishers/google/models/[^/:]+$`)
)

// --------------------------------------------------------------------------
// Catalog - the ordered list of known endpoint families
// --------------------------------------------------------------------------

// knownEndpointFamilies is the authoritative ordered list.
// The order matters: the first match wins.
var knownEndpointFamilies = []EndpointFamily{
	// ---- Claude surface ----
	{
		ID: "claude-messages", Surface: SurfaceClaude,
		AccountingTier: AccountingRequired, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return p == "/v1/messages" },
	},
	{
		ID: "claude-count-tokens", Surface: SurfaceClaude,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: true,
		match: func(p string) bool { return p == "/v1/messages/count_tokens" },
	},

	// ---- Codex / Response surface ----
	{
		ID: "response-compact", Surface: SurfaceCodex,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: true,
		match: func(p string) bool { return p == "/v1/responses/compact" },
	},
	{
		ID: "response-execution", Surface: SurfaceCodex,
		AccountingTier: AccountingRequired, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return p == "/v1/responses" },
	},
	{
		ID: "response-resources", Surface: SurfaceCodex,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1/responses") },
	},

	// ---- OpenAI surface ----
	{
		ID: "openai-chat-completions", Surface: SurfaceOpenAI,
		AccountingTier: AccountingRequired, ModelRequired: true, RawPassthrough: false,
		match: func(p string) bool { return p == "/v1/chat/completions" },
	},
	{
		ID: "openai-chat-completions-resources", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1/chat/completions") },
	},
	{
		ID: "openai-models", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return openAIModelsRe.MatchString(p) },
	},

	// ---- Gemini surface ----
	{
		ID: "gemini-generate-content", Surface: SurfaceGemini,
		AccountingTier: AccountingRequired, ModelRequired: true, RawPassthrough: false,
		match: func(p string) bool {
			return matchGeminiOnPrefixes(p, geminiStandardPrefixes, []string{"generatecontent"})
		},
	},
	{
		ID: "gemini-stream-generate-content", Surface: SurfaceGemini,
		AccountingTier: AccountingRequired, ModelRequired: true, RawPassthrough: false,
		match: func(p string) bool {
			return matchGeminiOnPrefixes(p, geminiStandardPrefixes, []string{"streamgeneratecontent"})
		},
	},
	{
		ID: "gemini-count-tokens", Surface: SurfaceGemini,
		AccountingTier: AccountingOptional, ModelRequired: true, RawPassthrough: false,
		match: func(p string) bool {
			return matchGeminiOnPrefixes(p, geminiStandardPrefixes, []string{"counttokens"})
		},
	},
	{
		ID: "gemini-embed-content", Surface: SurfaceGemini,
		AccountingTier: AccountingOptional, ModelRequired: true, RawPassthrough: false,
		match: func(p string) bool {
			return matchGeminiOnPrefixes(p, geminiStandardPrefixes, []string{"embedcontent"})
		},
	},
	{
		ID: "gemini-batch-generate-content", Surface: SurfaceGemini,
		AccountingTier: AccountingOptional, ModelRequired: true, RawPassthrough: false,
		match: func(p string) bool {
			return matchGeminiOnPrefixes(p, geminiStandardPrefixes, []string{"batchgeneratecontent"})
		},
	},
	{
		ID: "gemini-batch-embed-contents", Surface: SurfaceGemini,
		AccountingTier: AccountingOptional, ModelRequired: true, RawPassthrough: false,
		match: func(p string) bool {
			return matchGeminiOnPrefixes(p, geminiStandardPrefixes, []string{"batchembedcontents"})
		},
	},
	{
		ID: "gemini-async-batch-embed-content", Surface: SurfaceGemini,
		AccountingTier: AccountingOptional, ModelRequired: true, RawPassthrough: false,
		match: func(p string) bool {
			return matchGeminiOnPrefixes(p, geminiStandardPrefixes, []string{"asyncbatchembedcontent"})
		},
	},
	{
		ID: "gemini-predict", Surface: SurfaceGemini,
		AccountingTier: AccountingOptional, ModelRequired: true, RawPassthrough: false,
		match: func(p string) bool {
			return matchGeminiOnPrefixes(p, geminiPredictPrefixes, []string{"predict"})
		},
	},
	{
		ID: "gemini-predict-long-running", Surface: SurfaceGemini,
		AccountingTier: AccountingOptional, ModelRequired: true, RawPassthrough: false,
		match: func(p string) bool {
			return matchGeminiOnPrefixes(p, geminiPredictPrefixes, []string{"predictlongrunning"})
		},
	},
	{
		ID: "gemini-files", Surface: SurfaceGemini,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1beta/files") },
	},
	{
		ID: "gemini-models-resource", Surface: SurfaceGemini,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool {
			return p == "/v1beta/models" ||
				geminiBetaModelsRe.MatchString(p) ||
				geminiVertexModelsRe.MatchString(p)
		},
	},

	// ---- Gemini CLI surface ----
	{
		ID: "gemini-cli-generate-content", Surface: SurfaceGeminiCLI,
		AccountingTier: AccountingRequired, ModelRequired: true, RawPassthrough: false,
		match: func(p string) bool {
			return matchGeminiOnPrefixes(p, geminiCLIPrefixes, []string{"generatecontent"})
		},
	},
	{
		ID: "gemini-cli-stream-generate-content", Surface: SurfaceGeminiCLI,
		AccountingTier: AccountingRequired, ModelRequired: true, RawPassthrough: false,
		match: func(p string) bool {
			return matchGeminiOnPrefixes(p, geminiCLIPrefixes, []string{"streamgeneratecontent"})
		},
	},

	// ---- More OpenAI surface endpoints ----
	{
		ID: "openai-completions", Surface: SurfaceOpenAI,
		AccountingTier: AccountingRequired, ModelRequired: true, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1/completions") },
	},
	{
		ID: "openai-embeddings", Surface: SurfaceOpenAI,
		AccountingTier: AccountingRequired, ModelRequired: true, RawPassthrough: false,
		match: func(p string) bool { return p == "/v1/embeddings" },
	},
	{
		ID: "openai-moderations", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1/moderations") },
	},
	{
		ID: "openai-audio-generation", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return p == "/v1/audio/speech" },
	},
	{
		ID: "openai-audio-transcription", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool {
			return p == "/v1/audio/transcriptions" || p == "/v1/audio/translations"
		},
	},
	{
		ID: "openai-audio-resources", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool {
			return hasPrefix(p, "/v1/audio/voice_consents") || hasPrefix(p, "/v1/audio/voices")
		},
	},
	{
		ID: "openai-images", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1/images") },
	},
	{
		ID: "openai-files", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1/files") },
	},
	{
		ID: "openai-uploads", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1/uploads") },
	},
	{
		ID: "openai-batches", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1/batches") },
	},
	{
		ID: "openai-fine-tuning", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1/fine_tuning") },
	},
	{
		ID: "openai-evals", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1/evals") },
	},
	{
		ID: "openai-assistants", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1/assistants") },
	},
	{
		ID: "openai-threads", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1/threads") },
	},
	{
		ID: "openai-conversations", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1/conversations") },
	},
	{
		ID: "openai-vector-stores", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1/vector_stores") },
	},
	{
		ID: "openai-containers", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1/containers") },
	},
	{
		ID: "openai-realtime-http", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1/realtime") },
	},
	{
		ID: "openai-videos", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1/videos") },
	},
	{
		ID: "openai-skills", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1/skills") },
	},
	{
		ID: "openai-chatkit", Surface: SurfaceOpenAI,
		AccountingTier: AccountingNone, ModelRequired: false, RawPassthrough: false,
		match: func(p string) bool { return hasPrefix(p, "/v1/chatkit") },
	},
}

// MatchEndpoint finds the first matching EndpointFamily for the given method and path.
// The method parameter is currently unused (all matching is path-based) but is
// accepted for future extensions.
// Returns nil if no family matches.
func MatchEndpoint(method, path string) *EndpointFamily {
	normalized := NormalizeEndpointPath(path)
	for i := range knownEndpointFamilies {
		if knownEndpointFamilies[i].match(normalized) {
			return &knownEndpointFamilies[i]
		}
	}
	return nil
}

// ListEndpointFamilies returns all known endpoint families.
func ListEndpointFamilies() []EndpointFamily {
	result := make([]EndpointFamily, len(knownEndpointFamilies))
	copy(result, knownEndpointFamilies)
	return result
}

// DetectSurface returns the surface for a given pathname, or empty string if unknown.
func DetectSurface(pathname string) Surface {
	fam := MatchEndpoint("", pathname)
	if fam == nil {
		return ""
	}
	return fam.Surface
}

// IsKnownEndpoint returns true if the path matches any known endpoint family.
func IsKnownEndpoint(pathname string) bool {
	return MatchEndpoint("", pathname) != nil
}

// geminiGenerationActions are the actions considered "generation" for billing.
var geminiGenerationActions = map[string]bool{
	"generatecontent":       true,
	"streamgeneratecontent": true,
}

// IsGeminiGenerationEndpoint returns true if the path is a Gemini generation endpoint.
func IsGeminiGenerationEndpoint(pathname string) bool {
	normalized := NormalizeEndpointPath(pathname)
	// Extract the :action suffix
	colonIdx := strings.LastIndexByte(normalized, ':')
	if colonIdx < 0 || colonIdx == len(normalized)-1 {
		return false
	}
	action := normalized[colonIdx+1:]
	return geminiGenerationActions[action]
}
