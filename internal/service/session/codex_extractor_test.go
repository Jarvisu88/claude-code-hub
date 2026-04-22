package session

import (
	"net/http"
	"strings"
	"testing"
)

func TestExtractCodexSessionID(t *testing.T) {
	t.Run("extracts from header session_id", func(t *testing.T) {
		headerSessionID := "sess_123456789012345678901"

		result := ExtractCodexSessionID(
			headerWith("session_id", headerSessionID),
			map[string]any{
				"metadata":             map[string]any{"session_id": "sess_aaaaaaaaaaaaaaaaaaaaa"},
				"previous_response_id": "resp_123456789012345678901",
			},
		)

		assertExtractionResult(t, result, headerSessionID, SourceHeaderSessionID)
	})

	t.Run("extracts from header x-session-id", func(t *testing.T) {
		headerSessionID := "sess_123456789012345678902"

		result := ExtractCodexSessionID(
			headerWith("x-session-id", headerSessionID),
			map[string]any{
				"prompt_cache_key":     "019b82ff-08ff-75a3-a203-7e10274fdbd8",
				"metadata":             map[string]any{"session_id": "sess_aaaaaaaaaaaaaaaaaaaaa"},
				"previous_response_id": "resp_123456789012345678901",
			},
		)

		assertExtractionResult(t, result, headerSessionID, SourceHeaderXSessionID)
	})

	t.Run("extracts from body metadata.session_id", func(t *testing.T) {
		bodySessionID := "sess_123456789012345678903"

		result := ExtractCodexSessionID(nil, map[string]any{
			"metadata": map[string]any{"session_id": bodySessionID},
		})

		assertExtractionResult(t, result, bodySessionID, SourceBodyMetadataSession)
	})

	t.Run("extracts from body prompt_cache_key", func(t *testing.T) {
		promptCacheKey := "019b82ff-08ff-75a3-a203-7e10274fdbd8"

		result := ExtractCodexSessionID(nil, map[string]any{"prompt_cache_key": promptCacheKey})

		assertExtractionResult(t, result, promptCacheKey, SourceBodyPromptCacheKey)
	})

	t.Run("prompt_cache_key has higher priority than metadata.session_id", func(t *testing.T) {
		promptCacheKey := "019b82ff-08ff-75a3-a203-7e10274fdbd8"
		metadataSessionID := "sess_123456789012345678903"

		result := ExtractCodexSessionID(nil, map[string]any{
			"prompt_cache_key": promptCacheKey,
			"metadata":         map[string]any{"session_id": metadataSessionID},
		})

		assertExtractionResult(t, result, promptCacheKey, SourceBodyPromptCacheKey)
	})

	t.Run("ignores invalid prompt_cache_key and falls back to metadata.session_id", func(t *testing.T) {
		metadataSessionID := "sess_123456789012345678903"

		result := ExtractCodexSessionID(nil, map[string]any{
			"prompt_cache_key": "short",
			"metadata":         map[string]any{"session_id": metadataSessionID},
		})

		assertExtractionResult(t, result, metadataSessionID, SourceBodyMetadataSession)
	})

	t.Run("falls back to previous_response_id", func(t *testing.T) {
		previousResponseID := "resp_123456789012345678901"

		result := ExtractCodexSessionID(nil, map[string]any{
			"previous_response_id": previousResponseID,
		})

		assertExtractionResult(t, result, codexPreviousResponseIDNS+previousResponseID, SourceBodyPreviousResponse)
	})

	t.Run("rejects previous_response_id that would exceed 256 after prefix", func(t *testing.T) {
		result := ExtractCodexSessionID(nil, map[string]any{
			"previous_response_id": strings.Repeat("a", 250),
		})

		assertNoExtractionResult(t, result)
	})

	t.Run("respects extraction priority", func(t *testing.T) {
		sessionIDFromHeader := "sess_123456789012345678904"

		result := ExtractCodexSessionID(
			headersWith(map[string]string{
				"session_id":   sessionIDFromHeader,
				"x-session-id": "sess_123456789012345678905",
			}),
			map[string]any{
				"prompt_cache_key":     "019b82ff-08ff-75a3-a203-7e10274fdbd8",
				"metadata":             map[string]any{"session_id": "sess_123456789012345678906"},
				"previous_response_id": "resp_123456789012345678901",
			},
		)

		assertExtractionResult(t, result, sessionIDFromHeader, SourceHeaderSessionID)
	})

	t.Run("rejects session_id shorter than 21 characters", func(t *testing.T) {
		result := ExtractCodexSessionID(headerWith("session_id", "short_id_12345"), nil)
		assertNoExtractionResult(t, result)
	})

	t.Run("accepts session_id with exactly 21 characters", func(t *testing.T) {
		minID := strings.Repeat("a", 21)
		result := ExtractCodexSessionID(headerWith("session_id", minID), nil)
		assertExtractionResult(t, result, minID, SourceHeaderSessionID)
	})

	t.Run("accepts session_id with exactly 256 characters", func(t *testing.T) {
		maxID := strings.Repeat("a", 256)
		result := ExtractCodexSessionID(headerWith("session_id", maxID), nil)
		assertExtractionResult(t, result, maxID, SourceHeaderSessionID)
	})

	t.Run("rejects session_id longer than 256 characters", func(t *testing.T) {
		result := ExtractCodexSessionID(headerWith("session_id", strings.Repeat("a", 300)), nil)
		assertNoExtractionResult(t, result)
	})

	t.Run("rejects session_id with invalid characters", func(t *testing.T) {
		result := ExtractCodexSessionID(nil, map[string]any{
			"metadata": map[string]any{"session_id": "sess_123456789@#$%^&*()!"},
		})
		assertNoExtractionResult(t, result)
	})

	t.Run("accepts session_id with allowed special characters", func(t *testing.T) {
		validID := "sess-123_456.789:abc012345"
		result := ExtractCodexSessionID(headerWith("session_id", validID), nil)
		assertExtractionResult(t, result, validID, SourceHeaderSessionID)
	})

	t.Run("returns empty result when no valid session_id found", func(t *testing.T) {
		result := ExtractCodexSessionID(nil, nil)
		assertNoExtractionResult(t, result)
	})

	t.Run("ignores non-map metadata payloads", func(t *testing.T) {
		result := ExtractCodexSessionID(nil, map[string]any{
			"metadata": "not-a-map",
		})
		assertNoExtractionResult(t, result)
	})

	t.Run("ignores non-string values before falling through", func(t *testing.T) {
		result := ExtractCodexSessionID(nil, map[string]any{
			"prompt_cache_key": 12345,
			"metadata": map[string]any{
				"session_id": true,
			},
		})
		assertNoExtractionResult(t, result)
	})
}

func headerWith(key, value string) http.Header {
	headers := make(http.Header)
	headers.Set(key, value)
	return headers
}

func headersWith(values map[string]string) http.Header {
	headers := make(http.Header, len(values))
	for key, value := range values {
		headers.Set(key, value)
	}
	return headers
}

func assertExtractionResult(t *testing.T, result CodexSessionExtractionResult, sessionID string, source CodexSessionIDSource) {
	t.Helper()

	if result.SessionID != sessionID {
		t.Fatalf("expected session id %q, got %q", sessionID, result.SessionID)
	}
	if result.Source != source {
		t.Fatalf("expected source %q, got %q", source, result.Source)
	}
	if !result.Found() {
		t.Fatal("expected found result")
	}
}

func assertNoExtractionResult(t *testing.T, result CodexSessionExtractionResult) {
	t.Helper()

	if result.SessionID != "" {
		t.Fatalf("expected empty session id, got %q", result.SessionID)
	}
	if result.Source != "" {
		t.Fatalf("expected empty source, got %q", result.Source)
	}
	if result.Found() {
		t.Fatal("expected missing result")
	}
}
