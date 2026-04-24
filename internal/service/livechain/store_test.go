package livechain

import (
	"context"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
)

func TestInferPhase(t *testing.T) {
	if phase := InferPhase(nil); phase != "queued" {
		t.Fatalf("expected queued for empty chain, got %s", phase)
	}
	if phase := InferPhase([]model.ProviderChainItem{{Reason: stringPtr("initial_selection")}}); phase != "provider_selected" {
		t.Fatalf("expected provider_selected, got %s", phase)
	}
	if phase := InferPhase([]model.ProviderChainItem{{Reason: stringPtr("hedge_triggered")}}); phase != "hedge_racing" {
		t.Fatalf("expected hedge_racing, got %s", phase)
	}
	if phase := InferPhase([]model.ProviderChainItem{{Reason: stringPtr("request_success")}}); phase != "streaming" {
		t.Fatalf("expected streaming, got %s", phase)
	}
}

func TestMemoryStoreWriteReadBatchDelete(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	if err := Write(context.Background(), "sess_live", 2, []model.ProviderChainItem{{Name: "provider-a", Reason: stringPtr("initial_selection")}}); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	results, err := ReadBatch(context.Background(), []Key{{SessionID: "sess_live", RequestSequence: 2}})
	if err != nil {
		t.Fatalf("read batch failed: %v", err)
	}
	snapshot, ok := results["sess_live:2"]
	if !ok {
		t.Fatalf("expected snapshot in results, got %+v", results)
	}
	if snapshot.Phase != "provider_selected" || len(snapshot.Chain) != 1 || snapshot.Chain[0].Name != "provider-a" {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if err := Delete(context.Background(), "sess_live", 2); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	results, err = ReadBatch(context.Background(), []Key{{SessionID: "sess_live", RequestSequence: 2}})
	if err != nil {
		t.Fatalf("read batch after delete failed: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty results after delete, got %+v", results)
	}
}

func TestMemoryStoreExpiresEntries(t *testing.T) {
	originalNow := now
	base := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	now = func() time.Time { return base }
	defer func() {
		now = originalNow
		ResetForTest()
	}()

	currentStore = newMemoryStore(1 * time.Second)
	if err := Write(context.Background(), "sess_ttl", 1, []model.ProviderChainItem{{Name: "provider-a"}}); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	now = func() time.Time { return base.Add(2 * time.Second) }
	results, err := ReadBatch(context.Background(), []Key{{SessionID: "sess_ttl", RequestSequence: 1}})
	if err != nil {
		t.Fatalf("read batch failed: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected entry to expire, got %+v", results)
	}
}

func stringPtr(value string) *string { return &value }
