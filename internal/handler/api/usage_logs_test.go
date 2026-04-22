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

type fakeUsageLogsStore struct {
	logs        []*model.MessageRequest
	limit       int
	model       string
	endpoint    string
	sessionID   string
	status      *int
	summary     repository.MessageRequestSummary
	options     repository.MessageRequestFilterOptions
	suggestions []string
	term        string
}

func (f *fakeUsageLogsStore) ListRecent(_ context.Context, limit int) ([]*model.MessageRequest, error) {
	f.limit = limit
	return f.logs, nil
}

func (f *fakeUsageLogsStore) ListFiltered(_ context.Context, limit int, modelName, endpoint, sessionID string, statusCode *int) ([]*model.MessageRequest, error) {
	f.limit = limit
	f.model = modelName
	f.endpoint = endpoint
	f.sessionID = sessionID
	f.status = statusCode
	return f.logs, nil
}

func (f *fakeUsageLogsStore) GetByID(_ context.Context, id int) (*model.MessageRequest, error) {
	for _, log := range f.logs {
		if log.ID == id {
			return log, nil
		}
	}
	return nil, nil
}

func (f *fakeUsageLogsStore) GetSummary(_ context.Context, modelName, endpoint string, statusCode *int) (repository.MessageRequestSummary, error) {
	f.model = modelName
	f.endpoint = endpoint
	f.status = statusCode
	return f.summary, nil
}

func (f *fakeUsageLogsStore) GetFilterOptions(_ context.Context) (repository.MessageRequestFilterOptions, error) {
	return f.options, nil
}

func (f *fakeUsageLogsStore) FindSessionIDSuggestions(_ context.Context, term string, limit int) ([]string, error) {
	f.term = term
	f.limit = limit
	return f.suggestions, nil
}

func TestUsageLogsActionReturnsRecentLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeUsageLogsStore{logs: []*model.MessageRequest{{
		ID:        1,
		Model:     "gpt-5.4",
		Key:       "sk-123",
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
	if !strings.Contains(resp.Body.String(), "\"ok\":true") || !strings.Contains(resp.Body.String(), "gpt-5.4") {
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

	req := httptest.NewRequest(http.MethodGet, "/api/actions/usage-logs?limit=5&model=gpt-5.4&endpoint=/v1/responses&sessionId=sess_123&statusCode=201", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if store.limit != 5 || store.model != "gpt-5.4" || store.endpoint != "/v1/responses" || store.sessionID != "sess_123" {
		t.Fatalf("unexpected captured filters: %+v", store)
	}
	if store.status == nil || *store.status != 201 {
		t.Fatalf("expected status filter 201, got %+v", store.status)
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
		ID:        7,
		Model:     "gpt-5.4",
		Key:       "sk-123",
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
	if !strings.Contains(resp.Body.String(), "\"id\":7") || !strings.Contains(resp.Body.String(), "gpt-5.4") {
		t.Fatalf("expected log detail payload, got %s", resp.Body.String())
	}
}

func TestUsageLogsActionReturnsSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeUsageLogsStore{summary: repository.MessageRequestSummary{
		TotalRequests:              3,
		TotalCost:                  "1.25",
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
	if store.model != "gpt-5.4" || store.status == nil || *store.status != 201 {
		t.Fatalf("unexpected summary filters: %+v", store)
	}
	if !strings.Contains(resp.Body.String(), "\"totalRequests\":3") ||
		!strings.Contains(resp.Body.String(), "\"totalTokens\":120") ||
		!strings.Contains(resp.Body.String(), "\"totalCost\":\"1.25\"") ||
		!strings.Contains(resp.Body.String(), "\"totalInputTokens\":80") {
		t.Fatalf("expected summary payload, got %s", resp.Body.String())
	}
}

func TestUsageLogsActionReturnsFilterOptions(t *testing.T) {
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
		{path: "/api/actions/usage-logs/getUsageLogs", method: http.MethodPost, body: `{}`, wantContains: "gpt-5.4"},
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
	if store.term != "sess" || store.limit != 5 {
		t.Fatalf("unexpected suggestion query capture: %+v", store)
	}
	if !strings.Contains(resp.Body.String(), "sess_123") {
		t.Fatalf("expected suggestion payload, got %s", resp.Body.String())
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/actions/usage-logs/getUsageLogSessionIdSuggestions", strings.NewReader(`{"term":"sess","limit":5}`))
	postReq.Header.Set("Authorization", "Bearer admin-token")
	postReq.Header.Set("Content-Type", "application/json")
	postResp := httptest.NewRecorder()
	router.ServeHTTP(postResp, postReq)
	if postResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", postResp.Code, postResp.Body.String())
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
	if store.term != "" {
		t.Fatalf("expected store not to be queried for short term, got %+v", store)
	}
	if !strings.Contains(resp.Body.String(), "\"data\":[]") {
		t.Fatalf("expected empty suggestions payload, got %s", resp.Body.String())
	}
}
