package v1

import (
	"context"
	stderrors "errors"
	"net/http"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

const authResultContextKey = "proxy_auth_result"

type activeProviderLister interface {
	GetActiveProviders(ctx context.Context) ([]*model.Provider, error)
}

// Handler 承载 /v1 代理入口的最小可用接线。
// 当前阶段先把鉴权链接入 Gin，后续再逐步替换 501 占位逻辑。
type Handler struct {
	auth      *authsvc.Service
	providers activeProviderLister
}

func NewHandler(auth *authsvc.Service, providers ...activeProviderLister) *Handler {
	handler := &Handler{auth: auth}
	if len(providers) > 0 {
		handler.providers = providers[0]
	}
	return handler
}

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("")
	protected.Use(h.AuthMiddleware())
	protected.POST("/messages", h.notImplemented)
	protected.POST("/chat/completions", h.notImplemented)
	protected.POST("/responses", h.notImplemented)
	protected.GET("/models", h.handleModels)
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
