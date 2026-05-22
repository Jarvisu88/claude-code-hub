package probe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProber_HealthyEndpoint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	p := NewProber(5 * time.Second)
	ep := &model.ProviderEndpoint{ID: 1, URL: ts.URL, IsEnabled: true}
	result := p.Probe(context.Background(), ep)

	assert.True(t, result.OK)
	assert.Equal(t, http.StatusOK, result.StatusCode)
	assert.Greater(t, result.LatencyMs, int64(0))
	assert.Empty(t, result.ErrorType)
}

func TestProber_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer ts.Close()

	p := NewProber(5 * time.Second)
	ep := &model.ProviderEndpoint{ID: 2, URL: ts.URL, IsEnabled: true}
	result := p.Probe(context.Background(), ep)

	// 5xx is considered NOT ok
	assert.False(t, result.OK)
	assert.Equal(t, http.StatusBadGateway, result.StatusCode)
	assert.Equal(t, "http_error", result.ErrorType)
}

func TestProber_ClientErrorIsOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	p := NewProber(5 * time.Second)
	ep := &model.ProviderEndpoint{ID: 3, URL: ts.URL, IsEnabled: true}
	result := p.Probe(context.Background(), ep)

	// 4xx means the server is reachable, so the endpoint is "OK".
	assert.True(t, result.OK)
	assert.Equal(t, http.StatusUnauthorized, result.StatusCode)
}

func TestProber_ConnectionRefused(t *testing.T) {
	p := NewProber(2 * time.Second)
	ep := &model.ProviderEndpoint{ID: 4, URL: "http://127.0.0.1:1", IsEnabled: true}
	result := p.Probe(context.Background(), ep)

	assert.False(t, result.OK)
	assert.NotEmpty(t, result.ErrorType)
	assert.NotEmpty(t, result.ErrorMsg)
}

func TestProber_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	p := NewProber(200 * time.Millisecond)
	ep := &model.ProviderEndpoint{ID: 5, URL: ts.URL, IsEnabled: true}
	result := p.Probe(context.Background(), ep)

	assert.False(t, result.OK)
	assert.Contains(t, result.ErrorType, "timeout")
}

func TestProber_NilEndpoint(t *testing.T) {
	p := NewProber(0)
	result := p.Probe(context.Background(), nil)
	assert.False(t, result.OK)
	assert.Equal(t, "invalid_endpoint", result.ErrorType)
}

func TestProber_EmptyURL(t *testing.T) {
	p := NewProber(0)
	result := p.Probe(context.Background(), &model.ProviderEndpoint{ID: 6, URL: ""})
	assert.False(t, result.OK)
	assert.Equal(t, "invalid_endpoint", result.ErrorType)
}

func TestProber_DNSError(t *testing.T) {
	p := NewProber(2 * time.Second)
	ep := &model.ProviderEndpoint{ID: 7, URL: "http://this-domain-does-not-exist-xyz123.invalid", IsEnabled: true}
	result := p.Probe(context.Background(), ep)

	assert.False(t, result.OK)
	assert.NotEmpty(t, result.ErrorType)
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://api.example.com", "https://api.example.com"},
		{"http://api.example.com", "http://api.example.com"},
		{"api.example.com", "https://api.example.com"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.expected, normalizeURL(tc.input), "input: %s", tc.input)
	}
}

func TestClassifyError(t *testing.T) {
	assert.Equal(t, "", classifyError(nil))
	assert.Equal(t, "timeout", classifyError(context.DeadlineExceeded))
	assert.Equal(t, "canceled", classifyError(context.Canceled))
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "abc", truncate("abc", 10))
	assert.Equal(t, "ab", truncate("abcde", 2))
}

// --- Batcher tests ---

type mockProbeLogRepo struct {
	logs []*model.ProviderEndpointProbeLog
}

func (m *mockProbeLogRepo) Create(_ context.Context, log *model.ProviderEndpointProbeLog) (*model.ProviderEndpointProbeLog, error) {
	log.ID = len(m.logs) + 1
	m.logs = append(m.logs, log)
	return log, nil
}

func TestLogBatcher_AddAndFlush(t *testing.T) {
	repo := &mockProbeLogRepo{}
	b := NewLogBatcher(repo, 100)

	b.Add(&model.ProviderEndpointProbeLog{EndpointID: 1, Ok: true, CreatedAt: time.Now()})
	b.Add(&model.ProviderEndpointProbeLog{EndpointID: 2, Ok: false, CreatedAt: time.Now()})

	assert.Equal(t, 2, b.Len())

	err := b.Flush(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 0, b.Len())
	assert.Len(t, repo.logs, 2)
}

func TestLogBatcher_AutoFlushOnFull(t *testing.T) {
	repo := &mockProbeLogRepo{}
	b := NewLogBatcher(repo, 2)

	b.Add(&model.ProviderEndpointProbeLog{EndpointID: 1, Ok: true, CreatedAt: time.Now()})
	b.Add(&model.ProviderEndpointProbeLog{EndpointID: 2, Ok: true, CreatedAt: time.Now()})

	// After adding 2 entries (= batchSize), flush should have happened.
	assert.Len(t, repo.logs, 2)
	assert.Equal(t, 0, b.Len())
}

func TestLogBatcher_FlushEmpty(t *testing.T) {
	repo := &mockProbeLogRepo{}
	b := NewLogBatcher(repo, 10)

	err := b.Flush(context.Background())
	require.NoError(t, err)
	assert.Len(t, repo.logs, 0)
}

func TestLogBatcher_NilEntry(t *testing.T) {
	repo := &mockProbeLogRepo{}
	b := NewLogBatcher(repo, 10)

	b.Add(nil)
	assert.Equal(t, 0, b.Len())
}

// --- Scheduler tests ---

type mockEndpointRepo struct {
	endpoints []*model.ProviderEndpoint
	snapshots map[int]*model.ProviderEndpointProbeLog
}

func newMockEndpointRepo(eps ...*model.ProviderEndpoint) *mockEndpointRepo {
	return &mockEndpointRepo{
		endpoints: eps,
		snapshots: map[int]*model.ProviderEndpointProbeLog{},
	}
}

func (m *mockEndpointRepo) List(_ context.Context, _ *repository.ListOptions) ([]*model.ProviderEndpoint, error) {
	return m.endpoints, nil
}

func (m *mockEndpointRepo) UpdateProbeSnapshot(_ context.Context, id int, log *model.ProviderEndpointProbeLog) (*model.ProviderEndpoint, error) {
	m.snapshots[id] = log
	for _, ep := range m.endpoints {
		if ep.ID == id {
			return ep, nil
		}
	}
	return nil, nil
}

func TestScheduler_RunOnce(t *testing.T) {
	// Start a healthy test server.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	logRepo := &mockProbeLogRepo{}
	batcher := NewLogBatcher(logRepo, 100)
	epRepo := newMockEndpointRepo(
		&model.ProviderEndpoint{ID: 10, URL: ts.URL, IsEnabled: true},
	)

	prober := NewProber(5 * time.Second)
	sched := NewScheduler(DefaultInterval, prober, epRepo, batcher)

	err := sched.RunOnce(context.Background())
	require.NoError(t, err)

	// The batcher should have been flushed.
	assert.Len(t, logRepo.logs, 1)
	assert.True(t, logRepo.logs[0].Ok)

	// The snapshot should have been updated.
	snap, exists := epRepo.snapshots[10]
	require.True(t, exists)
	assert.True(t, snap.Ok)
}

func TestScheduler_SkipsInactiveEndpoints(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	logRepo := &mockProbeLogRepo{}
	batcher := NewLogBatcher(logRepo, 100)
	now := time.Now()
	epRepo := newMockEndpointRepo(
		&model.ProviderEndpoint{ID: 20, URL: ts.URL, IsEnabled: false},
		&model.ProviderEndpoint{ID: 21, URL: ts.URL, IsEnabled: true, DeletedAt: &now},
	)

	prober := NewProber(5 * time.Second)
	sched := NewScheduler(DefaultInterval, prober, epRepo, batcher)

	err := sched.RunOnce(context.Background())
	require.NoError(t, err)

	// No logs should be created for disabled / deleted endpoints.
	assert.Len(t, logRepo.logs, 0)
}

func TestScheduler_StartStop(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	logRepo := &mockProbeLogRepo{}
	batcher := NewLogBatcher(logRepo, 100)
	epRepo := newMockEndpointRepo(
		&model.ProviderEndpoint{ID: 30, URL: ts.URL, IsEnabled: true},
	)

	prober := NewProber(5 * time.Second)
	sched := NewScheduler(100*time.Millisecond, prober, epRepo, batcher)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sched.Start(ctx)

	// Give it time for at least one probe cycle.
	time.Sleep(300 * time.Millisecond)

	sched.Stop()

	// After stop, the Done channel should be closed.
	select {
	case <-sched.Done():
		// expected
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not stop in time")
	}

	assert.Greater(t, len(logRepo.logs), 0, "at least one probe log should exist")
}
