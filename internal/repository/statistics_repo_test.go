package repository

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

func TestBuildUsageLedgerCostSumQueryUsesUsageLedger(t *testing.T) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN("postgres://user:pass@localhost:5432/test?sslmode=disable")))
	defer sqldb.Close()

	db := bun.NewDB(sqldb, pgdialect.New())
	start := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	query := buildUsageLedgerCostSumQuery(db, "user_id = ?", 7, &start, nil)
	sqlText := query.String()
	if !strings.Contains(sqlText, "\"usage_ledger\"") {
		t.Fatalf("expected usage_ledger table, got %s", sqlText)
	}
	if !strings.Contains(sqlText, "blocked_by IS NULL") {
		t.Fatalf("expected blocked_by guard, got %s", sqlText)
	}
	if !strings.Contains(sqlText, "user_id = 7") {
		t.Fatalf("expected user filter, got %s", sqlText)
	}
}

func TestBuildUsageLedgerCostEntriesQueryUsesFinalProvider(t *testing.T) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN("postgres://user:pass@localhost:5432/test?sslmode=disable")))
	defer sqldb.Close()

	db := bun.NewDB(sqldb, pgdialect.New())
	start := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	query := buildUsageLedgerCostEntriesQuery(db, "final_provider_id = ?", 9, start, end)
	sqlText := query.String()
	if !strings.Contains(sqlText, "\"usage_ledger\"") {
		t.Fatalf("expected usage_ledger table, got %s", sqlText)
	}
	if !strings.Contains(sqlText, "final_provider_id = 9") {
		t.Fatalf("expected final provider filter, got %s", sqlText)
	}
	if strings.Contains(sqlText, "\"message_request\"") {
		t.Fatalf("did not expect message_request table, got %s", sqlText)
	}
}

var _ = model.UsageLedger{}
