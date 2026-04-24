package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/repository"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/quagmt/udecimal"
)

type fakeDashboardProviderStore struct{ providers []*model.Provider }

func (f fakeDashboardProviderStore) GetActiveProviders(_ context.Context) ([]*model.Provider, error) {
	return f.providers, nil
}

func TestDashboardRealtimeActionReturnsBaselinePayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	origNow := dashboardRealtimeNow
	dashboardRealtimeNow = func() time.Time { return time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC) }
	defer func() { dashboardRealtimeNow = origNow }()
	enabled := true
	router := gin.New()

	logsStore := &fakeUsageLogsStore{
		logs: []*model.MessageRequest{
			{
				ID:            1,
				UserID:        7,
				ProviderID:    1,
				Model:         "gpt-5.4",
				OriginalModel: stringPtr("gpt-5.4"),
				CreatedAt:     time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC),
				DurationMs:    intPtr(120),
				StatusCode:    intPtr(200),
				SessionID:     stringPtr("sess_123"),
				UserName:      stringPtr("alice"),
				ProviderName:  stringPtr("provider-a"),
				CostUSD:       udecimal.MustParse("0.25"),
			},
			{
				ID:           99,
				UserID:       99,
				ProviderID:   99,
				Model:        "old-model",
				CreatedAt:    time.Date(2026, 4, 22, 9, 0, 0, 0, time.UTC),
				DurationMs:   intPtr(10),
				StatusCode:   intPtr(200),
				UserName:     stringPtr("yesterday-user"),
				ProviderName: stringPtr("yesterday-provider"),
				CostUSD:      udecimal.MustParse("99"),
				InputTokens:  intPtr(1),
				OutputTokens: intPtr(1),
			},
			{
				ID:            2,
				UserID:        8,
				ProviderID:    2,
				Model:         "gpt-4o-mini",
				OriginalModel: stringPtr("gpt-4o-mini"),
				CreatedAt:     time.Date(2026, 4, 23, 9, 10, 0, 0, time.UTC),
				DurationMs:    intPtr(80),
				StatusCode:    intPtr(200),
				UserName:      stringPtr("bob"),
				ProviderName:  stringPtr("provider-b"),
				CostUSD:       udecimal.MustParse("0.75"),
				InputTokens:   intPtr(100),
				OutputTokens:  intPtr(50),
				TtfbMs:        intPtr(40),
			},
			{
				ID:           3,
				UserID:       9,
				ProviderID:   2,
				Model:        "gpt-4o-mini",
				CreatedAt:    time.Date(2026, 4, 23, 9, 20, 0, 0, time.UTC),
				SessionID:    stringPtr("sess_active"),
				UserName:     stringPtr("carol"),
				ProviderName: stringPtr("provider-b"),
			},
			{
				ID:         4,
				UserID:     10,
				ProviderID: 3,
				Model:      "gpt-4.1",
				CreatedAt:  time.Date(2026, 4, 23, 9, 30, 0, 0, time.UTC),
				DurationMs: intPtr(50),
				StatusCode: intPtr(200),
				UserName:   stringPtr("dave"),
				TtfbMs:     intPtr(10),
			},
		},
		overview: repository.MessageRequestOverviewMetrics{
			TodayRequests:                      3,
			TodayCost:                          0.25,
			AvgResponseTime:                    120,
			TodayErrorRate:                     0,
			YesterdaySamePeriodRequests:        2,
			YesterdaySamePeriodCost:            0.1,
			YesterdaySamePeriodAvgResponseTime: 100,
			RecentMinuteRequests:               1,
			ConcurrentSessions:                 2,
		},
	}
	statsStore := fakeStatisticsStore{
		rows: []*repository.UserStatRow{
			{UserID: 7, UserName: "alice", Date: time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC), APICalls: 3, TotalCost: udecimal.MustParse("1.0")},
		},
		users: []*repository.ActiveUserItem{{ID: 7, Name: "alice"}},
	}
	providerStore := fakeDashboardProviderStore{providers: []*model.Provider{
		{ID: 1, Name: "provider-a", LimitConcurrentSessions: intPtr(5)},
		{ID: 2, Name: "provider-b", LimitConcurrentSessions: intPtr(2)},
	}}

	NewDashboardRealtimeActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		logsStore,
		statsStore,
		providerStore,
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/dashboard-realtime/getDashboardRealtimeData", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !strings.Contains(body, "\"metrics\"") || !strings.Contains(body, "\"activityStream\"") || !strings.Contains(body, "\"trendData\"") {
		t.Fatalf("expected dashboard realtime payload sections, got %s", body)
	}
	if !strings.Contains(body, "\"concurrentSessions\":2") || !strings.Contains(body, "\"id\":\"sess_123\"") || !strings.Contains(body, "\"user\":\"alice\"") {
		t.Fatalf("expected metrics/activity stream data, got %s", body)
	}
	if !strings.Contains(body, "\"id\":\"sess_active\"") || !strings.Contains(body, "\"status\":0") || !strings.Contains(body, "\"provider\":\"\"") {
		t.Fatalf("expected active activity stream item semantics, got %s", body)
	}
	if !strings.Contains(body, "\"provider\":\"Unknown\"") {
		t.Fatalf("expected finalized item without providerName to use Unknown fallback, got %s", body)
	}
	if strings.Contains(body, "\"userName\":\"yesterday-user\"") || strings.Contains(body, "\"providerName\":\"yesterday-provider\"") {
		t.Fatalf("expected daily rankings to exclude yesterday entries, got %s", body)
	}
	if !strings.Contains(body, "\"userRankings\"") || !strings.Contains(body, "\"providerRankings\"") || !strings.Contains(body, "\"modelDistribution\"") {
		t.Fatalf("expected rankings/distribution sections, got %s", body)
	}
	if !strings.Contains(body, "\"userName\":\"bob\"") || !strings.Contains(body, "\"providerName\":\"provider-b\"") || !strings.Contains(body, "\"model\":\"gpt-4o-mini\"") || !strings.Contains(body, "\"providerSlots\"") || !strings.Contains(body, "\"totalSlots\":2") || !strings.Contains(body, "\"totalVolume\":150") {
		t.Fatalf("expected aggregated ranking/distribution data, got %s", body)
	}
	if !strings.Contains(body, "\"avgTtfbMs\":40") || !strings.Contains(body, "\"avgTokensPerSecond\":1875") || !strings.Contains(body, "\"successRate\":1") {
		t.Fatalf("expected provider/model derived metrics, got %s", body)
	}
	if !strings.Contains(body, "\"avgCostPerRequest\":0.75") || !strings.Contains(body, "\"avgCostPerMillionTokens\":5000") {
		t.Fatalf("expected provider cost derived metrics, got %s", body)
	}
	if !strings.Contains(body, "\"totalCost\":0.75") {
		t.Fatalf("expected rounded totalCost fields in dashboard aggregates, got %s", body)
	}
	if !strings.Contains(body, "\"hour\":0") || !strings.Contains(body, "\"hour\":23") {
		t.Fatalf("expected trendData to cover full 24 hours, got %s", body)
	}
}

func TestDashboardRealtimeActivityStreamSortedByStartTimeDescAndLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	origNow := dashboardRealtimeNow
	dashboardRealtimeNow = func() time.Time { return time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC) }
	defer func() { dashboardRealtimeNow = origNow }()
	enabled := true
	router := gin.New()

	logs := make([]*model.MessageRequest, 0, 25)
	for i := 0; i < 25; i++ {
		logs = append(logs, &model.MessageRequest{
			ID:           i + 1,
			UserID:       1,
			ProviderID:   1,
			Model:        "gpt-5.4",
			CreatedAt:    time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Minute),
			DurationMs:   intPtr(10),
			StatusCode:   intPtr(200),
			UserName:     stringPtr("alice"),
			ProviderName: stringPtr("provider-a"),
		})
	}

	NewDashboardRealtimeActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		&fakeUsageLogsStore{
			logs: logs,
			overview: repository.MessageRequestOverviewMetrics{
				TodayRequests:        1,
				RecentMinuteRequests: 1,
			},
		},
		fakeStatisticsStore{
			rows:  []*repository.UserStatRow{{UserID: 1, UserName: "alice", Date: time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC), APICalls: 1, TotalCost: udecimal.MustParse("0")}},
			users: []*repository.ActiveUserItem{{ID: 1, Name: "alice"}},
		},
		fakeDashboardProviderStore{providers: []*model.Provider{{ID: 1, Name: "provider-a", LimitConcurrentSessions: intPtr(1), IsEnabled: &enabled}}},
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/dashboard-realtime/getDashboardRealtimeData", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if strings.Contains(body, "\"id\":\"req-1\"") {
		t.Fatalf("expected oldest activity stream item to be trimmed, got %s", body)
	}
	if !strings.Contains(body, "\"id\":\"req-25\"") {
		t.Fatalf("expected newest activity stream item to remain, got %s", body)
	}
}

func TestDashboardRealtimeActivityStreamDedupesBySessionID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	origNow := dashboardRealtimeNow
	dashboardRealtimeNow = func() time.Time { return time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC) }
	defer func() { dashboardRealtimeNow = origNow }()
	enabled := true
	router := gin.New()

	logs := []*model.MessageRequest{
		{
			ID:           1,
			UserID:       1,
			ProviderID:   1,
			Model:        "gpt-5.4",
			CreatedAt:    time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC),
			DurationMs:   intPtr(10),
			StatusCode:   intPtr(200),
			SessionID:    stringPtr("sess_dup"),
			UserName:     stringPtr("alice"),
			ProviderName: stringPtr("provider-a"),
		},
		{
			ID:           2,
			UserID:       1,
			ProviderID:   1,
			Model:        "gpt-5.4",
			CreatedAt:    time.Date(2026, 4, 23, 9, 1, 0, 0, time.UTC),
			DurationMs:   intPtr(10),
			StatusCode:   intPtr(200),
			SessionID:    stringPtr("sess_dup"),
			UserName:     stringPtr("alice"),
			ProviderName: stringPtr("provider-a"),
		},
	}

	NewDashboardRealtimeActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		&fakeUsageLogsStore{
			logs: logs,
			overview: repository.MessageRequestOverviewMetrics{
				TodayRequests:        1,
				RecentMinuteRequests: 1,
			},
		},
		fakeStatisticsStore{
			rows:  []*repository.UserStatRow{{UserID: 1, UserName: "alice", Date: time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC), APICalls: 1, TotalCost: udecimal.MustParse("0")}},
			users: []*repository.ActiveUserItem{{ID: 1, Name: "alice"}},
		},
		fakeDashboardProviderStore{providers: []*model.Provider{{ID: 1, Name: "provider-a", LimitConcurrentSessions: intPtr(1), IsEnabled: &enabled}}},
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/dashboard-realtime/getDashboardRealtimeData", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if strings.Count(body, "\"id\":\"sess_dup\"") != 1 {
		t.Fatalf("expected single activity stream entry per session, got %s", body)
	}
	if !strings.Contains(body, "\"startTime\":1776934860000") {
		t.Fatalf("expected dedupe to keep newest session record, got %s", body)
	}
}

func TestDashboardRealtimeProviderSlotsSortedAndLimitedToTopThree(t *testing.T) {
	gin.SetMode(gin.TestMode)
	origNow := dashboardRealtimeNow
	dashboardRealtimeNow = func() time.Time { return time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC) }
	defer func() { dashboardRealtimeNow = origNow }()
	enabled := true
	router := gin.New()

	logs := []*model.MessageRequest{
		{ID: 1, ProviderID: 1, SessionID: stringPtr("sess-1"), CreatedAt: time.Now()},
		{ID: 2, ProviderID: 2, SessionID: stringPtr("sess-2"), CreatedAt: time.Now()},
		{ID: 3, ProviderID: 2, SessionID: stringPtr("sess-3"), CreatedAt: time.Now()},
		{ID: 4, ProviderID: 3, SessionID: stringPtr("sess-4"), CreatedAt: time.Now()},
		{ID: 5, ProviderID: 3, SessionID: stringPtr("sess-5"), CreatedAt: time.Now()},
		{ID: 6, ProviderID: 3, SessionID: stringPtr("sess-6"), CreatedAt: time.Now()},
		{ID: 7, ProviderID: 4, SessionID: stringPtr("sess-7"), CreatedAt: time.Now()},
	}

	NewDashboardRealtimeActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		&fakeUsageLogsStore{
			logs: logs,
			overview: repository.MessageRequestOverviewMetrics{
				TodayRequests:        1,
				RecentMinuteRequests: 1,
			},
		},
		fakeStatisticsStore{
			rows:  []*repository.UserStatRow{{UserID: 1, UserName: "alice", Date: time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC), APICalls: 1, TotalCost: udecimal.MustParse("0")}},
			users: []*repository.ActiveUserItem{{ID: 1, Name: "alice"}},
		},
		fakeDashboardProviderStore{providers: []*model.Provider{
			{ID: 1, Name: "provider-a", LimitConcurrentSessions: intPtr(10), IsEnabled: &enabled},
			{ID: 2, Name: "provider-b", LimitConcurrentSessions: intPtr(4), IsEnabled: &enabled},
			{ID: 3, Name: "provider-c", LimitConcurrentSessions: intPtr(3), IsEnabled: &enabled},
			{ID: 4, Name: "provider-d", LimitConcurrentSessions: intPtr(20), IsEnabled: &enabled},
		}},
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/dashboard-realtime/getDashboardRealtimeData", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if strings.Contains(body, "\"providerId\":4") {
		t.Fatalf("expected lowest usage provider to be trimmed from top 3, got %s", body)
	}
	if !strings.Contains(body, "\"providerId\":3") || !strings.Contains(body, "\"providerId\":2") || !strings.Contains(body, "\"providerId\":1") {
		t.Fatalf("expected top 3 provider slots by usage, got %s", body)
	}
}

func TestDashboardRealtimeRankingsAndDistributionAreLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	origNow := dashboardRealtimeNow
	dashboardRealtimeNow = func() time.Time { return time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC) }
	defer func() { dashboardRealtimeNow = origNow }()

	enabled := true
	router := gin.New()

	logs := make([]*model.MessageRequest, 0, 12)
	for i := 0; i < 12; i++ {
		userID := 100 + i
		providerID := 200 + i
		modelName := "model-" + strconv.Itoa(i)
		logs = append(logs, &model.MessageRequest{
			ID:            i + 1,
			UserID:        userID,
			ProviderID:    providerID,
			Model:         modelName,
			OriginalModel: stringPtr(modelName),
			CreatedAt:     time.Date(2026, 4, 23, 9, i, 0, 0, time.UTC),
			DurationMs:    intPtr(10),
			StatusCode:    intPtr(200),
			UserName:      stringPtr("user-" + strconv.Itoa(i)),
			ProviderName:  stringPtr("provider-" + strconv.Itoa(i)),
			CostUSD:       udecimal.MustParse(strconv.Itoa(12 - i)),
		})
	}

	NewDashboardRealtimeActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		&fakeUsageLogsStore{
			logs: logs,
			overview: repository.MessageRequestOverviewMetrics{
				TodayRequests:        12,
				RecentMinuteRequests: 1,
			},
		},
		fakeStatisticsStore{
			rows:  []*repository.UserStatRow{{UserID: 1, UserName: "alice", Date: time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC), APICalls: 1, TotalCost: udecimal.MustParse("0")}},
			users: []*repository.ActiveUserItem{{ID: 1, Name: "alice"}},
		},
		fakeDashboardProviderStore{providers: []*model.Provider{}},
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/dashboard-realtime/getDashboardRealtimeData", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected object data payload, got %T", payload["data"])
	}
	if userRankings, ok := data["userRankings"].([]any); !ok || len(userRankings) != 5 {
		t.Fatalf("expected user rankings length 5, got %#v", data["userRankings"])
	}
	if providerRankings, ok := data["providerRankings"].([]any); !ok || len(providerRankings) != 5 {
		t.Fatalf("expected provider rankings length 5, got %#v", data["providerRankings"])
	}
	if modelDistribution, ok := data["modelDistribution"].([]any); !ok || len(modelDistribution) != 10 {
		t.Fatalf("expected model distribution length 10, got %#v", data["modelDistribution"])
	}
}
