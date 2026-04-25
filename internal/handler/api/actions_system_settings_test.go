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

func TestActionStyleSystemSettingsRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeSystemSettingsStore{}
	router := gin.New()
	NewSystemSettingsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	).RegisterRoutes(router.Group("/api/actions"))

	getReq := httptest.NewRequest(http.MethodGet, "/api/actions/system-settings", nil)
	getReq.Header.Set("Authorization", "Bearer admin-token")
	getResp := httptest.NewRecorder()
	router.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", getResp.Code, getResp.Body.String())
	}

	putReq := httptest.NewRequest(http.MethodPut, "/api/actions/system-settings", strings.NewReader(`{"siteTitle":"CCH Action","enableHttp2":true,"cleanupRetentionDays":45,"cleanupSchedule":"0 3 * * *","cleanupBatchSize":20000}`))
	putReq.Header.Set("Authorization", "Bearer admin-token")
	putReq.Header.Set("Content-Type", "application/json")
	putResp := httptest.NewRecorder()
	router.ServeHTTP(putResp, putReq)
	if putResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", putResp.Code, putResp.Body.String())
	}
	if !strings.Contains(putResp.Body.String(), "\"ok\":true") {
		t.Fatalf("expected action envelope, got %s", putResp.Body.String())
	}
	if store.fields["cleanup_retention_days"] != 45 || store.fields["cleanup_schedule"] != "0 3 * * *" || store.fields["cleanup_batch_size"] != 20000 {
		t.Fatalf("expected cleanup settings captured, got %+v", store.fields)
	}

	postGetReq := httptest.NewRequest(http.MethodPost, "/api/actions/system-settings/fetchSystemSettings", strings.NewReader(`{}`))
	postGetReq.Header.Set("Authorization", "Bearer admin-token")
	postGetReq.Header.Set("Content-Type", "application/json")
	postGetResp := httptest.NewRecorder()
	router.ServeHTTP(postGetResp, postGetReq)
	if postGetResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", postGetResp.Code, postGetResp.Body.String())
	}
	if !strings.Contains(postGetResp.Body.String(), "\"ok\":true") {
		t.Fatalf("expected action envelope from fetchSystemSettings, got %s", postGetResp.Body.String())
	}

	postPutReq := httptest.NewRequest(http.MethodPost, "/api/actions/system-settings/saveSystemSettings", strings.NewReader(`{"formData":{"siteTitle":"CCH Action Alias"}}`))
	postPutReq.Header.Set("Authorization", "Bearer admin-token")
	postPutReq.Header.Set("Content-Type", "application/json")
	postPutResp := httptest.NewRecorder()
	router.ServeHTTP(postPutResp, postPutReq)
	if postPutResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", postPutResp.Code, postPutResp.Body.String())
	}
	if !strings.Contains(postPutResp.Body.String(), "CCH Action Alias") {
		t.Fatalf("expected updated payload from saveSystemSettings, got %s", postPutResp.Body.String())
	}
}
