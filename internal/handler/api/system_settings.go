package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

type systemSettingsStore interface {
	Get(ctx context.Context) (*model.SystemSettings, error)
	UpdateFields(ctx context.Context, id int, fields map[string]any) (*model.SystemSettings, error)
}

type systemSettingsAuthenticator interface {
	AuthenticateAdminToken(token string) (*authsvc.AuthResult, error)
	AuthenticateProxy(ctx context.Context, input authsvc.ProxyAuthInput) (*authsvc.AuthResult, error)
}

type SystemSettingsHandler struct {
	auth  systemSettingsAuthenticator
	store systemSettingsStore
}

func NewSystemSettingsHandler(auth systemSettingsAuthenticator, store systemSettingsStore) *SystemSettingsHandler {
	return &SystemSettingsHandler{auth: auth, store: store}
}

func (h *SystemSettingsHandler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("")
	protected.Use((&Handler{auth: h.auth}).AdminAuthMiddleware())
	protected.GET("", h.get)
	protected.PUT("", h.update)
}

func (h *SystemSettingsHandler) get(c *gin.Context) {
	if !h.ensureAuthenticated(c) {
		return
	}
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("系统设置仓储未初始化"))
		return
	}
	settings, err := h.store.Get(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *SystemSettingsHandler) update(c *gin.Context) {
	if !h.ensureAdmin(c) {
		return
	}
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("系统设置仓储未初始化"))
		return
	}
	current, err := h.store.Get(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}

	var input struct {
		SiteTitle                           *string        `json:"siteTitle"`
		AllowGlobalUsageView                *bool          `json:"allowGlobalUsageView"`
		CurrencyDisplay                     *string        `json:"currencyDisplay"`
		BillingModelSource                  *string        `json:"billingModelSource"`
		CodexPriorityBillingSource          *string        `json:"codexPriorityBillingSource"`
		Timezone                            *string        `json:"timezone"`
		EnableAutoCleanup                   *bool          `json:"enableAutoCleanup"`
		CleanupRetentionDays                *int           `json:"cleanupRetentionDays"`
		CleanupSchedule                     *string        `json:"cleanupSchedule"`
		CleanupBatchSize                    *int           `json:"cleanupBatchSize"`
		EnableClientVersionCheck            *bool          `json:"enableClientVersionCheck"`
		VerboseProviderError                *bool          `json:"verboseProviderError"`
		EnableHTTP2                         *bool          `json:"enableHttp2"`
		EnableHighConcurrencyMode           *bool          `json:"enableHighConcurrencyMode"`
		InterceptAnthropicWarmupRequests    *bool          `json:"interceptAnthropicWarmupRequests"`
		EnableThinkingSignatureRectifier    *bool          `json:"enableThinkingSignatureRectifier"`
		EnableThinkingBudgetRectifier       *bool          `json:"enableThinkingBudgetRectifier"`
		EnableBillingHeaderRectifier        *bool          `json:"enableBillingHeaderRectifier"`
		EnableResponseInputRectifier        *bool          `json:"enableResponseInputRectifier"`
		EnableCodexSessionIDCompletion      *bool          `json:"enableCodexSessionIdCompletion"`
		EnableClaudeMetadataUserIDInjection *bool          `json:"enableClaudeMetadataUserIdInjection"`
		EnableResponseFixer                 *bool          `json:"enableResponseFixer"`
		ResponseFixerConfig                 map[string]any `json:"responseFixerConfig"`
		QuotaDbRefreshIntervalSeconds       *int           `json:"quotaDbRefreshIntervalSeconds"`
		QuotaLeasePercent5h                 *float64       `json:"quotaLeasePercent5h"`
		QuotaLeasePercentDaily              *float64       `json:"quotaLeasePercentDaily"`
		QuotaLeasePercentWeekly             *float64       `json:"quotaLeasePercentWeekly"`
		QuotaLeasePercentMonthly            *float64       `json:"quotaLeasePercentMonthly"`
		QuotaLeaseCapUsd                    *float64       `json:"quotaLeaseCapUsd"`
		IpExtractionConfig                  map[string]any `json:"ipExtractionConfig"`
		IpGeoLookupEnabled                  *bool          `json:"ipGeoLookupEnabled"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}

	fields := map[string]any{}
	if input.SiteTitle != nil {
		trimmed := strings.TrimSpace(*input.SiteTitle)
		if trimmed == "" {
			writeAdminError(c, appErrors.NewInvalidRequest("siteTitle 不能为空"))
			return
		}
		fields["site_title"] = trimmed
	}
	if input.AllowGlobalUsageView != nil {
		fields["allow_global_usage_view"] = *input.AllowGlobalUsageView
	}
	if input.CurrencyDisplay != nil {
		trimmed := strings.TrimSpace(*input.CurrencyDisplay)
		if trimmed == "" {
			writeAdminError(c, appErrors.NewInvalidRequest("currencyDisplay 不能为空"))
			return
		}
		fields["currency_display"] = trimmed
	}
	if input.BillingModelSource != nil {
		trimmed := strings.TrimSpace(*input.BillingModelSource)
		if trimmed == "" {
			writeAdminError(c, appErrors.NewInvalidRequest("billingModelSource 不能为空"))
			return
		}
		fields["billing_model_source"] = trimmed
	}
	if input.CodexPriorityBillingSource != nil {
		trimmed := strings.TrimSpace(*input.CodexPriorityBillingSource)
		if trimmed == "" {
			writeAdminError(c, appErrors.NewInvalidRequest("codexPriorityBillingSource 不能为空"))
			return
		}
		fields["codex_priority_billing_source"] = trimmed
	}
	if input.Timezone != nil {
		trimmed := strings.TrimSpace(*input.Timezone)
		if trimmed == "" {
			fields["timezone"] = nil
		} else {
			fields["timezone"] = trimmed
		}
	}
	if input.EnableAutoCleanup != nil {
		fields["enable_auto_cleanup"] = *input.EnableAutoCleanup
	}
	if input.CleanupRetentionDays != nil {
		if *input.CleanupRetentionDays < 1 || *input.CleanupRetentionDays > 365 {
			writeAdminError(c, appErrors.NewInvalidRequest("cleanupRetentionDays 必须在 1-365 之间"))
			return
		}
		fields["cleanup_retention_days"] = *input.CleanupRetentionDays
	}
	if input.CleanupSchedule != nil {
		trimmed := strings.TrimSpace(*input.CleanupSchedule)
		if trimmed == "" {
			writeAdminError(c, appErrors.NewInvalidRequest("cleanupSchedule 不能为空"))
			return
		}
		fields["cleanup_schedule"] = trimmed
	}
	if input.CleanupBatchSize != nil {
		if *input.CleanupBatchSize < 1000 || *input.CleanupBatchSize > 100000 {
			writeAdminError(c, appErrors.NewInvalidRequest("cleanupBatchSize 必须在 1000-100000 之间"))
			return
		}
		fields["cleanup_batch_size"] = *input.CleanupBatchSize
	}
	if input.EnableClientVersionCheck != nil {
		fields["enable_client_version_check"] = *input.EnableClientVersionCheck
	}
	if input.VerboseProviderError != nil {
		fields["verbose_provider_error"] = *input.VerboseProviderError
	}
	if input.EnableHTTP2 != nil {
		fields["enable_http2"] = *input.EnableHTTP2
	}
	if input.EnableHighConcurrencyMode != nil {
		fields["enable_high_concurrency_mode"] = *input.EnableHighConcurrencyMode
	}
	if input.InterceptAnthropicWarmupRequests != nil {
		fields["intercept_anthropic_warmup_requests"] = *input.InterceptAnthropicWarmupRequests
	}
	if input.EnableThinkingSignatureRectifier != nil {
		fields["enable_thinking_signature_rectifier"] = *input.EnableThinkingSignatureRectifier
	}
	if input.EnableThinkingBudgetRectifier != nil {
		fields["enable_thinking_budget_rectifier"] = *input.EnableThinkingBudgetRectifier
	}
	if input.EnableBillingHeaderRectifier != nil {
		fields["enable_billing_header_rectifier"] = *input.EnableBillingHeaderRectifier
	}
	if input.EnableResponseInputRectifier != nil {
		fields["enable_response_input_rectifier"] = *input.EnableResponseInputRectifier
	}
	if input.EnableCodexSessionIDCompletion != nil {
		fields["enable_codex_session_id_completion"] = *input.EnableCodexSessionIDCompletion
	}
	if input.EnableClaudeMetadataUserIDInjection != nil {
		fields["enable_claude_metadata_user_id_injection"] = *input.EnableClaudeMetadataUserIDInjection
	}
	if input.EnableResponseFixer != nil {
		fields["enable_response_fixer"] = *input.EnableResponseFixer
	}
	if input.ResponseFixerConfig != nil {
		fields["response_fixer_config"] = input.ResponseFixerConfig
	}
	if input.QuotaDbRefreshIntervalSeconds != nil {
		fields["quota_db_refresh_interval_seconds"] = *input.QuotaDbRefreshIntervalSeconds
	}
	if input.QuotaLeasePercent5h != nil {
		fields["quota_lease_percent_5h"] = *input.QuotaLeasePercent5h
	}
	if input.QuotaLeasePercentDaily != nil {
		fields["quota_lease_percent_daily"] = *input.QuotaLeasePercentDaily
	}
	if input.QuotaLeasePercentWeekly != nil {
		fields["quota_lease_percent_weekly"] = *input.QuotaLeasePercentWeekly
	}
	if input.QuotaLeasePercentMonthly != nil {
		fields["quota_lease_percent_monthly"] = *input.QuotaLeasePercentMonthly
	}
	if input.QuotaLeaseCapUsd != nil {
		fields["quota_lease_cap_usd"] = *input.QuotaLeaseCapUsd
	}
	if input.IpExtractionConfig != nil {
		fields["ip_extraction_config"] = input.IpExtractionConfig
	}
	if input.IpGeoLookupEnabled != nil {
		fields["ip_geo_lookup_enabled"] = *input.IpGeoLookupEnabled
	}

	updated, err := h.store.UpdateFields(c.Request.Context(), current.ID, fields)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *SystemSettingsHandler) ensureAuthenticated(c *gin.Context) bool {
	if h == nil || h.auth == nil {
		writeAdminError(c, appErrors.NewInternalError("系统设置鉴权服务未初始化"))
		return false
	}
	token := resolveAdminToken(c)
	if authResult, err := h.auth.AuthenticateAdminToken(token); err == nil && authResult != nil {
		return true
	}
	if authResult, err := h.auth.AuthenticateProxy(c.Request.Context(), authsvc.ProxyAuthInput{APIKeyHeader: token}); err == nil && authResult != nil {
		return true
	}
	writeAdminError(c, appErrors.NewAuthenticationError("未授权，请先登录", appErrors.CodeUnauthorized))
	return false
}

func (h *SystemSettingsHandler) ensureAdmin(c *gin.Context) bool {
	if h == nil || h.auth == nil {
		writeAdminError(c, appErrors.NewInternalError("系统设置鉴权服务未初始化"))
		return false
	}
	authResult, err := h.auth.AuthenticateAdminToken(resolveAdminToken(c))
	if err != nil {
		writeAdminError(c, err)
		return false
	}
	if authResult == nil || !authResult.IsAdmin {
		writeAdminError(c, appErrors.NewPermissionDenied("权限不足", appErrors.CodePermissionDenied))
		return false
	}
	return true
}
