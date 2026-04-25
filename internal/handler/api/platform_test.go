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
		{path: "/api/version", wantStatus: http.StatusOK, wantContains: "test-version"},
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
	if !strings.Contains(body, "\"current\":\"test-version\"") || !strings.Contains(body, "\"latest\":\"test-version\"") || !strings.Contains(body, "\"hasUpdate\":false") {
		t.Fatalf("expected version checker payload shape, got %s", body)
	}
}

func TestPlatformVersionRoutePrefersEnvVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := os.Getenv("NEXT_PUBLIC_APP_VERSION")
	t.Cleanup(func() { _ = os.Setenv("NEXT_PUBLIC_APP_VERSION", old) })
	_ = os.Setenv("NEXT_PUBLIC_APP_VERSION", "9.9.9")

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

	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "\"current\":\"9.9.9\"") {
		t.Fatalf("expected env version payload, got %d: %s", resp.Code, resp.Body.String())
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
