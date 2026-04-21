package v1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
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
	}, testUserRepo{}, ""))

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
	}, testUserRepo{}, ""))

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
