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
	limit := parseOptionalPositiveInt(c.Query("limit"), 200, 1000)
	offset := parseOptionalPositiveInt(c.Query("offset"), 0, 1_000_000)

	provider, err := h.providers.GetByID(c.Request.Context(), endpointID)
	if err != nil || provider == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	endpoint := providerToEndpoint(provider)
	c.JSON(http.StatusOK, gin.H{
		"endpoint": endpoint,
		"logs":     []gin.H{},
		"limit":    limit,
		"offset":   offset,
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

func parseOptionalPositiveInt(raw string, fallback int, max int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if value < 0 {
		return fallback
	}
	if max > 0 && value > max {
		return fallback
	}
	return value
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
	return gin.H{
		"id":                    provider.ID,
		"vendorId":              providerVendorID(provider.URL),
		"providerType":          provider.ProviderType,
		"url":                   provider.URL,
		"label":                 provider.Name,
		"sortOrder":             sortOrder,
		"isEnabled":             isEnabled,
		"lastProbedAt":          nil,
		"lastProbeOk":           nil,
		"lastProbeStatusCode":   nil,
		"lastProbeLatencyMs":    nil,
		"lastProbeErrorType":    nil,
		"lastProbeErrorMessage": nil,
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
