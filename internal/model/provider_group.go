package model

import (
	"time"

	"github.com/quagmt/udecimal"
	"github.com/uptrace/bun"
)

const DefaultProviderGroupName = "default"

// ProviderGroup mirrors Node's provider_groups table.
type ProviderGroup struct {
	bun.BaseModel `bun:"table:provider_groups,alias:pg"`

	ID             int              `bun:"id,pk,autoincrement" json:"id"`
	Name           string           `bun:"name,notnull,unique" json:"name"`
	CostMultiplier udecimal.Decimal `bun:"cost_multiplier,type:numeric(10,4),notnull,default:1.0" json:"costMultiplier"`
	Description    *string          `bun:"description" json:"description"`
	CreatedAt      time.Time        `bun:"created_at,notnull,default:current_timestamp" json:"createdAt"`
	UpdatedAt      time.Time        `bun:"updated_at,notnull,default:current_timestamp" json:"updatedAt"`
}

func (g *ProviderGroup) Normalize() {
	if g == nil {
		return
	}
	if g.CostMultiplier.IsZero() {
		g.CostMultiplier = udecimal.MustParse("1.0")
	}
}
