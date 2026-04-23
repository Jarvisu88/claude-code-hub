package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/repository"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

type fakeCountStore struct{ count int }

func (f fakeCountStore) Count(_ context.Context, _ bool) (int, error) { return f.count, nil }

func TestOverviewActionReturnsOverviewData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	store := &fakeUsageLogsStore{
		summary:  repository.MessageRequestSummary{TotalRequests: 5, TotalCost: 1.5},
		overview: repository.MessageRequestOverviewMetrics{TodayRequests: 3, TodayCost: 0.25, AvgResponseTime: 120, TodayErrorRate: 33.33, YesterdaySamePeriodRequests: 2, YesterdaySamePeriodCost: 0.1, YesterdaySamePeriodAvgResponseTime: 98, RecentMinuteRequests: 1, ConcurrentSessions: 2},
	}
	router := gin.New()
	NewOverviewActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeCountStore{count: 2},
		fakeCountStore{count: 3},
		fakeCountStore{count: 4},
		store,
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/overview/getOverviewData", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "\"totalUsers\":2") || !strings.Contains(resp.Body.String(), "\"totalRequests\":5") || !strings.Contains(resp.Body.String(), "\"todayRequests\":3") || !strings.Contains(resp.Body.String(), "\"avgResponseTime\":120") || !strings.Contains(resp.Body.String(), "\"concurrentSessions\":2") {
		t.Fatalf("expected overview payload, got %d: %s", resp.Code, resp.Body.String())
	}
	if store.overviewLocation == nil || store.overviewLocation.String() != repository.DefaultTimezone {
		t.Fatalf("expected overview to use repository default timezone, got %+v", store.overviewLocation)
	}
}
