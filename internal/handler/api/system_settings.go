package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/gin-gonic/gin"
)

type systemSettingsStore interface {
	Get(ctx context.Context) (*model.SystemSettings, error)
	UpdateFields(ctx context.Context, id int, fields map[string]any) (*model.SystemSettings, error)
}

type SystemSettingsHandler struct {
	auth  adminAuthenticator
	store systemSettingsStore
}

func NewSystemSettingsHandler(auth adminAuthenticator, store systemSettingsStore) *SystemSettingsHandler {
	return &SystemSettingsHandler{auth: auth, store: store}
}

func (h *SystemSettingsHandler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("")
	protected.Use((&Handler{auth: h.auth}).AdminAuthMiddleware())
	protected.GET("", h.get)
	protected.PUT("", h.update)
}

func (h *SystemSettingsHandler) get(c *gin.Context) {
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
		EnableAutoCleanup                *bool   `json:"enableAutoCleanup"`
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
