package api

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

func TestAvailabilityEndpointsRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	providers := []*model.Provider{
		{ID: 1, Name: "provider-a", URL: "https://api.vendor-a.com/v1", ProviderType: "claude", Priority: intPtr(2), IsEnabled: &enabled, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: 2, Name: "provider-b", URL: "https://api.vendor-a.com/v2", ProviderType: "claude", Priority: intPtr(1), IsEnabled: &enabled, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: 3, Name: "provider-c", URL: "https://api.vendor-b.com/v1", ProviderType: "gemini", IsEnabled: &enabled, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	vendorID := providerVendorID("https://api.vendor-a.com/v1")

	NewAvailabilityEndpointsHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeDashboardProviderStore{providers: providers},
	).RegisterRoutes(router)

	endpointsReq := httptest.NewRequest(http.MethodGet, "/api/availability/endpoints?vendorId="+strconv.Itoa(vendorID)+"&providerType=claude", nil)
	endpointsReq.Header.Set("Authorization", "Bearer admin-token")
	endpointsResp := httptest.NewRecorder()
	router.ServeHTTP(endpointsResp, endpointsReq)

	if endpointsResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", endpointsResp.Code, endpointsResp.Body.String())
	}
	body := endpointsResp.Body.String()
	if !strings.Contains(body, "\"endpoints\"") || !strings.Contains(body, "\"id\":2") || !strings.Contains(body, "\"id\":1") || strings.Contains(body, "\"id\":3") {
		t.Fatalf("expected synthesized vendor/type endpoints, got %s", body)
	}

	logsReq := httptest.NewRequest(http.MethodGet, "/api/availability/endpoints/probe-logs?endpointId=1&limit=50&offset=10", nil)
	logsReq.Header.Set("Authorization", "Bearer admin-token")
	logsResp := httptest.NewRecorder()
	router.ServeHTTP(logsResp, logsReq)

	if logsResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", logsResp.Code, logsResp.Body.String())
	}
	if !strings.Contains(logsResp.Body.String(), "\"endpoint\"") || !strings.Contains(logsResp.Body.String(), "\"logs\":[]") {
		t.Fatalf("expected empty probe logs payload, got %s", logsResp.Body.String())
	}
}

func TestAvailabilityEndpointsRejectInvalidQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	NewAvailabilityEndpointsHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeDashboardProviderStore{},
	).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/availability/endpoints?vendorId=bad&providerType=claude", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
	}
}
