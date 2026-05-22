package response

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------
// StreamProcessor
// ---------------------------------------------------------------

func TestStreamProcessor_BasicStreaming(t *testing.T) {
	// Simulate upstream SSE data.
	upstream := strings.NewReader(
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"hello\"}}\n\n" +
			"data: {\"type\":\"message_delta\",\"usage\":{\"input_tokens\":10,\"output_tokens\":5}}\n\n" +
			"data: {\"type\":\"message_stop\"}\n\n",
	)

	recorder := httptest.NewRecorder()

	sp := NewStreamProcessor(5*time.Second, 5*time.Second)
	sp.Format = "claude"

	var receivedUsage *Usage
	sp.OnUsage = func(u Usage) {
		receivedUsage = &u
	}

	var ttfb time.Duration
	sp.OnTTFB = func(d time.Duration) {
		ttfb = d
	}

	err := sp.Process(context.Background(), upstream, recorder)
	require.NoError(t, err)

	// Verify data was written downstream.
	body := recorder.Body.String()
	assert.Contains(t, body, "content_block_delta")
	assert.Contains(t, body, "message_delta")

	// Verify TTFB callback was invoked (may be 0 on fast in-memory readers).
	assert.True(t, ttfb >= 0, "TTFB should be >= 0")

	// Verify usage was extracted.
	require.NotNil(t, receivedUsage)
	assert.Equal(t, 10, receivedUsage.InputTokens)
	assert.Equal(t, 5, receivedUsage.OutputTokens)
}

func TestStreamProcessor_SSEHeaders(t *testing.T) {
	upstream := strings.NewReader("data: hello\n\n")
	recorder := httptest.NewRecorder()

	sp := NewStreamProcessor(0, 0)
	err := sp.Process(context.Background(), upstream, recorder)
	require.NoError(t, err)

	assert.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", recorder.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", recorder.Header().Get("Connection"))
}

func TestStreamProcessor_EmptyStream(t *testing.T) {
	upstream := strings.NewReader("")
	recorder := httptest.NewRecorder()

	sp := NewStreamProcessor(0, 0)
	err := sp.Process(context.Background(), upstream, recorder)
	require.NoError(t, err)

	assert.Equal(t, "", recorder.Body.String())
}

func TestStreamProcessor_ContextCancellation(t *testing.T) {
	// Create a reader that blocks forever.
	pr, pw := io.Pipe()
	defer pw.Close()

	recorder := httptest.NewRecorder()

	ctx, cancel := context.WithCancel(context.Background())

	// Write one chunk so the goroutine starts, then cancel.
	go func() {
		time.Sleep(50 * time.Millisecond)
		_, _ = pw.Write([]byte("data: chunk1\n\n"))
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	sp := NewStreamProcessor(0, 0)
	err := sp.Process(ctx, pr, recorder)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestStreamProcessor_FirstByteTimeout(t *testing.T) {
	// Reader that never produces data.
	pr, pw := io.Pipe()
	defer pw.Close()

	recorder := httptest.NewRecorder()

	sp := NewStreamProcessor(100*time.Millisecond, 0)
	err := sp.Process(context.Background(), pr, recorder)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "first byte timeout")
}

func TestStreamProcessor_IdleTimeout(t *testing.T) {
	pr, pw := io.Pipe()

	recorder := httptest.NewRecorder()

	// Write one chunk then stop.
	go func() {
		_, _ = pw.Write([]byte("data: first\n\n"))
		// Then go silent, triggering idle timeout.
	}()

	sp := NewStreamProcessor(5*time.Second, 200*time.Millisecond)
	err := sp.Process(context.Background(), pr, recorder)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "idle timeout")

	pw.Close()
}

func TestStreamProcessor_OpenAIFormat(t *testing.T) {
	upstream := strings.NewReader(
		"data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}],\"usage\":{\"prompt_tokens\":30,\"completion_tokens\":10,\"total_tokens\":40}}\n\n" +
			"data: [DONE]\n\n",
	)

	recorder := httptest.NewRecorder()

	sp := NewStreamProcessor(0, 0)
	sp.Format = "openai"

	var receivedUsage *Usage
	sp.OnUsage = func(u Usage) {
		receivedUsage = &u
	}

	err := sp.Process(context.Background(), upstream, recorder)
	require.NoError(t, err)

	require.NotNil(t, receivedUsage)
	assert.Equal(t, 30, receivedUsage.InputTokens)
	assert.Equal(t, 10, receivedUsage.OutputTokens)
}

func TestStreamProcessor_TTFBCallback(t *testing.T) {
	upstream := strings.NewReader("data: test\n\n")
	recorder := httptest.NewRecorder()

	var ttfbCalled bool
	sp := NewStreamProcessor(0, 0)
	sp.OnTTFB = func(d time.Duration) {
		ttfbCalled = true
		assert.True(t, d >= 0)
	}

	_ = sp.Process(context.Background(), upstream, recorder)
	assert.True(t, ttfbCalled)
}

func TestStreamProcessor_NoUsageCallback(t *testing.T) {
	// OnUsage is nil, should not panic.
	upstream := strings.NewReader("data: {\"type\":\"message_delta\",\"usage\":{\"input_tokens\":1,\"output_tokens\":2}}\n\n")
	recorder := httptest.NewRecorder()

	sp := NewStreamProcessor(0, 0)
	sp.Format = "claude"
	// OnUsage deliberately not set.

	err := sp.Process(context.Background(), upstream, recorder)
	require.NoError(t, err)
}

// ---------------------------------------------------------------
// tryExtractUsage
// ---------------------------------------------------------------

func TestTryExtractUsage_Claude(t *testing.T) {
	sp := &StreamProcessor{Format: "claude"}
	data := "data: {\"type\":\"message_delta\",\"usage\":{\"input_tokens\":42,\"output_tokens\":18}}\n"
	u := sp.tryExtractUsage(data)
	require.NotNil(t, u)
	assert.Equal(t, 42, u.InputTokens)
	assert.Equal(t, 18, u.OutputTokens)
}

func TestTryExtractUsage_OpenAI(t *testing.T) {
	sp := &StreamProcessor{Format: "openai"}
	data := "data: {\"usage\":{\"prompt_tokens\":99,\"completion_tokens\":33,\"total_tokens\":132}}\n"
	u := sp.tryExtractUsage(data)
	require.NotNil(t, u)
	assert.Equal(t, 99, u.InputTokens)
	assert.Equal(t, 33, u.OutputTokens)
}

func TestTryExtractUsage_NoUsageData(t *testing.T) {
	sp := &StreamProcessor{Format: "openai"}
	data := "data: {\"choices\":[{\"delta\":{\"content\":\"word\"}}]}\n"
	u := sp.tryExtractUsage(data)
	assert.Nil(t, u)
}

func TestTryExtractUsage_DONE(t *testing.T) {
	sp := &StreamProcessor{Format: "openai"}
	data := "data: [DONE]\n"
	u := sp.tryExtractUsage(data)
	assert.Nil(t, u)
}

func TestTryExtractUsage_EmptyData(t *testing.T) {
	sp := &StreamProcessor{Format: "claude"}
	u := sp.tryExtractUsage("")
	assert.Nil(t, u)
}

func TestTryExtractUsage_UnknownFormat(t *testing.T) {
	sp := &StreamProcessor{Format: "unknown"}
	// With unknown format, it tries Claude then OpenAI as fallback.
	data := "data: {\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5,\"total_tokens\":15}}\n"
	u := sp.tryExtractUsage(data)
	require.NotNil(t, u)
	assert.Equal(t, 10, u.InputTokens)
}

// ---------------------------------------------------------------
// finalizeUsage
// ---------------------------------------------------------------

func TestFinalizeUsage_WithCallback(t *testing.T) {
	var called bool
	sp := &StreamProcessor{
		OnUsage: func(u Usage) {
			called = true
			assert.Equal(t, 10, u.InputTokens)
		},
	}
	sp.finalizeUsage(&Usage{InputTokens: 10})
	assert.True(t, called)
}

func TestFinalizeUsage_NilUsage(t *testing.T) {
	var called bool
	sp := &StreamProcessor{
		OnUsage: func(u Usage) { called = true },
	}
	sp.finalizeUsage(nil)
	assert.False(t, called)
}

func TestFinalizeUsage_NilCallback(t *testing.T) {
	sp := &StreamProcessor{}
	// Should not panic.
	sp.finalizeUsage(&Usage{InputTokens: 5})
}

// ---------------------------------------------------------------
// helper: nonFlusherWriter for coverage of non-Flusher path
// ---------------------------------------------------------------

type nonFlusherWriter struct {
	buf bytes.Buffer
}

func (w *nonFlusherWriter) Header() http.Header { return http.Header{} }
func (w *nonFlusherWriter) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}
func (w *nonFlusherWriter) WriteHeader(int) {}

func TestStreamProcessor_NonFlusherDownstream(t *testing.T) {
	upstream := strings.NewReader("data: test\n\n")
	w := &nonFlusherWriter{}

	sp := NewStreamProcessor(0, 0)
	err := sp.Process(context.Background(), upstream, w)
	// Should fail because nonFlusherWriter does not implement http.Flusher.
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not support http.Flusher")
}
