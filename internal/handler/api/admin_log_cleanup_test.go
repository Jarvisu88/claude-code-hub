package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

type fakeLogCleanupRunner struct {
	beforeDate time.Time
	dryRun     bool
	result     gin.H
	err        error
}

func (f *fakeLogCleanupRunner) Run(_ context.Context, beforeDate time.Time, dryRun bool, _ time.Time) (gin.H, error) {
	f.beforeDate = beforeDate
	f.dryRun = dryRun
	return f.result, f.err
}

func TestAdminLogCleanupRouteDryRunAndExecute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true

	router := gin.New()
	runner := &fakeLogCleanupRunner{result: gin.H{
		"success":           true,
		"totalDeleted":      12,
		"batchCount":        1,
		"softDeletedPurged": 0,
		"vacuumPerformed":   false,
		"error":             nil,
	}}
	handler := NewAdminLogCleanupHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		runner,
	)
	handler.now = func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) }
	handler.RegisterRoutes(router)

	dryRunReq := httptest.NewRequest(http.MethodPost, "/api/admin/log-cleanup/manual", strings.NewReader(`{"beforeDate":"2026-03-01T00:00:00Z","dryRun":true}`))
	dryRunReq.Header.Set("Authorization", "Bearer admin-token")
	dryRunReq.Header.Set("Content-Type", "application/json")
	dryRunResp := httptest.NewRecorder()
	router.ServeHTTP(dryRunResp, dryRunReq)

	if dryRunResp.Code != http.StatusOK || !strings.Contains(dryRunResp.Body.String(), `"success":true`) {
		t.Fatalf("expected dry-run cleanup payload, got %d: %s", dryRunResp.Code, dryRunResp.Body.String())
	}
	if !runner.dryRun || !runner.beforeDate.Equal(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected dry-run args captured, got dryRun=%v beforeDate=%v", runner.dryRun, runner.beforeDate)
	}

	execReq := httptest.NewRequest(http.MethodPost, "/api/admin/log-cleanup/manual", strings.NewReader(`{"beforeDate":"2026-03-01T00:00:00Z"}`))
	execReq.Header.Set("Authorization", "Bearer admin-token")
	execReq.Header.Set("Content-Type", "application/json")
	execResp := httptest.NewRecorder()
	router.ServeHTTP(execResp, execReq)

	if execResp.Code != http.StatusOK || !strings.Contains(execResp.Body.String(), `"vacuumPerformed":false`) {
		t.Fatalf("expected cleanup execution payload, got %d: %s", execResp.Code, execResp.Body.String())
	}
}

func TestAdminLogCleanupRejectsInvalidDate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	handler := NewAdminLogCleanupHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		&fakeLogCleanupRunner{},
	)
	handler.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/log-cleanup/manual", strings.NewReader(`{"beforeDate":"bad-date"}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
	}
}
