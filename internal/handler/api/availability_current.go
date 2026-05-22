package api

import (
	"context"
	"net/http"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	"github.com/gin-gonic/gin"
)

type availabilityProviderStore interface {
	GetActiveProviders(ctx context.Context) ([]*model.Provider, error)
}

type availabilityLogStore interface {
	GetCurrentProviderStatus(ctx context.Context, providerIDs []int, now time.Time, window time.Duration) (map[int]repository.ProviderCurrentStatus, error)
}

type CurrentAvailabilityHandler struct {
	auth      adminAuthenticator
	providers availabilityProviderStore
	logs      availabilityLogStore
}

func NewCurrentAvailabilityHandler(auth adminAuthenticator, providers availabilityProviderStore, logs availabilityLogStore) *CurrentAvailabilityHandler {
	return &CurrentAvailabilityHandler{auth: auth, providers: providers, logs: logs}
}

func (h *CurrentAvailabilityHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/api/availability/current", h.getCurrent)
}

func (h *CurrentAvailabilityHandler) getCurrent(c *gin.Context) {
	if h == nil || h.auth == nil || h.providers == nil || h.logs == nil {
		writeAdminError(c, appErrors.NewInternalError("可用性服务未初始化"))
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

	providers, err := h.providers.GetActiveProviders(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	providerIDs := make([]int, 0, len(providers))
	for _, provider := range providers {
		if provider != nil && provider.ID > 0 {
			providerIDs = append(providerIDs, provider.ID)
		}
	}
	stats, err := h.logs.GetCurrentProviderStatus(c.Request.Context(), providerIDs, time.Now(), 15*time.Minute)
	if err != nil {
		writeAdminError(c, err)
		return
	}

	response := make([]gin.H, 0, len(providers))
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		stat := stats[provider.ID]
		total := stat.GreenCount + stat.RedCount
		availability := 0.0
		status := "unknown"
		if total > 0 {
			availability = float64(stat.GreenCount) / float64(total)
			if availability >= 0.5 {
				status = "green"
			} else {
				status = "red"
			}
		}
		var lastRequestAt any
		if stat.LastRequestAt != nil {
			lastRequestAt = stat.LastRequestAt.UTC().Format(time.RFC3339Nano)
		}
		response = append(response, gin.H{
			"providerId":    provider.ID,
			"providerName":  provider.Name,
			"status":        status,
			"availability":  availability,
			"requestCount":  total,
			"lastRequestAt": lastRequestAt,
		})
	}

	c.JSON(http.StatusOK, response)
}
