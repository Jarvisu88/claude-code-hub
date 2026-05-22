package api

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

type fakeDatabaseBackupRunner struct {
	exportMode string
	importPath string
	cleanFirst bool
}

func (f *fakeDatabaseBackupRunner) Export(_ context.Context, mode string) (string, []byte, error) {
	f.exportMode = mode
	return "backup_test.dump", []byte("dump-bytes"), nil
}

func (f *fakeDatabaseBackupRunner) Import(_ context.Context, dumpPath string, cleanFirst bool) ([]importProgressEvent, error) {
	f.importPath = dumpPath
	f.cleanFirst = cleanFirst
	return []importProgressEvent{
		{Type: "progress", Message: "step 1"},
		{Type: "complete", Message: "done"},
	}, nil
}

func TestAdminDatabaseExportRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	runner := &fakeDatabaseBackupRunner{}
	NewAdminDatabaseExportHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		runner,
	).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/database/export?mode=excludeLogs", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || runner.exportMode != "excludeLogs" || !strings.Contains(resp.Header().Get("Content-Disposition"), "backup_test.dump") || resp.Body.String() != "dump-bytes" {
		t.Fatalf("expected database export response, got %d mode=%s headers=%v body=%s", resp.Code, runner.exportMode, resp.Header(), resp.Body.String())
	}
}

func TestAdminDatabaseImportRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	runner := &fakeDatabaseBackupRunner{}
	NewAdminDatabaseImportHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		runner,
	).RegisterRoutes(router)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "backup.dump")
	_, _ = io.WriteString(part, "dump")
	_ = writer.WriteField("cleanFirst", "true")
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/database/import", body)
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK || !runner.cleanFirst || !strings.Contains(resp.Body.String(), `"type":"progress"`) || !strings.Contains(resp.Body.String(), `"type":"complete"`) {
		t.Fatalf("expected import SSE response, got %d clean=%v body=%s", resp.Code, runner.cleanFirst, resp.Body.String())
	}
}
