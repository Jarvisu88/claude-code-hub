package api

import (
	"context"
	"net/http"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	endpointprobesvc "github.com/ding113/claude-code-hub/internal/service/endpointprobe"
	"github.com/gin-gonic/gin"
)

type availabilityProbeAllEndpointStore interface {
	List(ctx context.Context, opts *repository.ListOptions) ([]*model.ProviderEndpoint, error)
	UpdateProbeSnapshot(ctx context.Context, id int, log *model.ProviderEndpointProbeLog) (*model.ProviderEndpoint, error)
}

type availabilityProbeAllProbeLogStore interface {
	Create(ctx context.Context, log *model.ProviderEndpointProbeLog) (*model.ProviderEndpointProbeLog, error)
}

type AvailabilityProbeAllHandler struct {
	auth          adminAuthenticator
	providers     availabilityEndpointsProviderStore
	endpoints     availabilityProbeAllEndpointStore
	probeLogStore availabilityProbeAllProbeLogStore
	http          httpDoer
	now           func() time.Time
}

func NewAvailabilityProbeAllHandler(auth adminAuthenticator, providers availabilityEndpointsProviderStore, httpClient httpDoer, options ...any) *AvailabilityProbeAllHandler {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	handler := &AvailabilityProbeAllHandler{
		auth:      auth,
		providers: providers,
		http:      httpClient,
		now:       time.Now,
	}
	for _, option := range options {
		switch typed := option.(type) {
		case availabilityProbeAllEndpointStore:
			if handler.endpoints == nil {
				handler.endpoints = typed
			}
		case availabilityProbeAllProbeLogStore:
			if handler.probeLogStore == nil {
				handler.probeLogStore = typed
			}
		}
	}
	return handler
}

func (h *AvailabilityProbeAllHandler) RegisterRoutes(router gin.IRouter) {
	router.POST("/api/availability/probe-all", h.probeAll)
}

func (h *AvailabilityProbeAllHandler) probeAll(c *gin.Context) {
	if h == nil || h.auth == nil || h.providers == nil || h.http == nil {
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
	probed := 0
	if h.endpoints != nil {
		endpoints, err := h.endpoints.List(c.Request.Context(), repository.NewListOptions().WithPagination(1, 1000))
		if err != nil {
			writeAdminError(c, err)
			return
		}
		for _, endpoint := range endpoints {
			if !endpoint.IsActive() {
				continue
			}
			h.probeEndpoint(c, endpoint)
			probed++
		}
	} else {
		providers, err := h.providers.ListAll(c.Request.Context())
		if err != nil {
			writeAdminError(c, err)
			return
		}
		for _, provider := range providers {
			if provider == nil || provider.DeletedAt != nil || provider.IsEnabled != nil && !*provider.IsEnabled {
				continue
			}
			h.probeProvider(c, provider)
			probed++
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "probe scheduled",
		"count":   probed,
	})
}

func (h *AvailabilityProbeAllHandler) probeProvider(c *gin.Context, provider *model.Provider) {
	_ = h.probeURL(c, provider.ID, provider.URL)
}

func (h *AvailabilityProbeAllHandler) probeEndpoint(c *gin.Context, endpoint *model.ProviderEndpoint) {
	log := h.probeURL(c, endpoint.ID, endpoint.URL)
	if h.probeLogStore != nil {
		persisted, err := h.probeLogStore.Create(c.Request.Context(), log)
		if err == nil && persisted != nil {
			log = persisted
		}
	}
	if h.endpoints != nil {
		_, _ = h.endpoints.UpdateProbeSnapshot(c.Request.Context(), endpoint.ID, log)
	}
}

func (h *AvailabilityProbeAllHandler) probeURL(c *gin.Context, endpointID int, rawURL string) *model.ProviderEndpointProbeLog {
	startedAt := h.now()
	log := &model.ProviderEndpointProbeLog{
		EndpointID: endpointID,
		Source:     "manual",
		CreatedAt:  startedAt,
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodHead, rawURL, nil)
	if err != nil {
		log.Ok = false
		errorType := "request_build_error"
		errorMessage := err.Error()
		log.ErrorType = &errorType
		log.ErrorMessage = &errorMessage
		endpointprobesvc.Record(endpointID, log.Source, log.Ok, log.StatusCode, log.LatencyMs, log.ErrorType, log.ErrorMessage, log.CreatedAt)
		return log
	}
	resp, err := h.http.Do(req)
	finishedAt := h.now()
	log.CreatedAt = finishedAt
	latencyMs := int(finishedAt.Sub(startedAt).Milliseconds())
	log.LatencyMs = &latencyMs
	if err != nil {
		log.Ok = false
		errorType := "network_error"
		errorMessage := err.Error()
		log.ErrorType = &errorType
		log.ErrorMessage = &errorMessage
		endpointprobesvc.Record(endpointID, log.Source, log.Ok, log.StatusCode, log.LatencyMs, log.ErrorType, log.ErrorMessage, log.CreatedAt)
		return log
	}
	defer resp.Body.Close()
	statusCode := resp.StatusCode
	log.StatusCode = &statusCode
	log.Ok = resp.StatusCode >= 200 && resp.StatusCode < 400
	if !log.Ok {
		errorType := "http_error"
		errorMessage := resp.Status
		log.ErrorType = &errorType
		log.ErrorMessage = &errorMessage
	}
	endpointprobesvc.Record(endpointID, log.Source, log.Ok, log.StatusCode, log.LatencyMs, log.ErrorType, log.ErrorMessage, log.CreatedAt)
	return log
}
