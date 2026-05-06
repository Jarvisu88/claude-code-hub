package api

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/gin-gonic/gin"
)

type notificationBindingStore interface {
	List(ctx context.Context, notificationType string) ([]*model.NotificationTargetBinding, error)
	ListAll(ctx context.Context) ([]*model.NotificationTargetBinding, error)
	ReplaceByNotificationType(ctx context.Context, notificationType string, bindings []*model.NotificationTargetBinding) ([]*model.NotificationTargetBinding, error)
}

type NotificationBindingsActionHandler struct {
	auth  adminAuthenticator
	store notificationBindingStore
}

func NewNotificationBindingsActionHandler(auth adminAuthenticator, store notificationBindingStore) *NotificationBindingsActionHandler {
	return &NotificationBindingsActionHandler{auth: auth, store: store}
}

func (h *NotificationBindingsActionHandler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("/notification-bindings")
	protected.Use((&Handler{auth: h.auth}).AdminAuthMiddleware())
	protected.GET("", h.get)
	protected.PUT("", h.update)
	protected.POST("/getNotificationBindings", h.getAction)
	protected.POST("/updateNotificationBindings", h.updateAction)
	protected.POST("/saveNotificationBindings", h.updateAction)
}

func (h *NotificationBindingsActionHandler) get(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("notification binding store is not configured"))
		return
	}
	notificationType := strings.TrimSpace(c.Query("notificationType"))
	var (
		bindings []*model.NotificationTargetBinding
		err      error
	)
	if notificationType == "" {
		bindings, err = h.store.ListAll(c.Request.Context())
	} else {
		if !isValidNotificationType(notificationType) {
			writeAdminError(c, appErrors.NewInvalidRequest("notificationType is invalid"))
			return
		}
		bindings, err = h.store.List(c.Request.Context(), notificationType)
	}
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": bindings})
}

func (h *NotificationBindingsActionHandler) getAction(c *gin.Context) {
	if !rewriteBodyForActionUpdate(c, func(raw map[string]any) (map[string]any, error) {
		if notificationType, ok := raw["notificationType"].(string); ok {
			c.Request.URL.RawQuery = url.Values{"notificationType": []string{notificationType}}.Encode()
		}
		return raw, nil
	}) {
		return
	}
	h.get(c)
}

func (h *NotificationBindingsActionHandler) update(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("notification binding store is not configured"))
		return
	}
	var input struct {
		NotificationType string `json:"notificationType"`
		Bindings         []struct {
			TargetID         int            `json:"targetId"`
			IsEnabled        *bool          `json:"isEnabled"`
			ScheduleCron     *string        `json:"scheduleCron"`
			ScheduleTimezone *string        `json:"scheduleTimezone"`
			TemplateOverride map[string]any `json:"templateOverride"`
		} `json:"bindings"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("request body must be valid JSON"))
		return
	}
	input.NotificationType = strings.TrimSpace(input.NotificationType)
	if !isValidNotificationType(input.NotificationType) {
		writeAdminError(c, appErrors.NewInvalidRequest("notificationType is invalid"))
		return
	}
	bindings := make([]*model.NotificationTargetBinding, 0, len(input.Bindings))
	for _, item := range input.Bindings {
		if item.TargetID <= 0 {
			writeAdminError(c, appErrors.NewInvalidRequest("targetId must be greater than 0"))
			return
		}
		isEnabled := true
		if item.IsEnabled != nil {
			isEnabled = *item.IsEnabled
		}
		timezone := ""
		if item.ScheduleTimezone != nil {
			timezone = strings.TrimSpace(*item.ScheduleTimezone)
		}
		bindings = append(bindings, &model.NotificationTargetBinding{
			NotificationType: input.NotificationType,
			TargetID:         item.TargetID,
			IsEnabled:        isEnabled,
			ScheduleCron:     normalizeOptionalString(item.ScheduleCron),
			ScheduleTimezone: timezone,
			TemplateOverride: item.TemplateOverride,
		})
	}
	updated, err := h.store.ReplaceByNotificationType(c.Request.Context(), input.NotificationType, bindings)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": updated})
}

func (h *NotificationBindingsActionHandler) updateAction(c *gin.Context) {
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
