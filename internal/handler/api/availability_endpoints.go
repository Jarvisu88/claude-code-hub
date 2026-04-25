package api

import (
	"context"
	"hash/fnv"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	endpointprobesvc "github.com/ding113/claude-code-hub/internal/service/endpointprobe"
	"github.com/gin-gonic/gin"
)

type availabilityEndpointsProviderStore interface {
	ListAll(ctx context.Context) ([]*model.Provider, error)
	GetByID(ctx context.Context, id int) (*model.Provider, error)
}

type AvailabilityEndpointsHandler struct {
	auth      adminAuthenticator
	providers availabilityEndpointsProviderStore
}

func NewAvailabilityEndpointsHandler(auth adminAuthenticator, providers availabilityEndpointsProviderStore) *AvailabilityEndpointsHandler {
	return &AvailabilityEndpointsHandler{auth: auth, providers: providers}
}

func (h *AvailabilityEndpointsHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/api/availability/endpoints", h.listEndpoints)
	router.GET("/api/availability/endpoints/probe-logs", h.probeLogs)
}

func (h *AvailabilityEndpointsHandler) ensureAdmin(c *gin.Context) bool {
	if h == nil || h.auth == nil {
		writeAdminError(c, appErrors.NewInternalError("端点可用性服务未初始化"))
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

func (h *AvailabilityEndpointsHandler) listEndpoints(c *gin.Context) {
	if !h.ensureAdmin(c) {
		return
	}
	vendorID, providerType, err := decodeAvailabilityEndpointsQuery(c)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	providers, err := h.providers.ListAll(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	endpoints := synthesizeProviderEndpoints(providers, vendorID, providerType)
	c.JSON(http.StatusOK, gin.H{
		"vendorId":     vendorID,
		"providerType": providerType,
		"endpoints":    endpoints,
	})
}

func (h *AvailabilityEndpointsHandler) probeLogs(c *gin.Context) {
	if !h.ensureAdmin(c) {
		return
	}
	endpointID, err := parseIntQueryParam(c, "endpointId")
	if err != nil || endpointID <= 0 {
		writeAdminError(c, appErrors.NewInvalidRequest("Invalid query"))
		return
	}
	limit, err := parseAvailabilityProbeLogsLimit(c.Query("limit"))
	if err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("Invalid query"))
		return
	}
	offset, err := parseAvailabilityProbeLogsOffset(c.Query("offset"))
	if err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("Invalid query"))
		return
	}

	provider, err := h.providers.GetByID(c.Request.Context(), endpointID)
	if err != nil || provider == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	endpoint := providerToEndpoint(provider)
	logs := endpointprobesvc.ListLogs(endpointID, limit, offset)
	c.JSON(http.StatusOK, gin.H{
		"endpoint": endpoint,
		"logs":     logs,
	})
}

func decodeAvailabilityEndpointsQuery(c *gin.Context) (int, string, error) {
	vendorID, err := parseIntQueryParam(c, "vendorId")
	if err != nil || vendorID <= 0 {
		return 0, "", appErrors.NewInvalidRequest("Invalid query")
	}
	providerType := strings.TrimSpace(c.Query("providerType"))
	switch providerType {
	case string(model.ProviderTypeClaude), string(model.ProviderTypeClaudeAuth), string(model.ProviderTypeCodex), string(model.ProviderTypeGeminiCli), string(model.ProviderTypeGemini), string(model.ProviderTypeOpenAICompatible):
		return vendorID, providerType, nil
	default:
		return 0, "", appErrors.NewInvalidRequest("Invalid query")
	}
}

func parseIntQueryParam(c *gin.Context, key string) (int, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return 0, appErrors.NewInvalidRequest("Invalid query")
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func parseAvailabilityProbeLogsLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 200, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 || value > 1000 {
		return 0, appErrors.NewInvalidRequest("Invalid query")
	}
	return value, nil
}

func parseAvailabilityProbeLogsOffset(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, appErrors.NewInvalidRequest("Invalid query")
	}
	return value, nil
}

func synthesizeProviderEndpoints(providers []*model.Provider, vendorID int, providerType string) []gin.H {
	endpoints := make([]gin.H, 0)
	for _, provider := range providers {
		if provider == nil || provider.DeletedAt != nil {
			continue
		}
		if provider.ProviderType != providerType {
			continue
		}
		if providerVendorID(provider.URL) != vendorID {
			continue
		}
		endpoints = append(endpoints, providerToEndpoint(provider))
	}
	sort.Slice(endpoints, func(i, j int) bool {
		leftSort, _ := endpoints[i]["sortOrder"].(int)
		rightSort, _ := endpoints[j]["sortOrder"].(int)
		if leftSort != rightSort {
			return leftSort < rightSort
		}
		return endpoints[i]["id"].(int) < endpoints[j]["id"].(int)
	})
	return endpoints
}

func providerToEndpoint(provider *model.Provider) gin.H {
	sortOrder := 0
	if provider.Priority != nil {
		sortOrder = *provider.Priority
	}
	isEnabled := provider.IsEnabled == nil || *provider.IsEnabled
	status := endpointprobesvc.GetStatus(provider.ID)
	return gin.H{
		"id":                    provider.ID,
		"vendorId":              providerVendorID(provider.URL),
		"providerType":          provider.ProviderType,
		"url":                   provider.URL,
		"label":                 provider.Name,
		"sortOrder":             sortOrder,
		"isEnabled":             isEnabled,
		"lastProbedAt":          status.LastProbedAt,
		"lastProbeOk":           status.LastProbeOk,
		"lastProbeStatusCode":   status.LastProbeStatusCode,
		"lastProbeLatencyMs":    status.LastProbeLatencyMs,
		"lastProbeErrorType":    status.LastProbeErrorType,
		"lastProbeErrorMessage": status.LastProbeErrorMessage,
		"createdAt":             provider.CreatedAt,
		"updatedAt":             provider.UpdatedAt,
		"deletedAt":             provider.DeletedAt,
	}
}

func providerVendorID(rawURL string) int {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	host := ""
	if err == nil {
		host = strings.TrimSpace(parsed.Hostname())
	}
	if host == "" {
		host = strings.TrimSpace(rawURL)
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(strings.ToLower(host)))
	return int(hasher.Sum32())
}
