package repository

import (
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
)

func TestAuditLogFilterCarriesSupportedFields(t *testing.T) {
	category := "auth"
	actionType := "login.success"
	operatorUserID := 9
	operatorIP := "127.0.0.1"
	targetType := "user"
	targetID := "42"
	success := true
	now := time.Now()

	filter := &model.AuditLogFilter{
		Category:       &category,
		ActionType:     &actionType,
		OperatorUserID: &operatorUserID,
		OperatorIP:     &operatorIP,
		TargetType:     &targetType,
		TargetID:       &targetID,
		Success:        &success,
		From:           &now,
		To:             &now,
	}

	if filter.Category == nil || *filter.Category != "auth" || filter.OperatorUserID == nil || *filter.OperatorUserID != 9 {
		t.Fatalf("expected filter fields to be preserved, got %+v", filter)
	}
}
