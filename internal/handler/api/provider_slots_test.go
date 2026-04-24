package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	providertrackersvc "github.com/ding113/claude-code-hub/internal/service/providertracker"
	"github.com/gin-gonic/gin"
)

func TestProviderSlotsActionReturnsBaselinePayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()

	handler := NewProviderSlotsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeDashboardProviderStore{providers: []*model.Provider{
			{ID: 1, Name: "provider-a", LimitConcurrentSessions: intPtr(5), IsEnabled: &enabled},
			{ID: 2, Name: "provider-b", LimitConcurrentSessions: intPtr(2), IsEnabled: &enabled},
		}},
		&fakeUsageLogsStore{
			logs: []*model.MessageRequest{
				{ID: 1, ProviderID: 2, SessionID: stringPtr("sess_active"), CreatedAt: time.Now()},
			},
		},
	)
	handler.RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/provider-slots/getProviderSlots", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !strings.Contains(body, "\"providerId\":1") || !strings.Contains(body, "\"providerId\":2") || !strings.Contains(body, "\"usedSlots\":1") || !strings.Contains(body, "\"totalSlots\":2") || !strings.Contains(body, "\"totalVolume\":0") {
		t.Fatalf("expected provider slots payload, got %s", body)
	}
}

func TestProviderSlotsActionReturnsAllProvidersInStableOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()

	handler := NewProviderSlotsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeDashboardProviderStore{providers: []*model.Provider{
			{ID: 1, Name: "provider-a", LimitConcurrentSessions: intPtr(0), IsEnabled: &enabled},
			{ID: 2, Name: "provider-b", LimitConcurrentSessions: intPtr(2), IsEnabled: &enabled},
			{ID: 3, Name: "provider-c", LimitConcurrentSessions: intPtr(5), IsEnabled: &enabled},
		}},
		&fakeUsageLogsStore{},
	)
	handler.RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/provider-slots/getProviderSlots", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !strings.Contains(body, "\"providerId\":1") || !strings.Contains(body, "\"totalSlots\":0") || !strings.Contains(body, "\"providerId\":3") {
		t.Fatalf("expected all active providers in payload, got %s", body)
	}
	if strings.Index(body, "\"providerId\":1") > strings.Index(body, "\"providerId\":2") || strings.Index(body, "\"providerId\":2") > strings.Index(body, "\"providerId\":3") {
		t.Fatalf("expected stable provider ordering, got %s", body)
	}
}

func TestProviderSlotsActionToleratesRecentLogFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()

	handler := NewProviderSlotsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeDashboardProviderStore{providers: []*model.Provider{
			{ID: 1, Name: "provider-a", LimitConcurrentSessions: intPtr(5), IsEnabled: &enabled},
			{ID: 2, Name: "provider-b", LimitConcurrentSessions: intPtr(2), IsEnabled: &enabled},
		}},
		&fakeUsageLogsStore{recentErr: errors.New("recent logs unavailable")},
	)
	handler.RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/provider-slots/getProviderSlots", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 when recent logs fail, got %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !strings.Contains(body, "\"providerId\":1") || !strings.Contains(body, "\"usedSlots\":0") || !strings.Contains(body, "\"providerId\":2") {
		t.Fatalf("expected zero-used provider slots fallback, got %s", body)
	}
}

func TestProviderSlotsActionUsesProviderTrackerCountsWhenAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	providertrackersvc.SetCountsForTest(map[int]int{1: 4, 2: 1})
	defer providertrackersvc.ResetForTest()

	enabled := true
	router := gin.New()

	handler := NewProviderSlotsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeDashboardProviderStore{providers: []*model.Provider{
			{ID: 1, Name: "provider-a", LimitConcurrentSessions: intPtr(5), IsEnabled: &enabled},
			{ID: 2, Name: "provider-b", LimitConcurrentSessions: intPtr(2), IsEnabled: &enabled},
		}},
		&fakeUsageLogsStore{
			logs: []*model.MessageRequest{
				{ID: 1, ProviderID: 2, SessionID: stringPtr("sess_active"), CreatedAt: time.Now()},
			},
		},
	)
	handler.RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/provider-slots/getProviderSlots", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !strings.Contains(body, "\"providerId\":1") || !strings.Contains(body, "\"usedSlots\":4") || !strings.Contains(body, "\"providerId\":2") || !strings.Contains(body, "\"usedSlots\":1") {
		t.Fatalf("expected tracker-backed usedSlots, got %s", body)
	}
}
