package api

import (
	"net/http"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	endpointprobesvc "github.com/ding113/claude-code-hub/internal/service/endpointprobe"
	"github.com/gin-gonic/gin"
)

type AvailabilityProbeAllHandler struct {
	auth      adminAuthenticator
	providers availabilityEndpointsProviderStore
	http      httpDoer
	now       func() time.Time
}

func NewAvailabilityProbeAllHandler(auth adminAuthenticator, providers availabilityEndpointsProviderStore, httpClient httpDoer) *AvailabilityProbeAllHandler {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &AvailabilityProbeAllHandler{
		auth:      auth,
		providers: providers,
		http:      httpClient,
		now:       time.Now,
	}
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
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "probe scheduled",
	})
}

func (h *AvailabilityProbeAllHandler) probeProvider(c *gin.Context, provider *model.Provider) {
	startedAt := h.now()
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodHead, provider.URL, nil)
	if err != nil {
		errorType := "request_build_error"
		errorMessage := err.Error()
		endpointprobesvc.Record(provider.ID, "manual", false, nil, nil, &errorType, &errorMessage, startedAt)
		return
	}
	resp, err := h.http.Do(req)
	if err != nil {
		errorType := "network_error"
		errorMessage := err.Error()
		endpointprobesvc.Record(provider.ID, "manual", false, nil, nil, &errorType, &errorMessage, h.now())
		return
	}
	defer resp.Body.Close()
	statusCode := resp.StatusCode
	latencyMs := int(h.now().Sub(startedAt).Milliseconds())
	ok := resp.StatusCode >= 200 && resp.StatusCode < 400
	var errorType *string
	var errorMessage *string
	if !ok {
		value := "http_error"
		errorType = &value
		msg := resp.Status
		errorMessage = &msg
	}
	endpointprobesvc.Record(provider.ID, "manual", ok, &statusCode, &latencyMs, errorType, errorMessage, h.now())
}
