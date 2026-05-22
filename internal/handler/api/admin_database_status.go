package api

import (
	"context"
	"net/http"

	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/gin-gonic/gin"
)

type databaseStatusSource interface {
	GetStatus(ctx context.Context) (gin.H, error)
}

type AdminDatabaseStatusHandler struct {
	auth   adminAuthenticator
	source databaseStatusSource
}

func NewAdminDatabaseStatusHandler(auth adminAuthenticator, source databaseStatusSource) *AdminDatabaseStatusHandler {
	return &AdminDatabaseStatusHandler{auth: auth, source: source}
}

func (h *AdminDatabaseStatusHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/api/admin/database/status", h.get)
}

func (h *AdminDatabaseStatusHandler) get(c *gin.Context) {
	if h == nil || h.auth == nil || h.source == nil {
		writeAdminError(c, appErrors.NewInternalError("数据库状态服务未初始化"))
		return
	}
	authResult, err := h.auth.AuthenticateAdminToken(resolveAdminToken(c))
	if err != nil {
		writeAdminError(c, err)
		return
	}
	if authResult == nil || !authResult.IsAdmin {
		writeAdminError(c, appErrors.NewPermissionDenied("权限不足", appErrors.CodePermissionDenied))
		return
	}
	status, err := h.source.GetStatus(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}
