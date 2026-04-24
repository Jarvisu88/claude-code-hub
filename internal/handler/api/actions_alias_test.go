package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/repository"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/quagmt/udecimal"
)

func TestActionStyleAliasRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	handler := NewHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeUserLister{users: []*model.User{{ID: 1, Name: "alice"}}},
		fakeKeyLister{keys: []*model.Key{{ID: 1, Name: "key-1"}}},
		fakeProviderLister{providers: []*model.Provider{{ID: 1, Name: "provider-1"}}},
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/actions"))

	tests := []struct {
		name         string
		path         string
		body         string
		wantStatus   int
		wantContains string
	}{
		{name: "getUsers alias", path: "/api/actions/users/getUsers", body: `{}`, wantStatus: http.StatusOK, wantContains: "alice"},
		{name: "getUser alias", path: "/api/actions/users/getUser", body: `{"id":1}`, wantStatus: http.StatusOK, wantContains: "alice"},
		{name: "addUser alias", path: "/api/actions/users/addUser", body: `{"name":"bob"}`, wantStatus: http.StatusCreated, wantContains: "bob"},
		{name: "editUser alias", path: "/api/actions/users/editUser", body: `{"id":1,"name":"bob2"}`, wantStatus: http.StatusOK, wantContains: "bob2"},
		{name: "removeUser alias", path: "/api/actions/users/removeUser", body: `{"id":1}`, wantStatus: http.StatusOK, wantContains: "\"deleted\":true"},
		{name: "getKeys alias", path: "/api/actions/keys/getKeys", body: `{}`, wantStatus: http.StatusOK, wantContains: "key-1"},
		{name: "getProviders alias", path: "/api/actions/providers/getProviders", body: `{}`, wantStatus: http.StatusOK, wantContains: "provider-1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer admin-token")
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tc.wantStatus, resp.Code, resp.Body.String())
			}
			if !strings.Contains(resp.Body.String(), tc.wantContains) {
				t.Fatalf("expected body to contain %q, got %s", tc.wantContains, resp.Body.String())
			}
		})
	}
}

func TestActionStyleTelemetryRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	router := gin.New()

	logsStore := &fakeUsageLogsStore{
		logs: []*model.MessageRequest{{
			ID:           1,
			UserID:       1,
			ProviderID:   1,
			Model:        "gpt-5.4",
			CreatedAt:    time.Now(),
			DurationMs:   intPtr(100),
			StatusCode:   intPtr(200),
			UserName:     stringPtr("alice"),
			ProviderName: stringPtr("provider-a"),
		}},
		summary:  repository.MessageRequestSummary{TotalRequests: 1, TotalCost: 1.25},
		overview: repository.MessageRequestOverviewMetrics{TodayRequests: 1, ConcurrentSessions: 1, RecentMinuteRequests: 1},
	}
	statsStore := fakeStatisticsStore{
		rows:  []*repository.UserStatRow{{UserID: 1, UserName: "alice", Date: time.Now(), APICalls: 1, TotalCost: udecimal.MustParse("1.25")}},
		users: []*repository.ActiveUserItem{{ID: 1, Name: "alice"}},
	}
	providerStore := fakeDashboardProviderStore{providers: []*model.Provider{{ID: 1, Name: "provider-a", LimitConcurrentSessions: intPtr(2), IsEnabled: &enabled}}}

	NewProviderSlotsActionHandler(fakeAdminAuth{result: &authsvc.AuthResult{
		IsAdmin: true,
		User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
		Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
		APIKey:  "admin-token",
	}}, providerStore, logsStore).RegisterRoutes(router.Group("/api/actions"))
	NewDashboardRealtimeActionHandler(fakeAdminAuth{result: &authsvc.AuthResult{
		IsAdmin: true,
		User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
		Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
		APIKey:  "admin-token",
	}}, logsStore, statsStore, providerStore).RegisterRoutes(router.Group("/api/actions"))

	tests := []struct {
		path         string
		body         string
		wantContains string
	}{
		{path: "/api/actions/provider-slots/getProviderSlots", body: `{}`, wantContains: "\"providerId\":1"},
		{path: "/api/actions/dashboard-realtime/getDashboardRealtimeData", body: `{}`, wantContains: "\"metrics\""},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer admin-token")
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
			}
			if !strings.Contains(resp.Body.String(), tc.wantContains) {
				t.Fatalf("expected body to contain %q, got %s", tc.wantContains, resp.Body.String())
			}
		})
	}
}

func TestActionStyleAnalyticsRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	router := gin.New()

	logsStore := &fakeUsageLogsStore{
		summary:  repository.MessageRequestSummary{TotalRows: 1, TotalRequests: 1, TotalCost: 1.25},
		overview: repository.MessageRequestOverviewMetrics{TodayRequests: 1, ConcurrentSessions: 1, RecentMinuteRequests: 1},
	}
	statsStore := fakeStatisticsStore{
		rows:  []*repository.UserStatRow{{UserID: 1, UserName: "alice", Date: time.Now(), APICalls: 1, TotalCost: udecimal.MustParse("1.25")}},
		users: []*repository.ActiveUserItem{{ID: 1, Name: "alice"}},
	}
	adminAuth := fakeAdminAuth{result: &authsvc.AuthResult{
		IsAdmin: true,
		User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
		Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
		APIKey:  "admin-token",
	}}

	NewStatisticsActionHandler(adminAuth, statsStore).RegisterRoutes(router.Group("/api/actions"))
	NewOverviewActionHandler(adminAuth, fakeCountStore{count: 2}, fakeCountStore{count: 3}, fakeCountStore{count: 4}, logsStore).RegisterRoutes(router.Group("/api/actions"))
	NewProxyStatusHandler(adminAuth, fakeProxyStatusUserStore{users: []*model.User{{ID: 1, Name: "alice"}}}, &fakeProxyStatusLogStore{}).RegisterActionRoutes(router.Group("/api/actions"))

	tests := []struct {
		path         string
		body         string
		wantContains string
	}{
		{path: "/api/actions/statistics/getUserStatistics", body: `{"timeRange":"today"}`, wantContains: "\"mode\":\"users\""},
		{path: "/api/actions/overview/getOverviewData", body: `{}`, wantContains: "\"todayRequests\":1"},
		{path: "/api/actions/proxy-status/getProxyStatus", body: `{}`, wantContains: "\"users\""},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer admin-token")
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
			}
			if !strings.Contains(resp.Body.String(), tc.wantContains) {
				t.Fatalf("expected body to contain %q, got %s", tc.wantContains, resp.Body.String())
			}
		})
	}
}
