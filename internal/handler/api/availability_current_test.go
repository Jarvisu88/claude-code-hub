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
)

type fakeAvailabilityLogStore struct {
	statuses map[int]repository.ProviderCurrentStatus
}

func (f fakeAvailabilityLogStore) GetCurrentProviderStatus(_ context.Context, _ []int, _ time.Time, _ time.Duration) (map[int]repository.ProviderCurrentStatus, error) {
	return f.statuses, nil
}

func TestCurrentAvailabilityRouteReturnsStatuses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()

	NewCurrentAvailabilityHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeDashboardProviderStore{providers: []*model.Provider{
			{ID: 1, Name: "provider-a", IsEnabled: &enabled},
			{ID: 2, Name: "provider-b", IsEnabled: &enabled},
			{ID: 3, Name: "provider-c", IsEnabled: &enabled},
		}},
		fakeAvailabilityLogStore{statuses: map[int]repository.ProviderCurrentStatus{
			1: {ProviderID: 1, GreenCount: 4, RedCount: 1, LastRequestAt: timePtr(time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC))},
			2: {ProviderID: 2, GreenCount: 1, RedCount: 3},
		}},
	).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/availability/current", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !strings.Contains(body, "\"providerId\":1") || !strings.Contains(body, "\"status\":\"green\"") || !strings.Contains(body, "\"availability\":0.8") {
		t.Fatalf("expected green provider status, got %s", body)
	}
	if !strings.Contains(body, "\"providerId\":2") || !strings.Contains(body, "\"status\":\"red\"") {
		t.Fatalf("expected red provider status, got %s", body)
	}
	if !strings.Contains(body, "\"providerId\":3") || !strings.Contains(body, "\"status\":\"unknown\"") || !strings.Contains(body, "\"requestCount\":0") {
		t.Fatalf("expected unknown provider status fallback, got %s", body)
	}
}

func timePtr(value time.Time) *time.Time { return &value }
