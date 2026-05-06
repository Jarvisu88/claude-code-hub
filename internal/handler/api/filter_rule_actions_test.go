package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/repository"
	"github.com/gin-gonic/gin"
)

type routeFakeSensitiveWordStore struct {
	items []*model.SensitiveWord
	stats *repository.CacheStats
}

func (f *routeFakeSensitiveWordStore) Create(_ context.Context, word *model.SensitiveWord) (*model.SensitiveWord, error) {
	word.ID = len(f.items) + 1
	f.items = append(f.items, word)
	return word, nil
}
func (f *routeFakeSensitiveWordStore) GetByID(_ context.Context, id int) (*model.SensitiveWord, error) {
	for _, item := range f.items {
		if item.ID == id {
			return item, nil
		}
	}
	return nil, nil
}
func (f *routeFakeSensitiveWordStore) Update(_ context.Context, word *model.SensitiveWord) (*model.SensitiveWord, error) {
	return word, nil
}
func (f *routeFakeSensitiveWordStore) Delete(_ context.Context, _ int) error { return nil }
func (f *routeFakeSensitiveWordStore) List(_ context.Context, _ *repository.ListOptions) ([]*model.SensitiveWord, error) {
	return f.items, nil
}
func (f *routeFakeSensitiveWordStore) RefreshCache(_ context.Context) (*repository.CacheStats, error) {
	if f.stats == nil {
		f.stats = &repository.CacheStats{Resource: "sensitive-words"}
	}
	return f.stats, nil
}
func (f *routeFakeSensitiveWordStore) GetCacheStats(_ context.Context) (*repository.CacheStats, error) {
	if f.stats == nil {
		f.stats = &repository.CacheStats{Resource: "sensitive-words"}
	}
	return f.stats, nil
}

type routeFakeErrorRuleStore struct {
	items []*model.ErrorRule
	stats *repository.CacheStats
}

func (f *routeFakeErrorRuleStore) Create(_ context.Context, rule *model.ErrorRule) (*model.ErrorRule, error) {
	rule.ID = len(f.items) + 1
	f.items = append(f.items, rule)
	return rule, nil
}
func (f *routeFakeErrorRuleStore) GetByID(_ context.Context, id int) (*model.ErrorRule, error) {
	for _, item := range f.items {
		if item.ID == id {
			return item, nil
		}
	}
	return nil, nil
}
func (f *routeFakeErrorRuleStore) Update(_ context.Context, rule *model.ErrorRule) (*model.ErrorRule, error) {
	return rule, nil
}
func (f *routeFakeErrorRuleStore) Delete(_ context.Context, _ int) error { return nil }
func (f *routeFakeErrorRuleStore) List(_ context.Context, _ *repository.ListOptions) ([]*model.ErrorRule, error) {
	return f.items, nil
}
func (f *routeFakeErrorRuleStore) RefreshCache(_ context.Context) (*repository.CacheStats, error) {
	if f.stats == nil {
		f.stats = &repository.CacheStats{Resource: "error-rules"}
	}
	return f.stats, nil
}
func (f *routeFakeErrorRuleStore) GetCacheStats(_ context.Context) (*repository.CacheStats, error) {
	if f.stats == nil {
		f.stats = &repository.CacheStats{Resource: "error-rules"}
	}
	return f.stats, nil
}

type routeFakeRequestFilterStore struct {
	items []*model.RequestFilter
	stats *repository.CacheStats
}

func (f *routeFakeRequestFilterStore) Create(_ context.Context, filter *model.RequestFilter) (*model.RequestFilter, error) {
	filter.ID = len(f.items) + 1
	f.items = append(f.items, filter)
	return filter, nil
}
func (f *routeFakeRequestFilterStore) GetByID(_ context.Context, id int) (*model.RequestFilter, error) {
	for _, item := range f.items {
		if item.ID == id {
			return item, nil
		}
	}
	return nil, nil
}
func (f *routeFakeRequestFilterStore) Update(_ context.Context, filter *model.RequestFilter) (*model.RequestFilter, error) {
	return filter, nil
}
func (f *routeFakeRequestFilterStore) Delete(_ context.Context, _ int) error { return nil }
func (f *routeFakeRequestFilterStore) List(_ context.Context, _ *repository.ListOptions) ([]*model.RequestFilter, error) {
	return f.items, nil
}
func (f *routeFakeRequestFilterStore) RefreshCache(_ context.Context) (*repository.CacheStats, error) {
	if f.stats == nil {
		f.stats = &repository.CacheStats{Resource: "request-filters"}
	}
	return f.stats, nil
}
func (f *routeFakeRequestFilterStore) GetCacheStats(_ context.Context) (*repository.CacheStats, error) {
	if f.stats == nil {
		f.stats = &repository.CacheStats{Resource: "request-filters"}
	}
	return f.stats, nil
}

func TestSensitiveWordsActionHandlerRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &routeFakeSensitiveWordStore{items: []*model.SensitiveWord{{ID: 1, Word: "blocked", MatchType: "contains", IsEnabled: true}}}
	router := gin.New()
	NewSensitiveWordActionHandler(newAdminActionAuth(), store).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/sensitive-words/listSensitiveWords", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"blocked"`) {
		t.Fatalf("expected list success, got %d %s", resp.Code, resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/actions/sensitive-words/createSensitiveWordAction", strings.NewReader(`{"word":"new-word","matchType":"exact"}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"new-word"`) {
		t.Fatalf("expected create success, got %d %s", resp.Code, resp.Body.String())
	}
}

func TestErrorRulesActionHandlerRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &routeFakeErrorRuleStore{items: []*model.ErrorRule{{ID: 1, Pattern: "bad", MatchType: "contains", Category: "invalid_request", IsEnabled: true}}}
	router := gin.New()
	NewErrorRuleActionHandler(newAdminActionAuth(), store).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/error-rules/getCacheStats", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"error-rules"`) {
		t.Fatalf("expected cache stats success, got %d %s", resp.Code, resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/actions/error-rules/createErrorRuleAction", strings.NewReader(`{"pattern":"oops","matchType":"contains","category":"invalid_request"}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"oops"`) {
		t.Fatalf("expected create success, got %d %s", resp.Code, resp.Body.String())
	}
}

func TestRequestFiltersActionHandlerRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &routeFakeRequestFilterStore{items: []*model.RequestFilter{{ID: 1, Name: "strip-header", Scope: "header", Action: "remove", Target: "x-test", BindingType: "global", RuleMode: "simple", ExecutionPhase: "guard", IsEnabled: true}}}
	router := gin.New()
	NewRequestFilterActionHandler(newAdminActionAuth(), store).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodPost, "/api/actions/request-filters/listRequestFilters", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"strip-header"`) {
		t.Fatalf("expected list success, got %d %s", resp.Code, resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/actions/request-filters/createRequestFilterAction", strings.NewReader(`{"name":"rewrite","scope":"body","action":"set","target":"input","bindingType":"global","ruleMode":"simple","executionPhase":"guard"}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"rewrite"`) {
		t.Fatalf("expected create success, got %d %s", resp.Code, resp.Body.String())
	}
}
