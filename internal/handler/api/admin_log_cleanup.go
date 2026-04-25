package api

import (
	"context"
	"net/http"
	"time"

	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/gin-gonic/gin"
)

type logCleanupRunner interface {
	Run(ctx context.Context, beforeDate time.Time, dryRun bool, now time.Time) (gin.H, error)
}

type AdminLogCleanupHandler struct {
	auth   adminAuthenticator
	runner logCleanupRunner
	now    func() time.Time
}

func NewAdminLogCleanupHandler(auth adminAuthenticator, runner logCleanupRunner) *AdminLogCleanupHandler {
	return &AdminLogCleanupHandler{
		auth:   auth,
		runner: runner,
		now:    time.Now,
	}
}

func (h *AdminLogCleanupHandler) RegisterRoutes(router gin.IRouter) {
	router.POST("/api/admin/log-cleanup/manual", h.cleanup)
}

func (h *AdminLogCleanupHandler) cleanup(c *gin.Context) {
	if h == nil || h.auth == nil || h.runner == nil {
		writeAdminError(c, appErrors.NewInternalError("日志清理服务未初始化"))
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

	var input struct {
		BeforeDate string `json:"beforeDate"`
		DryRun     bool   `json:"dryRun"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求参数格式错误"))
		return
	}
	beforeDate := h.now().AddDate(0, 0, -30)
	if input.BeforeDate != "" {
		parsed, err := time.Parse(time.RFC3339, input.BeforeDate)
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("beforeDate 必须是合法 ISO 时间"))
			return
		}
		beforeDate = parsed
	}

	startedAt := time.Now()
	result, err := h.runner.Run(c.Request.Context(), beforeDate, input.DryRun, h.now())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	result["durationMs"] = time.Since(startedAt).Milliseconds()
	c.JSON(http.StatusOK, result)
}
