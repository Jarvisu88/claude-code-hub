package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

func TestAvailabilityProbeAllRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()

	NewAvailabilityProbeAllHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
	).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/availability/probe-all", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"ok":true`) {
		t.Fatalf("expected probe-all payload, got %d: %s", resp.Code, resp.Body.String())
	}
}
