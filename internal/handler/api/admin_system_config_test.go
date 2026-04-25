package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

func TestAdminSystemConfigRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	store := &fakeSystemSettingsStore{}

	NewAdminSystemConfigHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	).RegisterRoutes(router)

	getReq := httptest.NewRequest(http.MethodGet, "/api/admin/system-config", nil)
	getReq.Header.Set("Authorization", "Bearer admin-token")
	getResp := httptest.NewRecorder()
	router.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK || !strings.Contains(getResp.Body.String(), "Claude Code Hub") {
		t.Fatalf("expected admin system config payload, got %d: %s", getResp.Code, getResp.Body.String())
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/admin/system-config", strings.NewReader(`{"siteTitle":"CCH Go","allowGlobalUsageView":true,"enableAutoCleanup":true,"cleanupRetentionDays":45,"cleanupSchedule":"0 3 * * *","cleanupBatchSize":20000,"codexPriorityBillingSource":"actual","timezone":"Asia/Shanghai","enableHighConcurrencyMode":true,"enableThinkingSignatureRectifier":false,"enableThinkingBudgetRectifier":false,"enableBillingHeaderRectifier":false,"enableResponseInputRectifier":false,"enableCodexSessionIdCompletion":false,"enableClaudeMetadataUserIdInjection":false,"enableResponseFixer":false,"responseFixerConfig":{"fixTruncatedJson":false},"quotaDbRefreshIntervalSeconds":15,"quotaLeasePercentDaily":0.2,"ipExtractionConfig":{"strategy":"custom"},"ipGeoLookupEnabled":false}`))
	postReq.Header.Set("Authorization", "Bearer admin-token")
	postReq.Header.Set("Content-Type", "application/json")
	postResp := httptest.NewRecorder()
	router.ServeHTTP(postResp, postReq)
	if postResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", postResp.Code, postResp.Body.String())
	}
	if store.fields["site_title"] != "CCH Go" || store.fields["allow_global_usage_view"] != true || store.fields["cleanup_retention_days"] != 45 || store.fields["cleanup_schedule"] != "0 3 * * *" || store.fields["cleanup_batch_size"] != 20000 {
		t.Fatalf("expected admin system config fields captured, got %+v", store.fields)
	}
	if store.fields["codex_priority_billing_source"] != "actual" || store.fields["timezone"] != "Asia/Shanghai" || store.fields["enable_high_concurrency_mode"] != true || store.fields["enable_response_fixer"] != false || store.fields["quota_db_refresh_interval_seconds"] != 15 || store.fields["quota_lease_percent_daily"] != 0.2 || store.fields["ip_geo_lookup_enabled"] != false {
		t.Fatalf("expected extended admin system config fields captured, got %+v", store.fields)
	}
}
