package model

import (
	"time"

	"github.com/uptrace/bun"
)

// AuditLog mirrors Node's audit_log table.
type AuditLog struct {
	bun.BaseModel `bun:"table:audit_log,alias:al"`

	ID               int            `bun:"id,pk,autoincrement" json:"id"`
	ActionCategory   string         `bun:"action_category,notnull" json:"actionCategory"`
	ActionType       string         `bun:"action_type,notnull" json:"actionType"`
	TargetType       *string        `bun:"target_type" json:"targetType"`
	TargetID         *string        `bun:"target_id" json:"targetId"`
	TargetName       *string        `bun:"target_name" json:"targetName"`
	BeforeValue      map[string]any `bun:"before_value,type:jsonb" json:"beforeValue"`
	AfterValue       map[string]any `bun:"after_value,type:jsonb" json:"afterValue"`
	OperatorUserID   *int           `bun:"operator_user_id" json:"operatorUserId"`
	OperatorUserName *string        `bun:"operator_user_name" json:"operatorUserName"`
	OperatorKeyID    *int           `bun:"operator_key_id" json:"operatorKeyId"`
	OperatorKeyName  *string        `bun:"operator_key_name" json:"operatorKeyName"`
	OperatorIP       *string        `bun:"operator_ip" json:"operatorIp"`
	UserAgent        *string        `bun:"user_agent" json:"userAgent"`
	Success          bool           `bun:"success,notnull" json:"success"`
	ErrorMessage     *string        `bun:"error_message" json:"errorMessage"`
	CreatedAt        time.Time      `bun:"created_at,notnull,default:current_timestamp" json:"createdAt"`
}

type AuditLogFilter struct {
	Category       *string
	ActionType     *string
	OperatorUserID *int
	OperatorIP     *string
	TargetType     *string
	TargetID       *string
	Success        *bool
	From           *time.Time
	To             *time.Time
}

type AuditLogCursor struct {
	CreatedAt time.Time
	ID        int
}
