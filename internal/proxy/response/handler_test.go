package response

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/proxy"
	"github.com/quagmt/udecimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------
// mock CostCalculator
// ---------------------------------------------------------------

type mockCostCalculator struct {
	mock.Mock
}

func (m *mockCostCalculator) CalculateCost(ctx context.Context, modelName string, usage *Usage, costMultiplier udecimal.Decimal) (udecimal.Decimal, error) {
	args := m.Called(ctx, modelName, usage, costMultiplier)
	return args.Get(0).(udecimal.Decimal), args.Error(1)
}

// ---------------------------------------------------------------
// mock SessionManager
// ---------------------------------------------------------------

type mockSessionManager struct {
	mock.Mock
}

func (m *mockSessionManager) BindProviderToSession(ctx context.Context, sessionID string, providerID int) error {
	args := m.Called(ctx, sessionID, providerID)
	return args.Error(0)
}

// ---------------------------------------------------------------
// helper to build a test session
// ---------------------------------------------------------------

func newTestSession() *proxy.Session {
	sess := proxy.NewSession(context.Background())
	sess.SetUser(&model.User{ID: 1, Name: "testuser"})
	sess.SetAPIKey(&model.Key{ID: 10, Name: "test-key"})
	cm := udecimal.MustParse("1.0")
	sess.SetProvider(&model.Provider{
		ID:             5,
		Name:           "test-provider",
		ProviderType:   "anthropic",
		CostMultiplier: &cm,
	})
	sess.SetRequest(&proxy.Request{
		Model:  "claude-4-sonnet",
		Client: "claude",
		Stream: false,
		Path:   "/v1/messages",
	})
	return sess
}

// ---------------------------------------------------------------
// HandleNonStreaming
// ---------------------------------------------------------------

func TestHandleNonStreaming_Success(t *testing.T) {
	calc := new(mockCostCalculator)
	sm := new(mockSessionManager)
	mrRepo := new(mockMessageRequestRepo)
	ulRepo := new(mockUsageLedgerRepo)
	rlSvc := new(mockRateLimitService)

	calc.On("CalculateCost", mock.Anything, "claude-4-sonnet", mock.Anything, mock.Anything).
		Return(udecimal.MustParse("0.005"), nil)
	sm.On("BindProviderToSession", mock.Anything, mock.Anything, 5).Return(nil)
	mrRepo.On("UpdateAfterResponse", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ulRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	rlSvc.On("RecordUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	recorder := NewCostRecorder(mrRepo, ulRepo, rlSvc)
	handler := NewResponseHandler(calc, recorder, sm)

	sess := newTestSession()
	sess.SetResponse(&proxy.Response{
		StatusCode: 200,
		Body: []byte(`{
			"id":"msg_123",
			"type":"message",
			"usage":{"input_tokens":100,"output_tokens":50}
		}`),
	})

	err := handler.HandleNonStreaming(context.Background(), sess)
	require.NoError(t, err)

	// Verify session was updated.
	assert.Equal(t, 100, sess.Response.PromptTokens)
	assert.Equal(t, 50, sess.Response.CompletionTokens)
	assert.Equal(t, 150, sess.Response.TotalTokens)

	// Cost should be set on session.
	assert.False(t, sess.Cost.TotalCost.IsZero())

	// Session binding should have been called (200 = success).
	sm.AssertExpectations(t)
}

func TestHandleNonStreaming_NoResponse(t *testing.T) {
	handler := NewResponseHandler(nil, nil, nil)
	sess := newTestSession()
	// No response set.

	err := handler.HandleNonStreaming(context.Background(), sess)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no response")
}

func TestHandleNonStreaming_InvalidJSON(t *testing.T) {
	calc := new(mockCostCalculator)
	mrRepo := new(mockMessageRequestRepo)
	ulRepo := new(mockUsageLedgerRepo)
	rlSvc := new(mockRateLimitService)

	// Even with invalid JSON, cost calculator should be called with empty usage.
	calc.On("CalculateCost", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(udecimal.Zero, nil)
	mrRepo.On("UpdateAfterResponse", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ulRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	rlSvc.On("RecordUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	recorder := NewCostRecorder(mrRepo, ulRepo, rlSvc)
	handler := NewResponseHandler(calc, recorder, nil)

	sess := newTestSession()
	sess.SetResponse(&proxy.Response{
		StatusCode: 200,
		Body:       []byte(`{invalid json`),
	})

	err := handler.HandleNonStreaming(context.Background(), sess)
	require.NoError(t, err) // Non-fatal usage parse error.
}

func TestHandleNonStreaming_ErrorResponse_NoSessionBind(t *testing.T) {
	calc := new(mockCostCalculator)
	sm := new(mockSessionManager)
	mrRepo := new(mockMessageRequestRepo)
	ulRepo := new(mockUsageLedgerRepo)
	rlSvc := new(mockRateLimitService)

	calc.On("CalculateCost", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(udecimal.Zero, nil)
	mrRepo.On("UpdateAfterResponse", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ulRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	rlSvc.On("RecordUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	recorder := NewCostRecorder(mrRepo, ulRepo, rlSvc)
	handler := NewResponseHandler(calc, recorder, sm)

	sess := newTestSession()
	sess.SetResponse(&proxy.Response{
		StatusCode: 400,
		Body:       []byte(`{"error":{"type":"invalid_request"}}`),
	})

	err := handler.HandleNonStreaming(context.Background(), sess)
	require.NoError(t, err)

	// Session binding should NOT be called for error responses.
	sm.AssertNotCalled(t, "BindProviderToSession", mock.Anything, mock.Anything, mock.Anything)
}

func TestHandleNonStreaming_OpenAIFormat(t *testing.T) {
	calc := new(mockCostCalculator)
	mrRepo := new(mockMessageRequestRepo)
	ulRepo := new(mockUsageLedgerRepo)
	rlSvc := new(mockRateLimitService)

	calc.On("CalculateCost", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(udecimal.MustParse("0.003"), nil)
	mrRepo.On("UpdateAfterResponse", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ulRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	rlSvc.On("RecordUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	recorder := NewCostRecorder(mrRepo, ulRepo, rlSvc)
	handler := NewResponseHandler(calc, recorder, nil)

	sess := newTestSession()
	sess.Request.Client = "openai"
	sess.SetResponse(&proxy.Response{
		StatusCode: 200,
		Body: []byte(`{
			"id":"chatcmpl-abc",
			"usage":{"prompt_tokens":80,"completion_tokens":40,"total_tokens":120}
		}`),
	})

	err := handler.HandleNonStreaming(context.Background(), sess)
	require.NoError(t, err)
	assert.Equal(t, 80, sess.Response.PromptTokens)
	assert.Equal(t, 40, sess.Response.CompletionTokens)
}

// ---------------------------------------------------------------
// HandleStreaming
// ---------------------------------------------------------------

func TestHandleStreaming_Success(t *testing.T) {
	// Create a test HTTP server that streams SSE data.
	sseData := "data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"hi\"}}\n\n" +
		"data: {\"type\":\"message_delta\",\"usage\":{\"input_tokens\":20,\"output_tokens\":8}}\n\n" +
		"data: {\"type\":\"message_stop\"}\n\n"

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(sseData))
	}))
	defer upstreamServer.Close()

	// Make an HTTP request to the test server.
	resp, err := http.Get(upstreamServer.URL)
	require.NoError(t, err)

	calc := new(mockCostCalculator)
	sm := new(mockSessionManager)
	mrRepo := new(mockMessageRequestRepo)
	ulRepo := new(mockUsageLedgerRepo)
	rlSvc := new(mockRateLimitService)

	calc.On("CalculateCost", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(udecimal.MustParse("0.002"), nil)
	sm.On("BindProviderToSession", mock.Anything, mock.Anything, 5).Return(nil)
	mrRepo.On("UpdateAfterResponse", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ulRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	rlSvc.On("RecordUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	recorder := NewCostRecorder(mrRepo, ulRepo, rlSvc)
	handler := NewResponseHandler(calc, recorder, sm)

	sess := newTestSession()
	sess.Request.Stream = true

	downstreamRecorder := httptest.NewRecorder()

	err = handler.HandleStreaming(context.Background(), sess, resp, downstreamRecorder)
	require.NoError(t, err)

	// Verify downstream got the data.
	body := downstreamRecorder.Body.String()
	assert.Contains(t, body, "content_block_delta")
	assert.Contains(t, body, "message_delta")

	// Verify cost was calculated.
	sm.AssertExpectations(t)
}

func TestHandleStreaming_NilUpstream(t *testing.T) {
	handler := NewResponseHandler(nil, nil, nil)
	sess := newTestSession()

	err := handler.HandleStreaming(context.Background(), sess, nil, httptest.NewRecorder())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "upstream response is nil")
}

func TestHandleStreaming_WithTimeouts(t *testing.T) {
	// Upstream server that sends data with a small gap.
	sseData := "data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"a\"}}\n\n"

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(sseData))
	}))
	defer upstreamServer.Close()

	resp, err := http.Get(upstreamServer.URL)
	require.NoError(t, err)

	calc := new(mockCostCalculator)
	mrRepo := new(mockMessageRequestRepo)
	ulRepo := new(mockUsageLedgerRepo)
	rlSvc := new(mockRateLimitService)

	calc.On("CalculateCost", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(udecimal.Zero, nil)
	mrRepo.On("UpdateAfterResponse", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ulRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	rlSvc.On("RecordUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Set provider with timeouts.
	sess := newTestSession()
	sess.Request.Stream = true
	fbt := 5000
	sit := 5000
	sess.Provider.FirstByteTimeoutStreamingMs = &fbt
	sess.Provider.StreamingIdleTimeoutMs = &sit

	recorder := NewCostRecorder(mrRepo, ulRepo, rlSvc)
	handler := NewResponseHandler(calc, recorder, nil)

	downstreamRecorder := httptest.NewRecorder()
	err = handler.HandleStreaming(context.Background(), sess, resp, downstreamRecorder)
	require.NoError(t, err)
}

// ---------------------------------------------------------------
// parseUsage helper
// ---------------------------------------------------------------

func TestParseUsage_EmptyBody(t *testing.T) {
	handler := &ResponseHandler{}
	u, err := handler.parseUsage([]byte{}, "claude")
	require.NoError(t, err)
	assert.Equal(t, 0, u.InputTokens)
}

func TestParseUsage_UnknownFormat(t *testing.T) {
	handler := &ResponseHandler{}
	body := []byte(`{"usage":{"input_tokens":10,"output_tokens":5}}`)
	u, err := handler.parseUsage(body, "unknown")
	require.NoError(t, err)
	// Falls back to Claude parser which can parse this.
	assert.Equal(t, 10, u.InputTokens)
}

func TestParseUsage_GeminiFormat(t *testing.T) {
	handler := &ResponseHandler{}
	body := []byte(`{"usageMetadata":{"promptTokenCount":30,"candidatesTokenCount":15,"totalTokenCount":45}}`)
	u, err := handler.parseUsage(body, "gemini")
	require.NoError(t, err)
	assert.Equal(t, 30, u.InputTokens)
	assert.Equal(t, 15, u.OutputTokens)
}

func TestParseUsage_CodexFormat(t *testing.T) {
	handler := &ResponseHandler{}
	body := []byte(`{"usage":{"input_tokens":60,"output_tokens":20,"total_tokens":80}}`)
	u, err := handler.parseUsage(body, "codex")
	require.NoError(t, err)
	assert.Equal(t, 60, u.InputTokens)
	assert.Equal(t, 20, u.OutputTokens)
}

// ---------------------------------------------------------------
// getCostMultiplier helper
// ---------------------------------------------------------------

func TestGetCostMultiplier_WithProvider(t *testing.T) {
	handler := &ResponseHandler{}
	sess := newTestSession()
	m := handler.getCostMultiplier(sess)
	assert.Equal(t, "1", m.String())
}

func TestGetCostMultiplier_NilProvider(t *testing.T) {
	handler := &ResponseHandler{}
	sess := proxy.NewSession(context.Background())
	m := handler.getCostMultiplier(sess)
	assert.True(t, m.Equal(udecimal.MustParse("1.0")))
}

func TestGetCostMultiplier_NilCostMultiplier(t *testing.T) {
	handler := &ResponseHandler{}
	sess := proxy.NewSession(context.Background())
	sess.SetProvider(&model.Provider{ID: 1, Name: "p", CostMultiplier: nil})
	m := handler.getCostMultiplier(sess)
	assert.True(t, m.Equal(udecimal.MustParse("1.0")))
}

// ---------------------------------------------------------------
// bindSession helper
// ---------------------------------------------------------------

func TestBindSession_Success(t *testing.T) {
	sm := new(mockSessionManager)
	sm.On("BindProviderToSession", mock.Anything, mock.Anything, 5).Return(nil)

	handler := &ResponseHandler{sessionManager: sm}
	sess := newTestSession()
	handler.bindSession(context.Background(), sess)

	sm.AssertExpectations(t)
}

func TestBindSession_NilSessionManager(t *testing.T) {
	handler := &ResponseHandler{}
	sess := newTestSession()
	// Should not panic.
	handler.bindSession(context.Background(), sess)
}

func TestBindSession_EmptySessionID(t *testing.T) {
	sm := new(mockSessionManager)
	handler := &ResponseHandler{sessionManager: sm}
	sess := proxy.NewSession(context.Background())
	sess.ID = ""
	handler.bindSession(context.Background(), sess)
	sm.AssertNotCalled(t, "BindProviderToSession")
}

func TestBindSession_ZeroProviderID(t *testing.T) {
	sm := new(mockSessionManager)
	handler := &ResponseHandler{sessionManager: sm}
	sess := proxy.NewSession(context.Background())
	// No provider set, so GetProviderID() = 0.
	handler.bindSession(context.Background(), sess)
	sm.AssertNotCalled(t, "BindProviderToSession")
}

// ---------------------------------------------------------------
// NewResponseHandler
// ---------------------------------------------------------------

func TestNewResponseHandler(t *testing.T) {
	calc := new(mockCostCalculator)
	recorder := NewCostRecorder(nil, nil, nil)
	sm := new(mockSessionManager)

	handler := NewResponseHandler(calc, recorder, sm)
	require.NotNil(t, handler)
	assert.Equal(t, calc, handler.costCalculator)
	assert.Equal(t, recorder, handler.costRecorder)
	assert.Equal(t, sm, handler.sessionManager)
}

// ---------------------------------------------------------------
// recordCost helper (nil recorder guard)
// ---------------------------------------------------------------

func TestRecordCost_NilRecorder(t *testing.T) {
	handler := &ResponseHandler{}
	sess := newTestSession()
	sess.SetResponse(&proxy.Response{StatusCode: 200})
	// Should not panic.
	handler.recordCost(context.Background(), sess, &Usage{}, udecimal.Zero, udecimal.MustParse("1"), 100, 50)
}

// ---------------------------------------------------------------
// HandleNonStreaming with nil cost recorder (coverage)
// ---------------------------------------------------------------

func TestHandleNonStreaming_NilRecorder(t *testing.T) {
	calc := new(mockCostCalculator)
	calc.On("CalculateCost", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(udecimal.MustParse("0.001"), nil)

	handler := NewResponseHandler(calc, nil, nil)

	sess := newTestSession()
	sess.SetResponse(&proxy.Response{
		StatusCode: 200,
		Body:       []byte(`{"usage":{"input_tokens":10,"output_tokens":5}}`),
	})

	// Should not panic even with nil recorder.
	err := handler.HandleNonStreaming(context.Background(), sess)
	require.NoError(t, err)
}

// ---------------------------------------------------------------
// HandleNonStreaming timing coverage
// ---------------------------------------------------------------

func TestHandleNonStreaming_Duration(t *testing.T) {
	calc := new(mockCostCalculator)
	calc.On("CalculateCost", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(udecimal.Zero, nil)

	handler := NewResponseHandler(calc, nil, nil)

	sess := newTestSession()
	// Set a start time slightly in the past.
	sess.Timing.StartTime = time.Now().Add(-100 * time.Millisecond)
	sess.SetResponse(&proxy.Response{
		StatusCode: 200,
		Body:       []byte(`{}`),
	})

	err := handler.HandleNonStreaming(context.Background(), sess)
	require.NoError(t, err)
}
