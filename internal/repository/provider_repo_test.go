package repository

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProviderStatisticsQueryUsesUsageLedgerAndFinalProviderID(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve current file")
	}
	path := filepath.Join(filepath.Dir(currentFile), "provider_repo.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read provider repo source: %v", err)
	}
	source := string(content)
	if !strings.Contains(source, "LEFT JOIN usage_ledger ul") {
		t.Fatalf("expected provider statistics query to read usage_ledger, got %s", source)
	}
	if !strings.Contains(source, "ul.final_provider_id = p.id") {
		t.Fatalf("expected provider statistics query to use final_provider_id, got %s", source)
	}
	if strings.Contains(source, "FROM message_request") && strings.Contains(source, "GetProviderStatistics") {
		t.Fatalf("expected GetProviderStatistics to stop using message_request directly")
	}
}
