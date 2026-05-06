package model

import (
	"time"

	"github.com/uptrace/bun"
)

// RequestFilter represents an admin-managed request filter rule.
type RequestFilter struct {
	bun.BaseModel `bun:"table:request_filters,alias:rf"`

	ID          int     `bun:"id,pk,autoincrement" json:"id"`
	Name        string  `bun:"name,notnull" json:"name"`
	Description *string `bun:"description" json:"description"`
	Scope       string  `bun:"scope,notnull" json:"scope"`
	Action      string  `bun:"action,notnull" json:"action"`
	MatchType   *string `bun:"match_type" json:"matchType"`
	Target      string  `bun:"target,notnull" json:"target"`
	Replacement any     `bun:"replacement,type:jsonb" json:"replacement"`
	Priority    int     `bun:"priority,notnull,default:0" json:"priority"`
	IsEnabled   bool    `bun:"is_enabled,notnull,default:true" json:"isEnabled"`

	BindingType string   `bun:"binding_type,notnull,default:'global'" json:"bindingType"`
	ProviderIds []int    `bun:"provider_ids,type:jsonb" json:"providerIds"`
	GroupTags   []string `bun:"group_tags,type:jsonb" json:"groupTags"`

	RuleMode       string           `bun:"rule_mode,notnull,default:'simple'" json:"ruleMode"`
	ExecutionPhase string           `bun:"execution_phase,notnull,default:'guard'" json:"executionPhase"`
	Operations     []map[string]any `bun:"operations,type:jsonb" json:"operations"`

	CreatedAt time.Time  `bun:"created_at,notnull,default:current_timestamp" json:"createdAt"`
	UpdatedAt time.Time  `bun:"updated_at,notnull,default:current_timestamp" json:"updatedAt"`
	DeletedAt *time.Time `bun:"deleted_at,soft_delete" json:"deletedAt,omitempty"`
}

// IsActive reports whether the filter should be considered active.
func (r *RequestFilter) IsActive() bool {
	return r.IsEnabled && r.DeletedAt == nil
}

// IsGlobal reports whether the filter applies globally.
func (r *RequestFilter) IsGlobal() bool {
	return r.BindingType == string(RequestFilterBindingTypeGlobal)
}

// AppliesToProvider reports whether the filter applies to the provider/group pair.
func (r *RequestFilter) AppliesToProvider(providerID int, groupTag *string) bool {
	if r.IsGlobal() {
		return true
	}

	if r.BindingType == string(RequestFilterBindingTypeProviders) {
		for _, id := range r.ProviderIds {
			if id == providerID {
				return true
			}
		}
	}

	if r.BindingType == string(RequestFilterBindingTypeGroups) && groupTag != nil {
		for _, tag := range r.GroupTags {
			if tag == *groupTag {
				return true
			}
		}
	}

	return false
}
