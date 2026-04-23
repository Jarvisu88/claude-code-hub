package api

import (
	"context"
	"net/http"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/gin-gonic/gin"
)

type providerSlotsProviderStore interface {
	GetActiveProviders(ctx context.Context) ([]*model.Provider, error)
}

type ProviderSlotsActionHandler struct {
	auth      adminAuthenticator
	providers providerSlotsProviderStore
	logs      usageLogsStore
}

func NewProviderSlotsActionHandler(auth adminAuthenticator, providers providerSlotsProviderStore, logs usageLogsStore) *ProviderSlotsActionHandler {
	return &ProviderSlotsActionHandler{auth: auth, providers: providers, logs: logs}
}

func (h *ProviderSlotsActionHandler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("/provider-slots")
	protected.Use((&Handler{auth: h.auth}).AdminAuthMiddleware())
	protected.POST("/getProviderSlots", h.getProviderSlots)
}

func (h *ProviderSlotsActionHandler) getProviderSlots(c *gin.Context) {
	if h == nil || h.providers == nil || h.logs == nil {
		writeAdminError(c, appErrors.NewInternalError("供应商插槽服务未初始化"))
		return
	}
	providers, err := h.providers.GetActiveProviders(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	recentLogs, err := h.logs.ListRecent(c.Request.Context(), 200)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": buildProviderSlotsActionData(recentLogs, providers)})
}

func buildProviderSlotsActionData(logs []*model.MessageRequest, providers []*model.Provider) []gin.H {
	activeSessionsByProvider := map[int]map[string]struct{}{}
	for _, log := range logs {
		if log == nil || isWarmupProxyStatusRequest(log) || log.DurationMs != nil {
			continue
		}
		if log.ProviderID == 0 || log.SessionID == nil || *log.SessionID == "" {
			continue
		}
		sessions, ok := activeSessionsByProvider[log.ProviderID]
		if !ok {
			sessions = map[string]struct{}{}
			activeSessionsByProvider[log.ProviderID] = sessions
		}
		sessions[*log.SessionID] = struct{}{}
	}

	slots := make([]gin.H, 0, len(providers))
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		totalSlots := 0
		if provider.LimitConcurrentSessions != nil {
			totalSlots = *provider.LimitConcurrentSessions
		}
		slots = append(slots, gin.H{
			"providerId":  provider.ID,
			"name":        provider.Name,
			"usedSlots":   len(activeSessionsByProvider[provider.ID]),
			"totalSlots":  totalSlots,
			"totalVolume": 0,
		})
	}
	return slots
}
