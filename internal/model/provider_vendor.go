package model

import (
	"time"

	"github.com/uptrace/bun"
)

// ProviderVendor mirrors Node's provider_vendors table.
type ProviderVendor struct {
	bun.BaseModel `bun:"table:provider_vendors,alias:pv"`

	ID            int       `bun:"id,pk,autoincrement" json:"id"`
	WebsiteDomain string    `bun:"website_domain,notnull" json:"websiteDomain"`
	DisplayName   *string   `bun:"display_name" json:"displayName"`
	WebsiteURL    *string   `bun:"website_url" json:"websiteUrl"`
	FaviconURL    *string   `bun:"favicon_url" json:"faviconUrl"`
	CreatedAt     time.Time `bun:"created_at,notnull,default:current_timestamp" json:"createdAt"`
	UpdatedAt     time.Time `bun:"updated_at,notnull,default:current_timestamp" json:"updatedAt"`
}
