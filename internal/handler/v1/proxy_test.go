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

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	sessionsvc "github.com/ding113/claude-code-hub/internal/service/session"
	"github.com/gin-gonic/gin"
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
		id          int
		statusCode  int
		durationMs  int
		errorString *string
	}
	err error
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

func (f *fakeMessageRequestRepo) UpdateTerminal(_ context.Context, id int, statusCode int, durationMs int, errorMessage *string) error {
	if f.err != nil {
		return f.err
	}
	f.updated = append(f.updated, struct {
		id          int
		statusCode  int
		durationMs  int
		errorString *string
	}{id: id, statusCode: statusCode, durationMs: durationMs, errorString: errorMessage})
	return nil
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
				ID:           99,
				Name:         "codex-upstream",
				URL:          upstream.URL,
				Key:          "provider-secret",
				ProviderType: string(model.ProviderTypeCodex),
				IsEnabled:    &enabled,
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
	if len(requestLogs.created[0].ProviderChain) != 1 || requestLogs.created[0].ProviderChain[0].ID != 99 || requestLogs.created[0].ProviderChain[0].Name != "codex-upstream" {
		t.Fatalf("expected provider chain to capture chosen provider, got %+v", requestLogs.created[0].ProviderChain)
	}
	if len(requestLogs.updated) != 1 {
		t.Fatalf("expected one terminal update, got %d", len(requestLogs.updated))
	}
	if requestLogs.updated[0].statusCode != http.StatusCreated {
		t.Fatalf("expected terminal status 201, got %d", requestLogs.updated[0].statusCode)
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
	if len(requestLogs.updated) != 1 || requestLogs.updated[0].statusCode != http.StatusGatewayTimeout {
		t.Fatalf("expected persisted 504 terminal update, got %+v", requestLogs.updated)
	}
	if requestLogs.updated[0].errorString == nil || !strings.Contains(*requestLogs.updated[0].errorString, "请求超时") {
		t.Fatalf("expected timeout error message, got %+v", requestLogs.updated[0].errorString)
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
	if len(requestLogs.updated) != 1 || requestLogs.updated[0].statusCode != http.StatusBadGateway {
		t.Fatalf("expected persisted 502 terminal update, got %+v", requestLogs.updated)
	}
	if requestLogs.updated[0].errorString == nil || !strings.Contains(*requestLogs.updated[0].errorString, "上游 Responses 供应商请求失败") {
		t.Fatalf("expected upstream error message, got %+v", requestLogs.updated[0].errorString)
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
	if len(requestLogs.updated) != 1 || requestLogs.updated[0].statusCode != http.StatusOK {
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
			{ID: 1, Name: "claude", ProviderType: string(model.ProviderTypeClaude), IsEnabled: &enabled, AllowedModels: []string{"claude-sonnet-4", "claude-opus-4"}},
			{ID: 2, Name: "codex", ProviderType: string(model.ProviderTypeCodex), IsEnabled: &enabled, AllowedModels: []string{"gpt-5.4", "claude-sonnet-4"}},
			{ID: 3, Name: "openai", ProviderType: string(model.ProviderTypeOpenAICompatible), IsEnabled: &enabled, AllowedModels: []string{"gpt-4o-mini"}},
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
		{path: "/v1/responses/models", wantContains: []string{"gpt-5.4", "claude-sonnet-4", "gpt-4o-mini"}, wantNot: []string{"claude-opus-4"}},
		{path: "/v1/chat/completions/models", wantContains: []string{"gpt-4o-mini"}, wantNot: []string{"gpt-5.4", "claude-opus-4"}},
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
