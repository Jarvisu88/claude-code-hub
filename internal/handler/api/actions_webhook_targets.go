package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/gin-gonic/gin"
)

type webhookTargetStore interface {
	List(ctx context.Context) ([]*model.WebhookTarget, error)
	GetByID(ctx context.Context, id int) (*model.WebhookTarget, error)
	Create(ctx context.Context, target *model.WebhookTarget) (*model.WebhookTarget, error)
	UpdateFields(ctx context.Context, id int, fields map[string]any) (*model.WebhookTarget, error)
	Delete(ctx context.Context, id int) error
	UpdateTestResult(ctx context.Context, id int, result *model.WebhookTestResult, testedAt time.Time) (*model.WebhookTarget, error)
}

type WebhookTargetsActionHandler struct {
	auth   adminAuthenticator
	store  webhookTargetStore
	tester webhookDeliveryTester
}

func NewWebhookTargetsActionHandler(auth adminAuthenticator, store webhookTargetStore, tester webhookDeliveryTester) *WebhookTargetsActionHandler {
	return &WebhookTargetsActionHandler{auth: auth, store: store, tester: tester}
}

func (h *WebhookTargetsActionHandler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("/webhook-targets")
	protected.Use((&Handler{auth: h.auth}).AdminAuthMiddleware())
	protected.GET("", h.list)
	protected.POST("", h.create)
	protected.GET("/:id", h.get)
	protected.PUT("/:id", h.update)
	protected.DELETE("/:id", h.remove)
	protected.POST("/:id/test", h.test)
	protected.POST("/getWebhookTargets", h.list)
	protected.POST("/getWebhookTarget", h.getFromBody)
	protected.POST("/addWebhookTarget", h.create)
	protected.POST("/editWebhookTarget", h.updateFromBody)
	protected.POST("/removeWebhookTarget", h.removeFromBody)
	protected.POST("/testWebhookTarget", h.testFromBody)
}

func (h *WebhookTargetsActionHandler) list(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("webhook target store is not configured"))
		return
	}
	targets, err := h.store.List(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": targets})
}

func (h *WebhookTargetsActionHandler) get(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("webhook target store is not configured"))
		return
	}
	id, ok := parseIntParam(c, "id")
	if !ok {
		return
	}
	target, err := h.store.GetByID(c.Request.Context(), id)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": target})
}

func (h *WebhookTargetsActionHandler) getFromBody(c *gin.Context) {
	id, ok := parseRequiredBodyID(c)
	if !ok {
		return
	}
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(id)}}
	h.get(c)
}

func (h *WebhookTargetsActionHandler) create(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("webhook target store is not configured"))
		return
	}
	var input struct {
		Name                  string         `json:"name"`
		ProviderType          string         `json:"providerType"`
		WebhookURL            *string        `json:"webhookUrl"`
		TelegramBotToken      *string        `json:"telegramBotToken"`
		TelegramChatID        *string        `json:"telegramChatId"`
		DingtalkSecret        *string        `json:"dingtalkSecret"`
		CustomTemplate        map[string]any `json:"customTemplate"`
		CustomHeaders         map[string]any `json:"customHeaders"`
		ProxyURL              *string        `json:"proxyUrl"`
		ProxyFallbackToDirect *bool          `json:"proxyFallbackToDirect"`
		IsEnabled             *bool          `json:"isEnabled"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("request body must be valid JSON"))
		return
	}
	isEnabled := true
	if input.IsEnabled != nil {
		isEnabled = *input.IsEnabled
	}
	target := &model.WebhookTarget{
		Name:                  strings.TrimSpace(input.Name),
		ProviderType:          strings.TrimSpace(input.ProviderType),
		WebhookUrl:            normalizeOptionalString(input.WebhookURL),
		TelegramBotToken:      normalizeOptionalString(input.TelegramBotToken),
		TelegramChatId:        normalizeOptionalString(input.TelegramChatID),
		DingtalkSecret:        normalizeOptionalString(input.DingtalkSecret),
		CustomTemplate:        input.CustomTemplate,
		CustomHeaders:         input.CustomHeaders,
		ProxyUrl:              normalizeOptionalString(input.ProxyURL),
		ProxyFallbackToDirect: input.ProxyFallbackToDirect != nil && *input.ProxyFallbackToDirect,
		IsEnabled:             isEnabled,
	}
	if err := validateWebhookTargetInput(target, true); err != nil {
		writeAdminError(c, err)
		return
	}
	created, err := h.store.Create(c.Request.Context(), target)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true, "data": created})
}

func (h *WebhookTargetsActionHandler) update(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("webhook target store is not configured"))
		return
	}
	id, ok := parseIntParam(c, "id")
	if !ok {
		return
	}
	var input struct {
		Name                  *string        `json:"name"`
		ProviderType          *string        `json:"providerType"`
		WebhookURL            *string        `json:"webhookUrl"`
		TelegramBotToken      *string        `json:"telegramBotToken"`
		TelegramChatID        *string        `json:"telegramChatId"`
		DingtalkSecret        *string        `json:"dingtalkSecret"`
		CustomTemplate        map[string]any `json:"customTemplate"`
		CustomHeaders         map[string]any `json:"customHeaders"`
		ProxyURL              *string        `json:"proxyUrl"`
		ProxyFallbackToDirect *bool          `json:"proxyFallbackToDirect"`
		IsEnabled             *bool          `json:"isEnabled"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("request body must be valid JSON"))
		return
	}
	current, err := h.store.GetByID(c.Request.Context(), id)
	if err != nil {
		writeAdminError(c, err)
		return
	}

	fields := map[string]any{}
	candidate := *current
	if input.Name != nil {
		candidate.Name = strings.TrimSpace(*input.Name)
		fields["name"] = candidate.Name
	}
	if input.ProviderType != nil {
		candidate.ProviderType = strings.TrimSpace(*input.ProviderType)
		fields["provider_type"] = candidate.ProviderType
	}
	if input.WebhookURL != nil {
		candidate.WebhookUrl = normalizeOptionalString(input.WebhookURL)
		fields["webhook_url"] = candidate.WebhookUrl
	}
	if input.TelegramBotToken != nil {
		candidate.TelegramBotToken = normalizeOptionalString(input.TelegramBotToken)
		fields["telegram_bot_token"] = candidate.TelegramBotToken
	}
	if input.TelegramChatID != nil {
		candidate.TelegramChatId = normalizeOptionalString(input.TelegramChatID)
		fields["telegram_chat_id"] = candidate.TelegramChatId
	}
	if input.DingtalkSecret != nil {
		candidate.DingtalkSecret = normalizeOptionalString(input.DingtalkSecret)
		fields["dingtalk_secret"] = candidate.DingtalkSecret
	}
	if input.CustomTemplate != nil {
		candidate.CustomTemplate = input.CustomTemplate
		fields["custom_template"] = input.CustomTemplate
	}
	if input.CustomHeaders != nil {
		candidate.CustomHeaders = input.CustomHeaders
		fields["custom_headers"] = input.CustomHeaders
	}
	if input.ProxyURL != nil {
		candidate.ProxyUrl = normalizeOptionalString(input.ProxyURL)
		fields["proxy_url"] = candidate.ProxyUrl
	}
	if input.ProxyFallbackToDirect != nil {
		candidate.ProxyFallbackToDirect = *input.ProxyFallbackToDirect
		fields["proxy_fallback_to_direct"] = candidate.ProxyFallbackToDirect
	}
	if input.IsEnabled != nil {
		candidate.IsEnabled = *input.IsEnabled
		fields["is_enabled"] = candidate.IsEnabled
	}
	if err := validateWebhookTargetInput(&candidate, false); err != nil {
		writeAdminError(c, err)
		return
	}
	updated, err := h.store.UpdateFields(c.Request.Context(), id, fields)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": updated})
}

func (h *WebhookTargetsActionHandler) updateFromBody(c *gin.Context) {
	if !extractIDAndRewriteBody(c, nil) {
		return
	}
	h.update(c)
}

func (h *WebhookTargetsActionHandler) remove(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("webhook target store is not configured"))
		return
	}
	id, ok := parseIntParam(c, "id")
	if !ok {
		return
	}
	if err := h.store.Delete(c.Request.Context(), id); err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"id": id, "deleted": true}})
}

func (h *WebhookTargetsActionHandler) removeFromBody(c *gin.Context) {
	id, ok := parseRequiredBodyID(c)
	if !ok {
		return
	}
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(id)}}
	h.remove(c)
}

func (h *WebhookTargetsActionHandler) test(c *gin.Context) {
	if h == nil || h.store == nil || h.tester == nil {
		writeAdminError(c, appErrors.NewInternalError("webhook target test dependencies are not configured"))
		return
	}
	id, ok := parseIntParam(c, "id")
	if !ok {
		return
	}
	target, err := h.store.GetByID(c.Request.Context(), id)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	result := h.tester.Test(c.Request.Context(), target)
	updatedTarget, err := h.store.UpdateTestResult(c.Request.Context(), id, &result, time.Now())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"result": result, "target": updatedTarget}})
}

func (h *WebhookTargetsActionHandler) testFromBody(c *gin.Context) {
	id, ok := parseRequiredBodyID(c)
	if !ok {
		return
	}
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(id)}}
	h.test(c)
}
