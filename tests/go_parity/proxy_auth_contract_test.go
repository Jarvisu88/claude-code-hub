package go_parity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v1 "github.com/ding113/claude-code-hub/internal/handler/v1"
	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

type fixtureCase struct {
	Name                    string            `json:"name"`
	Method                  string            `json:"method"`
	Path                    string            `json:"path"`
	Headers                 map[string]string `json:"headers"`
	ExpectedStatus          int               `json:"expected_status"`
	ExpectedErrorCode       string            `json:"expected_error_code"`
	ExpectedMessageContains string            `json:"expected_message_contains"`
	NodeEvidence            string            `json:"node_evidence"`
}

type responseEnvelope struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
}

type parityKeyRepo struct {
	keys map[string]*model.Key
}

func (r *parityKeyRepo) GetByKeyWithUser(_ context.Context, key string) (*model.Key, error) {
	if result, ok := r.keys[key]; ok {
		return result, nil
	}
	return nil, appErrors.NewNotFoundError("Key")
}

type parityUserRepo struct{}

func (parityUserRepo) MarkUserExpired(_ context.Context, _ int) (bool, error) {
	return true, nil
}

func TestProxyAuthParityCases(t *testing.T) {
	fixtures := loadFixtures(t)
	router := buildParityRouter(t)

	for _, tc := range fixtures {
		t.Run(tc.Name, func(t *testing.T) {
			req := httptest.NewRequest(tc.Method, tc.Path, nil)
			for key, value := range tc.Headers {
				req.Header.Set(key, value)
			}

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != tc.ExpectedStatus {
				t.Fatalf("node evidence %s: expected status %d, got %d, body=%s", tc.NodeEvidence, tc.ExpectedStatus, resp.Code, resp.Body.String())
			}

			if tc.ExpectedStatus == http.StatusNotImplemented {
				return
			}

			var envelope responseEnvelope
			if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("node evidence %s: failed to decode response: %v; body=%s", tc.NodeEvidence, err, resp.Body.String())
			}
			if envelope.Error.Code != tc.ExpectedErrorCode {
				t.Fatalf("node evidence %s: expected error code %q, got %q", tc.NodeEvidence, tc.ExpectedErrorCode, envelope.Error.Code)
			}
			if tc.ExpectedMessageContains != "" && !contains(envelope.Error.Message, tc.ExpectedMessageContains) {
				t.Fatalf("node evidence %s: expected message %q to contain %q", tc.NodeEvidence, envelope.Error.Message, tc.ExpectedMessageContains)
			}
		})
	}
}

func buildParityRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	enabled := true
	disabled := false
	expiredAt := time.Date(2026, 4, 20, 8, 0, 0, 0, time.UTC)

	svc := authsvc.NewService(&parityKeyRepo{keys: map[string]*model.Key{
		"gemini-key": {
			ID:        1,
			UserID:    100,
			Key:       "gemini-key",
			Name:      "gemini",
			IsEnabled: &enabled,
			User: &model.User{
				ID:        100,
				Name:      "gemini-user",
				Role:      "user",
				IsEnabled: &enabled,
			},
		},
		"disabled-user-key": {
			ID:        2,
			UserID:    101,
			Key:       "disabled-user-key",
			Name:      "disabled",
			IsEnabled: &enabled,
			User: &model.User{
				ID:        101,
				Name:      "disabled-user",
				Role:      "user",
				IsEnabled: &disabled,
			},
		},
		"expired-user-key": {
			ID:        3,
			UserID:    102,
			Key:       "expired-user-key",
			Name:      "expired",
			IsEnabled: &enabled,
			User: &model.User{
				ID:        102,
				Name:      "expired-user",
				Role:      "user",
				IsEnabled: &enabled,
				ExpiresAt: &expiredAt,
			},
		},
	}}, parityUserRepo{}, "")

	handler := v1.NewHandler(svc, nil, nil, nil, nil)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/v1"))
	return router
}

func loadFixtures(t *testing.T) []fixtureCase {
	t.Helper()
	path := filepath.Join("..", "go-parity", "fixtures", "proxy-auth-cases.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture file %s: %v", path, err)
	}

	var cases []fixtureCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to decode fixture file %s: %v", path, err)
	}
	return cases
}

func contains(value, sub string) bool {
	return strings.Contains(value, sub)
}
