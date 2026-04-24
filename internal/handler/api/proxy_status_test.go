package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	sessiontrackersvc "github.com/ding113/claude-code-hub/internal/service/sessiontracker"
	"github.com/gin-gonic/gin"
)

type fakeProxyStatusUserStore struct{ users []*model.User }

func (f fakeProxyStatusUserStore) List(_ context.Context, _ *repository.ListOptions) ([]*model.User, error) {
	return f.users, nil
}

type fakeProxyStatusLogStore struct {
	logs               []*model.MessageRequest
	latestBySession    []*model.MessageRequest
	latestSessionIDs   []string
	latestSessionLimit int
}

func (f fakeProxyStatusLogStore) ListRecent(_ context.Context, _ int) ([]*model.MessageRequest, error) {
	return f.logs, nil
}

func (f *fakeProxyStatusLogStore) FindLatestBySessionIDs(_ context.Context, sessionIDs []string, limit int) ([]*model.MessageRequest, error) {
	f.latestSessionIDs = append([]string(nil), sessionIDs...)
	f.latestSessionLimit = limit
	return f.latestBySession, nil
}

func TestProxyStatusRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	handler := NewProxyStatusHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeProxyStatusUserStore{users: []*model.User{{ID: 1, Name: "alice"}}},
		&fakeProxyStatusLogStore{logs: []*model.MessageRequest{
			{ID: 9, UserID: 1, Key: "key-a", KeyName: stringPtr("Key A"), ProviderID: 7, ProviderName: stringPtr("provider-a"), ProviderChain: []model.ProviderChainItem{{ID: 7, Name: "provider-a"}}, Model: "gpt-5.4", CreatedAt: time.Now(), DurationMs: intPtr(250), StatusCode: intPtr(200)},
			{ID: 8, UserID: 1, Key: "key-b", KeyName: stringPtr("Key B"), ProviderID: 8, ProviderName: stringPtr("provider-b"), ProviderChain: []model.ProviderChainItem{{ID: 8, Name: "provider-b"}}, Model: "gpt-4o-mini", CreatedAt: time.Now().Add(-time.Minute), SessionID: stringPtr("sess_active")},
		}},
	)

	router := gin.New()
	handler.RegisterDirectRoutes(router)
	handler.RegisterActionRoutes(router.Group("/api/actions"))

	directReq := httptest.NewRequest(http.MethodGet, "/api/proxy-status", nil)
	directReq.Header.Set("Authorization", "Bearer admin-token")
	directResp := httptest.NewRecorder()
	router.ServeHTTP(directResp, directReq)
	if directResp.Code != http.StatusOK || !strings.Contains(directResp.Body.String(), "alice") || !strings.Contains(directResp.Body.String(), "\"activeCount\":1") {
		t.Fatalf("expected direct proxy status payload, got %d: %s", directResp.Code, directResp.Body.String())
	}
	if !strings.Contains(directResp.Body.String(), "\"startTime\":") || !strings.Contains(directResp.Body.String(), "\"duration\":") || !strings.Contains(directResp.Body.String(), "\"endTime\":") || !strings.Contains(directResp.Body.String(), "\"elapsed\":") || !strings.Contains(directResp.Body.String(), "\"providerId\":8") || !strings.Contains(directResp.Body.String(), "\"providerName\":\"provider-b\"") || !strings.Contains(directResp.Body.String(), "\"keyName\":\"Key B\"") {
		t.Fatalf("expected proxy status timing fields, got %s", directResp.Body.String())
	}

	actionReq := httptest.NewRequest(http.MethodPost, "/api/actions/proxy-status/getProxyStatus", strings.NewReader(`{}`))
	actionReq.Header.Set("Authorization", "Bearer admin-token")
	actionReq.Header.Set("Content-Type", "application/json")
	actionResp := httptest.NewRecorder()
	router.ServeHTTP(actionResp, actionReq)
	if actionResp.Code != http.StatusOK || !strings.Contains(actionResp.Body.String(), "\"ok\":true") || !strings.Contains(actionResp.Body.String(), "alice") {
		t.Fatalf("expected action proxy status payload, got %d: %s", actionResp.Code, actionResp.Body.String())
	}
}

func stringPtr(value string) *string { return &value }
func intPtr(value int) *int          { return &value }

func TestProxyStatusDirectRouteReturns403ForNonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	handler := NewProxyStatusHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: false,
			User:    &model.User{ID: 2, Name: "bob", Role: "user", IsEnabled: &enabled},
			APIKey:  "user-token",
		}},
		fakeProxyStatusUserStore{users: []*model.User{{ID: 1, Name: "alice"}}},
		&fakeProxyStatusLogStore{},
	)

	router := gin.New()
	handler.RegisterDirectRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/proxy-status", nil)
	req.Header.Set("Authorization", "Bearer user-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin direct route, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), string(appErrors.CodePermissionDenied)) {
		t.Fatalf("expected permission_denied code, got %s", resp.Body.String())
	}
}

func TestProxyStatusLastRequestUsesNewestRequestEvenIfActive(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	now := time.Now()
	handler := NewProxyStatusHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeProxyStatusUserStore{users: []*model.User{{ID: 1, Name: "alice"}}},
		&fakeProxyStatusLogStore{logs: []*model.MessageRequest{
			{ID: 10, UserID: 1, Key: "key-active-9999", ProviderID: 8, ProviderName: stringPtr("provider-b"), ProviderChain: []model.ProviderChainItem{{ID: 8, Name: "provider-b"}}, Model: "gpt-4o-mini", CreatedAt: now, SessionID: stringPtr("sess_active")},
			{ID: 9, UserID: 1, Key: "key-done-1234", ProviderID: 7, ProviderName: stringPtr("provider-a"), ProviderChain: []model.ProviderChainItem{{ID: 7, Name: "provider-a"}}, Model: "gpt-5.4", CreatedAt: now.Add(-time.Minute), DurationMs: intPtr(250), StatusCode: intPtr(200)},
		}},
	)

	router := gin.New()
	handler.RegisterDirectRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/proxy-status", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "\"requestId\":10") || !strings.Contains(resp.Body.String(), "\"keyName\":\"key-••••••9999\"") {
		t.Fatalf("expected lastRequest to use newest request, got %s", resp.Body.String())
	}
}

func TestProxyStatusTreatsStatusWithoutDurationAsActive(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	now := time.Now()
	handler := NewProxyStatusHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeProxyStatusUserStore{users: []*model.User{{ID: 1, Name: "alice"}}},
		&fakeProxyStatusLogStore{logs: []*model.MessageRequest{
			{ID: 11, UserID: 1, Key: "key-a", ProviderID: 8, ProviderName: stringPtr("provider-b"), ProviderChain: []model.ProviderChainItem{{ID: 8, Name: "provider-b"}}, Model: "gpt-4o-mini", CreatedAt: now, StatusCode: intPtr(200)},
		}},
	)

	router := gin.New()
	handler.RegisterDirectRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/proxy-status", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "\"activeCount\":1") {
		t.Fatalf("expected request without duration to remain active, got %s", resp.Body.String())
	}
}

func TestProxyStatusLastRequestUsesUpdatedAtAsEndTime(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	createdAt := time.Now().Add(-time.Minute)
	updatedAt := createdAt.Add(5 * time.Second)
	handler := NewProxyStatusHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeProxyStatusUserStore{users: []*model.User{{ID: 1, Name: "alice"}}},
		&fakeProxyStatusLogStore{logs: []*model.MessageRequest{
			{ID: 12, UserID: 1, Key: "key-done-1234", ProviderID: 7, ProviderName: stringPtr("provider-a"), ProviderChain: []model.ProviderChainItem{{ID: 7, Name: "provider-a"}}, Model: "gpt-5.4", CreatedAt: createdAt, UpdatedAt: updatedAt, DurationMs: intPtr(250), StatusCode: intPtr(200)},
		}},
	)

	router := gin.New()
	handler.RegisterDirectRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/proxy-status", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), strconv.FormatInt(updatedAt.UnixMilli(), 10)) {
		t.Fatalf("expected lastRequest endTime to use updatedAt, got %s", resp.Body.String())
	}
}

func TestProxyStatusLastRequestUsesLatestUpdatedAt(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	now := time.Now()
	olderCreated := now.Add(-2 * time.Minute)
	olderUpdated := now.Add(-10 * time.Second)
	newerCreated := now.Add(-time.Minute)
	newerUpdated := now.Add(-20 * time.Second)

	handler := NewProxyStatusHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeProxyStatusUserStore{users: []*model.User{{ID: 1, Name: "alice"}}},
		&fakeProxyStatusLogStore{logs: []*model.MessageRequest{
			{ID: 20, UserID: 1, Key: "key-oldr-1111", ProviderID: 7, ProviderName: stringPtr("provider-a"), ProviderChain: []model.ProviderChainItem{{ID: 7, Name: "provider-a"}}, Model: "gpt-5.4", CreatedAt: olderCreated, UpdatedAt: olderUpdated, DurationMs: intPtr(250), StatusCode: intPtr(200)},
			{ID: 21, UserID: 1, Key: "key-newr-2222", ProviderID: 8, ProviderName: stringPtr("provider-b"), ProviderChain: []model.ProviderChainItem{{ID: 8, Name: "provider-b"}}, Model: "gpt-4o-mini", CreatedAt: newerCreated, UpdatedAt: newerUpdated, DurationMs: intPtr(250), StatusCode: intPtr(200)},
		}},
	)

	router := gin.New()
	handler.RegisterDirectRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/proxy-status", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "\"requestId\":20") {
		t.Fatalf("expected lastRequest to choose latest updatedAt, got %s", resp.Body.String())
	}
}

func TestProxyStatusSkipsWarmupRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	now := time.Now()
	warmup := "warmup"
	handler := NewProxyStatusHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeProxyStatusUserStore{users: []*model.User{{ID: 1, Name: "alice"}}},
		&fakeProxyStatusLogStore{logs: []*model.MessageRequest{
			{ID: 30, UserID: 1, Key: "key-warm-1111", ProviderID: 8, ProviderName: stringPtr("provider-b"), ProviderChain: []model.ProviderChainItem{{ID: 8, Name: "provider-b"}}, Model: "gpt-4o-mini", CreatedAt: now, SessionID: stringPtr("sess_warm"), BlockedBy: &warmup},
			{ID: 31, UserID: 1, Key: "key-real-2222", ProviderID: 7, ProviderName: stringPtr("provider-a"), ProviderChain: []model.ProviderChainItem{{ID: 7, Name: "provider-a"}}, Model: "gpt-5.4", CreatedAt: now.Add(-time.Minute), DurationMs: intPtr(250), StatusCode: intPtr(200)},
		}},
	)

	router := gin.New()
	handler.RegisterDirectRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/proxy-status", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if strings.Contains(resp.Body.String(), "sess_warm") || strings.Contains(resp.Body.String(), "\"requestId\":30") {
		t.Fatalf("expected warmup request to be excluded, got %s", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "\"requestId\":31") {
		t.Fatalf("expected normal request to remain, got %s", resp.Body.String())
	}
}

func TestProxyStatusFallsBackToMaskedKeyWhenKeyNameMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	now := time.Now()
	handler := NewProxyStatusHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeProxyStatusUserStore{users: []*model.User{{ID: 1, Name: "alice"}}},
		&fakeProxyStatusLogStore{logs: []*model.MessageRequest{
			{ID: 40, UserID: 1, Key: "key-mask-1234", ProviderID: 7, ProviderName: stringPtr("provider-a"), ProviderChain: []model.ProviderChainItem{{ID: 7, Name: "provider-a"}}, Model: "gpt-5.4", CreatedAt: now, DurationMs: intPtr(250), StatusCode: intPtr(200)},
		}},
	)

	router := gin.New()
	handler.RegisterDirectRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/proxy-status", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "\"keyName\":\"key-••••••1234\"") {
		t.Fatalf("expected masked key fallback, got %s", resp.Body.String())
	}
}

func TestProxyStatusUsesProjectedProviderNameWhenAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	now := time.Now()
	handler := NewProxyStatusHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeProxyStatusUserStore{users: []*model.User{{ID: 1, Name: "alice"}}},
		&fakeProxyStatusLogStore{logs: []*model.MessageRequest{
			{ID: 41, UserID: 1, Key: "key-mask-1234", ProviderID: 7, ProviderName: stringPtr("Projected Provider"), Model: "gpt-5.4", CreatedAt: now, DurationMs: intPtr(250), StatusCode: intPtr(200)},
		}},
	)

	router := gin.New()
	handler.RegisterDirectRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/proxy-status", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "\"providerName\":\"Projected Provider\"") {
		t.Fatalf("expected projected provider name, got %s", resp.Body.String())
	}
}

func TestProxyStatusSkipsRequestsWithoutVisibleProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	now := time.Now()
	handler := NewProxyStatusHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeProxyStatusUserStore{users: []*model.User{{ID: 1, Name: "alice"}}},
		&fakeProxyStatusLogStore{logs: []*model.MessageRequest{
			{ID: 50, UserID: 1, Key: "key-mask-1234", ProviderID: 7, Model: "gpt-5.4", CreatedAt: now, DurationMs: intPtr(250), StatusCode: intPtr(200)},
		}},
	)

	router := gin.New()
	handler.RegisterDirectRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/proxy-status", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if strings.Contains(resp.Body.String(), "\"requestId\":50") {
		t.Fatalf("expected request without visible provider to be skipped, got %s", resp.Body.String())
	}
}

func TestMaskProxyStatusKeyReturnsDotsForEmptyKey(t *testing.T) {
	if got := maskProxyStatusKey(""); got != "••••••" {
		t.Fatalf("expected empty key to mask as dots, got %q", got)
	}
}

func TestProxyStatusLastRequestOmitsStartTimeAndDurationFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	now := time.Now()
	handler := NewProxyStatusHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeProxyStatusUserStore{users: []*model.User{{ID: 1, Name: "alice"}}},
		&fakeProxyStatusLogStore{logs: []*model.MessageRequest{
			{ID: 60, UserID: 1, Key: "key-last-1234", KeyName: stringPtr("Key Last"), ProviderID: 7, ProviderName: stringPtr("provider-a"), Model: "gpt-5.4", CreatedAt: now.Add(-time.Minute), DurationMs: intPtr(250), StatusCode: intPtr(200)},
		}},
	)

	router := gin.New()
	handler.RegisterDirectRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/proxy-status", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if strings.Contains(body, "\"startTime\":") || strings.Contains(body, "\"duration\":") {
		t.Fatalf("expected lastRequest to omit active-only timing fields, got %s", body)
	}
}

func TestProxyStatusIncludesTrackedActiveSessions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sessiontrackersvc.SetIDsForTest([]string{"sess_tracked"})
	defer sessiontrackersvc.ResetForTest()

	enabled := true
	logStore := &fakeProxyStatusLogStore{
		logs: []*model.MessageRequest{
			{ID: 30, UserID: 1, Key: "key-old-1234", ProviderID: 7, ProviderName: stringPtr("provider-a"), ProviderChain: []model.ProviderChainItem{{ID: 7, Name: "provider-a"}}, Model: "gpt-5.4", CreatedAt: time.Now().Add(-time.Hour), DurationMs: intPtr(250), StatusCode: intPtr(200)},
		},
		latestBySession: []*model.MessageRequest{
			{ID: 31, UserID: 1, Key: "key-active-9999", ProviderID: 8, ProviderName: stringPtr("provider-b"), ProviderChain: []model.ProviderChainItem{{ID: 8, Name: "provider-b"}}, Model: "gpt-4o-mini", CreatedAt: time.Now(), SessionID: stringPtr("sess_tracked")},
		},
	}
	handler := NewProxyStatusHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeProxyStatusUserStore{users: []*model.User{{ID: 1, Name: "alice"}}},
		logStore,
	)

	router := gin.New()
	handler.RegisterDirectRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/proxy-status", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if len(logStore.latestSessionIDs) != 1 || logStore.latestSessionIDs[0] != "sess_tracked" || logStore.latestSessionLimit != 50 {
		t.Fatalf("expected tracked session lookup, got ids=%v limit=%d", logStore.latestSessionIDs, logStore.latestSessionLimit)
	}
	body := resp.Body.String()
	if !strings.Contains(body, "\"requestId\":31") || !strings.Contains(body, "\"providerName\":\"provider-b\"") || !strings.Contains(body, "\"activeCount\":1") {
		t.Fatalf("expected tracked active request in proxy status, got %s", body)
	}
}
