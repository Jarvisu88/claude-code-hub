package api

import (
	"net/http"
	"strings"

	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	pkglogger "github.com/ding113/claude-code-hub/internal/pkg/logger"
	"github.com/gin-gonic/gin"
)

type AdminLogLevelHandler struct {
	auth adminAuthenticator
}

func NewAdminLogLevelHandler(auth adminAuthenticator) *AdminLogLevelHandler {
	return &AdminLogLevelHandler{auth: auth}
}

func (h *AdminLogLevelHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/api/admin/log-level", h.get)
	router.POST("/api/admin/log-level", h.update)
}

func (h *AdminLogLevelHandler) get(c *gin.Context) {
	if !h.ensureAdmin(c) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"level": pkglogger.GetLogLevel()})
}

func (h *AdminLogLevelHandler) update(c *gin.Context) {
	if !h.ensureAdmin(c) {
		return
	}
	var input struct {
		Level string `json:"level"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	level := strings.TrimSpace(strings.ToLower(input.Level))
	switch pkglogger.LogLevel(level) {
	case pkglogger.LevelFatal, pkglogger.LevelError, pkglogger.LevelWarn, pkglogger.LevelInfo, pkglogger.LevelDebug, pkglogger.LevelTrace:
		pkglogger.SetLogLevel(pkglogger.LogLevel(level))
		c.JSON(http.StatusOK, gin.H{"success": true, "level": level})
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "无效的日志级别",
			"validLevels": []string{"fatal", "error", "warn", "info", "debug", "trace"},
		})
	}
}

func (h *AdminLogLevelHandler) ensureAdmin(c *gin.Context) bool {
	if h == nil || h.auth == nil {
		writeAdminError(c, appErrors.NewInternalError("日志级别鉴权服务未初始化"))
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
