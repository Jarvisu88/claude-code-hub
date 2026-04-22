package session

import (
	"net/http"
	"regexp"
	"strings"
)

type CodexSessionIDSource string

const (
	SourceHeaderSessionID      CodexSessionIDSource = "header_session_id"
	SourceHeaderXSessionID     CodexSessionIDSource = "header_x_session_id"
	SourceBodyPromptCacheKey   CodexSessionIDSource = "body_prompt_cache_key"
	SourceBodyMetadataSession  CodexSessionIDSource = "body_metadata_session_id"
	SourceBodyPreviousResponse CodexSessionIDSource = "body_previous_response_id"
)

const (
	codexSessionIDMinLength   = 21
	codexSessionIDMaxLength   = 256
	codexPreviousResponseIDNS = "codex_prev_"
)

var codexSessionIDPattern = regexp.MustCompile(`^[-A-Za-z0-9_.:]+$`)

type CodexSessionExtractionResult struct {
	SessionID string
	Source    CodexSessionIDSource
}

func (r CodexSessionExtractionResult) Found() bool {
	return r.SessionID != ""
}

func NormalizeCodexSessionID(value any) string {
	raw, ok := value.(string)
	if !ok {
		return ""
	}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	if len(trimmed) < codexSessionIDMinLength {
		return ""
	}
	if len(trimmed) > codexSessionIDMaxLength {
		return ""
	}
	if !codexSessionIDPattern.MatchString(trimmed) {
		return ""
	}

	return trimmed
}

func ExtractCodexSessionID(headers http.Header, requestBody map[string]any) CodexSessionExtractionResult {
	if sessionID := NormalizeCodexSessionID(headers.Get("session_id")); sessionID != "" {
		return CodexSessionExtractionResult{
			SessionID: sessionID,
			Source:    SourceHeaderSessionID,
		}
	}

	if sessionID := NormalizeCodexSessionID(headers.Get("x-session-id")); sessionID != "" {
		return CodexSessionExtractionResult{
			SessionID: sessionID,
			Source:    SourceHeaderXSessionID,
		}
	}

	if sessionID := NormalizeCodexSessionID(requestBody["prompt_cache_key"]); sessionID != "" {
		return CodexSessionExtractionResult{
			SessionID: sessionID,
			Source:    SourceBodyPromptCacheKey,
		}
	}

	if metadata, ok := parseMetadata(requestBody); ok {
		if sessionID := NormalizeCodexSessionID(metadata["session_id"]); sessionID != "" {
			return CodexSessionExtractionResult{
				SessionID: sessionID,
				Source:    SourceBodyMetadataSession,
			}
		}
	}

	if previousResponseID := NormalizeCodexSessionID(requestBody["previous_response_id"]); previousResponseID != "" {
		sessionID := codexPreviousResponseIDNS + previousResponseID
		if len(sessionID) <= codexSessionIDMaxLength {
			return CodexSessionExtractionResult{
				SessionID: sessionID,
				Source:    SourceBodyPreviousResponse,
			}
		}
	}

	return CodexSessionExtractionResult{}
}

func parseMetadata(requestBody map[string]any) (map[string]any, bool) {
	if len(requestBody) == 0 {
		return nil, false
	}

	metadata, ok := requestBody["metadata"]
	if !ok || metadata == nil {
		return nil, false
	}

	parsed, ok := metadata.(map[string]any)
	if !ok {
		return nil, false
	}

	return parsed, true
}
