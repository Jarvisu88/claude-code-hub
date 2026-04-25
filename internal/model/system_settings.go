package model

import (
	"time"

	"github.com/uptrace/bun"
)

// SystemSettings 系统设置模型
type SystemSettings struct {
	bun.BaseModel `bun:"table:system_settings,alias:ss"`

	ID        int    `bun:"id,pk,autoincrement" json:"id"`
	SiteTitle string `bun:"site_title,notnull,default:'Claude Code Hub'" json:"siteTitle"`

	// 允许全局使用量查看
	AllowGlobalUsageView bool `bun:"allow_global_usage_view,notnull,default:false" json:"allowGlobalUsageView"`

	// 货币显示配置
	CurrencyDisplay string `bun:"currency_display,notnull,default:'USD'" json:"currencyDisplay"`

	// 计费模型来源配置: 'original' (重定向前) | 'redirected' (重定向后)
	BillingModelSource string `bun:"billing_model_source,notnull,default:'original'" json:"billingModelSource"`

	// Codex Priority 单独计费口径
	CodexPriorityBillingSource string `bun:"codex_priority_billing_source,notnull,default:'requested'" json:"codexPriorityBillingSource"`

	// 系统时区配置
	Timezone *string `bun:"timezone" json:"timezone"`

	// 日志清理配置
	EnableAutoCleanup    bool   `bun:"enable_auto_cleanup,default:false" json:"enableAutoCleanup"`
	CleanupRetentionDays *int   `bun:"cleanup_retention_days,default:30" json:"cleanupRetentionDays"`
	CleanupSchedule      string `bun:"cleanup_schedule,default:'0 2 * * *'" json:"cleanupSchedule"`
	CleanupBatchSize     *int   `bun:"cleanup_batch_size,default:10000" json:"cleanupBatchSize"`

	// 客户端版本检查配置
	EnableClientVersionCheck bool `bun:"enable_client_version_check,notnull,default:false" json:"enableClientVersionCheck"`

	// 供应商不可用时是否返回详细错误信息
	VerboseProviderError bool `bun:"verbose_provider_error,notnull,default:false" json:"verboseProviderError"`

	// 启用 HTTP/2 连接供应商（默认关闭，启用后自动回退到 HTTP/1.1 失败时）
	EnableHttp2 bool `bun:"enable_http2,notnull,default:false" json:"enableHttp2"`

	// 高并发模式
	EnableHighConcurrencyMode bool `bun:"enable_high_concurrency_mode,notnull,default:false" json:"enableHighConcurrencyMode"`

	// 可选拦截 Anthropic Warmup 请求（默认关闭）
	// 开启后：对 /v1/messages 的 Warmup 请求直接由 CCH 抢答，避免打到上游供应商
	InterceptAnthropicWarmupRequests bool `bun:"intercept_anthropic_warmup_requests,notnull,default:false" json:"interceptAnthropicWarmupRequests"`

	// thinking signature 整流器
	EnableThinkingSignatureRectifier bool `bun:"enable_thinking_signature_rectifier,notnull,default:true" json:"enableThinkingSignatureRectifier"`

	// thinking budget 整流器
	EnableThinkingBudgetRectifier bool `bun:"enable_thinking_budget_rectifier,notnull,default:true" json:"enableThinkingBudgetRectifier"`

	// billing header 整流器
	EnableBillingHeaderRectifier bool `bun:"enable_billing_header_rectifier,notnull,default:true" json:"enableBillingHeaderRectifier"`

	// Response API input 整流器
	EnableResponseInputRectifier bool `bun:"enable_response_input_rectifier,notnull,default:true" json:"enableResponseInputRectifier"`

	// Codex Session ID 补全
	EnableCodexSessionIDCompletion bool `bun:"enable_codex_session_id_completion,notnull,default:true" json:"enableCodexSessionIdCompletion"`

	// Claude metadata.user_id 注入
	EnableClaudeMetadataUserIDInjection bool `bun:"enable_claude_metadata_user_id_injection,notnull,default:true" json:"enableClaudeMetadataUserIdInjection"`

	// 响应整流
	EnableResponseFixer bool           `bun:"enable_response_fixer,notnull,default:true" json:"enableResponseFixer"`
	ResponseFixerConfig map[string]any `bun:"response_fixer_config,type:jsonb" json:"responseFixerConfig"`

	// Quota lease settings
	QuotaDbRefreshIntervalSeconds *int     `bun:"quota_db_refresh_interval_seconds" json:"quotaDbRefreshIntervalSeconds"`
	QuotaLeasePercent5h           *float64 `bun:"quota_lease_percent_5h" json:"quotaLeasePercent5h"`
	QuotaLeasePercentDaily        *float64 `bun:"quota_lease_percent_daily" json:"quotaLeasePercentDaily"`
	QuotaLeasePercentWeekly       *float64 `bun:"quota_lease_percent_weekly" json:"quotaLeasePercentWeekly"`
	QuotaLeasePercentMonthly      *float64 `bun:"quota_lease_percent_monthly" json:"quotaLeasePercentMonthly"`
	QuotaLeaseCapUsd              *float64 `bun:"quota_lease_cap_usd" json:"quotaLeaseCapUsd"`

	// 客户端 IP 提取链
	IpExtractionConfig map[string]any `bun:"ip_extraction_config,type:jsonb" json:"ipExtractionConfig"`

	// 是否启用 IP 归属地查询
	IpGeoLookupEnabled bool `bun:"ip_geo_lookup_enabled,notnull,default:true" json:"ipGeoLookupEnabled"`

	CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp" json:"createdAt"`
	UpdatedAt time.Time `bun:"updated_at,notnull,default:current_timestamp" json:"updatedAt"`
}

// GetCleanupRetentionDays 获取日志保留天数
func (s *SystemSettings) GetCleanupRetentionDays() int {
	if s.CleanupRetentionDays == nil {
		return 30
	}
	return *s.CleanupRetentionDays
}

// GetCleanupBatchSize 获取清理批次大小
func (s *SystemSettings) GetCleanupBatchSize() int {
	if s.CleanupBatchSize == nil {
		return 10000
	}
	return *s.CleanupBatchSize
}
