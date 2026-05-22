package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

type fakeSystemSettingsStore struct {
	settings *model.SystemSettings
	fields   map[string]any
}

func (f *fakeSystemSettingsStore) Get(_ context.Context) (*model.SystemSettings, error) {
	if f.settings == nil {
		f.settings = &model.SystemSettings{ID: 1, SiteTitle: "Claude Code Hub", CurrencyDisplay: "USD", BillingModelSource: "original", CodexPriorityBillingSource: "requested", EnableThinkingSignatureRectifier: true, EnableThinkingBudgetRectifier: true, EnableBillingHeaderRectifier: true, EnableResponseInputRectifier: true, EnableCodexSessionIDCompletion: true, EnableClaudeMetadataUserIDInjection: true, EnableResponseFixer: true, ResponseFixerConfig: map[string]any{"fixTruncatedJson": true}, IpGeoLookupEnabled: true}
	}
	return f.settings, nil
}

func (f *fakeSystemSettingsStore) UpdateFields(_ context.Context, _ int, fields map[string]any) (*model.SystemSettings, error) {
	f.fields = fields
	if title, ok := fields["site_title"].(string); ok {
		f.settings.SiteTitle = title
	}
	if value, ok := fields["enable_http2"].(bool); ok {
		f.settings.EnableHttp2 = value
	}
	if value, ok := fields["cleanup_retention_days"].(int); ok {
		f.settings.CleanupRetentionDays = &value
	}
	if value, ok := fields["cleanup_schedule"].(string); ok {
		f.settings.CleanupSchedule = value
	}
	if value, ok := fields["cleanup_batch_size"].(int); ok {
		f.settings.CleanupBatchSize = &value
	}
	if value, ok := fields["enable_high_concurrency_mode"].(bool); ok {
		f.settings.EnableHighConcurrencyMode = value
	}
	if value, ok := fields["ip_geo_lookup_enabled"].(bool); ok {
		f.settings.IpGeoLookupEnabled = value
	}
	if value, ok := fields["codex_priority_billing_source"].(string); ok {
		f.settings.CodexPriorityBillingSource = value
	}
	if value, ok := fields["enable_thinking_signature_rectifier"].(bool); ok {
		f.settings.EnableThinkingSignatureRectifier = value
	}
	if value, ok := fields["enable_thinking_budget_rectifier"].(bool); ok {
		f.settings.EnableThinkingBudgetRectifier = value
	}
	if value, ok := fields["enable_billing_header_rectifier"].(bool); ok {
		f.settings.EnableBillingHeaderRectifier = value
	}
	if value, ok := fields["enable_response_input_rectifier"].(bool); ok {
		f.settings.EnableResponseInputRectifier = value
	}
	if value, ok := fields["enable_codex_session_id_completion"].(bool); ok {
		f.settings.EnableCodexSessionIDCompletion = value
	}
	if value, ok := fields["enable_claude_metadata_user_id_injection"].(bool); ok {
		f.settings.EnableClaudeMetadataUserIDInjection = value
	}
	if value, ok := fields["enable_response_fixer"].(bool); ok {
		f.settings.EnableResponseFixer = value
	}
	if value, ok := fields["response_fixer_config"].(map[string]any); ok {
		f.settings.ResponseFixerConfig = value
	}
	if value, ok := fields["quota_db_refresh_interval_seconds"].(int); ok {
		f.settings.QuotaDbRefreshIntervalSeconds = &value
	}
	if value, ok := fields["quota_lease_percent_daily"].(float64); ok {
		f.settings.QuotaLeasePercentDaily = &value
	}
	if value, ok := fields["ip_extraction_config"].(map[string]any); ok {
		f.settings.IpExtractionConfig = value
	}
	if value, ok := fields["timezone"].(string); ok {
		f.settings.Timezone = &value
	}
	return f.settings, nil
}

func TestSystemSettingsRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeSystemSettingsStore{}
	handler := NewSystemSettingsHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/system-settings"))

	getReq := httptest.NewRequest(http.MethodGet, "/api/system-settings", nil)
	getReq.Header.Set("Authorization", "Bearer admin-token")
	getResp := httptest.NewRecorder()
	router.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", getResp.Code, getResp.Body.String())
	}
	if !strings.Contains(getResp.Body.String(), "Claude Code Hub") {
		t.Fatalf("expected system settings payload, got %s", getResp.Body.String())
	}

	putReq := httptest.NewRequest(http.MethodPut, "/api/system-settings", strings.NewReader(`{"siteTitle":"CCH Go","enableHttp2":true,"cleanupRetentionDays":45,"cleanupSchedule":"0 3 * * *","cleanupBatchSize":20000,"enableHighConcurrencyMode":true,"ipGeoLookupEnabled":false,"codexPriorityBillingSource":"actual","timezone":"Asia/Shanghai","enableThinkingSignatureRectifier":false,"enableThinkingBudgetRectifier":false,"enableBillingHeaderRectifier":false,"enableResponseInputRectifier":false,"enableCodexSessionIdCompletion":false,"enableClaudeMetadataUserIdInjection":false,"enableResponseFixer":false,"responseFixerConfig":{"fixTruncatedJson":false},"quotaDbRefreshIntervalSeconds":15,"quotaLeasePercentDaily":0.2,"ipExtractionConfig":{"strategy":"custom"}}`))
	putReq.Header.Set("Authorization", "Bearer admin-token")
	putReq.Header.Set("Content-Type", "application/json")
	putResp := httptest.NewRecorder()
	router.ServeHTTP(putResp, putReq)
	if putResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", putResp.Code, putResp.Body.String())
	}
	if !strings.Contains(putResp.Body.String(), "CCH Go") {
		t.Fatalf("expected updated system settings payload, got %s", putResp.Body.String())
	}
	if store.fields["site_title"] != "CCH Go" {
		t.Fatalf("expected site_title update field, got %+v", store.fields)
	}
	if store.fields["enable_high_concurrency_mode"] != true || store.fields["ip_geo_lookup_enabled"] != false || store.fields["codex_priority_billing_source"] != "actual" || store.fields["timezone"] != "Asia/Shanghai" {
		t.Fatalf("expected extended system settings fields, got %+v", store.fields)
	}
	if store.fields["cleanup_retention_days"] != 45 || store.fields["cleanup_schedule"] != "0 3 * * *" || store.fields["cleanup_batch_size"] != 20000 {
		t.Fatalf("expected cleanup settings captured, got %+v", store.fields)
	}
	if store.fields["enable_thinking_signature_rectifier"] != false || store.fields["enable_thinking_budget_rectifier"] != false || store.fields["enable_billing_header_rectifier"] != false || store.fields["enable_response_input_rectifier"] != false || store.fields["enable_codex_session_id_completion"] != false || store.fields["enable_claude_metadata_user_id_injection"] != false || store.fields["enable_response_fixer"] != false {
		t.Fatalf("expected rectifier toggles captured, got %+v", store.fields)
	}
	if store.fields["quota_db_refresh_interval_seconds"] != 15 || store.fields["quota_lease_percent_daily"] != 0.2 {
		t.Fatalf("expected quota fields captured, got %+v", store.fields)
	}
}

func TestSystemSettingsRoutesAcceptAuthCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeSystemSettingsStore{}
	handler := NewSystemSettingsHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/system-settings"))

	req := httptest.NewRequest(http.MethodGet, "/api/system-settings", nil)
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: "admin-token"})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "Claude Code Hub") {
		t.Fatalf("expected auth-cookie system settings payload, got %d: %s", resp.Code, resp.Body.String())
	}
}

type sessionAwareKeyRepo struct {
	key *model.Key
}

func (r sessionAwareKeyRepo) GetByKeyWithUser(_ context.Context, key string) (*model.Key, error) {
	if r.key != nil && r.key.Key == key {
		return r.key, nil
	}
	return nil, nil
}

func (r sessionAwareKeyRepo) ListByUserID(_ context.Context, userID int) ([]*model.Key, error) {
	if r.key != nil && r.key.User != nil && r.key.User.ID == userID {
		return []*model.Key{r.key}, nil
	}
	return nil, nil
}

type noopUserExpiryRepo struct{}

func (noopUserExpiryRepo) MarkUserExpired(_ context.Context, _ int) (bool, error) { return true, nil }

type opaqueSessionReader struct {
	session *authsvc.SessionTokenData
}

func (r opaqueSessionReader) Read(_ context.Context, _ string) (*authsvc.SessionTokenData, error) {
	return r.session, nil
}

func testSHA256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func TestSystemSettingsRoutesAcceptOpaqueSessionCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldMode := os.Getenv("SESSION_TOKEN_MODE")
	t.Cleanup(func() { _ = os.Setenv("SESSION_TOKEN_MODE", oldMode) })
	_ = os.Setenv("SESSION_TOKEN_MODE", "opaque")

	enabled := true
	userKey := &model.Key{
		ID:            2,
		UserID:        2,
		Key:           "user-token",
		Name:          "USER_KEY",
		IsEnabled:     &enabled,
		CanLoginWebUi: &enabled,
		User:          &model.User{ID: 2, Name: "bob", Role: "user", IsEnabled: &enabled},
	}
	authService := authsvc.NewService(
		sessionAwareKeyRepo{key: userKey},
		noopUserExpiryRepo{},
		"admin-token",
		opaqueSessionReader{session: &authsvc.SessionTokenData{
			SessionID:      "sid_user_opaque_123",
			KeyFingerprint: "sha256:" + testSHA256Hex("user-token"),
			UserID:         2,
			UserRole:       "user",
			CreatedAt:      time.Now().Add(-time.Minute).UnixMilli(),
			ExpiresAt:      time.Now().Add(time.Hour).UnixMilli(),
		}},
	)

	store := &fakeSystemSettingsStore{}
	handler := NewSystemSettingsHandler(authService, store)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/system-settings"))

	req := httptest.NewRequest(http.MethodGet, "/api/system-settings", nil)
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: "sid_user_opaque_123"})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "Claude Code Hub") {
		t.Fatalf("expected opaque-session auth-cookie system settings payload, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestSystemSettingsRoutesAcceptUserApiKeyForGet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeSystemSettingsStore{}
	handler := NewSystemSettingsHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: false,
			User:    &model.User{ID: 2, Name: "bob", Role: "user", IsEnabled: &enabled},
			Key:     &model.Key{ID: 2, Key: "user-token", Name: "USER_KEY", IsEnabled: &enabled},
			APIKey:  "user-token",
		}},
		store,
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/system-settings"))

	req := httptest.NewRequest(http.MethodGet, "/api/system-settings", nil)
	req.Header.Set("Authorization", "Bearer user-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "Claude Code Hub") {
		t.Fatalf("expected non-admin authenticated GET payload, got %d: %s", resp.Code, resp.Body.String())
	}
}
