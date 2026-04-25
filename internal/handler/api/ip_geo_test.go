package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

func TestIPGeoRouteReturnsPrivateMarker(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	NewIPGeoHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		&fakeSystemSettingsStore{settings: &model.SystemSettings{ID: 1, IpGeoLookupEnabled: true}},
		nil,
	).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/ip-geo/127.0.0.1", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"status":"private"`) {
		t.Fatalf("expected private IP marker, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestIPGeoRouteProxiesUpstreamLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.String(), "lang=zh-CN") {
			t.Fatalf("expected lang query to be forwarded, got %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ip":"8.8.8.8","version":"ipv4","location":{"country":{"code":"US","name":"United States","flag":{"emoji":"🇺🇸"}}},"timezone":{"id":"UTC"},"connection":{"asn":15169}}`))
	}))
	defer upstream.Close()

	oldURL := os.Getenv("IP_GEO_API_URL")
	oldToken := os.Getenv("IP_GEO_API_TOKEN")
	t.Cleanup(func() {
		_ = os.Setenv("IP_GEO_API_URL", oldURL)
		_ = os.Setenv("IP_GEO_API_TOKEN", oldToken)
	})
	_ = os.Setenv("IP_GEO_API_URL", upstream.URL)
	_ = os.Setenv("IP_GEO_API_TOKEN", "test-token")

	router := gin.New()
	NewIPGeoHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		&fakeSystemSettingsStore{settings: &model.SystemSettings{ID: 1, IpGeoLookupEnabled: true}},
		upstream.Client(),
	).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/ip-geo/8.8.8.8?lang=zh-CN", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"status":"ok"`) || !strings.Contains(resp.Body.String(), `"ip":"8.8.8.8"`) {
		t.Fatalf("expected upstream lookup payload, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestIPGeoRouteRequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewIPGeoHandler(
		fakeAdminAuth{},
		&fakeSystemSettingsStore{settings: &model.SystemSettings{ID: 1, IpGeoLookupEnabled: true}},
		nil,
	).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/ip-geo/8.8.8.8", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestIPGeoRouteHonorsDisabledSetting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	NewIPGeoHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		&fakeSystemSettingsStore{settings: &model.SystemSettings{ID: 1, IpGeoLookupEnabled: false}},
		nil,
	).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/ip-geo/8.8.8.8", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound || !strings.Contains(resp.Body.String(), "ip geolocation disabled") {
		t.Fatalf("expected disabled ip geo payload, got %d: %s", resp.Code, resp.Body.String())
	}
}
