package model

import (
	"time"

	"github.com/uptrace/bun"
)

// SensitiveWord represents a managed sensitive word rule.
type SensitiveWord struct {
	bun.BaseModel `bun:"table:sensitive_words,alias:sw"`

	ID          int     `bun:"id,pk,autoincrement" json:"id"`
	Word        string  `bun:"word,notnull" json:"word"`
	MatchType   string  `bun:"match_type,notnull,default:'contains'" json:"matchType"`
	Description *string `bun:"description" json:"description"`
	IsEnabled   bool    `bun:"is_enabled,notnull,default:true" json:"isEnabled"`

	CreatedAt time.Time  `bun:"created_at,notnull,default:current_timestamp" json:"createdAt"`
	UpdatedAt time.Time  `bun:"updated_at,notnull,default:current_timestamp" json:"updatedAt"`
	DeletedAt *time.Time `bun:"deleted_at,soft_delete" json:"deletedAt,omitempty"`
}

// IsActive reports whether the rule should be considered active.
func (s *SensitiveWord) IsActive() bool {
	return s.IsEnabled && s.DeletedAt == nil
}
