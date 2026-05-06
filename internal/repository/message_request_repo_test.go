package repository

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

func TestApplyMessageRequestQueryFiltersIncludesMinRetryCountExpression(t *testing.T) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN("postgres://user:pass@localhost:5432/test?sslmode=disable")))
	defer sqldb.Close()

	db := bun.NewDB(sqldb, pgdialect.New())
	query := db.NewSelect().
		Model((*model.MessageRequest)(nil)).
		Where("mr.deleted_at IS NULL")

	minRetryCount := 2
	query = applyMessageRequestQueryFilters(query, MessageRequestQueryFilters{
		SessionID:     "sess_123",
		MinRetryCount: &minRetryCount,
	}, false)

	sqlText := query.String()
	if !strings.Contains(sqlText, "jsonb_array_elements(COALESCE(mr.provider_chain, '[]'::jsonb))") {
		t.Fatalf("expected retry-count SQL expression, got %s", sqlText)
	}
	if !strings.Contains(sqlText, "mr.session_id = 'sess_123'") {
		t.Fatalf("expected session filter in query, got %s", sqlText)
	}
	if !strings.Contains(sqlText, ") >= 2") {
		t.Fatalf("expected minRetryCount threshold in query, got %s", sqlText)
	}
}

func TestApplyMessageRequestQueryFiltersSkipsNonPositiveMinRetryCount(t *testing.T) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN("postgres://user:pass@localhost:5432/test?sslmode=disable")))
	defer sqldb.Close()

	db := bun.NewDB(sqldb, pgdialect.New())
	query := db.NewSelect().
		Model((*model.MessageRequest)(nil)).
		Where("mr.deleted_at IS NULL")

	minRetryCount := 0
	query = applyMessageRequestQueryFilters(query, MessageRequestQueryFilters{
		MinRetryCount: &minRetryCount,
	}, false)

	sqlText := query.String()
	if strings.Contains(sqlText, "jsonb_array_elements(COALESCE(mr.provider_chain, '[]'::jsonb))") {
		t.Fatalf("expected retry-count SQL expression to be omitted, got %s", sqlText)
	}
}

func TestListBatchRejectsInvalidCursorTimestamp(t *testing.T) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN("postgres://user:pass@localhost:5432/test?sslmode=disable")))
	defer sqldb.Close()

	db := bun.NewDB(sqldb, pgdialect.New())
	repo := NewMessageRequestRepository(db)

	_, err := repo.ListBatch(t.Context(), MessageRequestBatchFilters{
		Cursor: &MessageRequestBatchCursor{
			CreatedAt: "not-a-time",
			ID:        1,
		},
		Limit: 20,
	})
	if err == nil || !strings.Contains(err.Error(), "cursor.createdAt") {
		t.Fatalf("expected invalid cursor timestamp error, got %v", err)
	}
}

func TestBuildUsageLedgerFromMessageRequestUsesFinalProviderFromChain(t *testing.T) {
	statusCode := 201
	inputTokens := 12
	req := &model.MessageRequest{
		ID:          7,
		ProviderID:  1,
		UserID:      2,
		Key:         "proxy-key",
		Model:       "gpt-5.4",
		StatusCode:  &statusCode,
		InputTokens: &inputTokens,
		ProviderChain: []model.ProviderChainItem{
			{ID: 1, Name: "primary"},
			{ID: 9, Name: "final"},
		},
	}
	entry := buildUsageLedgerFromMessageRequest(req)
	if entry == nil {
		t.Fatal("expected usage ledger entry")
	}
	if entry.RequestID != 7 || entry.ProviderID != 1 || entry.FinalProviderID != 9 {
		t.Fatalf("expected final provider to come from provider chain, got %+v", entry)
	}
	if entry.InputTokens == nil || *entry.InputTokens != 12 {
		t.Fatalf("expected input tokens copied, got %+v", entry)
	}
}
