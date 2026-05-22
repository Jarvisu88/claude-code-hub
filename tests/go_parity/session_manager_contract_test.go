package go_parity

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/ding113/claude-code-hub/internal/config"
	"github.com/ding113/claude-code-hub/internal/service/session"
)

type sessionManagerFixture struct {
	Source []string                    `json:"source"`
	Cases  []sessionManagerFixtureCase `json:"cases"`
}

type sessionManagerFixtureCase struct {
	Name         string            `json:"name"`
	Headers      map[string]string `json:"headers"`
	Body         map[string]any    `json:"body"`
	WantSession  string            `json:"want_session_id"`
	WantSource   string            `json:"want_source"`
	NodeEvidence string            `json:"node_evidence"`
}

func TestSessionManagerExtractClientSessionIDParityCases(t *testing.T) {
	fixture := loadSessionManagerFixture(t)
	manager := session.NewManager(config.SessionConfig{TTL: 300}, nil)

	for _, tc := range fixture.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			headers := http.Header{}
			for key, value := range tc.Headers {
				headers.Set(key, value)
			}

			result := manager.ExtractClientSessionID(tc.Body, headers)
			if result.SessionID != tc.WantSession {
				t.Fatalf("node evidence %s: expected session id %q, got %q", tc.NodeEvidence, tc.WantSession, result.SessionID)
			}
			if string(result.Source) != tc.WantSource {
				t.Fatalf("node evidence %s: expected source %q, got %q", tc.NodeEvidence, tc.WantSource, result.Source)
			}
		})
	}
}

func loadSessionManagerFixture(t *testing.T) sessionManagerFixture {
	t.Helper()

	path := filepath.Join("..", "go-parity", "fixtures", "session-manager-extract-cases.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture file %s: %v", path, err)
	}

	var fixture sessionManagerFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("failed to decode fixture file %s: %v", path, err)
	}

	return fixture
}
