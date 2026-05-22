package model

import (
	"time"

	"github.com/uptrace/bun"
)

// ProviderEndpoint mirrors Node's provider_endpoints table.
type ProviderEndpoint struct {
	bun.BaseModel `bun:"table:provider_endpoints,alias:pe"`

	ID                    int        `bun:"id,pk,autoincrement" json:"id"`
	VendorID              int        `bun:"vendor_id,notnull" json:"vendorId"`
	ProviderType          string     `bun:"provider_type,notnull,default:'claude'" json:"providerType"`
	URL                   string     `bun:"url,notnull" json:"url"`
	Label                 *string    `bun:"label" json:"label"`
	SortOrder             int        `bun:"sort_order,notnull,default:0" json:"sortOrder"`
	IsEnabled             bool       `bun:"is_enabled,notnull,default:true" json:"isEnabled"`
	LastProbedAt          *time.Time `bun:"last_probed_at" json:"lastProbedAt"`
	LastProbeOk           *bool      `bun:"last_probe_ok" json:"lastProbeOk"`
	LastProbeStatusCode   *int       `bun:"last_probe_status_code" json:"lastProbeStatusCode"`
	LastProbeLatencyMs    *int       `bun:"last_probe_latency_ms" json:"lastProbeLatencyMs"`
	LastProbeErrorType    *string    `bun:"last_probe_error_type" json:"lastProbeErrorType"`
	LastProbeErrorMessage *string    `bun:"last_probe_error_message" json:"lastProbeErrorMessage"`
	CreatedAt             time.Time  `bun:"created_at,notnull,default:current_timestamp" json:"createdAt"`
	UpdatedAt             time.Time  `bun:"updated_at,notnull,default:current_timestamp" json:"updatedAt"`
	DeletedAt             *time.Time `bun:"deleted_at,soft_delete" json:"deletedAt,omitempty"`
}

func (e *ProviderEndpoint) IsActive() bool {
	return e != nil && e.IsEnabled && e.DeletedAt == nil
}
