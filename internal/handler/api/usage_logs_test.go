package api

import (
	"context"
	"encoding/json"
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

type fakeUsageLogsStore struct {
	logs               []*model.MessageRequest
	recentErr          error
	limit              int
	page               int
	filters            repository.MessageRequestQueryFilters
	batchFilters       repository.MessageRequestBatchFilters
	batchResult        repository.MessageRequestBatchResult
	batchResults       []repository.MessageRequestBatchResult
	batchCalls         []repository.MessageRequestBatchFilters
	summary            repository.MessageRequestSummary
	overview           repository.MessageRequestOverviewMetrics
	options            repository.MessageRequestFilterOptions
	optionsErr         error
	filterOptionsCalls int
	suggestions        []string
	suggestionFilter   repository.MessageRequestSessionIDSuggestionFilters
	overviewLocation   *time.Location
}

func (f *fakeUsageLogsStore) ListRecent(_ context.Context, limit int) ([]*model.MessageRequest, error) {
	f.limit = limit
	return f.logs, f.recentErr
}

func (f *fakeUsageLogsStore) ListFiltered(_ context.Context, limit int, filters repository.MessageRequestQueryFilters) ([]*model.MessageRequest, error) {
	f.limit = limit
	f.filters = filters
	return f.logs, nil
}

func (f *fakeUsageLogsStore) ListPaginatedFiltered(_ context.Context, page, pageSize int, filters repository.MessageRequestQueryFilters) (repository.MessageRequestListResult, error) {
	f.page = page
	f.limit = pageSize
	f.filters = filters
	return repository.MessageRequestListResult{
		Logs:     f.logs,
		Total:    len(f.logs),
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (f *fakeUsageLogsStore) ListBatch(_ context.Context, filters repository.MessageRequestBatchFilters) (repository.MessageRequestBatchResult, error) {
	f.batchFilters = filters
	f.batchCalls = append(f.batchCalls, filters)
	f.limit = filters.Limit
	if len(f.batchResults) > 0 {
		result := f.batchResults[0]
		f.batchResults = f.batchResults[1:]
		return result, nil
	}
	if len(f.batchResult.Logs) == 0 && f.batchResult.NextCursor == nil && !f.batchResult.HasMore {
		return repository.MessageRequestBatchResult{
			Logs:       f.logs,
			NextCursor: nil,
			HasMore:    false,
		}, nil
	}
	return f.batchResult, nil
}

func (f *fakeUsageLogsStore) GetByID(_ context.Context, id int) (*model.MessageRequest, error) {
	for _, log := range f.logs {
		if log.ID == id {
			return log, nil
		}
	}
	return nil, nil
}

func (f *fakeUsageLogsStore) GetSummary(_ context.Context, filters repository.MessageRequestQueryFilters) (repository.MessageRequestSummary, error) {
	f.filters = filters
	return f.summary, nil
}

func (f *fakeUsageLogsStore) GetOverviewMetrics(_ context.Context, _ time.Time, loc *time.Location) (repository.MessageRequestOverviewMetrics, error) {
	f.overviewLocation = loc
	return f.overview, nil
}

func (f *fakeUsageLogsStore) GetFilterOptions(_ context.Context) (repository.MessageRequestFilterOptions, error) {
	f.filterOptionsCalls++
	return f.options, f.optionsErr
}

func (f *fakeUsageLogsStore) FindSessionIDSuggestions(_ context.Context, filters repository.MessageRequestSessionIDSuggestionFilters) ([]string, error) {
	f.suggestionFilter = filters
	f.limit = filters.Limit
	return f.suggestions, nil
}

func TestUsageLogsActionReturnsRecentLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeUsageLogsStore{logs: []*model.MessageRequest{{
		ID:                  1,
		Model:               "gpt-5.4",
		Key:                 "sk-123",
		UserName:            stringPtr("alice"),
		KeyName:             stringPtr("Key A"),
		ProviderName:        stringPtr("provider-a"),
		UserAgent:           stringPtr("Claude-Code/1.0"),
		ClientIP:            stringPtr("192.0.2.1"),
		CostMultiplier:      decimalPtr("1.2500"),
		GroupCostMultiplier: decimalPtr("1.5000"),
		SwapCacheTtlApplied: true,
		CostBreakdown:       map[string]any{"total": "2.500000", "provider_multiplier": 1.25},
		CostUSD:             udecimal.MustParse("0.25"),
		SpecialSettings: []model.SpecialSetting{
			{Type: "anthropic_effort", Scope: "request", Hit: true, Effort: stringPtr("medium")},
		},
		CreatedAt: time.Now(),
	}}, summary: repository.MessageRequestSummary{
		TotalRequests: 1,
		TotalRows:     1,
		TotalCost:     0.25,
		TotalTokens:   0,
	}}

	router := gin.New()
	NewUsageLogsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodGet, "/api/actions/usage-logs?limit=10", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if store.limit != 10 {
		t.Fatalf("expected limit 10, got %d", store.limit)
	}
	if !strings.Contains(resp.Body.String(), "\"page\":1") || !strings.Contains(resp.Body.String(), "\"pageSize\":10") {
		t.Fatalf("expected paging metadata, got %s", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "\"ok\":true") || !strings.Contains(resp.Body.String(), "gpt-5.4") || !strings.Contains(resp.Body.String(), "\"userName\":\"alice\"") || !strings.Contains(resp.Body.String(), "\"keyName\":\"Key A\"") || !strings.Contains(resp.Body.String(), "\"providerName\":\"provider-a\"") || !strings.Contains(resp.Body.String(), "\"userAgent\":\"Claude-Code/1.0\"") || !strings.Contains(resp.Body.String(), "\"clientIp\":\"192.0.2.1\"") || !strings.Contains(resp.Body.String(), "\"totalTokens\":0") || !strings.Contains(resp.Body.String(), "\"costUsd\":\"0.250000\"") || !strings.Contains(resp.Body.String(), "\"costMultiplier\":\"1.25\"") || !strings.Contains(resp.Body.String(), "\"groupCostMultiplier\":\"1.5\"") || !strings.Contains(resp.Body.String(), "\"swapCacheTtlApplied\":true") || !strings.Contains(resp.Body.String(), "\"costBreakdown\":{\"provider_multiplier\":1.25,\"total\":\"2.500000\"}") || !strings.Contains(resp.Body.String(), "\"anthropicEffort\":\"medium\"") || !strings.Contains(resp.Body.String(), "\"summary\":{\"totalRequests\":1,\"totalRows\":1,\"totalCost\":0.25") {
		t.Fatalf("expected usage logs payload, got %s", resp.Body.String())
	}
}

func TestUsageLogsActionRejectsInvalidLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	router := gin.New()
	NewUsageLogsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		&fakeUsageLogsStore{},
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodGet, "/api/actions/usage-logs?limit=bad", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestUsageLogsActionAcceptsFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeUsageLogsStore{summary: repository.MessageRequestSummary{TotalRequests: 3, TotalRows: 4}}
	router := gin.New()
	NewUsageLogsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodGet, "/api/actions/usage-logs?limit=5&model=gpt-5.4&endpoint=/v1/responses&sessionId=sess_123&statusCode=201&minRetryCount=2", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if store.limit != 5 || store.filters.Model != "gpt-5.4" || store.filters.Endpoint != "/v1/responses" || store.filters.SessionID != "sess_123" {
		t.Fatalf("unexpected captured filters: %+v", store)
	}
	if store.filters.StatusCode == nil || *store.filters.StatusCode != 201 {
		t.Fatalf("expected status filter 201, got %+v", store.filters.StatusCode)
	}
	if store.filters.MinRetryCount == nil || *store.filters.MinRetryCount != 2 {
		t.Fatalf("expected minRetryCount filter 2, got %+v", store.filters.MinRetryCount)
	}
	if !strings.Contains(resp.Body.String(), "\"total\":0") || !strings.Contains(resp.Body.String(), "\"summary\":{\"totalRequests\":3,\"totalRows\":4") || !strings.Contains(resp.Body.String(), "\"minRetryCount\":2") {
		t.Fatalf("expected total field in payload, got %s", resp.Body.String())
	}
}

func TestUsageLogsActionPostJSONAcceptsFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeUsageLogsStore{summary: repository.MessageRequestSummary{TotalRequests: 2, TotalRows: 2}}
	router := gin.New()
	NewUsageLogsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/usage-logs/getUsageLogs", strings.NewReader(`{"page":2,"pageSize":25,"userId":9,"keyId":11,"providerId":13,"minRetryCount":3,"model":"gpt-5.4","endpoint":"/v1/responses","sessionId":"sess_123","statusCode":201,"excludeStatusCode200":true,"startTime":1710000000000,"endTime":1710003600000}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if store.page != 2 || store.limit != 25 {
		t.Fatalf("expected page/pageSize capture, got page=%d limit=%d", store.page, store.limit)
	}
	if store.filters.UserID == nil || *store.filters.UserID != 9 || store.filters.KeyID == nil || *store.filters.KeyID != 11 || store.filters.ProviderID == nil || *store.filters.ProviderID != 13 {
		t.Fatalf("expected user/key/provider filters, got %+v", store.filters)
	}
	if store.filters.MinRetryCount == nil || *store.filters.MinRetryCount != 3 {
		t.Fatalf("expected minRetryCount filter, got %+v", store.filters.MinRetryCount)
	}
	if store.filters.StatusCode == nil || *store.filters.StatusCode != 201 || !store.filters.ExcludeStatusCode200 {
		t.Fatalf("expected status filters, got %+v", store.filters)
	}
	if store.filters.StartTime == nil || store.filters.EndTime == nil {
		t.Fatalf("expected start/end time filters, got %+v", store.filters)
	}
	if !strings.Contains(resp.Body.String(), "\"userId\":9") || !strings.Contains(resp.Body.String(), "\"excludeStatusCode200\":true") || !strings.Contains(resp.Body.String(), "\"minRetryCount\":3") || !strings.Contains(resp.Body.String(), "\"summary\":{\"totalRequests\":2,\"totalRows\":2") {
		t.Fatalf("expected response echo filters, got %s", resp.Body.String())
	}
}

func TestUsageLogsActionRejectsInvalidStatusCode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	router := gin.New()
	NewUsageLogsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		&fakeUsageLogsStore{},
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodGet, "/api/actions/usage-logs?statusCode=bad", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestUsageLogsActionReturnsLogDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeUsageLogsStore{logs: []*model.MessageRequest{{
		ID:                  7,
		Model:               "gpt-5.4",
		Key:                 "sk-123",
		UserName:            stringPtr("alice"),
		KeyName:             stringPtr("Key A"),
		ProviderName:        stringPtr("provider-a"),
		UserAgent:           stringPtr("Claude-Code/1.0"),
		ClientIP:            stringPtr("192.0.2.1"),
		CostMultiplier:      decimalPtr("1.2500"),
		GroupCostMultiplier: decimalPtr("1.5000"),
		SwapCacheTtlApplied: true,
		CostBreakdown:       map[string]any{"total": "2.500000", "provider_multiplier": 1.25},
		SpecialSettings: []model.SpecialSetting{
			{Type: "anthropic_effort", Scope: "request", Hit: true, Effort: stringPtr("medium")},
		},
		CostUSD:   udecimal.MustParse("0.25"),
		CreatedAt: time.Now(),
	}}}
	router := gin.New()
	NewUsageLogsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodGet, "/api/actions/usage-logs/7", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "\"id\":7") || !strings.Contains(resp.Body.String(), "gpt-5.4") || !strings.Contains(resp.Body.String(), "\"userName\":\"alice\"") || !strings.Contains(resp.Body.String(), "\"keyName\":\"Key A\"") || !strings.Contains(resp.Body.String(), "\"providerName\":\"provider-a\"") || !strings.Contains(resp.Body.String(), "\"userAgent\":\"Claude-Code/1.0\"") || !strings.Contains(resp.Body.String(), "\"clientIp\":\"192.0.2.1\"") || !strings.Contains(resp.Body.String(), "\"totalTokens\":0") || !strings.Contains(resp.Body.String(), "\"costUsd\":\"0.250000\"") || !strings.Contains(resp.Body.String(), "\"costMultiplier\":\"1.25\"") || !strings.Contains(resp.Body.String(), "\"groupCostMultiplier\":\"1.5\"") || !strings.Contains(resp.Body.String(), "\"swapCacheTtlApplied\":true") || !strings.Contains(resp.Body.String(), "\"costBreakdown\":{\"provider_multiplier\":1.25,\"total\":\"2.500000\"}") || !strings.Contains(resp.Body.String(), "\"anthropicEffort\":\"medium\"") {
		t.Fatalf("expected log detail payload, got %s", resp.Body.String())
	}
}

func TestUsageLogsActionFallsBackNamesAndProviderNull(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeUsageLogsStore{logs: []*model.MessageRequest{{
		ID:        8,
		UserID:    42,
		Key:       "sk-raw",
		Model:     "gpt-5.4",
		CreatedAt: time.Now(),
	}}}
	router := gin.New()
	NewUsageLogsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodGet, "/api/actions/usage-logs/8", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !strings.Contains(body, "\"userName\":\"User #42\"") || !strings.Contains(body, "\"keyName\":\"sk-raw\"") || !strings.Contains(body, "\"providerName\":null") || !strings.Contains(body, "\"providerChain\":null") || !strings.Contains(body, "\"specialSettings\":null") {
		t.Fatalf("expected fallback usage log fields, got %s", body)
	}
}

func decimalPtr(value string) *udecimal.Decimal {
	d := udecimal.MustParse(value)
	return &d
}

func TestUsageLogsActionReturnsSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeUsageLogsStore{summary: repository.MessageRequestSummary{
		TotalRows:                  4,
		TotalRequests:              3,
		TotalCost:                  1.25,
		TotalTokens:                120,
		TotalInputTokens:           80,
		TotalOutputTokens:          40,
		TotalCacheCreationTokens:   10,
		TotalCacheReadTokens:       5,
		TotalCacheCreation5mTokens: 7,
		TotalCacheCreation1hTokens: 3,
	}}
	router := gin.New()
	NewUsageLogsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodGet, "/api/actions/usage-logs/summary?model=gpt-5.4&statusCode=201", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if store.filters.Model != "gpt-5.4" || store.filters.StatusCode == nil || *store.filters.StatusCode != 201 {
		t.Fatalf("unexpected summary filters: %+v", store)
	}
	if !strings.Contains(resp.Body.String(), "\"totalRequests\":3") ||
		!strings.Contains(resp.Body.String(), "\"totalRows\":4") ||
		!strings.Contains(resp.Body.String(), "\"totalTokens\":120") ||
		!strings.Contains(resp.Body.String(), "\"totalCost\":1.25") ||
		!strings.Contains(resp.Body.String(), "\"totalInputTokens\":80") {
		t.Fatalf("expected summary payload, got %s", resp.Body.String())
	}
}

func TestUsageLogsStatsActionAcceptsJSONFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeUsageLogsStore{summary: repository.MessageRequestSummary{TotalRequests: 1}}
	router := gin.New()
	NewUsageLogsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/usage-logs/getUsageLogsStats", strings.NewReader(`{"providerId":7,"excludeStatusCode200":true,"minRetryCount":2}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if store.filters.ProviderID == nil || *store.filters.ProviderID != 7 || !store.filters.ExcludeStatusCode200 {
		t.Fatalf("expected summary JSON filters, got %+v", store.filters)
	}
	if store.filters.MinRetryCount == nil || *store.filters.MinRetryCount != 2 {
		t.Fatalf("expected summary minRetryCount, got %+v", store.filters.MinRetryCount)
	}
}

func TestUsageLogsActionReturnsFilterOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	origNow := usageLogsFilterOptionsNow
	usageLogsFilterOptionsNow = func() time.Time { return time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC) }
	defer func() {
		usageLogsFilterOptionsNow = origNow
		defaultUsageLogsFilterOptionsCache = &usageLogsFilterOptionsCacheStore{}
	}()
	defaultUsageLogsFilterOptionsCache = &usageLogsFilterOptionsCacheStore{}

	enabled := true
	store := &fakeUsageLogsStore{options: repository.MessageRequestFilterOptions{
		Models:      []string{"gpt-5.4", "claude-sonnet-4"},
		StatusCodes: []int{200, 502},
		Endpoints:   []string{"/v1/responses", "/v1/messages"},
	}, logs: []*model.MessageRequest{{ID: 1, Model: "gpt-5.4"}}, summary: repository.MessageRequestSummary{TotalRequests: 2, TotalTokens: 10}}
	router := gin.New()
	NewUsageLogsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodGet, "/api/actions/usage-logs/filter-options", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "gpt-5.4") || !strings.Contains(resp.Body.String(), "/v1/responses") || !strings.Contains(resp.Body.String(), "\"statusCodes\":[200,502]") {
		t.Fatalf("expected filter options payload, got %s", resp.Body.String())
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/actions/usage-logs/getFilterOptions", strings.NewReader(`{}`))
	postReq.Header.Set("Authorization", "Bearer admin-token")
	postReq.Header.Set("Content-Type", "application/json")
	postResp := httptest.NewRecorder()
	router.ServeHTTP(postResp, postReq)
	if postResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", postResp.Code, postResp.Body.String())
	}
	if !strings.Contains(postResp.Body.String(), "gpt-5.4") {
		t.Fatalf("expected action-style filter options payload, got %s", postResp.Body.String())
	}
	if store.filterOptionsCalls != 1 {
		t.Fatalf("expected filter options cache to reuse first fetch, got %d store calls", store.filterOptionsCalls)
	}

	usageLogsFilterOptionsNow = func() time.Time { return time.Date(2026, 4, 24, 12, 6, 0, 0, time.UTC) }
	expiredReq := httptest.NewRequest(http.MethodGet, "/api/actions/usage-logs/filter-options", nil)
	expiredReq.Header.Set("Authorization", "Bearer admin-token")
	expiredResp := httptest.NewRecorder()
	router.ServeHTTP(expiredResp, expiredReq)
	if expiredResp.Code != http.StatusOK {
		t.Fatalf("expected 200 after cache expiry, got %d: %s", expiredResp.Code, expiredResp.Body.String())
	}
	if store.filterOptionsCalls != 2 {
		t.Fatalf("expected cache expiry to trigger second fetch, got %d store calls", store.filterOptionsCalls)
	}
}

func TestUsageLogsActionReturnsConvenienceLists(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeUsageLogsStore{options: repository.MessageRequestFilterOptions{
		Models:      []string{"gpt-5.4", "claude-sonnet-4"},
		StatusCodes: []int{200, 502},
		Endpoints:   []string{"/v1/responses", "/v1/messages"},
	}, logs: []*model.MessageRequest{{ID: 1, Model: "gpt-5.4"}}, summary: repository.MessageRequestSummary{TotalRequests: 2, TotalTokens: 10}}
	router := gin.New()
	NewUsageLogsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	).RegisterRoutes(router.Group("/api/actions"))

	tests := []struct {
		path         string
		method       string
		body         string
		wantContains string
	}{
		{path: "/api/actions/usage-logs/models", method: http.MethodGet, wantContains: "gpt-5.4"},
		{path: "/api/actions/usage-logs/getModelList", method: http.MethodPost, body: `{}`, wantContains: "gpt-5.4"},
		{path: "/api/actions/usage-logs/status-codes", method: http.MethodGet, wantContains: "502"},
		{path: "/api/actions/usage-logs/getStatusCodeList", method: http.MethodPost, body: `{}`, wantContains: "502"},
		{path: "/api/actions/usage-logs/endpoints", method: http.MethodGet, wantContains: "/v1/responses"},
		{path: "/api/actions/usage-logs/getEndpointList", method: http.MethodPost, body: `{}`, wantContains: "/v1/responses"},
		{path: "/api/actions/usage-logs/getUsageLogs", method: http.MethodPost, body: `{}`, wantContains: "gpt-5.4"},
		{path: "/api/actions/usage-logs/getUsageLogsBatch", method: http.MethodPost, body: `{}`, wantContains: "\"logs\":[{\""},
		{path: "/api/actions/usage-logs/startUsageLogsExport", method: http.MethodPost, body: `{}`, wantContains: "\"jobId\":\""},
		{path: "/api/actions/usage-logs/getUsageLogsStats", method: http.MethodPost, body: `{}`, wantContains: "\"totalRequests\":2"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer admin-token")
			if tc.method == http.MethodPost {
				req.Header.Set("Content-Type", "application/json")
			}
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
			}
			if !strings.Contains(resp.Body.String(), tc.wantContains) {
				t.Fatalf("expected payload to contain %q, got %s", tc.wantContains, resp.Body.String())
			}
		})
	}
}

func TestUsageLogsBatchActionAcceptsCursorFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeUsageLogsStore{
		batchResult: repository.MessageRequestBatchResult{
			Logs: []*model.MessageRequest{{
				ID:        10,
				UserID:    7,
				Key:       "sk-123",
				Model:     "gpt-5.4",
				CreatedAt: time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC),
			}},
			NextCursor: &repository.MessageRequestBatchCursor{
				CreatedAt: "2026-04-24T09:59:00Z",
				ID:        9,
			},
			HasMore: true,
		},
	}
	router := gin.New()
	NewUsageLogsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/usage-logs/getUsageLogsBatch", strings.NewReader(`{"limit":150,"sessionId":"sess_123","minRetryCount":2,"cursor":{"createdAt":"2026-04-24T10:01:00Z","id":11}}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if store.batchFilters.Cursor == nil || store.batchFilters.Cursor.ID != 11 || store.batchFilters.Cursor.CreatedAt != "2026-04-24T10:01:00Z" {
		t.Fatalf("expected cursor filter capture, got %+v", store.batchFilters.Cursor)
	}
	if store.batchFilters.MinRetryCount == nil || *store.batchFilters.MinRetryCount != 2 {
		t.Fatalf("expected minRetryCount in batch filters, got %+v", store.batchFilters.MinRetryCount)
	}
	if store.limit != 100 {
		t.Fatalf("expected batch limit to normalize to 100, got %d", store.limit)
	}
	body := resp.Body.String()
	if !strings.Contains(body, "\"hasMore\":true") || !strings.Contains(body, "\"nextCursor\":{\"createdAt\":\"2026-04-24T09:59:00Z\",\"id\":9}") || !strings.Contains(body, "\"id\":10") {
		t.Fatalf("expected batch payload, got %s", body)
	}
}

func TestUsageLogsExportFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	origNow := usageLogsExportNow
	usageLogsExportNow = func() time.Time { return time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC) }
	defer func() {
		usageLogsExportNow = origNow
		defaultUsageLogsExportStore = &usageLogsExportStore{items: map[string]usageLogsExportRecord{}}
	}()
	defaultUsageLogsExportStore = &usageLogsExportStore{items: map[string]usageLogsExportRecord{}}

	enabled := true
	store := &fakeUsageLogsStore{
		summary: repository.MessageRequestSummary{TotalRows: 2, TotalRequests: 2},
		batchResults: []repository.MessageRequestBatchResult{
			{
				Logs: []*model.MessageRequest{
					{
						ID:            1,
						UserID:        7,
						Key:           "sk-a",
						Model:         "gpt-5.4",
						OriginalModel: stringPtr("gpt-5.4"),
						Endpoint:      stringPtr("/v1/responses"),
						StatusCode:    intPtr(200),
						InputTokens:   intPtr(10),
						OutputTokens:  intPtr(5),
						DurationMs:    intPtr(120),
						SessionID:     stringPtr("sess_export"),
						UserName:      stringPtr("alice"),
						KeyName:       stringPtr("Key A"),
						ProviderName:  stringPtr("provider-a"),
						CreatedAt:     time.Date(2026, 4, 24, 11, 0, 0, 0, time.UTC),
					},
				},
				NextCursor: &repository.MessageRequestBatchCursor{CreatedAt: "2026-04-24T10:59:00Z", ID: 1},
				HasMore:    true,
			},
			{
				Logs: []*model.MessageRequest{
					{
						ID:         2,
						UserID:     8,
						Key:        "sk-b",
						Model:      "gpt-4o-mini",
						Endpoint:   stringPtr("/v1/messages"),
						StatusCode: intPtr(502),
						SessionID:  stringPtr("sess_export_2"),
						UserName:   stringPtr("=danger"),
						CreatedAt:  time.Date(2026, 4, 24, 10, 59, 0, 0, time.UTC),
						ProviderChain: []model.ProviderChainItem{
							{Reason: stringPtr("retry_failed")},
							{Reason: stringPtr("request_success"), StatusCode: intPtr(200)},
						},
					},
				},
				HasMore: false,
			},
		},
	}
	router := gin.New()
	NewUsageLogsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	).RegisterRoutes(router.Group("/api/actions"))

	startReq := httptest.NewRequest(http.MethodPost, "/api/actions/usage-logs/startUsageLogsExport", strings.NewReader(`{"minRetryCount":1}`))
	startReq.Header.Set("Authorization", "Bearer admin-token")
	startReq.Header.Set("Content-Type", "application/json")
	startResp := httptest.NewRecorder()
	router.ServeHTTP(startResp, startReq)

	if startResp.Code != http.StatusOK {
		t.Fatalf("expected export start 200, got %d: %s", startResp.Code, startResp.Body.String())
	}
	var startPayload struct {
		OK   bool `json:"ok"`
		Data struct {
			JobID string `json:"jobId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(startResp.Body.Bytes(), &startPayload); err != nil {
		t.Fatalf("failed to decode start payload: %v", err)
	}
	if !startPayload.OK || startPayload.Data.JobID == "" {
		t.Fatalf("expected export job id, got %s", startResp.Body.String())
	}
	if len(store.batchCalls) != 2 {
		for i := 0; i < 50 && len(store.batchCalls) < 2; i++ {
			time.Sleep(5 * time.Millisecond)
		}
	}
	if len(store.batchCalls) != 2 {
		t.Fatalf("expected two batch calls for export, got %d", len(store.batchCalls))
	}
	if store.batchCalls[0].MinRetryCount == nil || *store.batchCalls[0].MinRetryCount != 1 {
		t.Fatalf("expected start export to pass minRetryCount, got %+v", store.batchCalls[0].MinRetryCount)
	}
	if store.batchCalls[1].Cursor == nil || store.batchCalls[1].Cursor.ID != 1 {
		t.Fatalf("expected next cursor on second batch call, got %+v", store.batchCalls[1].Cursor)
	}

	var statusBody string
	for i := 0; i < 50; i++ {
		statusReq := httptest.NewRequest(http.MethodPost, "/api/actions/usage-logs/getUsageLogsExportStatus", strings.NewReader(`{"jobId":"`+startPayload.Data.JobID+`"}`))
		statusReq.Header.Set("Authorization", "Bearer admin-token")
		statusReq.Header.Set("Content-Type", "application/json")
		statusResp := httptest.NewRecorder()
		router.ServeHTTP(statusResp, statusReq)

		if statusResp.Code != http.StatusOK {
			t.Fatalf("expected export status 200, got %d: %s", statusResp.Code, statusResp.Body.String())
		}
		statusBody = statusResp.Body.String()
		if strings.Contains(statusBody, `"status":"completed"`) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !strings.Contains(statusBody, `"status":"completed"`) || !strings.Contains(statusBody, `"processedRows":2`) || !strings.Contains(statusBody, `"progressPercent":100`) {
		t.Fatalf("expected completed export status, got %s", statusBody)
	}

	downloadReq := httptest.NewRequest(http.MethodPost, "/api/actions/usage-logs/downloadUsageLogsExport", strings.NewReader(`{"jobId":"`+startPayload.Data.JobID+`"}`))
	downloadReq.Header.Set("Authorization", "Bearer admin-token")
	downloadReq.Header.Set("Content-Type", "application/json")
	downloadResp := httptest.NewRecorder()
	router.ServeHTTP(downloadResp, downloadReq)

	if downloadResp.Code != http.StatusOK {
		t.Fatalf("expected export download 200, got %d: %s", downloadResp.Code, downloadResp.Body.String())
	}
	body := downloadResp.Body.String()
	if !strings.Contains(body, "Time,User,Key,Provider,Model,Original Model,Endpoint,Status Code") || !strings.Contains(body, "sess_export") || !strings.Contains(body, "'=danger") {
		t.Fatalf("expected csv payload, got %s", body)
	}
	if !strings.Contains(body, ",1\\n") && !strings.Contains(body, ",1\"") {
		t.Fatalf("expected retry count in csv payload, got %s", body)
	}
}

func TestUsageLogsActionReturnsSessionIDSuggestions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeUsageLogsStore{suggestions: []string{"sess_123", "sess_456"}}
	router := gin.New()
	NewUsageLogsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodGet, "/api/actions/usage-logs/session-id-suggestions?term=sess&limit=5", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if store.suggestionFilter.Term != "sess" || store.limit != 5 {
		t.Fatalf("unexpected suggestion query capture: %+v", store)
	}
	if !strings.Contains(resp.Body.String(), "sess_123") {
		t.Fatalf("expected suggestion payload, got %s", resp.Body.String())
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/actions/usage-logs/getUsageLogSessionIdSuggestions", strings.NewReader(`{"term":"sess","limit":5,"userId":1,"keyId":2,"providerId":3}`))
	postReq.Header.Set("Authorization", "Bearer admin-token")
	postReq.Header.Set("Content-Type", "application/json")
	postResp := httptest.NewRecorder()
	router.ServeHTTP(postResp, postReq)
	if postResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", postResp.Code, postResp.Body.String())
	}
	if store.suggestionFilter.UserID == nil || *store.suggestionFilter.UserID != 1 || store.suggestionFilter.KeyID == nil || *store.suggestionFilter.KeyID != 2 || store.suggestionFilter.ProviderID == nil || *store.suggestionFilter.ProviderID != 3 {
		t.Fatalf("expected action-style suggestion filters, got %+v", store.suggestionFilter)
	}
	if !strings.Contains(postResp.Body.String(), "sess_123") {
		t.Fatalf("expected action-style suggestion payload, got %s", postResp.Body.String())
	}
}

func TestUsageLogsActionReturnsEmptySuggestionsForShortTerm(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeUsageLogsStore{}
	router := gin.New()
	NewUsageLogsActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodGet, "/api/actions/usage-logs/session-id-suggestions?term=s", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if store.suggestionFilter.Term != "" {
		t.Fatalf("expected store not to be queried for short term, got %+v", store)
	}
	if !strings.Contains(resp.Body.String(), "\"data\":[]") {
		t.Fatalf("expected empty suggestions payload, got %s", resp.Body.String())
	}
}
