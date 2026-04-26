package v1

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	providertrackersvc "github.com/ding113/claude-code-hub/internal/service/providertracker"
	sessionsvc "github.com/ding113/claude-code-hub/internal/service/session"
	"github.com/gin-gonic/gin"
	"github.com/quagmt/udecimal"
)

type testKeyRepo struct {
	key *model.Key
	err error
}

func (r *testKeyRepo) GetByKeyWithUser(_ context.Context, _ string) (*model.Key, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.key, nil
}

type testUserRepo struct{}

func (testUserRepo) MarkUserExpired(_ context.Context, _ int) (bool, error) {
	return true, nil
}

type fakeSessionManager struct {
	extractedSessionID    string
	generatedSessionID    string
	updatedCodexSessionID string
	updatedPromptCacheKey string
	updatedProviderID     int
	concurrentCount       int
	extractCalls          int
	getOrCreateCalls      int
	incrementCalls        []string
	decrementCalls        []string
	lastExtractedBody     map[string]any
	lastGetOrCreateKeyID  int
	lastGetOrCreateMsgs   any
	lastGetOrCreateClient string
}

type fakeProviderRepo struct {
	providers []*model.Provider
	err       error
}

type fakeMessageRequestRepo struct {
	created []*model.MessageRequest
	updated []struct {
		id     int
		update repository.MessageRequestTerminalUpdate
	}
	err error
}

type fakeProxySystemSettingsStore struct {
	settings *model.SystemSettings
	err      error
}

type fakeProxyStatisticsStore struct {
	userTotal     udecimal.Decimal
	keyTotal      udecimal.Decimal
	user5h        udecimal.Decimal
	key5h         udecimal.Decimal
	userDaily     udecimal.Decimal
	keyDaily      udecimal.Decimal
	providerTotal udecimal.Decimal
	err           error
}

type timeoutHTTPClient struct{}

func (timeoutHTTPClient) Do(*http.Request) (*http.Response, error) {
	return nil, context.DeadlineExceeded
}

type genericFailHTTPClient struct{}

func (genericFailHTTPClient) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("boom")
}

func (f *fakeProviderRepo) GetActiveProviders(_ context.Context) ([]*model.Provider, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.providers, nil
}

func (f *fakeMessageRequestRepo) Create(_ context.Context, messageRequest *model.MessageRequest) (*model.MessageRequest, error) {
	if f.err != nil {
		return nil, f.err
	}
	if messageRequest.ID == 0 {
		messageRequest.ID = len(f.created) + 1
	}
	f.created = append(f.created, messageRequest)
	return messageRequest, nil
}

func (f *fakeMessageRequestRepo) UpdateTerminal(_ context.Context, id int, update repository.MessageRequestTerminalUpdate) error {
	if f.err != nil {
		return f.err
	}
	f.updated = append(f.updated, struct {
		id     int
		update repository.MessageRequestTerminalUpdate
	}{id: id, update: update})
	return nil
}

func (f *fakeProxySystemSettingsStore) Get(_ context.Context) (*model.SystemSettings, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.settings == nil {
		return &model.SystemSettings{ID: 1}, nil
	}
	return f.settings, nil
}

func (f *fakeProxyStatisticsStore) SumUserTotalCost(_ context.Context, _ int, _ int) (udecimal.Decimal, error) {
	if f.err != nil {
		return udecimal.Zero, f.err
	}
	return f.userTotal, nil
}

func (f *fakeProxyStatisticsStore) SumKeyTotalCost(_ context.Context, _ string, _ int) (udecimal.Decimal, error) {
	if f.err != nil {
		return udecimal.Zero, f.err
	}
	return f.keyTotal, nil
}

func isAbout5hWindow(startTime, endTime time.Time) bool {
	duration := endTime.Sub(startTime)
	return duration >= 5*time.Hour-2*time.Minute && duration <= 5*time.Hour+2*time.Minute
}

func (f *fakeProxyStatisticsStore) SumUserCostInTimeRange(_ context.Context, _ int, startTime, endTime time.Time) (udecimal.Decimal, error) {
	if f.err != nil {
		return udecimal.Zero, f.err
	}
	if isAbout5hWindow(startTime, endTime) {
		return f.user5h, nil
	}
	return f.userDaily, nil
}

func (f *fakeProxyStatisticsStore) SumKeyCostInTimeRangeByKeyString(_ context.Context, _ string, startTime, endTime time.Time) (udecimal.Decimal, error) {
	if f.err != nil {
		return udecimal.Zero, f.err
	}
	if isAbout5hWindow(startTime, endTime) {
		return f.key5h, nil
	}
	return f.keyDaily, nil
}

func (f *fakeProxyStatisticsStore) SumProviderTotalCost(_ context.Context, _ int, _ *time.Time) (udecimal.Decimal, error) {
	if f.err != nil {
		return udecimal.Zero, f.err
	}
	return f.providerTotal, nil
}

func (f *fakeSessionManager) ExtractClientSessionID(requestBody map[string]any, _ http.Header) sessionsvc.ClientSessionExtractionResult {
	f.extractCalls++
	f.lastExtractedBody = requestBody
	return sessionsvc.ClientSessionExtractionResult{SessionID: f.extractedSessionID}
}

func (f *fakeSessionManager) GetOrCreateSessionID(_ context.Context, keyID int, messages any, clientSessionID string) string {
	f.getOrCreateCalls++
	f.lastGetOrCreateKeyID = keyID
	f.lastGetOrCreateMsgs = messages
	f.lastGetOrCreateClient = clientSessionID
	return f.generatedSessionID
}

func (f *fakeSessionManager) GetNextRequestSequence(_ context.Context, _ string) int {
	return 1
}

func (f *fakeSessionManager) BindProvider(_ context.Context, _ string, _ int) {}

func (f *fakeSessionManager) UpdateCodexSessionWithPromptCacheKey(_ context.Context, currentSessionID, promptCacheKey string, providerID int) string {
	f.updatedCodexSessionID = currentSessionID
	f.updatedPromptCacheKey = promptCacheKey
	f.updatedProviderID = providerID
	return "codex_" + promptCacheKey
}

func (f *fakeSessionManager) GetConcurrentCount(_ context.Context, _ string) int {
	return f.concurrentCount
}

func (f *fakeSessionManager) IncrementConcurrentCount(_ context.Context, sessionID string) {
	f.incrementCalls = append(f.incrementCalls, sessionID)
}

func (f *fakeSessionManager) DecrementConcurrentCount(_ context.Context, sessionID string) {
	f.decrementCalls = append(f.decrementCalls, sessionID)
}

func newAuthorizedHandler(t *testing.T, sessionManager sessionManager) *Handler {
	t.Helper()

	enabled := true
	return NewHandler(authsvc.NewService(&testKeyRepo{
		key: &model.Key{
			ID:        1,
			UserID:    10,
			Key:       "proxy-key",
			Name:      "key-1",
			IsEnabled: &enabled,
			User: &model.User{
				ID:        10,
				Name:      "tester",
				Role:      "user",
				IsEnabled: &enabled,
			},
		},
	}, testUserRepo{}, ""), sessionManager, nil, nil, nil)
}

type failIfCalledHTTPClient struct {
	t *testing.T
}

func (c failIfCalledHTTPClient) Do(*http.Request) (*http.Response, error) {
	c.t.Fatal("expected upstream not to be called")
	return nil, nil
}

func TestAuthMiddlewareStoresAuthResult(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	handler := NewHandler(authsvc.NewService(&testKeyRepo{
		key: &model.Key{
			ID:        1,
			UserID:    10,
			Key:       "proxy-key",
			Name:      "key-1",
			IsEnabled: &enabled,
			User: &model.User{
				ID:        10,
				Name:      "tester",
				Role:      "user",
				IsEnabled: &enabled,
			},
		},
	}, testUserRepo{}, ""), nil, nil, nil, nil)

	router := gin.New()
	router.GET("/secured", handler.AuthMiddleware(), func(c *gin.Context) {
		result, ok := GetAuthResult(c)
		if !ok || result == nil {
			t.Fatalf("expected auth result in gin context")
		}
		c.JSON(http.StatusOK, gin.H{
			"userId": result.User.ID,
			"apiKey": result.APIKey,
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/secured", nil)
	req.Header.Set("Authorization", "Bearer proxy-key")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if got := resp.Body.String(); got != "{\"apiKey\":\"proxy-key\",\"userId\":10}" {
		t.Fatalf("unexpected response body: %s", got)
	}
}

func TestAuthMiddlewareRejectsInvalidAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(authsvc.NewService(&testKeyRepo{
		err: appErrors.NewNotFoundError("Key"),
	}, testUserRepo{}, ""), nil, nil, nil, nil)

	router := gin.New()
	router.GET("/secured", handler.AuthMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/secured", nil)
	req.Header.Set("x-api-key", "missing")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestSessionLifecycleMiddlewareTracksConcurrentRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
	}
	handler := NewHandler(authsvc.NewService(&testKeyRepo{
		key: &model.Key{
			ID:        1,
			UserID:    10,
			Key:       "proxy-key",
			Name:      "key-1",
			IsEnabled: &enabled,
			User: &model.User{
				ID:        10,
				Name:      "tester",
				Role:      "user",
				IsEnabled: &enabled,
			},
		},
	}, testUserRepo{}, ""), sessionManager, nil, nil, nil)

	router := gin.New()
	group := router.Group("/v1")
	group.Use(handler.AuthMiddleware(), handler.SessionLifecycleMiddleware())
	group.POST("/responses", func(c *gin.Context) {
		sessionID, ok := GetProxySessionID(c)
		if !ok || sessionID != "sess_generated_123" {
			t.Fatalf("expected proxy session id in context, got %q", sessionID)
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", resp.Code, resp.Body.String())
	}
	if sessionManager.extractCalls != 1 || sessionManager.getOrCreateCalls != 1 {
		t.Fatalf("expected session manager lifecycle calls, got extract=%d create=%d", sessionManager.extractCalls, sessionManager.getOrCreateCalls)
	}
	if sessionManager.lastGetOrCreateKeyID != 1 {
		t.Fatalf("expected key id 1, got %d", sessionManager.lastGetOrCreateKeyID)
	}
	if sessionManager.lastGetOrCreateClient != "sess_client_123" {
		t.Fatalf("expected extracted client session id, got %q", sessionManager.lastGetOrCreateClient)
	}
	if len(sessionManager.incrementCalls) != 1 || sessionManager.incrementCalls[0] != "sess_generated_123" {
		t.Fatalf("unexpected increment calls: %+v", sessionManager.incrementCalls)
	}
	if len(sessionManager.decrementCalls) != 1 || sessionManager.decrementCalls[0] != "sess_generated_123" {
		t.Fatalf("unexpected decrement calls: %+v", sessionManager.decrementCalls)
	}
}

func TestMessagesWarmupInterceptSkipsUpstreamWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
	}
	handler := NewHandler(authsvc.NewService(&testKeyRepo{
		key: &model.Key{
			ID:        1,
			UserID:    10,
			Key:       "proxy-key",
			Name:      "key-1",
			IsEnabled: &enabled,
			User: &model.User{
				ID:        10,
				Name:      "tester",
				Role:      "user",
				IsEnabled: &enabled,
			},
		},
	}, testUserRepo{}, ""), sessionManager, &fakeProviderRepo{}, &fakeMessageRequestRepo{}, failIfCalledHTTPClient{t: t}, &fakeProxySystemSettingsStore{
		settings: &model.SystemSettings{ID: 1, InterceptAnthropicWarmupRequests: true},
	})

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(`{"model":"claude-sonnet-4","messages":[{"role":"user","content":[{"type":"text","text":"Warmup","cache_control":{"type":"ephemeral"}}]}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"text":"I'm ready to help you."`) {
		t.Fatalf("expected warmup response payload, got %s", resp.Body.String())
	}
	if got := resp.Header().Get("x-cch-intercepted"); got != "warmup" {
		t.Fatalf("expected warmup intercept header, got %q", got)
	}
}

func TestResponsesProxyUpdatesCodexSessionFromPromptCacheKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
	}
	requestLogs := &fakeMessageRequestRepo{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"resp_123","status":"completed","prompt_cache_key":"019b82ff-08ff-75a3-a203-7e10274fdbd8"}`))
	}))
	defer upstream.Close()

	handler := NewHandler(authsvc.NewService(&testKeyRepo{
		key: &model.Key{
			ID:        1,
			UserID:    10,
			Key:       "proxy-key",
			Name:      "key-1",
			IsEnabled: &enabled,
			User: &model.User{
				ID:        10,
				Name:      "tester",
				Role:      "user",
				IsEnabled: &enabled,
			},
		},
	}, testUserRepo{}, ""), sessionManager, &fakeProviderRepo{providers: []*model.Provider{{
		ID:           99,
		Name:         "codex-upstream",
		URL:          upstream.URL,
		Key:          "provider-secret",
		ProviderType: string(model.ProviderTypeCodex),
		IsEnabled:    &enabled,
	}}}, requestLogs, upstream.Client())

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"role":"user","content":[{"type":"input_text","text":"hello"}]}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.Code, resp.Body.String())
	}
	if sessionManager.updatedCodexSessionID != "sess_generated_123" || sessionManager.updatedPromptCacheKey != "019b82ff-08ff-75a3-a203-7e10274fdbd8" || sessionManager.updatedProviderID != 99 {
		t.Fatalf("expected codex session update to be recorded, got session=%q promptCacheKey=%q provider=%d", sessionManager.updatedCodexSessionID, sessionManager.updatedPromptCacheKey, sessionManager.updatedProviderID)
	}
}

func TestResponsesStreamingProxyUpdatesCodexSessionFromSSEPromptCacheKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: response.created\n"))
		_, _ = w.Write([]byte("data: {\"response\":{\"id\":\"resp_123\",\"prompt_cache_key\":\"019b82ff-08ff-75a3-a203-7e10274fdbd8\"}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	handler := NewHandler(authsvc.NewService(&testKeyRepo{
		key: &model.Key{
			ID:        1,
			UserID:    10,
			Key:       "proxy-key",
			Name:      "key-1",
			IsEnabled: &enabled,
			User: &model.User{
				ID:        10,
				Name:      "tester",
				Role:      "user",
				IsEnabled: &enabled,
			},
		},
	}, testUserRepo{}, ""), sessionManager, &fakeProviderRepo{providers: []*model.Provider{{
		ID:           99,
		Name:         "codex-upstream",
		URL:          upstream.URL,
		Key:          "provider-secret",
		ProviderType: string(model.ProviderTypeCodex),
		IsEnabled:    &enabled,
	}}}, &fakeMessageRequestRepo{}, upstream.Client())

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"role":"user","content":[{"type":"input_text","text":"hello"}]}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if sessionManager.updatedCodexSessionID != "sess_generated_123" || sessionManager.updatedPromptCacheKey != "019b82ff-08ff-75a3-a203-7e10274fdbd8" || sessionManager.updatedProviderID != 99 {
		t.Fatalf("expected SSE codex session update to be recorded, got session=%q promptCacheKey=%q provider=%d", sessionManager.updatedCodexSessionID, sessionManager.updatedPromptCacheKey, sessionManager.updatedProviderID)
	}
	if !strings.Contains(resp.Body.String(), "\"prompt_cache_key\":\"019b82ff-08ff-75a3-a203-7e10274fdbd8\"") {
		t.Fatalf("expected streamed response body to be preserved, got %s", resp.Body.String())
	}
}

func TestSessionLifecycleMiddlewareSkipsModelsRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
	}
	handler := newAuthorizedHandler(t, sessionManager)

	router := gin.New()
	group := router.Group("/v1")
	group.Use(handler.AuthMiddleware(), handler.SessionLifecycleMiddleware())
	group.GET("/models", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer proxy-key")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", resp.Code, resp.Body.String())
	}
	if sessionManager.extractCalls != 0 || sessionManager.getOrCreateCalls != 0 {
		t.Fatalf("expected no session lifecycle calls on models route, got extract=%d create=%d", sessionManager.extractCalls, sessionManager.getOrCreateCalls)
	}
}

func TestSessionLifecycleMiddlewareTracksMessagesAndChatCompletionsRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		path         string
		body         string
		wantMessages any
	}{
		{
			name: "messages route",
			path: "/v1/messages",
			body: `{"messages":[{"role":"user","content":"hello"}]}`,
			wantMessages: []any{
				map[string]any{"role": "user", "content": "hello"},
			},
		},
		{
			name: "chat completions route",
			path: "/v1/chat/completions",
			body: `{"messages":[{"role":"user","content":"hello chat"}]}`,
			wantMessages: []any{
				map[string]any{"role": "user", "content": "hello chat"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sessionManager := &fakeSessionManager{
				extractedSessionID: "sess_client_123",
				generatedSessionID: "sess_generated_123",
			}
			handler := newAuthorizedHandler(t, sessionManager)

			router := gin.New()
			group := router.Group("/v1")
			group.Use(handler.AuthMiddleware(), handler.SessionLifecycleMiddleware())
			group.POST(tc.path[len("/v1"):], func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewBufferString(tc.body))
			req.Header.Set("Authorization", "Bearer proxy-key")
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusNoContent {
				t.Fatalf("expected status 204, got %d: %s", resp.Code, resp.Body.String())
			}
			if len(sessionManager.incrementCalls) != 1 || len(sessionManager.decrementCalls) != 1 {
				t.Fatalf("expected lifecycle calls, got increment=%+v decrement=%+v", sessionManager.incrementCalls, sessionManager.decrementCalls)
			}
			gotMessages, ok := sessionManager.lastGetOrCreateMsgs.([]any)
			if !ok || len(gotMessages) != len(tc.wantMessages.([]any)) {
				t.Fatalf("unexpected messages passed to GetOrCreateSessionID: %#v", sessionManager.lastGetOrCreateMsgs)
			}
		})
	}
}

func TestSessionLifecycleMiddlewareSkipsWhenAuthResultMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
	}
	handler := NewHandler(nil, sessionManager, nil, nil, nil)

	router := gin.New()
	router.POST("/v1/responses", handler.SessionLifecycleMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"content":"hello"}]}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", resp.Code, resp.Body.String())
	}
	if sessionManager.extractCalls != 0 || sessionManager.getOrCreateCalls != 0 || len(sessionManager.incrementCalls) != 0 || len(sessionManager.decrementCalls) != 0 {
		t.Fatalf("expected no session calls when auth result missing, got %+v", sessionManager)
	}
}

func TestSessionLifecycleMiddlewareSkipsWhenAuthKeyMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
	}
	handler := NewHandler(nil, sessionManager, nil, nil, nil)

	router := gin.New()
	router.POST("/v1/responses", func(c *gin.Context) {
		c.Set(authResultContextKey, &authsvc.AuthResult{User: &model.User{ID: 10}, Key: nil})
		c.Next()
	}, handler.SessionLifecycleMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"content":"hello"}]}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", resp.Code, resp.Body.String())
	}
	if sessionManager.extractCalls != 0 || sessionManager.getOrCreateCalls != 0 {
		t.Fatalf("expected no session calls when auth key missing, got extract=%d create=%d", sessionManager.extractCalls, sessionManager.getOrCreateCalls)
	}
}

func TestSessionLifecycleMiddlewareSkipsOnInvalidJSONBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
	}
	handler := newAuthorizedHandler(t, sessionManager)

	router := gin.New()
	group := router.Group("/v1")
	group.Use(handler.AuthMiddleware(), handler.SessionLifecycleMiddleware())
	group.POST("/responses", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", resp.Code, resp.Body.String())
	}
	if sessionManager.extractCalls != 0 || sessionManager.getOrCreateCalls != 0 || len(sessionManager.incrementCalls) != 0 || len(sessionManager.decrementCalls) != 0 {
		t.Fatalf("expected no session calls on invalid json body, got %+v", sessionManager)
	}
}

func TestSessionLifecycleMiddlewareSkipsWhenSessionIDEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "",
	}
	handler := newAuthorizedHandler(t, sessionManager)

	router := gin.New()
	group := router.Group("/v1")
	group.Use(handler.AuthMiddleware(), handler.SessionLifecycleMiddleware())
	group.POST("/responses", func(c *gin.Context) {
		if _, ok := GetProxySessionID(c); ok {
			t.Fatalf("expected no proxy session id in context")
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", resp.Code, resp.Body.String())
	}
	if sessionManager.extractCalls != 1 || sessionManager.getOrCreateCalls != 1 {
		t.Fatalf("expected extract/create calls before empty session skip, got extract=%d create=%d", sessionManager.extractCalls, sessionManager.getOrCreateCalls)
	}
	if len(sessionManager.incrementCalls) != 0 || len(sessionManager.decrementCalls) != 0 {
		t.Fatalf("expected no concurrent count calls when session id empty, got increment=%+v decrement=%+v", sessionManager.incrementCalls, sessionManager.decrementCalls)
	}
}

func TestSessionLifecycleMiddlewareRejectsConcurrentSessionLimitOnKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	keyLimit := 2
	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
		concurrentCount:    2,
	}
	handler := NewHandler(authsvc.NewService(&testKeyRepo{
		key: &model.Key{
			ID:                      1,
			UserID:                  10,
			Key:                     "proxy-key",
			Name:                    "key-1",
			IsEnabled:               &enabled,
			LimitConcurrentSessions: &keyLimit,
			User: &model.User{
				ID:        10,
				Name:      "tester",
				Role:      "user",
				IsEnabled: &enabled,
			},
		},
	}, testUserRepo{}, ""), sessionManager, nil, nil, nil)

	router := gin.New()
	group := router.Group("/v1")
	group.Use(handler.AuthMiddleware(), handler.SessionLifecycleMiddleware())
	group.POST("/responses", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", resp.Code, resp.Body.String())
	}
	if len(sessionManager.incrementCalls) != 0 || len(sessionManager.decrementCalls) != 0 {
		t.Fatalf("expected no concurrent count mutation on rejection, got increment=%+v decrement=%+v", sessionManager.incrementCalls, sessionManager.decrementCalls)
	}
}

func TestSessionLifecycleMiddlewareFallsBackToUserConcurrentLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	userLimit := 1
	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
		concurrentCount:    1,
	}
	handler := NewHandler(authsvc.NewService(&testKeyRepo{
		key: &model.Key{
			ID:        1,
			UserID:    10,
			Key:       "proxy-key",
			Name:      "key-1",
			IsEnabled: &enabled,
			User: &model.User{
				ID:                      10,
				Name:                    "tester",
				Role:                    "user",
				IsEnabled:               &enabled,
				LimitConcurrentSessions: &userLimit,
			},
		},
	}, testUserRepo{}, ""), sessionManager, nil, nil, nil)

	router := gin.New()
	group := router.Group("/v1")
	group.Use(handler.AuthMiddleware(), handler.SessionLifecycleMiddleware())
	group.POST("/responses", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestSessionLifecycleMiddlewareRejectsKeyTotalLimitExceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	limit := udecimal.MustParse("10")
	current := udecimal.MustParse("10")
	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
	}
	handler := NewHandler(authsvc.NewService(&testKeyRepo{
		key: &model.Key{
			ID:            1,
			UserID:        10,
			Key:           "proxy-key",
			Name:          "key-1",
			IsEnabled:     &enabled,
			LimitTotalUSD: &limit,
			User:          &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
		},
	}, testUserRepo{}, ""), sessionManager, nil, nil, nil, &fakeProxyStatisticsStore{keyTotal: current})

	router := gin.New()
	group := router.Group("/v1")
	group.Use(handler.AuthMiddleware(), handler.SessionLifecycleMiddleware())
	group.POST("/responses", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestSessionLifecycleMiddlewareFallsBackToUserTotalLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	userLimit := udecimal.MustParse("5")
	current := udecimal.MustParse("6")
	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
	}
	handler := NewHandler(authsvc.NewService(&testKeyRepo{
		key: &model.Key{
			ID:        1,
			UserID:    10,
			Key:       "proxy-key",
			Name:      "key-1",
			IsEnabled: &enabled,
			User: &model.User{
				ID:            10,
				Name:          "tester",
				Role:          "user",
				IsEnabled:     &enabled,
				LimitTotalUSD: &userLimit,
			},
		},
	}, testUserRepo{}, ""), sessionManager, nil, nil, nil, &fakeProxyStatisticsStore{userTotal: current})

	router := gin.New()
	group := router.Group("/v1")
	group.Use(handler.AuthMiddleware(), handler.SessionLifecycleMiddleware())
	group.POST("/responses", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestSessionLifecycleMiddlewareFailsOpenWhenTotalCostLookupFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	limit := udecimal.MustParse("1")
	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
	}
	handler := NewHandler(authsvc.NewService(&testKeyRepo{
		key: &model.Key{
			ID:            1,
			UserID:        10,
			Key:           "proxy-key",
			Name:          "key-1",
			IsEnabled:     &enabled,
			LimitTotalUSD: &limit,
			User:          &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
		},
	}, testUserRepo{}, ""), sessionManager, nil, nil, nil, &fakeProxyStatisticsStore{err: errors.New("db down")})

	router := gin.New()
	group := router.Group("/v1")
	group.Use(handler.AuthMiddleware(), handler.SessionLifecycleMiddleware())
	group.POST("/responses", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected fail-open 204, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestSessionLifecycleMiddlewareRejectsKey5hLimitExceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	limit := udecimal.MustParse("3")
	current := udecimal.MustParse("3")
	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
	}
	handler := NewHandler(authsvc.NewService(&testKeyRepo{
		key: &model.Key{
			ID:         1,
			UserID:     10,
			Key:        "proxy-key",
			Name:       "key-1",
			IsEnabled:  &enabled,
			Limit5hUSD: &limit,
			User:       &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
		},
	}, testUserRepo{}, ""), sessionManager, nil, nil, nil, &fakeProxyStatisticsStore{key5h: current})

	router := gin.New()
	group := router.Group("/v1")
	group.Use(handler.AuthMiddleware(), handler.SessionLifecycleMiddleware())
	group.POST("/responses", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestSessionLifecycleMiddlewareFallsBackToUser5hLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	userLimit := udecimal.MustParse("2")
	current := udecimal.MustParse("2.5")
	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
	}
	handler := NewHandler(authsvc.NewService(&testKeyRepo{
		key: &model.Key{
			ID:        1,
			UserID:    10,
			Key:       "proxy-key",
			Name:      "key-1",
			IsEnabled: &enabled,
			User: &model.User{
				ID:         10,
				Name:       "tester",
				Role:       "user",
				IsEnabled:  &enabled,
				Limit5hUSD: &userLimit,
			},
		},
	}, testUserRepo{}, ""), sessionManager, nil, nil, nil, &fakeProxyStatisticsStore{user5h: current})

	router := gin.New()
	group := router.Group("/v1")
	group.Use(handler.AuthMiddleware(), handler.SessionLifecycleMiddleware())
	group.POST("/responses", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestSessionLifecycleMiddlewareFailsOpenWhen5hCostLookupFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	limit := udecimal.MustParse("1")
	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
	}
	handler := NewHandler(authsvc.NewService(&testKeyRepo{
		key: &model.Key{
			ID:         1,
			UserID:     10,
			Key:        "proxy-key",
			Name:       "key-1",
			IsEnabled:  &enabled,
			Limit5hUSD: &limit,
			User:       &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
		},
	}, testUserRepo{}, ""), sessionManager, nil, nil, nil, &fakeProxyStatisticsStore{err: errors.New("db down")})

	router := gin.New()
	group := router.Group("/v1")
	group.Use(handler.AuthMiddleware(), handler.SessionLifecycleMiddleware())
	group.POST("/responses", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected fail-open 204, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestSessionLifecycleMiddlewareRejectsKeyDailyLimitExceededFixed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	limit := udecimal.MustParse("8")
	current := udecimal.MustParse("8")
	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
	}
	handler := NewHandler(authsvc.NewService(&testKeyRepo{
		key: &model.Key{
			ID:             1,
			UserID:         10,
			Key:            "proxy-key",
			Name:           "key-1",
			IsEnabled:      &enabled,
			LimitDailyUSD:  &limit,
			DailyResetMode: "fixed",
			DailyResetTime: "00:00",
			User:           &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
		},
	}, testUserRepo{}, ""), sessionManager, nil, nil, nil, &fakeProxySystemSettingsStore{settings: &model.SystemSettings{ID: 1}}, &fakeProxyStatisticsStore{keyDaily: current})

	router := gin.New()
	group := router.Group("/v1")
	group.Use(handler.AuthMiddleware(), handler.SessionLifecycleMiddleware())
	group.POST("/responses", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestSessionLifecycleMiddlewareRejectsUserDailyLimitExceededRolling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	userLimit := udecimal.MustParse("4")
	current := udecimal.MustParse("5")
	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
	}
	handler := NewHandler(authsvc.NewService(&testKeyRepo{
		key: &model.Key{
			ID:        1,
			UserID:    10,
			Key:       "proxy-key",
			Name:      "key-1",
			IsEnabled: &enabled,
			User: &model.User{
				ID:             10,
				Name:           "tester",
				Role:           "user",
				IsEnabled:      &enabled,
				DailyLimitUSD:  &userLimit,
				DailyResetMode: "rolling",
			},
		},
	}, testUserRepo{}, ""), sessionManager, nil, nil, nil, &fakeProxySystemSettingsStore{settings: &model.SystemSettings{ID: 1}}, &fakeProxyStatisticsStore{userDaily: current})

	router := gin.New()
	group := router.Group("/v1")
	group.Use(handler.AuthMiddleware(), handler.SessionLifecycleMiddleware())
	group.POST("/responses", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestSessionLifecycleMiddlewareFailsOpenWhenDailyCostLookupFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	limit := udecimal.MustParse("1")
	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
	}
	handler := NewHandler(authsvc.NewService(&testKeyRepo{
		key: &model.Key{
			ID:            1,
			UserID:        10,
			Key:           "proxy-key",
			Name:          "key-1",
			IsEnabled:     &enabled,
			LimitDailyUSD: &limit,
			User:          &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
		},
	}, testUserRepo{}, ""), sessionManager, nil, nil, nil, &fakeProxySystemSettingsStore{settings: &model.SystemSettings{ID: 1}}, &fakeProxyStatisticsStore{err: errors.New("db down")})

	router := gin.New()
	group := router.Group("/v1")
	group.Use(handler.AuthMiddleware(), handler.SessionLifecycleMiddleware())
	group.POST("/responses", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected fail-open 204, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestResponsesHandlerProxiesRequestToFirstAvailableProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedAuthHeader string
	var capturedPath string
	var capturedBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedAuthHeader = r.Header.Get("Authorization")
		capturedPath = r.URL.Path
		capturedBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"resp_123","status":"completed"}`))
	}))
	defer upstream.Close()

	enabled := true
	costMultiplier := udecimal.MustParse("1.2500")
	requestLogs := &fakeMessageRequestRepo{}
	sessionManager := &fakeSessionManager{
		extractedSessionID: "sess_client_123",
		generatedSessionID: "sess_generated_123",
	}
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User: &model.User{
					ID:        10,
					Name:      "tester",
					Role:      "user",
					IsEnabled: &enabled,
				},
			},
		}, testUserRepo{}, ""),
		sessionManager,
		&fakeProviderRepo{providers: []*model.Provider{
			{
				ID:             99,
				Name:           "codex-upstream",
				URL:            upstream.URL,
				Key:            "provider-secret",
				CostMultiplier: &costMultiplier,
				ProviderType:   string(model.ProviderTypeCodex),
				IsEnabled:      &enabled,
			},
		}},
		requestLogs,
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	reqBody := `{"input":[{"role":"user","content":"hello"}],"model":"gpt-5.4"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(reqBody))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.0.2.1:12345"
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", resp.Code, resp.Body.String())
	}
	if capturedAuthHeader != "Bearer provider-secret" {
		t.Fatalf("expected provider auth header, got %q", capturedAuthHeader)
	}
	if capturedPath != "/v1/responses" {
		t.Fatalf("expected upstream path /v1/responses, got %q", capturedPath)
	}
	if capturedBody != reqBody {
		t.Fatalf("expected upstream request body %q, got %q", reqBody, capturedBody)
	}
	if got := resp.Body.String(); got != `{"id":"resp_123","status":"completed"}` {
		t.Fatalf("unexpected proxied response body: %s", got)
	}
	if len(requestLogs.created) != 1 {
		t.Fatalf("expected one persisted message request, got %d", len(requestLogs.created))
	}
	if requestLogs.created[0].ProviderID != 99 || requestLogs.created[0].UserID != 10 {
		t.Fatalf("unexpected persisted message request: %+v", requestLogs.created[0])
	}
	if requestLogs.created[0].SessionID == nil || *requestLogs.created[0].SessionID != "sess_generated_123" {
		t.Fatalf("expected persisted session id, got %+v", requestLogs.created[0].SessionID)
	}
	if requestLogs.created[0].RequestSequence != 1 {
		t.Fatalf("expected request sequence 1, got %d", requestLogs.created[0].RequestSequence)
	}
	if requestLogs.created[0].CostMultiplier == nil || requestLogs.created[0].CostMultiplier.String() != "1.25" {
		t.Fatalf("expected provider cost multiplier to be persisted, got %+v", requestLogs.created[0].CostMultiplier)
	}
	if requestLogs.created[0].ClientIP == nil || *requestLogs.created[0].ClientIP != "192.0.2.1" {
		t.Fatalf("expected client ip to be persisted, got %+v", requestLogs.created[0].ClientIP)
	}
	if len(requestLogs.created[0].ProviderChain) != 1 || requestLogs.created[0].ProviderChain[0].ID != 99 || requestLogs.created[0].ProviderChain[0].Name != "codex-upstream" {
		t.Fatalf("expected provider chain to capture chosen provider, got %+v", requestLogs.created[0].ProviderChain)
	}
	if requestLogs.created[0].ProviderChain[0].Reason == nil || *requestLogs.created[0].ProviderChain[0].Reason != "initial_selection" {
		t.Fatalf("expected provider chain reason initial_selection, got %+v", requestLogs.created[0].ProviderChain[0])
	}
	if requestLogs.created[0].ProviderChain[0].ProviderType == nil || *requestLogs.created[0].ProviderChain[0].ProviderType != string(model.ProviderTypeCodex) {
		t.Fatalf("expected provider chain providerType codex, got %+v", requestLogs.created[0].ProviderChain[0])
	}
	if requestLogs.created[0].ProviderChain[0].EndpointURL == nil || *requestLogs.created[0].ProviderChain[0].EndpointURL != upstream.URL {
		t.Fatalf("expected provider chain endpointUrl %q, got %+v", upstream.URL, requestLogs.created[0].ProviderChain[0].EndpointURL)
	}
	if requestLogs.created[0].ProviderChain[0].Timestamp == nil || *requestLogs.created[0].ProviderChain[0].Timestamp <= 0 {
		t.Fatalf("expected provider chain timestamp to be recorded, got %+v", requestLogs.created[0].ProviderChain[0].Timestamp)
	}
	if len(requestLogs.updated) != 1 {
		t.Fatalf("expected one terminal update, got %d", len(requestLogs.updated))
	}
	if requestLogs.updated[0].update.StatusCode != http.StatusCreated {
		t.Fatalf("expected terminal status 201, got %d", requestLogs.updated[0].update.StatusCode)
	}
	if len(requestLogs.updated[0].update.ProviderChain) != 1 || requestLogs.updated[0].update.ProviderChain[0].StatusCode == nil || *requestLogs.updated[0].update.ProviderChain[0].StatusCode != http.StatusCreated {
		t.Fatalf("expected terminal provider chain status update, got %+v", requestLogs.updated[0].update.ProviderChain)
	}
}

func TestResponsesHandlerAppliesModelRedirectBeforeForwarding(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"resp_123","status":"completed"}`))
	}))
	defer upstream.Close()

	enabled := true
	requestLogs := &fakeMessageRequestRepo{}
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{
			{
				ID:           99,
				Name:         "codex-upstream",
				URL:          upstream.URL,
				Key:          "provider-secret",
				ProviderType: string(model.ProviderTypeCodex),
				IsEnabled:    &enabled,
				ModelRedirects: model.ProviderModelRedirectRules{
					{MatchType: "prefix", Source: "gpt-5", Target: "gpt-4.1"},
				},
			},
		}},
		requestLogs,
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"role":"user","content":"hello"}],"model":"gpt-5.4"}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(capturedBody, `"model":"gpt-4.1"`) {
		t.Fatalf("expected redirected upstream body, got %s", capturedBody)
	}
	if len(requestLogs.created) != 1 {
		t.Fatalf("expected one persisted request log, got %d", len(requestLogs.created))
	}
	if requestLogs.created[0].Model != "gpt-4.1" {
		t.Fatalf("expected persisted effective model gpt-4.1, got %+v", requestLogs.created[0].Model)
	}
	if requestLogs.created[0].OriginalModel == nil || *requestLogs.created[0].OriginalModel != "gpt-5.4" {
		t.Fatalf("expected persisted original model gpt-5.4, got %+v", requestLogs.created[0].OriginalModel)
	}
}

func TestResponsesHandlerUsesBaseURLWithExistingResponsesPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	enabled := true
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User: &model.User{
					ID:        10,
					Name:      "tester",
					Role:      "user",
					IsEnabled: &enabled,
				},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{
			{
				ID:           99,
				Name:         "codex-upstream",
				URL:          upstream.URL + "/openai/responses",
				Key:          "provider-secret",
				ProviderType: string(model.ProviderTypeCodex),
				IsEnabled:    &enabled,
			},
		}},
		nil,
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if capturedPath != "/openai/responses" {
		t.Fatalf("expected existing responses path to be preserved, got %q", capturedPath)
	}
}

func TestResponsesHandlerPersistsUsageFromUpstreamResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp_123","usage":{"input_tokens":120,"output_tokens":45,"input_tokens_details":{"cached_tokens":30}}}`))
	}))
	defer upstream.Close()

	enabled := true
	requestLogs := &fakeMessageRequestRepo{}
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{{
			ID:           99,
			Name:         "codex-upstream",
			URL:          upstream.URL,
			Key:          "provider-secret",
			ProviderType: string(model.ProviderTypeCodex),
			IsEnabled:    &enabled,
		}}},
		requestLogs,
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"role":"user","content":"hello"}],"model":"gpt-5.4"}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if len(requestLogs.updated) != 1 {
		t.Fatalf("expected one terminal update, got %d", len(requestLogs.updated))
	}
	if requestLogs.updated[0].update.InputTokens == nil || *requestLogs.updated[0].update.InputTokens != 120 {
		t.Fatalf("expected input tokens 120, got %+v", requestLogs.updated[0].update)
	}
	if requestLogs.updated[0].update.OutputTokens == nil || *requestLogs.updated[0].update.OutputTokens != 45 {
		t.Fatalf("expected output tokens 45, got %+v", requestLogs.updated[0].update)
	}
	if requestLogs.updated[0].update.CacheReadInputTokens == nil || *requestLogs.updated[0].update.CacheReadInputTokens != 30 {
		t.Fatalf("expected cached tokens 30, got %+v", requestLogs.updated[0].update)
	}
}

func TestMessagesHandlerPersistsAnthropicUsageFromUpstreamResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_123","usage":{"input_tokens":80,"output_tokens":20,"cache_creation_input_tokens":7,"cache_read_input_tokens":5,"cache_creation":{"ephemeral_5m_input_tokens":3,"ephemeral_1h_input_tokens":4}}}`))
	}))
	defer upstream.Close()

	enabled := true
	requestLogs := &fakeMessageRequestRepo{}
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{{
			ID:           200,
			Name:         "claude-upstream",
			URL:          upstream.URL,
			Key:          "provider-secret",
			ProviderType: string(model.ProviderTypeClaude),
			IsEnabled:    &enabled,
		}}},
		requestLogs,
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(`{"messages":[{"role":"user","content":"hello claude"}],"model":"claude-sonnet-4"}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	update := requestLogs.updated[0].update
	if update.InputTokens == nil || *update.InputTokens != 80 || update.OutputTokens == nil || *update.OutputTokens != 20 {
		t.Fatalf("expected anthropic usage tokens, got %+v", update)
	}
	if update.CacheCreationInputTokens == nil || *update.CacheCreationInputTokens != 7 || update.CacheReadInputTokens == nil || *update.CacheReadInputTokens != 5 {
		t.Fatalf("expected anthropic cache usage, got %+v", update)
	}
	if update.CacheCreation5mInputTokens == nil || *update.CacheCreation5mInputTokens != 3 || update.CacheCreation1hInputTokens == nil || *update.CacheCreation1hInputTokens != 4 {
		t.Fatalf("expected anthropic cache split tokens, got %+v", update)
	}
}

func TestChatCompletionsHandlerPersistsUsageFromUpstreamResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl_123","usage":{"prompt_tokens":70,"completion_tokens":15,"prompt_tokens_details":{"cached_tokens":9}}}`))
	}))
	defer upstream.Close()

	enabled := true
	requestLogs := &fakeMessageRequestRepo{}
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{{
			ID:           100,
			Name:         "openai-compatible",
			URL:          upstream.URL,
			Key:          "provider-secret",
			ProviderType: string(model.ProviderTypeOpenAICompatible),
			IsEnabled:    &enabled,
		}}},
		requestLogs,
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"messages":[{"role":"user","content":"hello chat"}],"model":"gpt-4o-mini"}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	update := requestLogs.updated[0].update
	if update.InputTokens == nil || *update.InputTokens != 70 || update.OutputTokens == nil || *update.OutputTokens != 15 {
		t.Fatalf("expected chat completions usage, got %+v", update)
	}
	if update.CacheReadInputTokens == nil || *update.CacheReadInputTokens != 9 {
		t.Fatalf("expected cached prompt tokens 9, got %+v", update)
	}
}

func TestResponsesHandlerPersistsErrorMessageFromUpstreamJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
	defer upstream.Close()

	enabled := true
	requestLogs := &fakeMessageRequestRepo{}
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{{
			ID:           99,
			Name:         "codex-upstream",
			URL:          upstream.URL,
			Key:          "provider-secret",
			ProviderType: string(model.ProviderTypeCodex),
			IsEnabled:    &enabled,
		}}},
		requestLogs,
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"role":"user","content":"hello"}],"model":"gpt-5.4"}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d: %s", resp.Code, resp.Body.String())
	}
	if len(requestLogs.updated) != 1 || requestLogs.updated[0].update.ErrorMessage == nil || *requestLogs.updated[0].update.ErrorMessage != "rate limited" {
		t.Fatalf("expected persisted upstream json error message, got %+v", requestLogs.updated)
	}
}

func TestResponsesHandlerMapsFake200JSONErrorTo429(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
	defer upstream.Close()

	enabled := true
	requestLogs := &fakeMessageRequestRepo{}
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{{
			ID:           99,
			Name:         "codex-upstream",
			URL:          upstream.URL,
			Key:          "provider-secret",
			ProviderType: string(model.ProviderTypeCodex),
			IsEnabled:    &enabled,
		}}},
		requestLogs,
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"role":"user","content":"hello"}],"model":"gpt-5.4"}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected fake 200 to be remapped to 429, got %d: %s", resp.Code, resp.Body.String())
	}
	if len(requestLogs.updated) != 1 || requestLogs.updated[0].update.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected persisted 429 terminal update, got %+v", requestLogs.updated)
	}
	if requestLogs.updated[0].update.ErrorMessage == nil || *requestLogs.updated[0].update.ErrorMessage != "rate limited" {
		t.Fatalf("expected persisted error message, got %+v", requestLogs.updated[0].update)
	}
}

func TestResponsesHandlerMapsFake200HTMLTo502(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!doctype html><html><body>gateway error</body></html>`))
	}))
	defer upstream.Close()

	enabled := true
	requestLogs := &fakeMessageRequestRepo{}
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{{
			ID:           99,
			Name:         "codex-upstream",
			URL:          upstream.URL,
			Key:          "provider-secret",
			ProviderType: string(model.ProviderTypeCodex),
			IsEnabled:    &enabled,
		}}},
		requestLogs,
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"role":"user","content":"hello"}],"model":"gpt-5.4"}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadGateway {
		t.Fatalf("expected fake 200 html to be remapped to 502, got %d: %s", resp.Code, resp.Body.String())
	}
	if len(requestLogs.updated) != 1 || requestLogs.updated[0].update.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected persisted 502 terminal update, got %+v", requestLogs.updated)
	}
	if requestLogs.updated[0].update.ErrorMessage == nil || !strings.Contains(*requestLogs.updated[0].update.ErrorMessage, "HTML") {
		t.Fatalf("expected persisted html error message, got %+v", requestLogs.updated[0].update)
	}
}

func TestResponsesHandlerMapsFake200PlainTextTo401(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`Unauthorized: invalid api key`))
	}))
	defer upstream.Close()

	enabled := true
	requestLogs := &fakeMessageRequestRepo{}
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{{
			ID:           99,
			Name:         "codex-upstream",
			URL:          upstream.URL,
			Key:          "provider-secret",
			ProviderType: string(model.ProviderTypeCodex),
			IsEnabled:    &enabled,
		}}},
		requestLogs,
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"role":"user","content":"hello"}],"model":"gpt-5.4"}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected fake 200 plain text to be remapped to 401, got %d: %s", resp.Code, resp.Body.String())
	}
	if len(requestLogs.updated) != 1 || requestLogs.updated[0].update.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected persisted 401 terminal update, got %+v", requestLogs.updated)
	}
	if requestLogs.updated[0].update.ErrorMessage == nil || *requestLogs.updated[0].update.ErrorMessage != "Unauthorized: invalid api key" {
		t.Fatalf("expected persisted plain text error message, got %+v", requestLogs.updated[0].update)
	}
}

func TestMessagesCountTokensHandlerPersistsInputTokensFromUpstreamResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"input_tokens":42}`))
	}))
	defer upstream.Close()

	enabled := true
	requestLogs := &fakeMessageRequestRepo{}
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{{
			ID:           202,
			Name:         "claude-upstream",
			URL:          upstream.URL,
			Key:          "provider-secret",
			ProviderType: string(model.ProviderTypeClaude),
			IsEnabled:    &enabled,
		}}},
		requestLogs,
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", bytes.NewBufferString(`{"messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	update := requestLogs.updated[0].update
	if update.InputTokens == nil || *update.InputTokens != 42 {
		t.Fatalf("expected persisted input tokens 42, got %+v", update)
	}
}

func TestResponsesHandlerReturns503WhenNoProviderAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User: &model.User{
					ID:        10,
					Name:      "tester",
					Role:      "user",
					IsEnabled: &enabled,
				},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{}},
		nil,
		http.DefaultClient,
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestResponsesHandlerPrefersLowerCostMultiplierWithinSamePriority(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedAuthHeader string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	enabled := true
	priority := 10
	lowWeight := 1
	highWeight := 100
	cheapCost := udecimal.MustParse("0.8")
	expensiveCost := udecimal.MustParse("1.5")
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{
			{
				ID:             1,
				Name:           "expensive-heavy",
				URL:            upstream.URL,
				Key:            "expensive-secret",
				ProviderType:   string(model.ProviderTypeCodex),
				IsEnabled:      &enabled,
				Priority:       &priority,
				Weight:         &highWeight,
				CostMultiplier: &expensiveCost,
			},
			{
				ID:             2,
				Name:           "cheap-light",
				URL:            upstream.URL,
				Key:            "cheap-secret",
				ProviderType:   string(model.ProviderTypeCodex),
				IsEnabled:      &enabled,
				Priority:       &priority,
				Weight:         &lowWeight,
				CostMultiplier: &cheapCost,
			},
		}},
		nil,
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"role":"user","content":"hello"}],"model":"gpt-5.4"}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if capturedAuthHeader != "Bearer cheap-secret" {
		t.Fatalf("expected lower-cost provider to win within same priority, got auth header %q", capturedAuthHeader)
	}
}

func TestResponsesHandlerSkipsProviderWhenConcurrentLimitReached(t *testing.T) {
	gin.SetMode(gin.TestMode)
	providertrackersvc.SetCountsForTest(map[int]int{1: 1})
	t.Cleanup(providertrackersvc.ResetForTest)

	var capturedAuthHeader string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	enabled := true
	priority := 10
	limit := 1
	cheapCost := udecimal.MustParse("0.8")
	expensiveCost := udecimal.MustParse("1.5")
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{
			{
				ID:                      1,
				Name:                    "cheap-but-full",
				URL:                     upstream.URL,
				Key:                     "cheap-secret",
				ProviderType:            string(model.ProviderTypeCodex),
				IsEnabled:               &enabled,
				Priority:                &priority,
				CostMultiplier:          &cheapCost,
				LimitConcurrentSessions: &limit,
			},
			{
				ID:             2,
				Name:           "fallback-provider",
				URL:            upstream.URL,
				Key:            "fallback-secret",
				ProviderType:   string(model.ProviderTypeCodex),
				IsEnabled:      &enabled,
				Priority:       &priority,
				CostMultiplier: &expensiveCost,
			},
		}},
		nil,
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"role":"user","content":"hello"}],"model":"gpt-5.4"}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if capturedAuthHeader != "Bearer fallback-secret" {
		t.Fatalf("expected fallback provider when first provider concurrent limit is reached, got auth header %q", capturedAuthHeader)
	}
}

func TestResponsesHandlerFailsOpenWhenProviderConcurrentCountUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	providertrackersvc.SetCounterForTest(nil)
	t.Cleanup(providertrackersvc.ResetForTest)

	var capturedAuthHeader string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	enabled := true
	priority := 10
	limit := 1
	cheapCost := udecimal.MustParse("0.8")
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{{
			ID:                      1,
			Name:                    "cheap-provider",
			URL:                     upstream.URL,
			Key:                     "cheap-secret",
			ProviderType:            string(model.ProviderTypeCodex),
			IsEnabled:               &enabled,
			Priority:                &priority,
			CostMultiplier:          &cheapCost,
			LimitConcurrentSessions: &limit,
		}}},
		nil,
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"role":"user","content":"hello"}],"model":"gpt-5.4"}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if capturedAuthHeader != "Bearer cheap-secret" {
		t.Fatalf("expected fail-open to keep provider available when concurrent count unavailable, got auth header %q", capturedAuthHeader)
	}
}

func TestResponsesHandlerSkipsProviderWhenTotalCostLimitReached(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedAuthHeader string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	enabled := true
	priority := 10
	limit := udecimal.MustParse("5")
	current := udecimal.MustParse("5")
	cheapCost := udecimal.MustParse("0.8")
	expensiveCost := udecimal.MustParse("1.5")
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{
			{
				ID:             1,
				Name:           "cheap-but-capped",
				URL:            upstream.URL,
				Key:            "cheap-secret",
				ProviderType:   string(model.ProviderTypeCodex),
				IsEnabled:      &enabled,
				Priority:       &priority,
				CostMultiplier: &cheapCost,
				LimitTotalUSD:  &limit,
			},
			{
				ID:             2,
				Name:           "fallback-provider",
				URL:            upstream.URL,
				Key:            "fallback-secret",
				ProviderType:   string(model.ProviderTypeCodex),
				IsEnabled:      &enabled,
				Priority:       &priority,
				CostMultiplier: &expensiveCost,
			},
		}},
		nil,
		upstream.Client(),
		&fakeProxyStatisticsStore{providerTotal: current},
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"role":"user","content":"hello"}],"model":"gpt-5.4"}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if capturedAuthHeader != "Bearer fallback-secret" {
		t.Fatalf("expected fallback provider when first provider total limit is reached, got auth header %q", capturedAuthHeader)
	}
}

func TestResponsesHandlerFailsOpenWhenProviderTotalLookupFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedAuthHeader string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuthHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	enabled := true
	priority := 10
	limit := udecimal.MustParse("1")
	cheapCost := udecimal.MustParse("0.8")
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{{
			ID:             1,
			Name:           "cheap-provider",
			URL:            upstream.URL,
			Key:            "cheap-secret",
			ProviderType:   string(model.ProviderTypeCodex),
			IsEnabled:      &enabled,
			Priority:       &priority,
			CostMultiplier: &cheapCost,
			LimitTotalUSD:  &limit,
		}}},
		nil,
		upstream.Client(),
		&fakeProxyStatisticsStore{err: errors.New("db down")},
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"role":"user","content":"hello"}],"model":"gpt-5.4"}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if capturedAuthHeader != "Bearer cheap-secret" {
		t.Fatalf("expected fail-open to keep provider available when total lookup fails, got auth header %q", capturedAuthHeader)
	}
}

func TestResponsesHandlerReturns400ForInvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User: &model.User{
					ID:        10,
					Name:      "tester",
					Role:      "user",
					IsEnabled: &enabled,
				},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{
			{
				ID:           99,
				Name:         "codex-upstream",
				URL:          "https://example.com",
				Key:          "provider-secret",
				ProviderType: string(model.ProviderTypeCodex),
				IsEnabled:    &enabled,
			},
		}},
		nil,
		http.DefaultClient,
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestResponsesHandlerReturns504ForUpstreamTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	requestLogs := &fakeMessageRequestRepo{}
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{{
			ID:           99,
			Name:         "codex-upstream",
			URL:          "https://example.com",
			Key:          "provider-secret",
			ProviderType: string(model.ProviderTypeCodex),
			IsEnabled:    &enabled,
		}}},
		requestLogs,
		timeoutHTTPClient{},
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected status 504, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), string(appErrors.CodeProviderTimeout)) {
		t.Fatalf("expected provider_timeout code, got %s", resp.Body.String())
	}
	if len(requestLogs.created) != 1 {
		t.Fatalf("expected one persisted timeout request log, got %d", len(requestLogs.created))
	}
	if len(requestLogs.updated) != 1 || requestLogs.updated[0].update.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("expected persisted 504 terminal update, got %+v", requestLogs.updated)
	}
	if requestLogs.updated[0].update.ErrorMessage == nil || !strings.Contains(*requestLogs.updated[0].update.ErrorMessage, "请求超时") {
		t.Fatalf("expected timeout error message, got %+v", requestLogs.updated[0].update.ErrorMessage)
	}
	if len(requestLogs.updated[0].update.ProviderChain) != 1 || requestLogs.updated[0].update.ProviderChain[0].StatusCode == nil || *requestLogs.updated[0].update.ProviderChain[0].StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("expected timeout status recorded on provider chain, got %+v", requestLogs.updated[0].update.ProviderChain)
	}
	if requestLogs.updated[0].update.ProviderChain[0].ErrorMessage == nil || !strings.Contains(*requestLogs.updated[0].update.ProviderChain[0].ErrorMessage, "请求超时") {
		t.Fatalf("expected timeout error recorded on provider chain, got %+v", requestLogs.updated[0].update.ProviderChain[0].ErrorMessage)
	}
}

func TestResponsesHandlerReturns502ForGenericUpstreamError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	requestLogs := &fakeMessageRequestRepo{}
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{{
			ID:           99,
			Name:         "codex-upstream",
			URL:          "https://example.com",
			Key:          "provider-secret",
			ProviderType: string(model.ProviderTypeCodex),
			IsEnabled:    &enabled,
		}}},
		requestLogs,
		genericFailHTTPClient{},
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), string(appErrors.CodeProviderError)) {
		t.Fatalf("expected provider_error code, got %s", resp.Body.String())
	}
	if len(requestLogs.created) != 1 {
		t.Fatalf("expected one persisted upstream error request log, got %d", len(requestLogs.created))
	}
	if len(requestLogs.updated) != 1 || requestLogs.updated[0].update.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected persisted 502 terminal update, got %+v", requestLogs.updated)
	}
	if requestLogs.updated[0].update.ErrorMessage == nil || !strings.Contains(*requestLogs.updated[0].update.ErrorMessage, "上游 Responses 供应商请求失败") {
		t.Fatalf("expected upstream error message, got %+v", requestLogs.updated[0].update.ErrorMessage)
	}
	if len(requestLogs.updated[0].update.ProviderChain) != 1 || requestLogs.updated[0].update.ProviderChain[0].StatusCode == nil || *requestLogs.updated[0].update.ProviderChain[0].StatusCode != http.StatusBadGateway {
		t.Fatalf("expected upstream error status recorded on provider chain, got %+v", requestLogs.updated[0].update.ProviderChain)
	}
	if requestLogs.updated[0].update.ProviderChain[0].ErrorMessage == nil || !strings.Contains(*requestLogs.updated[0].update.ProviderChain[0].ErrorMessage, "上游 Responses 供应商请求失败") {
		t.Fatalf("expected upstream error message recorded on provider chain, got %+v", requestLogs.updated[0].update.ProviderChain[0].ErrorMessage)
	}
}

func TestChatCompletionsHandlerProxiesRequestToOpenAICompatibleProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedAuthHeader string
	var capturedPath string
	var capturedBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedAuthHeader = r.Header.Get("Authorization")
		capturedPath = r.URL.Path
		capturedBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl_123","object":"chat.completion"}`))
	}))
	defer upstream.Close()

	enabled := true
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User: &model.User{
					ID:        10,
					Name:      "tester",
					Role:      "user",
					IsEnabled: &enabled,
				},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{
			{
				ID:           100,
				Name:         "openai-compatible",
				URL:          upstream.URL,
				Key:          "provider-secret",
				ProviderType: string(model.ProviderTypeOpenAICompatible),
				IsEnabled:    &enabled,
			},
			{
				ID:           101,
				Name:         "codex-ignored",
				URL:          upstream.URL,
				Key:          "provider-secret-2",
				ProviderType: string(model.ProviderTypeCodex),
				IsEnabled:    &enabled,
			},
		}},
		nil,
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	reqBody := `{"messages":[{"role":"user","content":"hello chat"}],"model":"gpt-4o-mini"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(reqBody))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if capturedAuthHeader != "Bearer provider-secret" {
		t.Fatalf("expected provider auth header, got %q", capturedAuthHeader)
	}
	if capturedPath != "/v1/chat/completions" {
		t.Fatalf("expected upstream path /v1/chat/completions, got %q", capturedPath)
	}
	if capturedBody != reqBody {
		t.Fatalf("expected upstream request body %q, got %q", reqBody, capturedBody)
	}
}

func TestResponsesModelsExcludeGeminiProviders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{},
		&fakeProviderRepo{providers: []*model.Provider{
			{ID: 1, Name: "gemini", ProviderType: string(model.ProviderTypeGemini), IsEnabled: &enabled, AllowedModels: model.ExactAllowedModelRules("gemini-2.5-pro")},
			{ID: 2, Name: "gemini-cli", ProviderType: string(model.ProviderTypeGeminiCli), IsEnabled: &enabled, AllowedModels: model.ExactAllowedModelRules("gemini-cli-model")},
		}},
		nil,
		http.DefaultClient,
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodGet, "/v1/responses/models", nil)
	req.Header.Set("Authorization", "Bearer proxy-key")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if strings.Contains(resp.Body.String(), "gemini-2.5-pro") || strings.Contains(resp.Body.String(), "gemini-cli-model") {
		t.Fatalf("expected gemini models to be excluded from responses catalog, got %s", resp.Body.String())
	}
}

func TestChatCompletionsModelsExcludeGeminiProviders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{},
		&fakeProviderRepo{providers: []*model.Provider{
			{ID: 1, Name: "gemini", ProviderType: string(model.ProviderTypeGemini), IsEnabled: &enabled, AllowedModels: model.ExactAllowedModelRules("gemini-2.5-pro")},
			{ID: 2, Name: "gemini-cli", ProviderType: string(model.ProviderTypeGeminiCli), IsEnabled: &enabled, AllowedModels: model.ExactAllowedModelRules("gemini-cli-model")},
		}},
		nil,
		http.DefaultClient,
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions/models", nil)
	req.Header.Set("Authorization", "Bearer proxy-key")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if strings.Contains(resp.Body.String(), "gemini-2.5-pro") || strings.Contains(resp.Body.String(), "gemini-cli-model") {
		t.Fatalf("expected gemini models to be excluded from chat completions catalog, got %s", resp.Body.String())
	}
}

func TestApplyProviderAuthHeadersUsesXGoogAPIKeyForGeminiProvider(t *testing.T) {
	headers := make(http.Header)
	applyProviderAuthHeaders(headers, &model.Provider{
		Key:          "gemini-secret",
		ProviderType: string(model.ProviderTypeGemini),
	}, proxyEndpointResponses)

	if headers.Get("Authorization") != "" {
		t.Fatalf("expected Authorization to be cleared for gemini provider, got %q", headers.Get("Authorization"))
	}
	if headers.Get("x-goog-api-key") != "gemini-secret" {
		t.Fatalf("expected x-goog-api-key gemini-secret, got %q", headers.Get("x-goog-api-key"))
	}
}

func TestApplyProviderAuthHeadersUsesXGoogAPIKeyForGeminiCLIProvider(t *testing.T) {
	headers := make(http.Header)
	applyProviderAuthHeaders(headers, &model.Provider{
		Key:          "gemini-cli-secret",
		ProviderType: string(model.ProviderTypeGeminiCli),
	}, proxyEndpointChatCompletions)

	if headers.Get("Authorization") != "" {
		t.Fatalf("expected Authorization to be cleared for gemini-cli provider, got %q", headers.Get("Authorization"))
	}
	if headers.Get("x-goog-api-key") != "gemini-cli-secret" {
		t.Fatalf("expected x-goog-api-key gemini-cli-secret, got %q", headers.Get("x-goog-api-key"))
	}
}

func TestMessagesHandlerProxiesRequestToClaudeProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedAuthHeader string
	var capturedAPIKey string
	var capturedPath string
	var capturedBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedAuthHeader = r.Header.Get("Authorization")
		capturedAPIKey = r.Header.Get("x-api-key")
		capturedPath = r.URL.Path
		capturedBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_123","type":"message"}`))
	}))
	defer upstream.Close()

	enabled := true
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User: &model.User{
					ID:        10,
					Name:      "tester",
					Role:      "user",
					IsEnabled: &enabled,
				},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{
			{
				ID:           200,
				Name:         "claude-upstream",
				URL:          upstream.URL,
				Key:          "provider-secret",
				ProviderType: string(model.ProviderTypeClaude),
				IsEnabled:    &enabled,
			},
		}},
		nil,
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	reqBody := `{"messages":[{"role":"user","content":"hello claude"}],"model":"claude-sonnet-4"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(reqBody))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if capturedAuthHeader != "Bearer provider-secret" {
		t.Fatalf("expected provider auth header, got %q", capturedAuthHeader)
	}
	if capturedAPIKey != "provider-secret" {
		t.Fatalf("expected x-api-key provider-secret, got %q", capturedAPIKey)
	}
	if capturedPath != "/v1/messages" {
		t.Fatalf("expected upstream path /v1/messages, got %q", capturedPath)
	}
	if capturedBody != reqBody {
		t.Fatalf("expected upstream request body %q, got %q", reqBody, capturedBody)
	}
}

func TestMessagesHandlerDropsXAPIKeyForClaudeAuthProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedAPIKey string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAPIKey = r.Header.Get("x-api-key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_123","type":"message"}`))
	}))
	defer upstream.Close()

	enabled := true
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User: &model.User{
					ID:        10,
					Name:      "tester",
					Role:      "user",
					IsEnabled: &enabled,
				},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{
			{
				ID:           201,
				Name:         "claude-auth-upstream",
				URL:          upstream.URL,
				Key:          "provider-secret",
				ProviderType: string(model.ProviderTypeClaudeAuth),
				IsEnabled:    &enabled,
			},
		}},
		nil,
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(`{"messages":[{"role":"user","content":"hello claude"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if capturedAPIKey != "" {
		t.Fatalf("expected x-api-key to be dropped for claude-auth provider, got %q", capturedAPIKey)
	}
}

func TestMessagesHandlerPersistsMessageRequestBestEffort(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_123","type":"message"}`))
	}))
	defer upstream.Close()

	enabled := true
	requestLogs := &fakeMessageRequestRepo{}
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{{
			ID:           200,
			Name:         "claude-upstream",
			URL:          upstream.URL,
			Key:          "provider-secret",
			ProviderType: string(model.ProviderTypeClaude),
			IsEnabled:    &enabled,
		}}},
		requestLogs,
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(`{"messages":[{"role":"user","content":"hello claude"}],"model":"claude-sonnet-4"}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if len(requestLogs.created) != 1 {
		t.Fatalf("expected one message request log, got %d", len(requestLogs.created))
	}
	if requestLogs.created[0].Endpoint == nil || *requestLogs.created[0].Endpoint != "/v1/messages" {
		t.Fatalf("expected /v1/messages endpoint log, got %+v", requestLogs.created[0].Endpoint)
	}
	if requestLogs.created[0].ApiType == nil || *requestLogs.created[0].ApiType != "claude" {
		t.Fatalf("expected claude api type, got %+v", requestLogs.created[0].ApiType)
	}
	if len(requestLogs.updated) != 1 || requestLogs.updated[0].update.StatusCode != http.StatusOK {
		t.Fatalf("expected one terminal update for messages path, got %+v", requestLogs.updated)
	}
}

func TestResponsesHandlerIgnoresMessageRequestPersistenceFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"resp_123","status":"completed"}`))
	}))
	defer upstream.Close()

	enabled := true
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User:      &model.User{ID: 10, Name: "tester", Role: "user", IsEnabled: &enabled},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{{
			ID:           99,
			Name:         "codex-upstream",
			URL:          upstream.URL,
			Key:          "provider-secret",
			ProviderType: string(model.ProviderTypeCodex),
			IsEnabled:    &enabled,
		}}},
		&fakeMessageRequestRepo{err: errors.New("db down")},
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"input":[{"role":"user","content":"hello"}],"model":"gpt-5.4"}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status 201 despite persistence failure, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestMessagesCountTokensHandlerProxiesRequestToClaudeProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var capturedPath string
	var capturedAPIKey string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAPIKey = r.Header.Get("x-api-key")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"input_tokens":42}`))
	}))
	defer upstream.Close()

	enabled := true
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User: &model.User{
					ID:        10,
					Name:      "tester",
					Role:      "user",
					IsEnabled: &enabled,
				},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{generatedSessionID: "sess_generated_123"},
		&fakeProviderRepo{providers: []*model.Provider{{
			ID:           202,
			Name:         "claude-upstream",
			URL:          upstream.URL + "/v1/messages",
			Key:          "provider-secret",
			ProviderType: string(model.ProviderTypeClaude),
			IsEnabled:    &enabled,
		}}},
		nil,
		upstream.Client(),
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	req := httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", bytes.NewBufferString(`{"messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if capturedPath != "/v1/messages/count_tokens" {
		t.Fatalf("expected upstream path /v1/messages/count_tokens, got %q", capturedPath)
	}
	if capturedAPIKey != "provider-secret" {
		t.Fatalf("expected x-api-key provider-secret, got %q", capturedAPIKey)
	}
	if got := resp.Body.String(); got != `{"input_tokens":42}` {
		t.Fatalf("unexpected proxied response body: %s", got)
	}
}

func TestModelsHandlerAggregatesFilteredModels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	handler := NewHandler(
		authsvc.NewService(&testKeyRepo{
			key: &model.Key{
				ID:        1,
				UserID:    10,
				Key:       "proxy-key",
				Name:      "key-1",
				IsEnabled: &enabled,
				User: &model.User{
					ID:        10,
					Name:      "tester",
					Role:      "user",
					IsEnabled: &enabled,
				},
			},
		}, testUserRepo{}, ""),
		&fakeSessionManager{},
		&fakeProviderRepo{providers: []*model.Provider{
			{ID: 1, Name: "claude", ProviderType: string(model.ProviderTypeClaude), IsEnabled: &enabled, AllowedModels: model.ExactAllowedModelRules("claude-sonnet-4", "claude-opus-4")},
			{ID: 2, Name: "codex", ProviderType: string(model.ProviderTypeCodex), IsEnabled: &enabled, AllowedModels: model.ExactAllowedModelRules("gpt-5.4", "claude-sonnet-4")},
			{ID: 3, Name: "openai", ProviderType: string(model.ProviderTypeOpenAICompatible), IsEnabled: &enabled, AllowedModels: model.ExactAllowedModelRules("gpt-4o-mini")},
		}},
		nil,
		http.DefaultClient,
	)

	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))

	tests := []struct {
		path         string
		wantContains []string
		wantNot      []string
	}{
		{path: "/v1/models", wantContains: []string{"claude-sonnet-4", "claude-opus-4", "gpt-5.4", "gpt-4o-mini"}},
		{path: "/v1/responses/models", wantContains: []string{"gpt-5.4", "claude-sonnet-4"}, wantNot: []string{"claude-opus-4", "gpt-4o-mini"}},
		{path: "/v1/chat/completions/models", wantContains: []string{"gpt-4o-mini"}, wantNot: []string{"gpt-5.4", "claude-opus-4", "claude-sonnet-4"}},
		{path: "/v1/chat/models", wantContains: []string{"gpt-4o-mini"}, wantNot: []string{"gpt-5.4", "claude-opus-4"}},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("Authorization", "Bearer proxy-key")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
			}
			for _, want := range tc.wantContains {
				if !strings.Contains(resp.Body.String(), want) {
					t.Fatalf("expected body %s to contain %q", resp.Body.String(), want)
				}
			}
			for _, forbidden := range tc.wantNot {
				if strings.Contains(resp.Body.String(), forbidden) {
					t.Fatalf("expected body %s to not contain %q", resp.Body.String(), forbidden)
				}
			}
		})
	}
}
