package model

import (
	"time"

	"github.com/uptrace/bun"
)

// ErrorRule represents a managed upstream error rewrite/classification rule.
type ErrorRule struct {
	bun.BaseModel `bun:"table:error_rules,alias:er"`

	ID                 int            `bun:"id,pk,autoincrement" json:"id"`
	Pattern            string         `bun:"pattern,notnull" json:"pattern"`
	MatchType          string         `bun:"match_type,notnull,default:'regex'" json:"matchType"`
	Category           string         `bun:"category,notnull" json:"category"`
	Description        *string        `bun:"description" json:"description"`
	OverrideResponse   map[string]any `bun:"override_response,type:jsonb" json:"overrideResponse"`
	OverrideStatusCode *int           `bun:"override_status_code" json:"overrideStatusCode"`
	IsEnabled          bool           `bun:"is_enabled,notnull,default:true" json:"isEnabled"`
	IsDefault          bool           `bun:"is_default,notnull,default:false" json:"isDefault"`
	Priority           int            `bun:"priority,notnull,default:0" json:"priority"`

	CreatedAt time.Time  `bun:"created_at,notnull,default:current_timestamp" json:"createdAt"`
	UpdatedAt time.Time  `bun:"updated_at,notnull,default:current_timestamp" json:"updatedAt"`
	DeletedAt *time.Time `bun:"deleted_at,soft_delete" json:"deletedAt,omitempty"`
}

// IsActive reports whether the rule should be considered active.
func (e *ErrorRule) IsActive() bool {
	return e.IsEnabled && e.DeletedAt == nil
}
