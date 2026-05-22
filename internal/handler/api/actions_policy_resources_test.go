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
	"github.com/ding113/claude-code-hub/internal/repository"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

type fakeRequestFilterStore struct {
	items      []*model.RequestFilter
	cacheStats *repository.CacheStats
}

func (f *fakeRequestFilterStore) List(_ context.Context, _ *repository.ListOptions) ([]*model.RequestFilter, error) {
	return f.items, nil
}
func (f *fakeRequestFilterStore) GetByID(_ context.Context, id int) (*model.RequestFilter, error) {
	for _, item := range f.items {
		if item.ID == id {
			return item, nil
		}
	}
	return nil, appErrors.NewNotFoundError("RequestFilter")
}
func (f *fakeRequestFilterStore) Create(_ context.Context, filter *model.RequestFilter) (*model.RequestFilter, error) {
	filter.ID = 99
	f.items = append(f.items, filter)
	return filter, nil
}
func (f *fakeRequestFilterStore) Update(_ context.Context, filter *model.RequestFilter) (*model.RequestFilter, error) {
	for idx, item := range f.items {
		if item.ID == filter.ID {
			f.items[idx] = filter
			return filter, nil
		}
	}
	return nil, appErrors.NewNotFoundError("RequestFilter")
}
func (f *fakeRequestFilterStore) Delete(_ context.Context, id int) error {
	for idx, item := range f.items {
		if item.ID == id {
			f.items = append(f.items[:idx], f.items[idx+1:]...)
			return nil
		}
	}
	return appErrors.NewNotFoundError("RequestFilter")
}
func (f *fakeRequestFilterStore) RefreshCache(_ context.Context) (*repository.CacheStats, error) {
	return f.cacheStats, nil
}
func (f *fakeRequestFilterStore) GetCacheStats(_ context.Context) (*repository.CacheStats, error) {
	return f.cacheStats, nil
}

type fakeSensitiveWordStore struct {
	items      []*model.SensitiveWord
	cacheStats *repository.CacheStats
}

func (f *fakeSensitiveWordStore) List(_ context.Context, _ *repository.ListOptions) ([]*model.SensitiveWord, error) {
	return f.items, nil
}
func (f *fakeSensitiveWordStore) GetByID(_ context.Context, id int) (*model.SensitiveWord, error) {
	for _, item := range f.items {
		if item.ID == id {
			return item, nil
		}
	}
	return nil, appErrors.NewNotFoundError("SensitiveWord")
}
func (f *fakeSensitiveWordStore) Create(_ context.Context, word *model.SensitiveWord) (*model.SensitiveWord, error) {
	word.ID = 77
	f.items = append(f.items, word)
	return word, nil
}
func (f *fakeSensitiveWordStore) Update(_ context.Context, word *model.SensitiveWord) (*model.SensitiveWord, error) {
	for idx, item := range f.items {
		if item.ID == word.ID {
			f.items[idx] = word
			return word, nil
		}
	}
	return nil, appErrors.NewNotFoundError("SensitiveWord")
}
func (f *fakeSensitiveWordStore) Delete(_ context.Context, id int) error {
	for idx, item := range f.items {
		if item.ID == id {
			f.items = append(f.items[:idx], f.items[idx+1:]...)
			return nil
		}
	}
	return appErrors.NewNotFoundError("SensitiveWord")
}
func (f *fakeSensitiveWordStore) RefreshCache(_ context.Context) (*repository.CacheStats, error) {
	return f.cacheStats, nil
}
func (f *fakeSensitiveWordStore) GetCacheStats(_ context.Context) (*repository.CacheStats, error) {
	return f.cacheStats, nil
}

type fakeErrorRuleStore struct {
	items      []*model.ErrorRule
	cacheStats *repository.CacheStats
}

func (f *fakeErrorRuleStore) List(_ context.Context, _ *repository.ListOptions) ([]*model.ErrorRule, error) {
	return f.items, nil
}
func (f *fakeErrorRuleStore) GetByID(_ context.Context, id int) (*model.ErrorRule, error) {
	for _, item := range f.items {
		if item.ID == id {
			return item, nil
		}
	}
	return nil, appErrors.NewNotFoundError("ErrorRule")
}
func (f *fakeErrorRuleStore) Create(_ context.Context, rule *model.ErrorRule) (*model.ErrorRule, error) {
	rule.ID = 55
	f.items = append(f.items, rule)
	return rule, nil
}
func (f *fakeErrorRuleStore) Update(_ context.Context, rule *model.ErrorRule) (*model.ErrorRule, error) {
	for idx, item := range f.items {
		if item.ID == rule.ID {
			f.items[idx] = rule
			return rule, nil
		}
	}
	return nil, appErrors.NewNotFoundError("ErrorRule")
}
func (f *fakeErrorRuleStore) Delete(_ context.Context, id int) error {
	for idx, item := range f.items {
		if item.ID == id {
			f.items = append(f.items[:idx], f.items[idx+1:]...)
			return nil
		}
	}
	return appErrors.NewNotFoundError("ErrorRule")
}
func (f *fakeErrorRuleStore) RefreshCache(_ context.Context) (*repository.CacheStats, error) {
	return f.cacheStats, nil
}
func (f *fakeErrorRuleStore) GetCacheStats(_ context.Context) (*repository.CacheStats, error) {
	return f.cacheStats, nil
}

func adminAuthForPolicyTests() fakeAdminAuth {
	enabled := true
	return fakeAdminAuth{result: &authsvc.AuthResult{
		IsAdmin: true,
		User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
		Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
		APIKey:  "admin-token",
	}}
}

func TestRequestFilterActionRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := &fakeRequestFilterStore{
		items: []*model.RequestFilter{{
			ID:             1,
			Name:           "mask-user-id",
			Scope:          "body",
			Action:         "set",
			Target:         "metadata.user_id",
			IsEnabled:      true,
			BindingType:    "global",
			RuleMode:       "simple",
			ExecutionPhase: "guard",
		}},
		cacheStats: &repository.CacheStats{Resource: "request-filters", Total: 1, ActiveCount: 1},
	}

	router := gin.New()
	NewRequestFilterActionHandler(adminAuthForPolicyTests(), store).RegisterRoutes(router.Group("/api/actions"))

	tests := []struct {
		name         string
		method       string
		path         string
		body         string
		wantStatus   int
		wantContains string
	}{
		{"list", http.MethodPost, "/api/actions/request-filters/listRequestFilters", `{}`, http.StatusOK, "mask-user-id"},
		{"create", http.MethodPost, "/api/actions/request-filters/createRequestFilterAction", `{"name":"simple-filter","scope":"body","action":"set","target":"messages.content","ruleMode":"simple","executionPhase":"guard"}`, http.StatusOK, "simple-filter"},
		{"update", http.MethodPost, "/api/actions/request-filters/updateRequestFilterAction", `{"id":1,"name":"mask-user-id-2","isEnabled":false,"priority":0,"bindingType":"providers","providerIds":[9],"ruleMode":"simple","executionPhase":"guard","description":null}`, http.StatusOK, "mask-user-id-2"},
		{"refresh", http.MethodPost, "/api/actions/request-filters/refreshRequestFiltersCache", `{}`, http.StatusOK, `"count":1`},
		{"stats", http.MethodPost, "/api/actions/request-filters/getCacheStats", `{}`, http.StatusOK, `"activeCount":1`},
		{"delete", http.MethodPost, "/api/actions/request-filters/deleteRequestFilterAction", `{"id":1}`, http.StatusOK, `"deleted":true`},
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

	created := store.items[len(store.items)-1]
	if created.RuleMode != "simple" || created.ExecutionPhase != "guard" {
		t.Fatalf("expected simple/guard on created filter, got %+v", created)
	}
}

func TestSensitiveWordActionRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Now()
	store := &fakeSensitiveWordStore{
		items: []*model.SensitiveWord{{
			ID:        1,
			Word:      "secret",
			MatchType: "contains",
			IsEnabled: true,
			CreatedAt: now,
			UpdatedAt: now,
		}},
		cacheStats: &repository.CacheStats{Resource: "sensitive-words", Total: 1, ActiveCount: 1},
	}

	router := gin.New()
	NewSensitiveWordActionHandler(adminAuthForPolicyTests(), store).RegisterRoutes(router.Group("/api/actions"))

	tests := []struct {
		name         string
		method       string
		path         string
		body         string
		wantStatus   int
		wantContains string
	}{
		{"list", http.MethodPost, "/api/actions/sensitive-words/listSensitiveWords", `{}`, http.StatusOK, "secret"},
		{"create", http.MethodPost, "/api/actions/sensitive-words/createSensitiveWordAction", `{"word":"blocked","matchType":"exact"}`, http.StatusOK, "blocked"},
		{"update", http.MethodPost, "/api/actions/sensitive-words/updateSensitiveWordAction", `{"id":1,"word":"blockedregex","matchType":"regex","isEnabled":false}`, http.StatusOK, "blockedregex"},
		{"refresh", http.MethodPost, "/api/actions/sensitive-words/refreshCacheAction", `{}`, http.StatusOK, `"stats"`},
		{"stats", http.MethodPost, "/api/actions/sensitive-words/getCacheStats", `{}`, http.StatusOK, `"activeCount":1`},
		{"delete", http.MethodPost, "/api/actions/sensitive-words/deleteSensitiveWordAction", `{"id":1}`, http.StatusOK, `"deleted":true`},
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
}

func TestErrorRuleActionRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := &fakeErrorRuleStore{
		items: []*model.ErrorRule{{
			ID:        1,
			Pattern:   "rate limit",
			MatchType: "contains",
			Category:  "rate_limit",
			IsEnabled: true,
			IsDefault: false,
			Priority:  10,
		}},
		cacheStats: &repository.CacheStats{Resource: "error-rules", Total: 1, ActiveCount: 1},
	}

	router := gin.New()
	NewErrorRuleActionHandler(adminAuthForPolicyTests(), store).RegisterRoutes(router.Group("/api/actions"))

	tests := []struct {
		name         string
		method       string
		path         string
		body         string
		wantStatus   int
		wantContains string
	}{
		{"list", http.MethodPost, "/api/actions/error-rules/listErrorRules", `{}`, http.StatusOK, "rate limit"},
		{"create", http.MethodPost, "/api/actions/error-rules/createErrorRuleAction", `{"pattern":"^quota.*$","matchType":"regex","category":"quota_limit","overrideStatusCode":429}`, http.StatusOK, "quota_limit"},
		{"update", http.MethodPost, "/api/actions/error-rules/updateErrorRuleAction", `{"id":1,"pattern":"quota exceeded","matchType":"contains","isEnabled":false,"priority":0,"overrideResponse":null,"overrideStatusCode":null}`, http.StatusOK, "quota exceeded"},
		{"refresh", http.MethodPost, "/api/actions/error-rules/refreshCacheAction", `{}`, http.StatusOK, `"syncResult"`},
		{"stats", http.MethodPost, "/api/actions/error-rules/getCacheStats", `{}`, http.StatusOK, `"activeCount":1`},
		{"delete", http.MethodPost, "/api/actions/error-rules/deleteErrorRuleAction", `{"id":1}`, http.StatusOK, `"deleted":true`},
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
}
