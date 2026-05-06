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
	endpointprobesvc "github.com/ding113/claude-code-hub/internal/service/endpointprobe"
	"github.com/gin-gonic/gin"
)

func TestAvailabilityProbeAllRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	enabled := true
	router := gin.New()

	NewAvailabilityProbeAllHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeDashboardProviderStore{},
		nil,
	).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/availability/probe-all", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"ok":true`) {
		t.Fatalf("expected probe-all payload, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestAvailabilityProbeAllRecordsProbeStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	endpointprobesvc.ResetForTest()
	defer endpointprobesvc.ResetForTest()

	enabled := true
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	router := gin.New()
	handler := NewAvailabilityProbeAllHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeDashboardProviderStore{providers: []*model.Provider{{ID: 1, Name: "provider-a", URL: upstream.URL, IsEnabled: &enabled, CreatedAt: time.Now(), UpdatedAt: time.Now()}}},
		upstream.Client(),
	)
	handler.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/availability/probe-all", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	status := endpointprobesvc.GetStatus(1)
	if status.LastProbeOk == nil || !*status.LastProbeOk || status.LastProbeStatusCode == nil || *status.LastProbeStatusCode != 200 {
		t.Fatalf("expected stored probe status, got %+v", status)
	}
	logs := endpointprobesvc.ListLogs(1, 10, 0)
	if len(logs) != 1 || !logs[0].OK {
		t.Fatalf("expected stored probe log, got %+v", logs)
	}
}

type fakeProbeAllEndpointStore struct {
	endpoints []*model.ProviderEndpoint
	updatedID int
	updated   *model.ProviderEndpointProbeLog
}

func (f *fakeProbeAllEndpointStore) List(_ context.Context, _ *repository.ListOptions) ([]*model.ProviderEndpoint, error) {
	return f.endpoints, nil
}

func (f *fakeProbeAllEndpointStore) UpdateProbeSnapshot(_ context.Context, id int, log *model.ProviderEndpointProbeLog) (*model.ProviderEndpoint, error) {
	f.updatedID = id
	f.updated = log
	for _, endpoint := range f.endpoints {
		if endpoint != nil && endpoint.ID == id {
			if log != nil {
				endpoint.LastProbedAt = &log.CreatedAt
				endpoint.LastProbeOk = &log.Ok
				endpoint.LastProbeStatusCode = log.StatusCode
				endpoint.LastProbeLatencyMs = log.LatencyMs
				endpoint.LastProbeErrorType = log.ErrorType
				endpoint.LastProbeErrorMessage = log.ErrorMessage
			}
			return endpoint, nil
		}
	}
	return nil, nil
}

type fakeProbeAllProbeLogStore struct {
	created []*model.ProviderEndpointProbeLog
}

func (f *fakeProbeAllProbeLogStore) Create(_ context.Context, log *model.ProviderEndpointProbeLog) (*model.ProviderEndpointProbeLog, error) {
	if log != nil {
		copy := *log
		copy.ID = len(f.created) + 1
		f.created = append(f.created, &copy)
		return &copy, nil
	}
	return nil, nil
}

func TestAvailabilityProbeAllUsesProviderEndpointsAndPersistsLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	endpointprobesvc.ResetForTest()
	defer endpointprobesvc.ResetForTest()

	enabled := true
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Fatalf("expected HEAD probe, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	endpointStore := &fakeProbeAllEndpointStore{endpoints: []*model.ProviderEndpoint{
		{ID: 41, VendorID: 7, ProviderType: "claude", URL: upstream.URL, IsEnabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: 42, VendorID: 7, ProviderType: "claude", URL: "http://disabled.invalid", IsEnabled: false, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}}
	logStore := &fakeProbeAllProbeLogStore{}

	router := gin.New()
	handler := NewAvailabilityProbeAllHandler(
		fakeAdminAuth{result: &authsvc.AuthResult{
			IsAdmin: true,
			User:    &model.User{ID: -1, Name: "admin", Role: "admin", IsEnabled: &enabled},
			Key:     &model.Key{ID: -1, Key: "admin-token", Name: "ADMIN_TOKEN", IsEnabled: &enabled},
			APIKey:  "admin-token",
		}},
		fakeDashboardProviderStore{providers: []*model.Provider{{ID: 1, Name: "provider fallback should not run", URL: "http://provider.invalid", IsEnabled: &enabled}}},
		upstream.Client(),
		endpointStore,
		logStore,
	)
	handler.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/availability/probe-all", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"count":1`) {
		t.Fatalf("expected one active endpoint probed, got %s", resp.Body.String())
	}
	if len(logStore.created) != 1 || logStore.created[0].EndpointID != 41 || !logStore.created[0].Ok || logStore.created[0].StatusCode == nil || *logStore.created[0].StatusCode != http.StatusNoContent {
		t.Fatalf("expected persisted endpoint probe log, got %+v", logStore.created)
	}
	if endpointStore.updatedID != 41 || endpointStore.updated == nil || !endpointStore.updated.Ok {
		t.Fatalf("expected endpoint probe snapshot update, got id=%d log=%+v", endpointStore.updatedID, endpointStore.updated)
	}
	status := endpointprobesvc.GetStatus(41)
	if status.LastProbeOk == nil || !*status.LastProbeOk || status.LastProbeStatusCode == nil || *status.LastProbeStatusCode != http.StatusNoContent {
		t.Fatalf("expected runtime probe cache for endpoint, got %+v", status)
	}
	if fallback := endpointprobesvc.GetStatus(1); fallback.LastProbeOk != nil {
		t.Fatalf("expected provider fallback not to run when endpoint store is configured, got %+v", fallback)
	}
}
