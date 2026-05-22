package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/quagmt/udecimal"
)

type fakeNotificationSettingsStore struct {
	settings      *model.NotificationSettings
	updatedID     int
	updatedFields map[string]any
}

func (f *fakeNotificationSettingsStore) Get(_ context.Context) (*model.NotificationSettings, error) {
	if f.settings == nil {
		topN := 5
		interval := 60
		f.settings = &model.NotificationSettings{ID: 1, DailyLeaderboardTime: "09:00", DailyLeaderboardTopN: &topN, CostAlertThreshold: udecimal.MustParse("0.80"), CostAlertCheckInterval: &interval}
	}
	return f.settings, nil
}

func (f *fakeNotificationSettingsStore) UpdateFields(_ context.Context, id int, fields map[string]any) (*model.NotificationSettings, error) {
	f.updatedID = id
	f.updatedFields = fields
	if value, ok := fields["enabled"].(bool); ok {
		f.settings.Enabled = value
	}
	if value, ok := fields["daily_leaderboard_time"].(string); ok {
		f.settings.DailyLeaderboardTime = value
	}
	if value, ok := fields["cost_alert_threshold"].(udecimal.Decimal); ok {
		f.settings.CostAlertThreshold = value
	}
	return f.settings, nil
}

type fakeWebhookTargetStore struct {
	targets        map[int]*model.WebhookTarget
	created        *model.WebhookTarget
	updatedFields  map[string]any
	deletedID      int
	updatedTestID  int
	updatedTestRes *model.WebhookTestResult
}

func newFakeWebhookTargetStore() *fakeWebhookTargetStore {
	url := "https://example.com/webhook"
	return &fakeWebhookTargetStore{targets: map[int]*model.WebhookTarget{
		1: {ID: 1, Name: "ops", ProviderType: "custom", WebhookUrl: &url, IsEnabled: true},
	}}
}

func (f *fakeWebhookTargetStore) List(_ context.Context) ([]*model.WebhookTarget, error) {
	items := make([]*model.WebhookTarget, 0, len(f.targets))
	for _, target := range f.targets {
		items = append(items, target)
	}
	return items, nil
}

func (f *fakeWebhookTargetStore) GetByID(_ context.Context, id int) (*model.WebhookTarget, error) {
	if target, ok := f.targets[id]; ok {
		return target, nil
	}
	return nil, appErrors.NewNotFoundError("WebhookTarget")
}

func (f *fakeWebhookTargetStore) Create(_ context.Context, target *model.WebhookTarget) (*model.WebhookTarget, error) {
	target.ID = len(f.targets) + 1
	f.targets[target.ID] = target
	f.created = target
	return target, nil
}

func (f *fakeWebhookTargetStore) UpdateFields(_ context.Context, id int, fields map[string]any) (*model.WebhookTarget, error) {
	target, ok := f.targets[id]
	if !ok {
		return nil, appErrors.NewNotFoundError("WebhookTarget")
	}
	f.updatedFields = fields
	if value, ok := fields["name"].(string); ok {
		target.Name = value
	}
	if value, ok := fields["provider_type"].(string); ok {
		target.ProviderType = value
	}
	if value, ok := fields["webhook_url"].(*string); ok {
		target.WebhookUrl = value
	}
	if value, ok := fields["is_enabled"].(bool); ok {
		target.IsEnabled = value
	}
	return target, nil
}

func (f *fakeWebhookTargetStore) Delete(_ context.Context, id int) error {
	f.deletedID = id
	delete(f.targets, id)
	return nil
}

func (f *fakeWebhookTargetStore) UpdateTestResult(_ context.Context, id int, result *model.WebhookTestResult, testedAt time.Time) (*model.WebhookTarget, error) {
	target, ok := f.targets[id]
	if !ok {
		return nil, appErrors.NewNotFoundError("WebhookTarget")
	}
	f.updatedTestID = id
	f.updatedTestRes = result
	target.LastTestAt = &testedAt
	target.LastTestResult = result
	return target, nil
}

type fakeNotificationBindingStore struct {
	bindingsByType map[string][]*model.NotificationTargetBinding
	updatedType    string
	updatedItems   []*model.NotificationTargetBinding
}

func newFakeNotificationBindingStore() *fakeNotificationBindingStore {
	url := "https://example.com/webhook"
	return &fakeNotificationBindingStore{bindingsByType: map[string][]*model.NotificationTargetBinding{
		"cost_alert": {{ID: 1, NotificationType: "cost_alert", TargetID: 1, IsEnabled: true, Target: &model.WebhookTarget{ID: 1, Name: "ops", ProviderType: "custom", WebhookUrl: &url, IsEnabled: true}}},
	}}
}

func (f *fakeNotificationBindingStore) List(_ context.Context, notificationType string) ([]*model.NotificationTargetBinding, error) {
	return f.bindingsByType[notificationType], nil
}

func (f *fakeNotificationBindingStore) ListAll(_ context.Context) ([]*model.NotificationTargetBinding, error) {
	var items []*model.NotificationTargetBinding
	for _, slice := range f.bindingsByType {
		items = append(items, slice...)
	}
	return items, nil
}

func (f *fakeNotificationBindingStore) ReplaceByNotificationType(_ context.Context, notificationType string, bindings []*model.NotificationTargetBinding) ([]*model.NotificationTargetBinding, error) {
	f.updatedType = notificationType
	f.updatedItems = bindings
	f.bindingsByType[notificationType] = bindings
	return bindings, nil
}

type stubWebhookTester struct {
	result     model.WebhookTestResult
	lastTarget *model.WebhookTarget
}

func (s *stubWebhookTester) Test(_ context.Context, target *model.WebhookTarget) model.WebhookTestResult {
	s.lastTarget = target
	return s.result
}

func newAdminActionAuth() fakeAdminAuth {
	enabled := true
	return fakeAdminAuth{result: &authsvc.AuthResult{
		IsAdmin: true,
		User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
		Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
		APIKey:  "admin-token",
	}}
}

func TestNotificationsActionHandlerRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settingsStore := &fakeNotificationSettingsStore{}
	targetStore := newFakeWebhookTargetStore()
	tester := &stubWebhookTester{result: model.WebhookTestResult{Success: true, Message: stringPtr("ok"), Timestamp: time.Now().UTC().Format(time.RFC3339)}}

	router := gin.New()
	NewNotificationsActionHandler(newAdminActionAuth(), settingsStore, targetStore, tester).RegisterRoutes(router.Group("/api/actions"))

	t.Run("get settings alias", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/actions/notifications/getNotificationSettings", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer admin-token")
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}
		if !strings.Contains(resp.Body.String(), "dailyLeaderboardTime") {
			t.Fatalf("expected settings payload, got %s", resp.Body.String())
		}
	})

	t.Run("update settings alias", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/actions/notifications/updateNotificationSettings", strings.NewReader(`{"enabled":true,"dailyLeaderboardTime":"10:15","costAlertThreshold":0.9}`))
		req.Header.Set("Authorization", "Bearer admin-token")
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}
		if got := settingsStore.updatedFields["daily_leaderboard_time"]; got != "10:15" {
			t.Fatalf("expected updated time, got %#v", got)
		}
		if !strings.Contains(resp.Body.String(), `"ok":true`) {
			t.Fatalf("expected ok envelope, got %s", resp.Body.String())
		}
	})

	t.Run("test ephemeral webhook", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/actions/notifications/testWebhook", strings.NewReader(`{"providerType":"custom","webhookUrl":"https://example.com/hook"}`))
		req.Header.Set("Authorization", "Bearer admin-token")
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}
		if tester.lastTarget == nil || tester.lastTarget.WebhookUrl == nil || *tester.lastTarget.WebhookUrl != "https://example.com/hook" {
			t.Fatalf("expected tester to receive webhook target, got %#v", tester.lastTarget)
		}
		if !strings.Contains(resp.Body.String(), `"success":true`) {
			t.Fatalf("expected success payload, got %s", resp.Body.String())
		}
	})

	t.Run("invalid leaderboard time rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/actions/notifications", strings.NewReader(`{"dailyLeaderboardTime":"25:61"}`))
		req.Header.Set("Authorization", "Bearer admin-token")
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
		}
	})
}

func TestWebhookTargetsActionHandlerRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := newFakeWebhookTargetStore()
	tester := &stubWebhookTester{result: model.WebhookTestResult{Success: true, Message: stringPtr("sent"), Timestamp: time.Now().UTC().Format(time.RFC3339)}}
	router := gin.New()
	NewWebhookTargetsActionHandler(newAdminActionAuth(), store, tester).RegisterRoutes(router.Group("/api/actions"))

	tests := []struct {
		name         string
		method       string
		path         string
		body         string
		wantStatus   int
		wantContains string
	}{
		{name: "list alias", method: http.MethodPost, path: "/api/actions/webhook-targets/getWebhookTargets", body: `{}`, wantStatus: http.StatusOK, wantContains: `"ops"`},
		{name: "create alias", method: http.MethodPost, path: "/api/actions/webhook-targets/addWebhookTarget", body: `{"name":"alerts","providerType":"custom","webhookUrl":"https://example.com/new"}`, wantStatus: http.StatusCreated, wantContains: `"alerts"`},
		{name: "update alias", method: http.MethodPost, path: "/api/actions/webhook-targets/editWebhookTarget", body: `{"id":1,"name":"ops-updated"}`, wantStatus: http.StatusOK, wantContains: `"ops-updated"`},
		{name: "test alias", method: http.MethodPost, path: "/api/actions/webhook-targets/testWebhookTarget", body: `{"id":1}`, wantStatus: http.StatusOK, wantContains: `"success":true`},
		{name: "delete alias", method: http.MethodPost, path: "/api/actions/webhook-targets/removeWebhookTarget", body: `{"id":1}`, wantStatus: http.StatusOK, wantContains: `"deleted":true`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
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

	if store.updatedTestID != 1 || store.updatedTestRes == nil || !store.updatedTestRes.Success {
		t.Fatalf("expected test result to be persisted, got id=%d result=%#v", store.updatedTestID, store.updatedTestRes)
	}
}

func TestNotificationBindingsActionHandlerRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := newFakeNotificationBindingStore()
	router := gin.New()
	NewNotificationBindingsActionHandler(newAdminActionAuth(), store).RegisterRoutes(router.Group("/api/actions"))

	t.Run("get bindings filtered", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/actions/notification-bindings?notificationType=cost_alert", nil)
		req.Header.Set("Authorization", "Bearer admin-token")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}
		if !strings.Contains(resp.Body.String(), `"notificationType":"cost_alert"`) {
			t.Fatalf("expected cost_alert binding, got %s", resp.Body.String())
		}
	})

	t.Run("get bindings alias", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/actions/notification-bindings/getNotificationBindings", strings.NewReader(`{"notificationType":"cost_alert"}`))
		req.Header.Set("Authorization", "Bearer admin-token")
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}
	})

	t.Run("update bindings", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/actions/notification-bindings", strings.NewReader(`{"notificationType":"cost_alert","bindings":[{"targetId":1,"isEnabled":false,"scheduleTimezone":"UTC"}]}`))
		req.Header.Set("Authorization", "Bearer admin-token")
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
		}
		if store.updatedType != "cost_alert" || len(store.updatedItems) != 1 || store.updatedItems[0].TargetID != 1 {
			t.Fatalf("expected binding replacement to be recorded, got type=%s items=%#v", store.updatedType, store.updatedItems)
		}
	})

	t.Run("invalid notification type rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/actions/notification-bindings", strings.NewReader(`{"notificationType":"unknown","bindings":[]}`))
		req.Header.Set("Authorization", "Bearer admin-token")
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
		}
	})
}
