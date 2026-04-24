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

type fakeLeaderboardLogStore struct {
	rows      []repository.LeaderboardRequestRow
	startTime time.Time
	endTime   time.Time
}

func (f *fakeLeaderboardLogStore) ListLeaderboardRows(_ context.Context, startTime, endTime time.Time) ([]repository.LeaderboardRequestRow, error) {
	f.startTime = startTime
	f.endTime = endTime
	return f.rows, nil
}

func TestLeaderboardRouteReturnsProviderScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	store := &fakeLeaderboardLogStore{
		rows: []repository.LeaderboardRequestRow{
			{UserID: 1, UserName: "alice", ProviderID: 7, ProviderName: "provider-a", ProviderType: "claude", Model: "gpt-5.4", StatusCode: 200, CostUSD: udecimal.MustParse("1.5"), DurationMs: intPtr(100), TtfbMs: intPtr(30), InputTokens: intPtr(100), OutputTokens: intPtr(50)},
			{UserID: 2, UserName: "bob", ProviderID: 7, ProviderName: "provider-a", ProviderType: "claude", Model: "gpt-5.4", StatusCode: 500, CostUSD: udecimal.MustParse("0.5"), DurationMs: intPtr(200), TtfbMs: intPtr(60), InputTokens: intPtr(50), OutputTokens: intPtr(50)},
			{UserID: 1, UserName: "alice", ProviderID: 8, ProviderName: "provider-b", ProviderType: "gemini", Model: "gpt-4o-mini", StatusCode: 200, CostUSD: udecimal.MustParse("2.0"), DurationMs: intPtr(100), TtfbMs: intPtr(20), InputTokens: intPtr(100), OutputTokens: intPtr(100)},
		},
	}
	handler := NewLeaderboardHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	)
	handler.now = func() time.Time { return time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC) }
	handler.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/leaderboard?period=daily&scope=provider", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !strings.Contains(body, "\"providerId\":8") || !strings.Contains(body, "\"providerName\":\"provider-b\"") || !strings.Contains(body, "\"successRate\":1") {
		t.Fatalf("expected provider-b leaderboard entry, got %s", body)
	}
	if !strings.Contains(body, "\"providerId\":7") || !strings.Contains(body, "\"avgTtfbMs\":45") {
		t.Fatalf("expected aggregated provider-a metrics, got %s", body)
	}
}

func TestLeaderboardRouteSupportsProviderTypeAndModelStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	store := &fakeLeaderboardLogStore{
		rows: []repository.LeaderboardRequestRow{
			{UserID: 1, UserName: "alice", ProviderID: 7, ProviderName: "provider-a", ProviderType: "claude", Model: "gpt-5.4", StatusCode: 200, CostUSD: udecimal.MustParse("1.5"), DurationMs: intPtr(100), TtfbMs: intPtr(30), InputTokens: intPtr(100), OutputTokens: intPtr(50)},
			{UserID: 2, UserName: "bob", ProviderID: 7, ProviderName: "provider-a", ProviderType: "claude", Model: "gpt-4.1", StatusCode: 200, CostUSD: udecimal.MustParse("0.5"), DurationMs: intPtr(200), TtfbMs: intPtr(60), InputTokens: intPtr(50), OutputTokens: intPtr(50)},
			{UserID: 1, UserName: "alice", ProviderID: 8, ProviderName: "provider-b", ProviderType: "gemini", Model: "gemini-2.5-pro", StatusCode: 200, CostUSD: udecimal.MustParse("2.0"), DurationMs: intPtr(100), TtfbMs: intPtr(20), InputTokens: intPtr(100), OutputTokens: intPtr(100)},
		},
	}
	handler := NewLeaderboardHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	)
	handler.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/leaderboard?period=daily&scope=provider&providerType=claude&includeModelStats=1", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if strings.Contains(body, "\"providerId\":8") || !strings.Contains(body, "\"providerId\":7") {
		t.Fatalf("expected providerType filter to exclude provider-b, got %s", body)
	}
	if !strings.Contains(body, "\"modelStats\":[") || !strings.Contains(body, "\"model\":\"gpt-5.4\"") || !strings.Contains(body, "\"model\":\"gpt-4.1\"") {
		t.Fatalf("expected provider modelStats in payload, got %s", body)
	}
}

func TestLeaderboardRouteSupportsUserModelStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	store := &fakeLeaderboardLogStore{
		rows: []repository.LeaderboardRequestRow{
			{UserID: 1, UserName: "alice", ProviderID: 7, ProviderName: "provider-a", Model: "gpt-5.4", StatusCode: 200, CostUSD: udecimal.MustParse("1.5"), InputTokens: intPtr(100), OutputTokens: intPtr(50)},
			{UserID: 1, UserName: "alice", ProviderID: 8, ProviderName: "provider-b", Model: "gpt-4o-mini", StatusCode: 200, CostUSD: udecimal.MustParse("2.0"), InputTokens: intPtr(100), OutputTokens: intPtr(100)},
		},
	}
	handler := NewLeaderboardHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	)
	handler.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/leaderboard?period=daily&scope=user&includeUserModelStats=1", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !strings.Contains(body, "\"userId\":1") || !strings.Contains(body, "\"modelStats\":[") || !strings.Contains(body, "\"model\":\"gpt-5.4\"") || !strings.Contains(body, "\"model\":\"gpt-4o-mini\"") {
		t.Fatalf("expected user modelStats in payload, got %s", body)
	}
}

func TestLeaderboardRouteRejectsUnsupportedScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	handler := NewLeaderboardHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		&fakeLeaderboardLogStore{},
	)
	handler.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/leaderboard?scope=userCacheHitRate", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
	}
}
