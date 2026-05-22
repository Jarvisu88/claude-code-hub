package repository

import (
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
)

func TestUsageLedgerModelCarriesImmutableFields(t *testing.T) {
	now := time.Now()
	entry := &model.UsageLedger{
		RequestID:       1,
		UserID:          2,
		Key:             "proxy-key",
		ProviderID:      3,
		FinalProviderID: 4,
		CreatedAt:       now,
	}
	if entry.RequestID != 1 || entry.FinalProviderID != 4 || entry.CreatedAt.IsZero() {
		t.Fatalf("expected immutable ledger fields to be preserved, got %+v", entry)
	}
}
