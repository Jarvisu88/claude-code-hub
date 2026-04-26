package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPlatformRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewPlatformHandler(
		func(_ context.Context) error { return nil },
		func(_ context.Context) error { return nil },
		"test-version",
	)

	router := gin.New()
	handler.RegisterRoutes(router)

	tests := []struct {
		path         string
		wantStatus   int
		wantContains string
	}{
		{path: "/api/health", wantStatus: http.StatusOK, wantContains: "healthy"},
		{path: "/api/health/live", wantStatus: http.StatusOK, wantContains: "alive"},
		{path: "/api/health/ready", wantStatus: http.StatusOK, wantContains: "ready"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tc.wantStatus, resp.Code, resp.Body.String())
			}
			if !strings.Contains(resp.Body.String(), tc.wantContains) {
				t.Fatalf("expected body to contain %q, got %s", tc.wantContains, resp.Body.String())
			}
		})
	}
}

func TestPlatformVersionRouteReturnsVersionCheckerShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldReleaseURL := os.Getenv("VERSION_RELEASE_API_URL")
	t.Cleanup(func() { _ = os.Setenv("VERSION_RELEASE_API_URL", oldReleaseURL) })
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"","html_url":"","published_at":""}`))
	}))
	defer upstream.Close()
	_ = os.Setenv("VERSION_RELEASE_API_URL", upstream.URL)

	handler := NewPlatformHandler(
		func(_ context.Context) error { return nil },
		func(_ context.Context) error { return nil },
		"test-version",
	)

	router := gin.New()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !strings.Contains(body, "\"current\":\"test-version\"") || !strings.Contains(body, "\"latest\":null") || !strings.Contains(body, "\"hasUpdate\":false") || !strings.Contains(body, "暂无发布版本") {
		t.Fatalf("expected version checker payload shape, got %s", body)
	}
}

func TestPlatformVersionRoutePrefersEnvVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := os.Getenv("NEXT_PUBLIC_APP_VERSION")
	oldReleaseURL := os.Getenv("VERSION_RELEASE_API_URL")
	t.Cleanup(func() { _ = os.Setenv("NEXT_PUBLIC_APP_VERSION", old) })
	t.Cleanup(func() { _ = os.Setenv("VERSION_RELEASE_API_URL", oldReleaseURL) })
	_ = os.Setenv("NEXT_PUBLIC_APP_VERSION", "9.9.9")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"","html_url":"","published_at":""}`))
	}))
	defer upstream.Close()
	_ = os.Setenv("VERSION_RELEASE_API_URL", upstream.URL)

	handler := NewPlatformHandler(
		func(_ context.Context) error { return nil },
		func(_ context.Context) error { return nil },
		"test-version",
	)

	router := gin.New()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "\"current\":\"9.9.9\"") || !strings.Contains(resp.Body.String(), "\"latest\":null") {
		t.Fatalf("expected env version payload, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestPlatformVersionRouteDetectsUpdateFromReleaseAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldVersion := os.Getenv("NEXT_PUBLIC_APP_VERSION")
	oldReleaseURL := os.Getenv("VERSION_RELEASE_API_URL")
	t.Cleanup(func() {
		_ = os.Setenv("NEXT_PUBLIC_APP_VERSION", oldVersion)
		_ = os.Setenv("VERSION_RELEASE_API_URL", oldReleaseURL)
	})
	_ = os.Setenv("NEXT_PUBLIC_APP_VERSION", "v1.0.0")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.0","html_url":"https://example.com/release","published_at":"2026-04-25T00:00:00Z"}`))
	}))
	defer upstream.Close()
	_ = os.Setenv("VERSION_RELEASE_API_URL", upstream.URL)

	handler := NewPlatformHandler(
		func(_ context.Context) error { return nil },
		func(_ context.Context) error { return nil },
		"test-version",
	)

	router := gin.New()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	body := resp.Body.String()
	if resp.Code != http.StatusOK || !strings.Contains(body, "\"current\":\"v1.0.0\"") || !strings.Contains(body, "\"latest\":\"v1.2.0\"") || !strings.Contains(body, "\"hasUpdate\":true") || !strings.Contains(body, "\"releaseUrl\":\"https://example.com/release\"") {
		t.Fatalf("expected update-aware version payload, got %d: %s", resp.Code, body)
	}
}

func TestPlatformVersionRouteReturns500WhenReleaseLookupFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldReleaseURL := os.Getenv("VERSION_RELEASE_API_URL")
	t.Cleanup(func() { _ = os.Setenv("VERSION_RELEASE_API_URL", oldReleaseURL) })
	_ = os.Setenv("VERSION_RELEASE_API_URL", "http://127.0.0.1:0")

	oldVersion := os.Getenv("NEXT_PUBLIC_APP_VERSION")
	t.Cleanup(func() { _ = os.Setenv("NEXT_PUBLIC_APP_VERSION", oldVersion) })
	_ = os.Setenv("NEXT_PUBLIC_APP_VERSION", "v1.0.0")

	handler := NewPlatformHandler(
		func(_ context.Context) error { return nil },
		func(_ context.Context) error { return nil },
		"test-version",
	)

	router := gin.New()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError || !strings.Contains(resp.Body.String(), "\"latest\":null") || !strings.Contains(resp.Body.String(), "无法获取最新版本信息") {
		t.Fatalf("expected 500 fallback payload, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestPlatformReadyReturns503WhenDependencyFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewPlatformHandler(
		func(_ context.Context) error { return errors.New("db down") },
		nil,
		"",
	)

	router := gin.New()
	handler.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/health/ready", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", resp.Code, resp.Body.String())
	}
}
