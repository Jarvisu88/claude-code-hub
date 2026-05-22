package repository

import (
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
)

func TestProviderEndpointIsActive(t *testing.T) {
	item := &model.ProviderEndpoint{IsEnabled: true}
	if !item.IsActive() {
		t.Fatal("expected enabled non-deleted endpoint to be active")
	}
	now := time.Now()
	item.DeletedAt = &now
	if item.IsActive() {
		t.Fatal("expected deleted endpoint to be inactive")
	}
}
