package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/gin-gonic/gin"
)

type notificationSettingsStore interface {
	Get(ctx context.Context) (*model.NotificationSettings, error)
	UpdateFields(ctx context.Context, id int, fields map[string]any) (*model.NotificationSettings, error)
}

type webhookTargetLookupStore interface {
	GetByID(ctx context.Context, id int) (*model.WebhookTarget, error)
	UpdateTestResult(ctx context.Context, id int, result *model.WebhookTestResult, testedAt time.Time) (*model.WebhookTarget, error)
}

type NotificationsActionHandler struct {
	auth     adminAuthenticator
	settings notificationSettingsStore
	targets  webhookTargetLookupStore
	tester   webhookDeliveryTester
}

func NewNotificationsActionHandler(auth adminAuthenticator, settings notificationSettingsStore, targets webhookTargetLookupStore, tester webhookDeliveryTester) *NotificationsActionHandler {
	return &NotificationsActionHandler{auth: auth, settings: settings, targets: targets, tester: tester}
}

func (h *NotificationsActionHandler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("/notifications")
	protected.Use((&Handler{auth: h.auth}).AdminAuthMiddleware())
	protected.GET("", h.get)
	protected.PUT("", h.update)
	protected.POST("/getNotificationSettings", h.get)
	protected.POST("/fetchNotificationSettings", h.get)
	protected.POST("/updateNotificationSettings", h.updateAction)
	protected.POST("/saveNotificationSettings", h.updateAction)
	protected.POST("/testWebhook", h.testWebhook)
}

func (h *NotificationsActionHandler) get(c *gin.Context) {
	if h == nil || h.settings == nil {
		writeAdminError(c, appErrors.NewInternalError("notification settings store is not configured"))
		return
	}
	settings, err := h.settings.Get(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": settings})
}

func (h *NotificationsActionHandler) update(c *gin.Context) {
	if h == nil || h.settings == nil {
		writeAdminError(c, appErrors.NewInternalError("notification settings store is not configured"))
		return
	}
	current, err := h.settings.Get(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}

	var input struct {
		Enabled                 *bool    `json:"enabled"`
		UseLegacyMode           *bool    `json:"useLegacyMode"`
		CircuitBreakerEnabled   *bool    `json:"circuitBreakerEnabled"`
		CircuitBreakerWebhook   *string  `json:"circuitBreakerWebhook"`
		DailyLeaderboardEnabled *bool    `json:"dailyLeaderboardEnabled"`
		DailyLeaderboardWebhook *string  `json:"dailyLeaderboardWebhook"`
		DailyLeaderboardTime    *string  `json:"dailyLeaderboardTime"`
		DailyLeaderboardTopN    *int     `json:"dailyLeaderboardTopN"`
		CostAlertEnabled        *bool    `json:"costAlertEnabled"`
		CostAlertWebhook        *string  `json:"costAlertWebhook"`
		CostAlertThreshold      *float64 `json:"costAlertThreshold"`
		CostAlertCheckInterval  *int     `json:"costAlertCheckInterval"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("request body must be valid JSON"))
		return
	}

	fields := map[string]any{}
	if input.Enabled != nil {
		fields["enabled"] = *input.Enabled
	}
	if input.UseLegacyMode != nil {
		fields["use_legacy_mode"] = *input.UseLegacyMode
	}
	if input.CircuitBreakerEnabled != nil {
		fields["circuit_breaker_enabled"] = *input.CircuitBreakerEnabled
	}
	if input.CircuitBreakerWebhook != nil {
		fields["circuit_breaker_webhook"] = normalizeOptionalString(input.CircuitBreakerWebhook)
	}
	if input.DailyLeaderboardEnabled != nil {
		fields["daily_leaderboard_enabled"] = *input.DailyLeaderboardEnabled
	}
	if input.DailyLeaderboardWebhook != nil {
		fields["daily_leaderboard_webhook"] = normalizeOptionalString(input.DailyLeaderboardWebhook)
	}
	if input.DailyLeaderboardTime != nil {
		trimmed := strings.TrimSpace(*input.DailyLeaderboardTime)
		if !isValidHHMM(trimmed) {
			writeAdminError(c, appErrors.NewInvalidRequest("dailyLeaderboardTime must use HH:mm"))
			return
		}
		fields["daily_leaderboard_time"] = trimmed
	}
	if input.DailyLeaderboardTopN != nil {
		if *input.DailyLeaderboardTopN < 1 || *input.DailyLeaderboardTopN > 100 {
			writeAdminError(c, appErrors.NewInvalidRequest("dailyLeaderboardTopN must be between 1 and 100"))
			return
		}
		fields["daily_leaderboard_top_n"] = *input.DailyLeaderboardTopN
	}
	if input.CostAlertEnabled != nil {
		fields["cost_alert_enabled"] = *input.CostAlertEnabled
	}
	if input.CostAlertWebhook != nil {
		fields["cost_alert_webhook"] = normalizeOptionalString(input.CostAlertWebhook)
	}
	if input.CostAlertThreshold != nil {
		threshold, err := parseCostAlertThreshold(*input.CostAlertThreshold)
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest(err.Error()))
			return
		}
		fields["cost_alert_threshold"] = threshold
	}
	if input.CostAlertCheckInterval != nil {
		if *input.CostAlertCheckInterval < 1 || *input.CostAlertCheckInterval > 1440 {
			writeAdminError(c, appErrors.NewInvalidRequest("costAlertCheckInterval must be between 1 and 1440"))
			return
		}
		fields["cost_alert_check_interval"] = *input.CostAlertCheckInterval
	}

	updated, err := h.settings.UpdateFields(c.Request.Context(), current.ID, fields)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": updated})
}

func (h *NotificationsActionHandler) updateAction(c *gin.Context) {
	if !rewriteBodyForActionUpdate(c, func(raw map[string]any) (map[string]any, error) {
		if formData, ok := raw["formData"].(map[string]any); ok {
			return formData, nil
		}
		return raw, nil
	}) {
		return
	}
	h.update(c)
}

func (h *NotificationsActionHandler) testWebhook(c *gin.Context) {
	if h == nil || h.tester == nil {
		writeAdminError(c, appErrors.NewInternalError("webhook tester is not configured"))
		return
	}

	var input struct {
		TargetID         *int           `json:"targetId"`
		ProviderType     string         `json:"providerType"`
		WebhookURL       *string        `json:"webhookUrl"`
		TelegramBotToken *string        `json:"telegramBotToken"`
		TelegramChatID   *string        `json:"telegramChatId"`
		DingtalkSecret   *string        `json:"dingtalkSecret"`
		CustomHeaders    map[string]any `json:"customHeaders"`
		CustomTemplate   map[string]any `json:"customTemplate"`
		ProxyURL         *string        `json:"proxyUrl"`
		ProxyFallback    *bool          `json:"proxyFallbackToDirect"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("request body must be valid JSON"))
		return
	}

	var (
		target   *model.WebhookTarget
		targetID int
	)
	if input.TargetID != nil {
		if h.targets == nil {
			writeAdminError(c, appErrors.NewInternalError("webhook target store is not configured"))
			return
		}
		loaded, err := h.targets.GetByID(c.Request.Context(), *input.TargetID)
		if err != nil {
			writeAdminError(c, err)
			return
		}
		target = loaded
		targetID = loaded.ID
	} else {
		target = &model.WebhookTarget{
			ProviderType:          strings.TrimSpace(input.ProviderType),
			WebhookUrl:            normalizeOptionalString(input.WebhookURL),
			TelegramBotToken:      normalizeOptionalString(input.TelegramBotToken),
			TelegramChatId:        normalizeOptionalString(input.TelegramChatID),
			DingtalkSecret:        normalizeOptionalString(input.DingtalkSecret),
			CustomHeaders:         input.CustomHeaders,
			CustomTemplate:        input.CustomTemplate,
			ProxyUrl:              normalizeOptionalString(input.ProxyURL),
			ProxyFallbackToDirect: input.ProxyFallback != nil && *input.ProxyFallback,
		}
		if target.ProviderType == "" {
			target.ProviderType = string(model.WebhookProviderTypeCustom)
		}
		if err := validateWebhookTargetInput(target, false); err != nil {
			writeAdminError(c, err)
			return
		}
	}

	result := h.tester.Test(c.Request.Context(), target)
	if targetID > 0 && h.targets != nil {
		if _, err := h.targets.UpdateTestResult(c.Request.Context(), targetID, &result, time.Now()); err != nil {
			writeAdminError(c, err)
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": result})
}
