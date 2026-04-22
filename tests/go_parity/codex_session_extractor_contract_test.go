package go_parity

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/ding113/claude-code-hub/internal/service/session"
)

type codexSessionExtractorFixture struct {
	Source string                             `json:"source"`
	Cases  []codexSessionExtractorFixtureCase `json:"cases"`
}

type codexSessionExtractorFixtureCase struct {
	Name         string            `json:"name"`
	Headers      map[string]string `json:"headers"`
	Body         map[string]any    `json:"body"`
	WantSession  string            `json:"want_session_id"`
	WantSource   string            `json:"want_source"`
	NodeEvidence string            `json:"node_evidence"`
}

func TestCodexSessionExtractorParityCases(t *testing.T) {
	fixture := loadCodexSessionExtractorFixture(t)
	t.Logf("node parity source: %s", fixture.Source)

	for _, tc := range fixture.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			headers := http.Header{}
			for key, value := range tc.Headers {
				headers.Set(key, value)
			}

			result := session.ExtractCodexSessionID(headers, tc.Body)
			if result.SessionID != tc.WantSession {
				t.Fatalf("node evidence %s: expected session id %q, got %q", tc.NodeEvidence, tc.WantSession, result.SessionID)
			}
			if string(result.Source) != tc.WantSource {
				t.Fatalf("node evidence %s: expected source %q, got %q", tc.NodeEvidence, tc.WantSource, result.Source)
			}
		})
	}
}

func loadCodexSessionExtractorFixture(t *testing.T) codexSessionExtractorFixture {
	t.Helper()

	path := filepath.Join("..", "go-parity", "fixtures", "codex-session-extractor-cases.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture file %s: %v", path, err)
	}

	var fixture codexSessionExtractorFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("failed to decode fixture file %s: %v", path, err)
	}

	return fixture
}
