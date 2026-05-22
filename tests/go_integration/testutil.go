// Package go_integration provides integration test helpers for the proxy pipeline.
//
// It contains mock providers (fake HTTP servers), in-memory repository
// implementations, and helper functions for creating test fixtures. All
// helpers are designed to be used with httptest and require no real
// database or Redis connections.
package go_integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/quagmt/udecimal"
)

// ---------------------------------------------------------------------------
// Helper constructors
// ---------------------------------------------------------------------------

// boolPtr returns a pointer to a bool.
func boolPtr(v bool) *bool { return &v }

// intPtr returns a pointer to an int.
func intPtr(v int) *int { return &v }

// strPtr returns a pointer to a string.
func strPtr(v string) *string { return &v }

// decimalPtr returns a pointer to a udecimal.Decimal.
func decimalPtr(v string) *udecimal.Decimal {
	d := udecimal.MustParse(v)
	return &d
}

// ---------------------------------------------------------------------------
// Mock upstream HTTP servers
// ---------------------------------------------------------------------------

// claudeResponse builds a minimal Claude Messages API response body.
func claudeResponse(inputTokens, outputTokens int) []byte {
	resp := map[string]interface{}{
		"id":    "msg_test_001",
		"type":  "message",
		"role":  "assistant",
		"model": "claude-sonnet-4-20250514",
		"content": []map[string]interface{}{
			{"type": "text", "text": "Hello from mock Claude"},
		},
		"usage": map[string]interface{}{
			"input_tokens":                inputTokens,
			"output_tokens":               outputTokens,
			"cache_creation_input_tokens": 0,
			"cache_read_input_tokens":     0,
		},
		"stop_reason": "end_turn",
	}
	b, _ := json.Marshal(resp)
	return b
}

// openAIResponse builds a minimal OpenAI Chat Completions response body.
func openAIResponse(promptTokens, completionTokens int) []byte {
	resp := map[string]interface{}{
		"id":      "chatcmpl-test001",
		"object":  "chat.completion",
		"created": 1700000000,
		"model":   "gpt-4",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "Hello from mock OpenAI",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":      promptTokens + completionTokens,
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

// codexResponse builds a minimal Codex / Responses API response body.
func codexResponse(inputTokens, outputTokens int) []byte {
	resp := map[string]interface{}{
		"id":     "resp_test001",
		"object": "response",
		"status": "completed",
		"output": []map[string]interface{}{
			{
				"type": "message",
				"role": "assistant",
				"content": []map[string]interface{}{
					{"type": "output_text", "text": "Hello from mock Codex"},
				},
			},
		},
		"usage": map[string]interface{}{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  inputTokens + outputTokens,
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

// geminiResponse builds a minimal Gemini generateContent response body.
func geminiResponse(promptTokens, candidateTokens int) []byte {
	resp := map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{
						{"text": "Hello from mock Gemini"},
					},
					"role": "model",
				},
				"finishReason": "STOP",
			},
		},
		"usageMetadata": map[string]interface{}{
			"promptTokenCount":     promptTokens,
			"candidatesTokenCount": candidateTokens,
			"totalTokenCount":      promptTokens + candidateTokens,
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

// claudeSSEStream builds a Claude-style SSE stream with usage in the final event.
func claudeSSEStream(inputTokens, outputTokens int) []byte {
	events := ""
	events += "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_test\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet-4-20250514\",\"content\":[],\"usage\":{\"input_tokens\":" + fmt.Sprintf("%d", inputTokens) + ",\"output_tokens\":0}}}\n\n"
	events += "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"
	events += "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n"
	events += "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n"
	events += "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":" + fmt.Sprintf("%d", outputTokens) + "}}\n\n"
	events += "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	return []byte(events)
}

// openAISSEStream builds an OpenAI-style SSE stream with usage in the final chunk.
func openAISSEStream(promptTokens, completionTokens int) []byte {
	events := ""
	events += "data: {\"id\":\"chatcmpl-test\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"},\"finish_reason\":null}]}\n\n"
	events += "data: {\"id\":\"chatcmpl-test\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"},\"finish_reason\":null}]}\n\n"
	events += fmt.Sprintf("data: {\"id\":\"chatcmpl-test\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":%d,\"completion_tokens\":%d,\"total_tokens\":%d}}\n\n", promptTokens, completionTokens, promptTokens+completionTokens)
	events += "data: [DONE]\n\n"
	return []byte(events)
}

// newMockUpstream creates an httptest.Server that responds with the given
// status code, content type and body. Caller must defer .Close().
func newMockUpstream(statusCode int, contentType string, body []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(statusCode)
		_, _ = w.Write(body)
	}))
}

// newMockUpstreamSSE creates an httptest.Server that responds with SSE.
func newMockUpstreamSSE(body []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
}

// newMockUpstreamSequence creates an httptest.Server that returns responses
// from the given slice in order. After all responses are consumed, it
// returns 500 Internal Server Error.
type seqResponse struct {
	StatusCode  int
	ContentType string
	Body        []byte
}

func newMockUpstreamSequence(responses []seqResponse) *httptest.Server {
	mu := sync.Mutex{}
	idx := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		i := idx
		idx++
		mu.Unlock()

		if i >= len(responses) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"exhausted mock responses"}`))
			return
		}

		resp := responses[i]
		if resp.ContentType != "" {
			w.Header().Set("Content-Type", resp.ContentType)
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(resp.Body)
	}))
}

// ---------------------------------------------------------------------------
// Mock repositories (in-memory)
// ---------------------------------------------------------------------------

// MockSensitiveWordRepo implements guard.SensitiveWordRepo in-memory.
type MockSensitiveWordRepo struct {
	Words []model.SensitiveWord
	Err   error
}

func (m *MockSensitiveWordRepo) GetAll(_ context.Context) ([]model.SensitiveWord, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Words, nil
}

// MockProviderRepo implements selector.ProviderRepo in-memory.
type MockProviderRepo struct {
	Providers []*model.Provider
	ByIDMap   map[int]*model.Provider
	Err       error
}

func NewMockProviderRepo(providers ...*model.Provider) *MockProviderRepo {
	byID := make(map[int]*model.Provider, len(providers))
	for _, p := range providers {
		byID[p.ID] = p
	}
	return &MockProviderRepo{
		Providers: providers,
		ByIDMap:   byID,
	}
}

func (m *MockProviderRepo) GetActiveProviders(_ context.Context) ([]*model.Provider, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Providers, nil
}

func (m *MockProviderRepo) GetByID(_ context.Context, id int) (*model.Provider, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if p, ok := m.ByIDMap[id]; ok {
		return p, nil
	}
	return nil, nil
}

// MockGroupRepo implements selector.ProviderGroupRepo.
type MockGroupRepo struct {
	Multipliers map[string]udecimal.Decimal
}

func (m *MockGroupRepo) GetCostMultiplier(_ context.Context, group string) (udecimal.Decimal, error) {
	if m == nil || m.Multipliers == nil {
		return udecimal.MustParse("1.0"), nil
	}
	if v, ok := m.Multipliers[group]; ok {
		return v, nil
	}
	return udecimal.MustParse("1.0"), nil
}

// MockCircuitBreaker implements selector.CircuitBreakerService.
type MockCircuitBreaker struct {
	OpenProviders map[int]bool
}

func (m *MockCircuitBreaker) IsOpen(p *model.Provider) bool {
	if m == nil || m.OpenProviders == nil {
		return false
	}
	return m.OpenProviders[p.ID]
}

// MockSessionService implements selector.SessionService.
type MockSessionService struct {
	mu       sync.Mutex
	Bindings map[string]int // sessionID -> providerID
}

func NewMockSessionService() *MockSessionService {
	return &MockSessionService{Bindings: make(map[string]int)}
}

func (m *MockSessionService) GetBoundProvider(_ context.Context, sessionID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Bindings[sessionID], nil
}

func (m *MockSessionService) BindProvider(_ context.Context, sessionID string, providerID int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Bindings[sessionID] = providerID
}

// MockCostLimitChecker implements selector.CostLimitChecker (always allows).
type MockCostLimitChecker struct {
	Denied map[int]bool
}

func (m *MockCostLimitChecker) CheckCostLimits(_ context.Context, p *model.Provider) (bool, error) {
	if m == nil || m.Denied == nil {
		return true, nil
	}
	return !m.Denied[p.ID], nil
}

// MockConcurrencyChecker implements selector.ConcurrencyChecker (always allows).
type MockConcurrencyChecker struct {
	Denied map[int]bool
}

func (m *MockConcurrencyChecker) CheckAndTrack(_ context.Context, providerID int, _ string, _ int) (bool, int, error) {
	if m == nil || m.Denied == nil {
		return true, 0, nil
	}
	return !m.Denied[providerID], 0, nil
}

// ---------------------------------------------------------------------------
// Provider factory helpers
// ---------------------------------------------------------------------------

// newProvider creates a minimal enabled Provider for testing.
func newProvider(id int, name string, url string, providerType string) *model.Provider {
	return &model.Provider{
		ID:           id,
		Name:         name,
		URL:          url,
		Key:          "test-key-" + name,
		IsEnabled:    boolPtr(true),
		Weight:       intPtr(1),
		Priority:     intPtr(0),
		ProviderType: providerType,
	}
}

// newClaudeProvider creates a Claude-type provider pointing at the given URL.
func newClaudeProvider(id int, url string) *model.Provider {
	return newProvider(id, fmt.Sprintf("claude-%d", id), url, "claude")
}

// newOpenAIProvider creates an OpenAI-compatible provider pointing at the given URL.
func newOpenAIProvider(id int, url string) *model.Provider {
	return newProvider(id, fmt.Sprintf("openai-%d", id), url, "openai-compatible")
}

// newCodexProvider creates a Codex-type provider pointing at the given URL.
func newCodexProvider(id int, url string) *model.Provider {
	return newProvider(id, fmt.Sprintf("codex-%d", id), url, "codex")
}

// newGeminiProvider creates a Gemini-type provider pointing at the given URL.
func newGeminiProvider(id int, url string) *model.Provider {
	return newProvider(id, fmt.Sprintf("gemini-%d", id), url, "gemini")
}
