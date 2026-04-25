package api

import (
	"net/http"
	"strings"

	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/gin-gonic/gin"
)

type AdminSystemConfigHandler struct {
	auth  adminAuthenticator
	store systemSettingsStore
}

func NewAdminSystemConfigHandler(auth adminAuthenticator, store systemSettingsStore) *AdminSystemConfigHandler {
	return &AdminSystemConfigHandler{auth: auth, store: store}
}

func (h *AdminSystemConfigHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/api/admin/system-config", h.get)
	router.POST("/api/admin/system-config", h.update)
}

func (h *AdminSystemConfigHandler) get(c *gin.Context) {
	if !h.ensureAdmin(c) {
		return
	}
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("系统配置仓储未初始化"))
		return
	}
	settings, err := h.store.Get(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *AdminSystemConfigHandler) update(c *gin.Context) {
	if !h.ensureAdmin(c) {
		return
	}
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("系统配置仓储未初始化"))
		return
	}
	current, err := h.store.Get(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}

	var input struct {
		SiteTitle                        *string `json:"siteTitle"`
		AllowGlobalUsageView             *bool   `json:"allowGlobalUsageView"`
		CurrencyDisplay                  *string `json:"currencyDisplay"`
		BillingModelSource               *string `json:"billingModelSource"`
		EnableAutoCleanup                *bool   `json:"enableAutoCleanup"`
		CleanupRetentionDays             *int    `json:"cleanupRetentionDays"`
		CleanupSchedule                  *string `json:"cleanupSchedule"`
		CleanupBatchSize                 *int    `json:"cleanupBatchSize"`
		EnableClientVersionCheck         *bool   `json:"enableClientVersionCheck"`
		VerboseProviderError             *bool   `json:"verboseProviderError"`
		EnableHTTP2                      *bool   `json:"enableHttp2"`
		InterceptAnthropicWarmupRequests *bool   `json:"interceptAnthropicWarmupRequests"`
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
	if input.InterceptAnthropicWarmupRequests != nil {
		fields["intercept_anthropic_warmup_requests"] = *input.InterceptAnthropicWarmupRequests
	}

	updated, err := h.store.UpdateFields(c.Request.Context(), current.ID, fields)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *AdminSystemConfigHandler) ensureAdmin(c *gin.Context) bool {
	if h == nil || h.auth == nil {
		writeAdminError(c, appErrors.NewInternalError("系统配置鉴权服务未初始化"))
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
