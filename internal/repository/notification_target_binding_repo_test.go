package repository

import (
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
)

func TestNotificationTargetBindingRepositoryRejectsDuplicateTargetIDsBeforeDBAccess(t *testing.T) {
	repo := &notificationTargetBindingRepository{}
	_, err := repo.ReplaceByNotificationType(t.Context(), "cost_alert", []*model.NotificationTargetBinding{
		{TargetID: 1},
		{TargetID: 1},
	})
	if err == nil {
		t.Fatal("expected duplicate target validation error")
	}
	if !appErrors.Is(err, appErrors.ErrorTypeInvalidRequest) {
		t.Fatalf("expected invalid request error, got %v", err)
	}
}
