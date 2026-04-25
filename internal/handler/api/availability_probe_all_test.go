package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	endpointprobesvc "github.com/ding113/claude-code-hub/internal/service/endpointprobe"
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
		fakeDashboardProviderStore{},
		nil,
	).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/availability/probe-all", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"ok":true`) {
		t.Fatalf("expected probe-all payload, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestAvailabilityProbeAllRecordsProbeStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	endpointprobesvc.ResetForTest()
	defer endpointprobesvc.ResetForTest()

	enabled := true
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	router := gin.New()
	handler := NewAvailabilityProbeAllHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeDashboardProviderStore{providers: []*model.Provider{{ID: 1, Name: "provider-a", URL: upstream.URL, IsEnabled: &enabled, CreatedAt: time.Now(), UpdatedAt: time.Now()}}},
		upstream.Client(),
	)
	handler.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/availability/probe-all", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	status := endpointprobesvc.GetStatus(1)
	if status.LastProbeOk == nil || !*status.LastProbeOk || status.LastProbeStatusCode == nil || *status.LastProbeStatusCode != 200 {
		t.Fatalf("expected stored probe status, got %+v", status)
	}
	logs := endpointprobesvc.ListLogs(1, 10, 0)
	if len(logs) != 1 || !logs[0].OK {
		t.Fatalf("expected stored probe log, got %+v", logs)
	}
}
