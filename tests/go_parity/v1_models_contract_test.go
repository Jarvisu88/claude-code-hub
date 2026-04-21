package go_parity

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type v1ModelsFixtureCase struct {
	Name              string            `json:"name"`
	Method            string            `json:"method"`
	Path              string            `json:"path"`
	Headers           map[string]string `json:"headers"`
	ExpectedStatus    int               `json:"expected_status"`
	ExpectedErrorCode string            `json:"expected_error_code"`
	ExpectAuthPass    bool              `json:"expect_auth_pass"`
	NodeEvidence      string            `json:"node_evidence"`
}

func TestV1ModelsContractCases(t *testing.T) {
	fixtures := loadV1ModelsFixtures(t)
	router := buildParityRouter(t)

	for _, tc := range fixtures {
		t.Run(tc.Name, func(t *testing.T) {
			req := httptest.NewRequest(tc.Method, tc.Path, nil)
			for key, value := range tc.Headers {
				req.Header.Set(key, value)
			}

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if tc.ExpectAuthPass {
				assertAuthedModelsResponse(t, tc, resp)
				return
			}

			if resp.Code != tc.ExpectedStatus {
				t.Fatalf("node evidence %s: expected status %d, got %d, body=%s", tc.NodeEvidence, tc.ExpectedStatus, resp.Code, resp.Body.String())
			}

			var envelope responseEnvelope
			if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("node evidence %s: failed to decode auth error response: %v; body=%s", tc.NodeEvidence, err, resp.Body.String())
			}
			if envelope.Error.Code != tc.ExpectedErrorCode {
				t.Fatalf("node evidence %s: expected error code %q, got %q", tc.NodeEvidence, tc.ExpectedErrorCode, envelope.Error.Code)
			}
		})
	}
}

func loadV1ModelsFixtures(t *testing.T) []v1ModelsFixtureCase {
	t.Helper()

	path := filepath.Join("..", "go-parity", "fixtures", "v1-models-contract-cases.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture file %s: %v", path, err)
	}

	var cases []v1ModelsFixtureCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to decode fixture file %s: %v", path, err)
	}

	return cases
}

func assertAuthedModelsResponse(t *testing.T, tc v1ModelsFixtureCase, resp *httptest.ResponseRecorder) {
	t.Helper()

	if resp.Code == http.StatusUnauthorized || resp.Code == http.StatusForbidden {
		t.Fatalf("node evidence %s: expected auth to pass through /v1/models, got status %d body=%s", tc.NodeEvidence, resp.Code, resp.Body.String())
	}

	contentType := resp.Header().Get("Content-Type")
	if contentType != "" && !strings.Contains(contentType, "application/json") {
		t.Fatalf("node evidence %s: expected JSON response, got Content-Type %q", tc.NodeEvidence, contentType)
	}

	var payload any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("node evidence %s: failed to decode JSON response: %v; body=%s", tc.NodeEvidence, err, resp.Body.String())
	}

	switch resp.Code {
	case http.StatusOK:
		switch decoded := payload.(type) {
		case map[string]any:
			if data, ok := decoded["data"]; ok {
				if _, ok := data.([]any); !ok {
					t.Fatalf("node evidence %s: expected data field to be an array, got %T", tc.NodeEvidence, data)
				}
				return
			}
			if models, ok := decoded["models"]; ok {
				if _, ok := models.([]any); !ok {
					t.Fatalf("node evidence %s: expected models field to be an array, got %T", tc.NodeEvidence, models)
				}
				return
			}
			t.Fatalf("node evidence %s: expected 200 response to expose a top-level data or models field, body=%s", tc.NodeEvidence, resp.Body.String())
		case []any:
			// Accept a direct array to avoid over-constraining the implementation lane before parity is fully locked.
		default:
			t.Fatalf("node evidence %s: expected 200 response to decode as object or array, got %T", tc.NodeEvidence, payload)
		}
	case http.StatusNotImplemented:
		decoded, ok := payload.(map[string]any)
		if !ok {
			t.Fatalf("node evidence %s: expected 501 response to decode as object, got %T", tc.NodeEvidence, payload)
		}
		errorBody, ok := decoded["error"].(map[string]any)
		if !ok {
			t.Fatalf("node evidence %s: expected 501 response to include error object, body=%s", tc.NodeEvidence, resp.Body.String())
		}
		if got := fmt.Sprint(errorBody["type"]); got != "not_implemented" {
			t.Fatalf("node evidence %s: expected 501 error.type=not_implemented, got %q", tc.NodeEvidence, got)
		}
		if got := strings.TrimSpace(fmt.Sprint(errorBody["message"])); got == "" {
			t.Fatalf("node evidence %s: expected 501 error.message to be non-empty", tc.NodeEvidence)
		}
	default:
		t.Fatalf("node evidence %s: expected /v1/models authenticated response status to be 200 or 501 during this slice, got %d body=%s", tc.NodeEvidence, resp.Code, resp.Body.String())
	}
}
