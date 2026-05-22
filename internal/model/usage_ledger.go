package model

import (
	"time"

	"github.com/quagmt/udecimal"
	"github.com/uptrace/bun"
)

// UsageLedger mirrors Node's immutable usage_ledger table.
type UsageLedger struct {
	bun.BaseModel `bun:"table:usage_ledger,alias:ul"`

	ID                         int               `bun:"id,pk,autoincrement" json:"id"`
	RequestID                  int               `bun:"request_id,notnull" json:"requestId"`
	UserID                     int               `bun:"user_id,notnull" json:"userId"`
	Key                        string            `bun:"key,notnull" json:"key"`
	ProviderID                 int               `bun:"provider_id,notnull" json:"providerId"`
	FinalProviderID            int               `bun:"final_provider_id,notnull" json:"finalProviderId"`
	Model                      *string           `bun:"model" json:"model"`
	OriginalModel              *string           `bun:"original_model" json:"originalModel"`
	Endpoint                   *string           `bun:"endpoint" json:"endpoint"`
	APIType                    *string           `bun:"api_type" json:"apiType"`
	SessionID                  *string           `bun:"session_id" json:"sessionId"`
	StatusCode                 *int              `bun:"status_code" json:"statusCode"`
	IsSuccess                  bool              `bun:"is_success,notnull,default:false" json:"isSuccess"`
	BlockedBy                  *string           `bun:"blocked_by" json:"blockedBy"`
	CostUSD                    udecimal.Decimal  `bun:"cost_usd,type:numeric(21,15),default:0" json:"costUsd"`
	CostMultiplier             *udecimal.Decimal `bun:"cost_multiplier,type:numeric(10,4)" json:"costMultiplier"`
	GroupCostMultiplier        *udecimal.Decimal `bun:"group_cost_multiplier,type:numeric(10,4)" json:"groupCostMultiplier"`
	InputTokens                *int64            `bun:"input_tokens" json:"inputTokens"`
	OutputTokens               *int64            `bun:"output_tokens" json:"outputTokens"`
	CacheCreationInputTokens   *int64            `bun:"cache_creation_input_tokens" json:"cacheCreationInputTokens"`
	CacheReadInputTokens       *int64            `bun:"cache_read_input_tokens" json:"cacheReadInputTokens"`
	CacheCreation5mInputTokens *int64            `bun:"cache_creation_5m_input_tokens" json:"cacheCreation5mInputTokens"`
	CacheCreation1hInputTokens *int64            `bun:"cache_creation_1h_input_tokens" json:"cacheCreation1hInputTokens"`
	CacheTtlApplied            *string           `bun:"cache_ttl_applied" json:"cacheTtlApplied"`
	Context1mApplied           bool              `bun:"context_1m_applied,default:false" json:"context1mApplied"`
	SwapCacheTtlApplied        bool              `bun:"swap_cache_ttl_applied,default:false" json:"swapCacheTtlApplied"`
	DurationMs                 *int              `bun:"duration_ms" json:"durationMs"`
	TtfbMs                     *int              `bun:"ttfb_ms" json:"ttfbMs"`
	ClientIP                   *string           `bun:"client_ip" json:"clientIp"`
	CreatedAt                  time.Time         `bun:"created_at,notnull" json:"createdAt"`
}
