package session

import (
	"net/http"
	"regexp"
	"strings"
)

type CodexSessionIDSource string

const (
	CodexSessionIDSourceHeaderSessionID      CodexSessionIDSource = "header_session_id"
	CodexSessionIDSourceHeaderXSessionID     CodexSessionIDSource = "header_x_session_id"
	CodexSessionIDSourceBodyPromptCacheKey   CodexSessionIDSource = "body_prompt_cache_key"
	CodexSessionIDSourceBodyMetadataSession  CodexSessionIDSource = "body_metadata_session_id"
	CodexSessionIDSourceBodyPreviousResponse CodexSessionIDSource = "body_previous_response_id"
)

const (
	codexSessionIDMinLength = 21
	codexSessionIDMaxLength = 256
	codexPrevPrefix         = "codex_prev_"
)

var codexSessionIDPattern = regexp.MustCompile(`^[\w\-.:]+$`)

type CodexSessionExtractionResult struct {
	SessionID string
	Source    CodexSessionIDSource
}

func NormalizeCodexSessionID(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) < codexSessionIDMinLength || len(trimmed) > codexSessionIDMaxLength {
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
			Source:    CodexSessionIDSourceHeaderSessionID,
		}
	}

	if sessionID := NormalizeCodexSessionID(headers.Get("x-session-id")); sessionID != "" {
		return CodexSessionExtractionResult{
			SessionID: sessionID,
			Source:    CodexSessionIDSourceHeaderXSessionID,
		}
	}

	if sessionID := NormalizeCodexSessionID(stringValue(requestBody["prompt_cache_key"])); sessionID != "" {
		return CodexSessionExtractionResult{
			SessionID: sessionID,
			Source:    CodexSessionIDSourceBodyPromptCacheKey,
		}
	}

	if metadata := metadataMap(requestBody); metadata != nil {
		if sessionID := NormalizeCodexSessionID(stringValue(metadata["session_id"])); sessionID != "" {
			return CodexSessionExtractionResult{
				SessionID: sessionID,
				Source:    CodexSessionIDSourceBodyMetadataSession,
			}
		}
	}

	if previousResponseID := NormalizeCodexSessionID(stringValue(requestBody["previous_response_id"])); previousResponseID != "" {
		sessionID := codexPrevPrefix + previousResponseID
		if len(sessionID) <= codexSessionIDMaxLength {
			return CodexSessionExtractionResult{
				SessionID: sessionID,
				Source:    CodexSessionIDSourceBodyPreviousResponse,
			}
		}
	}

	return CodexSessionExtractionResult{}
}

func metadataMap(requestBody map[string]any) map[string]any {
	if requestBody == nil {
		return nil
	}
	metadata, ok := requestBody["metadata"]
	if !ok {
		return nil
	}
	metadataMap, ok := metadata.(map[string]any)
	if !ok {
		return nil
	}
	return metadataMap
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}
