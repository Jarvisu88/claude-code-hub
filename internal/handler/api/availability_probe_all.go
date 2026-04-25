package api

import (
	"net/http"

	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/gin-gonic/gin"
)

type AvailabilityProbeAllHandler struct {
	auth adminAuthenticator
}

func NewAvailabilityProbeAllHandler(auth adminAuthenticator) *AvailabilityProbeAllHandler {
	return &AvailabilityProbeAllHandler{auth: auth}
}

func (h *AvailabilityProbeAllHandler) RegisterRoutes(router gin.IRouter) {
	router.POST("/api/availability/probe-all", h.probeAll)
}

func (h *AvailabilityProbeAllHandler) probeAll(c *gin.Context) {
	if h == nil || h.auth == nil {
		writeAdminError(c, appErrors.NewInternalError("可用性探测服务未初始化"))
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
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "probe scheduled",
	})
}
