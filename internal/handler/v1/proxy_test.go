package v1

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
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
	}, testUserRepo{}, ""), sessionManager)
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
	}, testUserRepo{}, ""), nil)

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
	}, testUserRepo{}, ""), nil)

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
	}, testUserRepo{}, ""), sessionManager)

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
	handler := NewHandler(nil, sessionManager)

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
	handler := NewHandler(nil, sessionManager)

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
