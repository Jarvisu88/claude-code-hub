package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	pkglogger "github.com/ding113/claude-code-hub/internal/pkg/logger"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

func TestAdminLogLevelRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pkglogger.Init(pkglogger.Config{Level: "info"})

	enabled := true
	router := gin.New()
	NewAdminLogLevelHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
	).RegisterRoutes(router)

	getReq := httptest.NewRequest(http.MethodGet, "/api/admin/log-level", nil)
	getReq.Header.Set("Authorization", "Bearer admin-token")
	getResp := httptest.NewRecorder()
	router.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK || !strings.Contains(getResp.Body.String(), "\"level\":\"info\"") {
		t.Fatalf("expected log level payload, got %d: %s", getResp.Code, getResp.Body.String())
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/admin/log-level", strings.NewReader(`{"level":"debug"}`))
	postReq.Header.Set("Authorization", "Bearer admin-token")
	postReq.Header.Set("Content-Type", "application/json")
	postResp := httptest.NewRecorder()
	router.ServeHTTP(postResp, postReq)
	if postResp.Code != http.StatusOK || !strings.Contains(postResp.Body.String(), "\"success\":true") || !strings.Contains(postResp.Body.String(), "\"level\":\"debug\"") {
		t.Fatalf("expected updated log level payload, got %d: %s", postResp.Code, postResp.Body.String())
	}
	if pkglogger.GetLogLevel() != pkglogger.LevelDebug {
		t.Fatalf("expected global log level debug, got %s", pkglogger.GetLogLevel())
	}
}

func TestAdminLogLevelRejectsInvalidLevel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	enabled := true
	NewAdminLogLevelHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
	).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/log-level", strings.NewReader(`{"level":"verbose"}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest || !strings.Contains(resp.Body.String(), "validLevels") {
		t.Fatalf("expected invalid level payload, got %d: %s", resp.Code, resp.Body.String())
	}
}
