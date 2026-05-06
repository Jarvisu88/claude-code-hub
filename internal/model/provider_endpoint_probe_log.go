package model

import (
	"time"

	"github.com/uptrace/bun"
)

// ProviderEndpointProbeLog mirrors Node's provider_endpoint_probe_logs table.
type ProviderEndpointProbeLog struct {
	bun.BaseModel `bun:"table:provider_endpoint_probe_logs,alias:pepl"`

	ID           int       `bun:"id,pk,autoincrement" json:"id"`
	EndpointID   int       `bun:"endpoint_id,notnull" json:"endpointId"`
	Source       string    `bun:"source,notnull,default:'scheduled'" json:"source"`
	Ok           bool      `bun:"ok,notnull" json:"ok"`
	StatusCode   *int      `bun:"status_code" json:"statusCode"`
	LatencyMs    *int      `bun:"latency_ms" json:"latencyMs"`
	ErrorType    *string   `bun:"error_type" json:"errorType"`
	ErrorMessage *string   `bun:"error_message" json:"errorMessage"`
	CreatedAt    time.Time `bun:"created_at,notnull,default:current_timestamp" json:"createdAt"`
}
