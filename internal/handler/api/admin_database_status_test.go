package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

type fakeDatabaseStatusSource struct {
	status gin.H
	err    error
}

func (f fakeDatabaseStatusSource) GetStatus(_ context.Context) (gin.H, error) {
	return f.status, f.err
}

func TestAdminDatabaseStatusRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	NewAdminDatabaseStatusHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeDatabaseStatusSource{status: gin.H{
			"isAvailable":     true,
			"containerName":   "db:5432",
			"databaseName":    "claude_code_hub",
			"databaseSize":    "12 MB",
			"tableCount":      42,
			"postgresVersion": "PostgreSQL 16",
		}},
	).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/database/status", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "\"databaseSize\":\"12 MB\"") || !strings.Contains(resp.Body.String(), "\"tableCount\":42") {
		t.Fatalf("expected database status payload, got %d: %s", resp.Code, resp.Body.String())
	}
}
