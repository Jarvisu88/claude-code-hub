package go_parity

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	sessionsvc "github.com/ding113/claude-code-hub/internal/service/session"
)

type codexSessionFixtureCase struct {
	Name          string            `json:"name"`
	Headers       map[string]string `json:"headers"`
	Body          map[string]any    `json:"body"`
	WantSessionID string            `json:"want_session_id"`
	WantSource    string            `json:"want_source"`
	NodeEvidence  string            `json:"node_evidence"`
}

func TestCodexSessionExtractorContractCases(t *testing.T) {
	fixtures := loadCodexSessionFixtures(t)

	for _, tc := range fixtures {
		t.Run(tc.Name, func(t *testing.T) {
			headers := make(http.Header, len(tc.Headers))
			for key, value := range tc.Headers {
				headers.Set(key, value)
			}

			result := sessionsvc.ExtractCodexSessionID(headers, tc.Body)
			if result.SessionID != tc.WantSessionID {
				t.Fatalf("node evidence %s: expected session id %q, got %q", tc.NodeEvidence, tc.WantSessionID, result.SessionID)
			}
			if string(result.Source) != tc.WantSource {
				t.Fatalf("node evidence %s: expected source %q, got %q", tc.NodeEvidence, tc.WantSource, result.Source)
			}
			if (tc.WantSessionID != "") != result.Found() {
				t.Fatalf("node evidence %s: expected found=%t, got %t", tc.NodeEvidence, tc.WantSessionID != "", result.Found())
			}
		})
	}
}

func loadCodexSessionFixtures(t *testing.T) []codexSessionFixtureCase {
	t.Helper()

	path := filepath.Join("..", "go-parity", "fixtures", "codex-session-extractor-cases.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture file %s: %v", path, err)
	}

	var cases []codexSessionFixtureCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to decode fixture file %s: %v", path, err)
	}

	return cases
}
