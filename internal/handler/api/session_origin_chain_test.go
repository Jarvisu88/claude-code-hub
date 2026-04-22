package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

type fakeSessionOriginStore struct {
	sessionID string
	log       *model.MessageRequest
}

func (f *fakeSessionOriginStore) FindLatestBySessionID(_ context.Context, sessionID string) (*model.MessageRequest, error) {
	f.sessionID = sessionID
	return f.log, nil
}

func TestSessionOriginChainActionReturnsProviderChain(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	store := &fakeSessionOriginStore{log: &model.MessageRequest{
		ProviderChain: []model.ProviderChainItem{{ID: 1, Name: "provider-a"}},
	}}
	router := gin.New()
	NewSessionOriginChainActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		store,
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodGet, "/api/actions/session-origin-chain?sessionId=sess_123", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if store.sessionID != "sess_123" {
		t.Fatalf("expected session id sess_123, got %q", store.sessionID)
	}
	if !strings.Contains(resp.Body.String(), "provider-a") {
		t.Fatalf("expected provider chain payload, got %s", resp.Body.String())
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/actions/session-origin-chain/getSessionOriginChain", strings.NewReader(`{"sessionId":"sess_123"}`))
	postReq.Header.Set("Authorization", "Bearer admin-token")
	postReq.Header.Set("Content-Type", "application/json")
	postResp := httptest.NewRecorder()
	router.ServeHTTP(postResp, postReq)
	if postResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", postResp.Code, postResp.Body.String())
	}
	if !strings.Contains(postResp.Body.String(), "provider-a") {
		t.Fatalf("expected action-style provider chain payload, got %s", postResp.Body.String())
	}
}

func TestSessionOriginChainActionRejectsMissingSessionID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	router := gin.New()
	NewSessionOriginChainActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		&fakeSessionOriginStore{},
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodGet, "/api/actions/session-origin-chain", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestSessionOriginChainActionReturnsNullWhenSessionMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	enabled := true
	router := gin.New()
	NewSessionOriginChainActionHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		&fakeSessionOriginStore{log: nil},
	).RegisterRoutes(router.Group("/api/actions"))

	req := httptest.NewRequest(http.MethodGet, "/api/actions/session-origin-chain?sessionId=missing", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "\"data\":null") {
		t.Fatalf("expected null payload, got %s", resp.Body.String())
	}
}
