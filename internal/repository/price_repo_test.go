package repository

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

func TestBuildLatestModelPricesBaseQueryPrefersManualSource(t *testing.T) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN("postgres://user:pass@localhost:5432/test?sslmode=disable")))
	defer sqldb.Close()
	db := bun.NewDB(sqldb, pgdialect.New())

	query := buildLatestModelPricesBaseQuery(db, "gpt", "manual", "anthropic")
	sqlText := query.String()
	if !strings.Contains(sqlText, "DISTINCT ON (model_name)") {
		t.Fatalf("expected DISTINCT ON query, got %s", sqlText)
	}
	if !strings.Contains(sqlText, "(source = 'manual') DESC") {
		t.Fatalf("expected manual-first ordering, got %s", sqlText)
	}
	if !strings.Contains(sqlText, "price_data->>'litellm_provider' = 'anthropic'") {
		t.Fatalf("expected litellm provider filter, got %s", sqlText)
	}
}
