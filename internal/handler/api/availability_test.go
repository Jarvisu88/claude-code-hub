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

type fakeAvailabilityRowsStore struct {
	rows        []repository.AvailabilityRequestRow
	startTime   time.Time
	endTime     time.Time
	providerIDs []int
}

func (f *fakeAvailabilityRowsStore) ListAvailabilityRows(_ context.Context, startTime, endTime time.Time, providerIDs []int) ([]repository.AvailabilityRequestRow, error) {
	f.startTime = startTime
	f.endTime = endTime
	f.providerIDs = append([]int(nil), providerIDs...)
	return f.rows, nil
}

func TestAvailabilityRouteReturnsProviderSummaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	store := &fakeAvailabilityRowsStore{
		rows: []repository.AvailabilityRequestRow{
			{ProviderID: 1, CreatedAt: time.Date(2026, 4, 24, 11, 0, 0, 0, time.UTC), StatusCode: 200, DurationMs: intPtr(100)},
			{ProviderID: 1, CreatedAt: time.Date(2026, 4, 24, 11, 5, 0, 0, time.UTC), StatusCode: 500, DurationMs: intPtr(300)},
			{ProviderID: 2, CreatedAt: time.Date(2026, 4, 24, 11, 10, 0, 0, time.UTC), StatusCode: 200, DurationMs: intPtr(200)},
		},
	}
	handler := NewAvailabilityHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeDashboardProviderStore{providers: []*model.Provider{
			{ID: 1, Name: "provider-a", ProviderType: "claude", IsEnabled: &enabled},
			{ID: 2, Name: "provider-b", ProviderType: "gemini", IsEnabled: &enabled},
			{ID: 3, Name: "provider-c", ProviderType: "openai-compatible", IsEnabled: &enabled},
		}},
		store,
	)
	handler.now = func() time.Time { return time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC) }
	handler.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/availability?startTime=2026-04-24T11:00:00Z&endTime=2026-04-24T12:00:00Z&bucketSizeMinutes=15&maxBuckets=10", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !strings.Contains(body, "\"providerId\":1") || !strings.Contains(body, "\"currentStatus\":\"green\"") || !strings.Contains(body, "\"providerType\":\"claude\"") {
		t.Fatalf("expected provider-a summary, got %s", body)
	}
	if !strings.Contains(body, "\"providerId\":2") || !strings.Contains(body, "\"currentAvailability\":1") {
		t.Fatalf("expected provider-b summary, got %s", body)
	}
	if !strings.Contains(body, "\"providerId\":3") || !strings.Contains(body, "\"currentStatus\":\"unknown\"") {
		t.Fatalf("expected provider-c no-data summary, got %s", body)
	}
	if !strings.Contains(body, "\"systemAvailability\":0.666666") {
		t.Fatalf("expected weighted system availability, got %s", body)
	}
	if len(store.providerIDs) != 3 {
		t.Fatalf("expected provider ids filter to include active providers, got %+v", store.providerIDs)
	}
}

func TestAvailabilityRouteRejectsInvalidQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	handler := NewAvailabilityHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeDashboardProviderStore{},
		&fakeAvailabilityRowsStore{},
	)
	handler.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/availability?bucketSizeMinutes=0.1", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
	}
}
