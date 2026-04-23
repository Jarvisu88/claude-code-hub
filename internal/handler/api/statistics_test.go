package api

import (
	"context"
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

type fakeStatisticsStore struct {
	rows       []*repository.UserStatRow
	keyRows    []*repository.KeyStatRow
	mixed      *repository.MixedStatistics
	users      []*repository.ActiveUserItem
	activeKeys []*repository.ActiveKeyItem
}

func (f fakeStatisticsStore) GetUserStatistics(_ context.Context, _ repository.TimeRange, _ string) ([]*repository.UserStatRow, error) {
	return f.rows, nil
}

func (f fakeStatisticsStore) GetKeyStatistics(_ context.Context, _ int, _ repository.TimeRange, _ string) ([]*repository.KeyStatRow, error) {
	return f.keyRows, nil
}

func (f fakeStatisticsStore) GetMixedStatistics(_ context.Context, _ int, _ repository.TimeRange, _ string) (*repository.MixedStatistics, error) {
	return f.mixed, nil
}

func (f fakeStatisticsStore) GetActiveUsers(_ context.Context) ([]*repository.ActiveUserItem, error) {
	return f.users, nil
}

func (f fakeStatisticsStore) GetActiveKeysForUser(_ context.Context, _ int) ([]*repository.ActiveKeyItem, error) {
	return f.activeKeys, nil
}

func TestStatisticsActionReturnsRows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	NewStatisticsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeStatisticsStore{
			rows:  []*repository.UserStatRow{{UserID: 1, UserName: "alice", Date: time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC), APICalls: 3, TotalCost: udecimal.MustParse("1.5")}},
			users: []*repository.ActiveUserItem{{ID: 1, Name: "alice"}},
		},
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/statistics/getUserStatistics", strings.NewReader(`{"timeRange":"today"}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "alice") || !strings.Contains(resp.Body.String(), "\"mode\":\"users\"") || !strings.Contains(resp.Body.String(), "\"resolution\":\"hour\"") || !strings.Contains(resp.Body.String(), "\"dataKey\":\"user-1\"") || !strings.Contains(resp.Body.String(), "\"user-1_cost\":\"1.500000\"") {
		t.Fatalf("expected statistics payload, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestStatisticsActionReturnsKeyMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	NewStatisticsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeStatisticsStore{
			keyRows:    []*repository.KeyStatRow{{KeyID: 2, KeyName: "key-a", Date: time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC), APICalls: 4, TotalCost: udecimal.MustParse("2")}},
			activeKeys: []*repository.ActiveKeyItem{{ID: 2, Name: "key-a"}},
		},
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/statistics/getUserStatistics", strings.NewReader(`{"timeRange":"7days","mode":"keys","userId":1}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "\"mode\":\"keys\"") || !strings.Contains(resp.Body.String(), "\"dataKey\":\"key-2\"") || !strings.Contains(resp.Body.String(), "\"key-2_cost\":\"2.000000\"") {
		t.Fatalf("expected key statistics payload, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestStatisticsActionReturnsMixedMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	NewStatisticsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeStatisticsStore{
			mixed: &repository.MixedStatistics{
				OwnKeys:         []*repository.KeyStatRow{{KeyID: 2, KeyName: "key-a", Date: time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC), APICalls: 4, TotalCost: udecimal.MustParse("2")}},
				OthersAggregate: []*repository.UserStatRow{{UserID: -1, UserName: "其他用户", Date: time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC), APICalls: 6, TotalCost: udecimal.MustParse("3.25")}},
			},
			activeKeys: []*repository.ActiveKeyItem{{ID: 2, Name: "key-a"}},
		},
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/statistics/getUserStatistics", strings.NewReader(`{"timeRange":"7days","mode":"mixed","userId":1}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "\"mode\":\"mixed\"") || !strings.Contains(resp.Body.String(), "\"dataKey\":\"key--1\"") || !strings.Contains(resp.Body.String(), "__others__") || !strings.Contains(resp.Body.String(), "\"key--1_cost\":\"3.250000\"") {
		t.Fatalf("expected mixed statistics payload, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestStatisticsActionRejectsMissingUserIDForKeyModes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	NewStatisticsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeStatisticsStore{},
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/statistics/getUserStatistics", strings.NewReader(`{"mode":"keys"}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing userId, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestStatisticsActionFallsBackEntityNames(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	NewStatisticsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeStatisticsStore{
			rows:  []*repository.UserStatRow{{UserID: 7, Date: time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC), APICalls: 1, TotalCost: udecimal.MustParse("0")}},
			users: []*repository.ActiveUserItem{{ID: 7, Name: ""}},
		},
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/statistics/getUserStatistics", strings.NewReader(`{"timeRange":"today"}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "\"name\":\"User7\"") {
		t.Fatalf("expected fallback entity name, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestStatisticsActionRoundsCostStringsToSixDecimals(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	NewStatisticsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeStatisticsStore{
			rows:  []*repository.UserStatRow{{UserID: 1, UserName: "alice", Date: time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC), APICalls: 1, TotalCost: udecimal.MustParse("1.23456789")}},
			users: []*repository.ActiveUserItem{{ID: 1, Name: "alice"}},
		},
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/statistics/getUserStatistics", strings.NewReader(`{"timeRange":"today"}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "\"user-1_cost\":\"1.234568\"") {
		t.Fatalf("expected rounded cost string, got %d: %s", resp.Code, resp.Body.String())
	}
}
