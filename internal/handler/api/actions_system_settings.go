package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/gin-gonic/gin"
)

type SystemSettingsActionHandler struct {
	auth  adminAuthenticator
	store systemSettingsStore
}

func NewSystemSettingsActionHandler(auth adminAuthenticator, store systemSettingsStore) *SystemSettingsActionHandler {
	return &SystemSettingsActionHandler{auth: auth, store: store}
}

func (h *SystemSettingsActionHandler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("/system-settings")
	protected.Use((&Handler{auth: h.auth}).AdminAuthMiddleware())
	protected.GET("", h.get)
	protected.PUT("", h.update)
	protected.POST("/fetchSystemSettings", h.get)
	protected.POST("/saveSystemSettings", h.updateAction)
}

func (h *SystemSettingsActionHandler) get(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("系统设置仓储未初始化"))
		return
	}
	settings, err := h.store.Get(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": settings})
}

func (h *SystemSettingsActionHandler) update(c *gin.Context) {
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
		SiteTitle                        *string `json:"siteTitle"`
		AllowGlobalUsageView             *bool   `json:"allowGlobalUsageView"`
		CurrencyDisplay                  *string `json:"currencyDisplay"`
		BillingModelSource               *string `json:"billingModelSource"`
		CodexPriorityBillingSource       *string `json:"codexPriorityBillingSource"`
		Timezone                         *string `json:"timezone"`
		EnableAutoCleanup                *bool   `json:"enableAutoCleanup"`
		EnableClientVersionCheck         *bool   `json:"enableClientVersionCheck"`
		VerboseProviderError             *bool   `json:"verboseProviderError"`
		EnableHTTP2                      *bool   `json:"enableHttp2"`
		EnableHighConcurrencyMode        *bool   `json:"enableHighConcurrencyMode"`
		InterceptAnthropicWarmupRequests *bool   `json:"interceptAnthropicWarmupRequests"`
		IpGeoLookupEnabled               *bool   `json:"ipGeoLookupEnabled"`
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
	if input.IpGeoLookupEnabled != nil {
		fields["ip_geo_lookup_enabled"] = *input.IpGeoLookupEnabled
	}

	updated, err := h.store.UpdateFields(c.Request.Context(), current.ID, fields)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": updated})
}

func (h *SystemSettingsActionHandler) updateAction(c *gin.Context) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	var raw map[string]any
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	if formData, ok := raw["formData"].(map[string]any); ok {
		raw = formData
	}
	normalized, _ := json.Marshal(raw)
	c.Request.Body = io.NopCloser(bytes.NewReader(normalized))
	h.update(c)
}
