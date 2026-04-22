package session

import (
	"net/http"
	"strings"
	"testing"
)

func TestNormalizeCodexSessionID(t *testing.T) {
	valid := "resp_1234567890abcdefghijk"
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "trim and keep valid", input: "  " + valid + "  ", want: valid},
		{name: "reject empty", input: "   ", want: ""},
		{name: "reject short", input: "too-short-session-id", want: ""},
		{name: "reject too long", input: strings.Repeat("a", codexSessionIDMaxLength+1), want: ""},
		{name: "reject invalid chars", input: "resp_1234567890abcdefghi/", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeCodexSessionID(tc.input); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestExtractCodexSessionIDPriority(t *testing.T) {
	headers := http.Header{}
	headers.Set("session_id", "sess_header_priority_1234567890")
	headers.Set("x-session-id", "sess_x_header_123456789012345")

	result := ExtractCodexSessionID(headers, map[string]any{
		"prompt_cache_key":     "resp_prompt_cache_1234567890123",
		"previous_response_id": "resp_previous_1234567890123456",
		"metadata": map[string]any{
			"session_id": "sess_metadata_123456789012345",
		},
	})

	if result.SessionID != "sess_header_priority_1234567890" {
		t.Fatalf("expected header session_id to win, got %q", result.SessionID)
	}
	if result.Source != CodexSessionIDSourceHeaderSessionID {
		t.Fatalf("expected source %q, got %q", CodexSessionIDSourceHeaderSessionID, result.Source)
	}
}

func TestExtractCodexSessionIDFallsBackToBodyFields(t *testing.T) {
	tests := []struct {
		name       string
		body       map[string]any
		wantID     string
		wantSource CodexSessionIDSource
	}{
		{
			name: "prompt cache key",
			body: map[string]any{
				"prompt_cache_key": "resp_prompt_cache_1234567890123",
			},
			wantID:     "resp_prompt_cache_1234567890123",
			wantSource: CodexSessionIDSourceBodyPromptCacheKey,
		},
		{
			name: "metadata session id",
			body: map[string]any{
				"metadata": map[string]any{
					"session_id": "sess_metadata_123456789012345",
				},
			},
			wantID:     "sess_metadata_123456789012345",
			wantSource: CodexSessionIDSourceBodyMetadataSession,
		},
		{
			name: "previous response id",
			body: map[string]any{
				"previous_response_id": "resp_previous_1234567890123456",
			},
			wantID:     "codex_prev_resp_previous_1234567890123456",
			wantSource: CodexSessionIDSourceBodyPreviousResponse,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractCodexSessionID(http.Header{}, tc.body)
			if result.SessionID != tc.wantID {
				t.Fatalf("expected session id %q, got %q", tc.wantID, result.SessionID)
			}
			if result.Source != tc.wantSource {
				t.Fatalf("expected source %q, got %q", tc.wantSource, result.Source)
			}
		})
	}
}

func TestExtractCodexSessionIDRejectsInvalidValues(t *testing.T) {
	headers := http.Header{}
	headers.Set("session_id", "bad")

	result := ExtractCodexSessionID(headers, map[string]any{
		"prompt_cache_key":     "bad",
		"previous_response_id": strings.Repeat("r", codexSessionIDMaxLength),
		"metadata": map[string]any{
			"session_id": "invalid/value",
		},
	})

	if result.SessionID != "" {
		t.Fatalf("expected empty session id, got %q", result.SessionID)
	}
	if result.Source != "" {
		t.Fatalf("expected empty source, got %q", result.Source)
	}
}
