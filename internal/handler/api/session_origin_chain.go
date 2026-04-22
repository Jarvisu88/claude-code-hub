package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/gin-gonic/gin"
)

type sessionOriginStore interface {
	FindLatestBySessionID(ctx context.Context, sessionID string) (*model.MessageRequest, error)
}

type SessionOriginChainActionHandler struct {
	auth  adminAuthenticator
	store sessionOriginStore
}

func NewSessionOriginChainActionHandler(auth adminAuthenticator, store sessionOriginStore) *SessionOriginChainActionHandler {
	return &SessionOriginChainActionHandler{auth: auth, store: store}
}

func (h *SessionOriginChainActionHandler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("/session-origin-chain")
	protected.Use((&Handler{auth: h.auth}).AdminAuthMiddleware())
	protected.GET("", h.get)
	protected.POST("/getSessionOriginChain", h.getAction)
}

func (h *SessionOriginChainActionHandler) get(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("会话来源仓储未初始化"))
		return
	}
	sessionID := strings.TrimSpace(c.Query("sessionId"))
	if sessionID == "" {
		writeAdminError(c, appErrors.NewInvalidRequest("sessionId 不能为空"))
		return
	}
	log, err := h.store.FindLatestBySessionID(c.Request.Context(), sessionID)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	if log == nil {
		c.JSON(http.StatusOK, gin.H{"ok": true, "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": log.ProviderChain})
}

func (h *SessionOriginChainActionHandler) getAction(c *gin.Context) {
	var input struct {
		SessionID string `json:"sessionId"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	c.Request.URL.RawQuery = "sessionId=" + input.SessionID
	h.get(c)
}
