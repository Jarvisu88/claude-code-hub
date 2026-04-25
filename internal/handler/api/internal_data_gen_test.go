package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/quagmt/udecimal"
)

func TestInternalDataGenRouteReturnsUsagePayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	store := &fakeUsageLogsStore{
		logs: []*model.MessageRequest{
			{
				ID:           1,
				Model:        "gpt-5.4",
				UserID:       7,
				Key:          "sk-123",
				UserName:     stringPtr("alice"),
				KeyName:      stringPtr("Key A"),
				ProviderName: stringPtr("provider-a"),
				CreatedAt:    time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC),
				StatusCode:   intPtr(200),
				InputTokens:  intPtr(100),
				OutputTokens: intPtr(50),
				CostUSD:      udecimal.MustParse("1.25"),
			},
		},
	}
	NewInternalDataGenHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/internal/data-gen", strings.NewReader(`{"mode":"usage","startDate":"2026-04-25T00:00:00Z","endDate":"2026-04-26T00:00:00Z"}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "\"logs\"") || !strings.Contains(resp.Body.String(), "\"totalRecords\":1") {
		t.Fatalf("expected generated usage payload, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestInternalDataGenRouteReturnsUserBreakdownPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()
	store := &fakeUsageLogsStore{
		logs: []*model.MessageRequest{
			{
				ID:        1,
				Model:     "gpt-5.4",
				UserID:    7,
				Key:       "sk-123",
				UserName:  stringPtr("alice"),
				KeyName:   stringPtr("Key A"),
				CreatedAt: time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC),
				CostUSD:   udecimal.MustParse("1.25"),
			},
		},
	}
	NewInternalDataGenHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/internal/data-gen", strings.NewReader(`{"mode":"userBreakdown","serviceName":"AI大模型推理服务","startDate":"2026-04-25T00:00:00Z","endDate":"2026-04-26T00:00:00Z"}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "\"items\"") || !strings.Contains(resp.Body.String(), "\"serviceName\":\"AI大模型推理服务\"") {
		t.Fatalf("expected generated user breakdown payload, got %d: %s", resp.Code, resp.Body.String())
	}
}
