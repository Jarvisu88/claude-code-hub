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

type fakeSystemSettingsStore struct {
	settings *model.SystemSettings
	fields   map[string]any
}

func (f *fakeSystemSettingsStore) Get(_ context.Context) (*model.SystemSettings, error) {
	if f.settings == nil {
		f.settings = &model.SystemSettings{ID: 1, SiteTitle: "Claude Code Hub", CurrencyDisplay: "USD", BillingModelSource: "original"}
	}
	return f.settings, nil
}

func (f *fakeSystemSettingsStore) UpdateFields(_ context.Context, _ int, fields map[string]any) (*model.SystemSettings, error) {
	f.fields = fields
	if title, ok := fields["site_title"].(string); ok {
		f.settings.SiteTitle = title
	}
	if value, ok := fields["enable_http2"].(bool); ok {
		f.settings.EnableHttp2 = value
	}
	return f.settings, nil
}

func TestSystemSettingsRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeSystemSettingsStore{}
	handler := NewSystemSettingsHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/system-settings"))

	getReq := httptest.NewRequest(http.MethodGet, "/api/system-settings", nil)
	getReq.Header.Set("Authorization", "Bearer admin-token")
	getResp := httptest.NewRecorder()
	router.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", getResp.Code, getResp.Body.String())
	}
	if !strings.Contains(getResp.Body.String(), "Claude Code Hub") {
		t.Fatalf("expected system settings payload, got %s", getResp.Body.String())
	}

	putReq := httptest.NewRequest(http.MethodPut, "/api/system-settings", strings.NewReader(`{"siteTitle":"CCH Go","enableHttp2":true}`))
	putReq.Header.Set("Authorization", "Bearer admin-token")
	putReq.Header.Set("Content-Type", "application/json")
	putResp := httptest.NewRecorder()
	router.ServeHTTP(putResp, putReq)
	if putResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", putResp.Code, putResp.Body.String())
	}
	if !strings.Contains(putResp.Body.String(), "CCH Go") {
		t.Fatalf("expected updated system settings payload, got %s", putResp.Body.String())
	}
	if store.fields["site_title"] != "CCH Go" {
		t.Fatalf("expected site_title update field, got %+v", store.fields)
	}
}
