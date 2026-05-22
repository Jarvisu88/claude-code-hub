package go_parity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ding113/claude-code-hub/internal/config"
	"github.com/ding113/claude-code-hub/internal/service/session"
)

type sessionManagerHashFixture struct {
	Source []string                        `json:"source"`
	Cases  []sessionManagerHashFixtureCase `json:"cases"`
}

type sessionManagerHashFixtureCase struct {
	Name         string `json:"name"`
	Messages     any    `json:"messages"`
	WantHash     string `json:"want_hash"`
	NodeEvidence string `json:"node_evidence"`
}

func TestSessionManagerCalculateMessagesHashParityCases(t *testing.T) {
	fixture := loadSessionManagerHashFixture(t)
	manager := session.NewManager(config.SessionConfig{TTL: 300}, nil)

	for _, tc := range fixture.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			if got := manager.CalculateMessagesHash(tc.Messages); got != tc.WantHash {
				t.Fatalf("node evidence %s: expected hash %q, got %q", tc.NodeEvidence, tc.WantHash, got)
			}
		})
	}
}

func loadSessionManagerHashFixture(t *testing.T) sessionManagerHashFixture {
	t.Helper()

	path := filepath.Join("..", "go-parity", "fixtures", "session-manager-hash-cases.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture file %s: %v", path, err)
	}

	var fixture sessionManagerHashFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("failed to decode fixture file %s: %v", path, err)
	}

	return fixture
}
