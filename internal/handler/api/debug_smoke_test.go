package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSmokeDebugHandlerServesHTMLPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	NewSmokeDebugHandler().RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/__debug__/smoke", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("expected text/html content type, got %q", got)
	}
	body := resp.Body.String()
	for _, want := range []string{
		"Smoke Debug Console",
		"/api/auth/login",
		"/api/health",
		"/api/system-settings",
		"/api/actions/users",
		"/v1/responses",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected body to contain %q, got %s", want, body)
		}
	}
}
