package v1

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"

	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	sessionsvc "github.com/ding113/claude-code-hub/internal/service/session"
	"github.com/gin-gonic/gin"
)

const authResultContextKey = "proxy_auth_result"
const proxySessionIDContextKey = "proxy_session_id"

type sessionManager interface {
	ExtractClientSessionID(requestBody map[string]any, headers http.Header) sessionsvc.ClientSessionExtractionResult
	GetOrCreateSessionID(ctx context.Context, keyID int, messages any, clientSessionID string) string
	IncrementConcurrentCount(ctx context.Context, sessionID string)
	DecrementConcurrentCount(ctx context.Context, sessionID string)
}

// Handler 承载 /v1 代理入口的最小可用接线。
// 当前阶段先把鉴权链接入 Gin，后续再逐步替换 501 占位逻辑。
type Handler struct {
	auth     *authsvc.Service
	sessions sessionManager
}

func NewHandler(auth *authsvc.Service, sessions sessionManager) *Handler {
	return &Handler{auth: auth, sessions: sessions}
}

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("")
	protected.Use(h.AuthMiddleware())
	protected.Use(h.SessionLifecycleMiddleware())
	protected.POST("/messages", h.notImplemented)
	protected.POST("/chat/completions", h.notImplemented)
	protected.POST("/responses", h.notImplemented)
	protected.GET("/models", h.notImplemented)
}

func (h *Handler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.auth == nil {
			appErr := appErrors.NewInternalError("代理鉴权服务未初始化")
			c.AbortWithStatusJSON(appErr.HTTPStatus, appErr.ToResponse())
			return
		}

		result, err := h.auth.AuthenticateProxy(c.Request.Context(), authsvc.ProxyAuthInput{
			AuthorizationHeader: c.GetHeader("Authorization"),
			APIKeyHeader:        c.GetHeader("x-api-key"),
			GeminiAPIKeyHeader:  c.GetHeader("x-goog-api-key"),
			GeminiAPIKeyQuery:   c.Query("key"),
		})
		if err != nil {
			writeAppError(c, err)
			return
		}

		c.Set(authResultContextKey, result)
		c.Next()
	}
}

func GetAuthResult(c *gin.Context) (*authsvc.AuthResult, bool) {
	if c == nil {
		return nil, false
	}
	value, ok := c.Get(authResultContextKey)
	if !ok {
		return nil, false
	}
	result, ok := value.(*authsvc.AuthResult)
	return result, ok && result != nil
}

func GetProxySessionID(c *gin.Context) (string, bool) {
	if c == nil {
		return "", false
	}
	value, ok := c.Get(proxySessionIDContextKey)
	if !ok {
		return "", false
	}
	sessionID, ok := value.(string)
	return sessionID, ok && sessionID != ""
}

func (h *Handler) SessionLifecycleMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !shouldTrackConcurrentRequests(c) || h == nil || h.sessions == nil {
			c.Next()
			return
		}

		authResult, ok := GetAuthResult(c)
		if !ok || authResult == nil || authResult.Key == nil {
			c.Next()
			return
		}

		requestBody, ok := decodeRequestBody(c)
		if !ok {
			c.Next()
			return
		}

		extracted := h.sessions.ExtractClientSessionID(requestBody, c.Request.Header)
		sessionID := h.sessions.GetOrCreateSessionID(c.Request.Context(), authResult.Key.ID, extractRequestMessages(requestBody), extracted.SessionID)
		if sessionID == "" {
			c.Next()
			return
		}

		c.Set(proxySessionIDContextKey, sessionID)
		h.sessions.IncrementConcurrentCount(c.Request.Context(), sessionID)
		defer h.sessions.DecrementConcurrentCount(c.Request.Context(), sessionID)

		c.Next()
	}
}

func (h *Handler) notImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": gin.H{
			"type":    "not_implemented",
			"message": "This endpoint is not yet implemented",
		},
	})
}

func writeAppError(c *gin.Context, err error) {
	var appErr *appErrors.AppError
	if stderrors.As(err, &appErr) {
		c.AbortWithStatusJSON(appErr.HTTPStatus, appErr.ToResponse())
		return
	}

	fallback := appErrors.NewInternalError("代理鉴权失败")
	c.AbortWithStatusJSON(fallback.HTTPStatus, fallback.ToResponse())
}

func shouldTrackConcurrentRequests(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	if c.Request.Method != http.MethodPost {
		return false
	}

	switch c.FullPath() {
	case "/v1/messages", "/v1/chat/completions", "/v1/responses":
		return true
	default:
		return false
	}
}

func decodeRequestBody(c *gin.Context) (map[string]any, bool) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return nil, false
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, false
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	if len(bytes.TrimSpace(bodyBytes)) == 0 {
		return map[string]any{}, true
	}

	var requestBody map[string]any
	if err := json.Unmarshal(bodyBytes, &requestBody); err != nil {
		return nil, false
	}

	return requestBody, true
}

func extractRequestMessages(requestBody map[string]any) any {
	if requestBody == nil {
		return nil
	}
	if messages, ok := requestBody["messages"]; ok {
		return messages
	}
	if input, ok := requestBody["input"]; ok {
		return input
	}
	return nil
}
