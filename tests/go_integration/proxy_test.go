package go_integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/sse"
	"github.com/ding113/claude-code-hub/internal/proxy"
	"github.com/ding113/claude-code-hub/internal/proxy/forwarder"
	"github.com/ding113/claude-code-hub/internal/proxy/guard"
	"github.com/ding113/claude-code-hub/internal/proxy/response"
	"github.com/ding113/claude-code-hub/internal/proxy/selector"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =========================================================================
// 1. Guard Pipeline Chain Tests
// =========================================================================

func TestGuardChain_AllGuardsPass(t *testing.T) {
	// Build a chain with ProbeGuard + ModelGuard + SensitiveWordGuard.
	// All conditions are benign so every guard should pass.
	chain := guard.NewChain(
		guard.NewProbeGuard(),
		guard.NewModelGuard(),
		guard.NewSensitiveWordGuard(&MockSensitiveWordRepo{
			Words: []model.SensitiveWord{
				{ID: 1, Word: "forbidden", MatchType: "contains", IsEnabled: true},
			},
		}),
	)

	req := &guard.Request{
		User: &model.User{
			ID:   1,
			Name: "testuser",
		},
		APIKey: &model.Key{
			ID:  1,
			Key: "sk-test-key",
		},
		Model: "claude-sonnet-4-20250514",
		Messages: []guard.RequestMessage{
			{Role: "user", Content: "Hello, how are you?"},
		},
	}

	err := chain.Execute(context.Background(), req)
	assert.NoError(t, err, "all guards should pass for a benign request")
}

func TestGuardChain_AuthGuardRejectsInvalidKey(t *testing.T) {
	// The AuthGuard requires a valid AuthService. We create a mock that
	// rejects any key not in its lookup.
	authSvc := authsvc.NewService(&mockKeyLookup{keys: map[string]*model.Key{}}, nil, "admin-token")

	chain := guard.NewChain(
		guard.NewAuthGuard(authSvc),
		guard.NewModelGuard(),
	)

	req := &guard.Request{
		APIKey: &model.Key{
			ID:  1,
			Key: "sk-invalid-key",
		},
		Model: "claude-sonnet-4-20250514",
	}

	err := chain.Execute(context.Background(), req)
	require.Error(t, err, "auth guard should reject invalid key")
}

func TestGuardChain_SensitiveWordBlocks(t *testing.T) {
	repo := &MockSensitiveWordRepo{
		Words: []model.SensitiveWord{
			{ID: 1, Word: "attack_payload", MatchType: "contains", IsEnabled: true},
		},
	}
	chain := guard.NewChain(
		guard.NewSensitiveWordGuard(repo),
	)

	req := &guard.Request{
		Messages: []guard.RequestMessage{
			{Role: "user", Content: "Please execute attack_payload now"},
		},
	}

	err := chain.Execute(context.Background(), req)
	require.Error(t, err, "sensitive word guard should block matching content")
	assert.Contains(t, err.Error(), "sensitive word")
}

func TestGuardChain_ModelGuardBlocksDisallowed(t *testing.T) {
	chain := guard.NewChain(
		guard.NewModelGuard(),
	)

	req := &guard.Request{
		User: &model.User{
			ID:            1,
			Name:          "restricteduser",
			AllowedModels: []string{"claude-sonnet-4-20250514"},
		},
		Model: "gpt-4-turbo",
	}

	err := chain.Execute(context.Background(), req)
	require.Error(t, err, "model guard should block disallowed model")
	assert.Contains(t, err.Error(), "not allowed")
}

func TestGuardChain_ProbeGuardShortCircuits(t *testing.T) {
	// ProbeGuard should detect "foo" as a probe and return ErrProbeDetected.
	chain := guard.NewChain(
		guard.NewProbeGuard(),
		guard.NewModelGuard(),
	)

	req := &guard.Request{
		Messages: []guard.RequestMessage{
			{Role: "user", Content: "foo"},
		},
	}

	err := chain.Execute(context.Background(), req)
	require.Error(t, err)
	assert.ErrorIs(t, err, guard.ErrProbeDetected, "should return probe detected sentinel")
}

func TestGuardChain_ProbeDetects_Count(t *testing.T) {
	chain := guard.NewChain(guard.NewProbeGuard())
	req := &guard.Request{
		Messages: []guard.RequestMessage{
			{Role: "user", Content: "  Count  "},
		},
	}
	err := chain.Execute(context.Background(), req)
	assert.ErrorIs(t, err, guard.ErrProbeDetected)
}

func TestGuardChain_SensitiveWordRegex(t *testing.T) {
	repo := &MockSensitiveWordRepo{
		Words: []model.SensitiveWord{
			{ID: 1, Word: `(?i)hack\s+the\s+system`, MatchType: "regex", IsEnabled: true},
		},
	}
	chain := guard.NewChain(guard.NewSensitiveWordGuard(repo))
	req := &guard.Request{
		Messages: []guard.RequestMessage{
			{Role: "user", Content: "Let me Hack The System now"},
		},
	}
	err := chain.Execute(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sensitive word")
}

func TestGuardChain_SensitiveWordDisabledRuleIgnored(t *testing.T) {
	repo := &MockSensitiveWordRepo{
		Words: []model.SensitiveWord{
			{ID: 1, Word: "blockedword", MatchType: "contains", IsEnabled: false},
		},
	}
	chain := guard.NewChain(guard.NewSensitiveWordGuard(repo))
	req := &guard.Request{
		Messages: []guard.RequestMessage{
			{Role: "user", Content: "This contains blockedword but rule is disabled"},
		},
	}
	err := chain.Execute(context.Background(), req)
	assert.NoError(t, err, "disabled sensitive word rules should be skipped")
}

// =========================================================================
// 2. Provider Selection Tests
// =========================================================================

func TestSelector_BasicWeightedSelection(t *testing.T) {
	// Create 3 providers with different weights.
	p1 := newClaudeProvider(1, "http://p1.test")
	p1.Weight = intPtr(10)
	p2 := newClaudeProvider(2, "http://p2.test")
	p2.Weight = intPtr(1)
	p3 := newClaudeProvider(3, "http://p3.test")
	p3.Weight = intPtr(1)

	sel := selector.New(
		NewMockProviderRepo(p1, p2, p3),
		&MockGroupRepo{},
		&MockCircuitBreaker{},
		NewMockSessionService(),
		&MockCostLimitChecker{},
		nil, // no concurrency checker
	)

	ctx := context.Background()
	req := selector.SelectRequest{
		Model:  "claude-sonnet-4-20250514",
		Format: "claude",
	}

	// Run selection multiple times to verify a result is always returned.
	for i := 0; i < 20; i++ {
		result, err := sel.Select(ctx, req)
		require.NoError(t, err, "selection should succeed")
		require.NotNil(t, result)
		require.NotNil(t, result.Provider)
		assert.Contains(t, []int{1, 2, 3}, result.Provider.ID)
	}
}

func TestSelector_PriorityTierSelection(t *testing.T) {
	// p1 has priority 0 (highest), p2 has priority 10 (lower).
	p1 := newClaudeProvider(1, "http://p1.test")
	p1.Priority = intPtr(0)
	p1.Weight = intPtr(100)
	p2 := newClaudeProvider(2, "http://p2.test")
	p2.Priority = intPtr(10)
	p2.Weight = intPtr(100)

	sel := selector.New(
		NewMockProviderRepo(p1, p2),
		&MockGroupRepo{},
		&MockCircuitBreaker{},
		NewMockSessionService(),
		&MockCostLimitChecker{},
		nil,
	)

	ctx := context.Background()
	req := selector.SelectRequest{
		Model:  "claude-sonnet-4-20250514",
		Format: "claude",
	}

	// Priority 0 should always be selected since it is strictly higher priority.
	for i := 0; i < 10; i++ {
		result, err := sel.Select(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 1, result.Provider.ID, "should always pick the highest priority provider")
	}
}

func TestSelector_GroupFilteringNarrows(t *testing.T) {
	pDefault := newClaudeProvider(1, "http://default.test")
	pDefault.GroupTag = strPtr("default")

	pPremium := newClaudeProvider(2, "http://premium.test")
	pPremium.GroupTag = strPtr("premium")

	sel := selector.New(
		NewMockProviderRepo(pDefault, pPremium),
		&MockGroupRepo{},
		&MockCircuitBreaker{},
		NewMockSessionService(),
		&MockCostLimitChecker{},
		nil,
	)

	ctx := context.Background()

	// Request with "premium" group should only get provider 2.
	req := selector.SelectRequest{
		Model:         "claude-sonnet-4-20250514",
		Format:        "claude",
		ProviderGroup: "premium",
	}

	for i := 0; i < 10; i++ {
		result, err := sel.Select(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 2, result.Provider.ID, "should only return premium provider")
	}
}

func TestSelector_ModelPatternMatching(t *testing.T) {
	// Provider 1 supports only claude-* models (exact rules).
	p1 := newClaudeProvider(1, "http://p1.test")
	p1.AllowedModels = model.ExactAllowedModelRules("claude-sonnet-4-20250514", "claude-opus-4-20250514")

	// Provider 2 supports gpt models.
	p2 := newOpenAIProvider(2, "http://p2.test")
	p2.AllowedModels = model.ExactAllowedModelRules("gpt-4", "gpt-4-turbo")

	sel := selector.New(
		NewMockProviderRepo(p1, p2),
		&MockGroupRepo{},
		&MockCircuitBreaker{},
		NewMockSessionService(),
		&MockCostLimitChecker{},
		nil,
	)

	ctx := context.Background()

	// Ask for gpt-4: only p2 should match.
	req := selector.SelectRequest{
		Model:  "gpt-4",
		Format: "openai",
	}
	result, err := sel.Select(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Provider.ID, "should pick the provider supporting gpt-4")

	// Ask for claude-sonnet-4-20250514: only p1 should match.
	req2 := selector.SelectRequest{
		Model:  "claude-sonnet-4-20250514",
		Format: "claude",
	}
	result2, err := sel.Select(ctx, req2)
	require.NoError(t, err)
	assert.Equal(t, 1, result2.Provider.ID, "should pick the provider supporting claude model")
}

func TestSelector_EmptyProviderListReturnsError(t *testing.T) {
	sel := selector.New(
		NewMockProviderRepo(), // no providers
		&MockGroupRepo{},
		&MockCircuitBreaker{},
		NewMockSessionService(),
		&MockCostLimitChecker{},
		nil,
	)

	ctx := context.Background()
	req := selector.SelectRequest{
		Model:  "claude-sonnet-4-20250514",
		Format: "claude",
	}

	_, err := sel.Select(ctx, req)
	require.Error(t, err, "should return error when no providers available")
	assert.Contains(t, err.Error(), "no providers")
}

func TestSelector_CircuitBreakerFiltersOut(t *testing.T) {
	p1 := newClaudeProvider(1, "http://p1.test")
	p2 := newClaudeProvider(2, "http://p2.test")

	sel := selector.New(
		NewMockProviderRepo(p1, p2),
		&MockGroupRepo{},
		&MockCircuitBreaker{OpenProviders: map[int]bool{1: true}}, // p1 is tripped
		NewMockSessionService(),
		&MockCostLimitChecker{},
		nil,
	)

	ctx := context.Background()
	req := selector.SelectRequest{
		Model:  "claude-sonnet-4-20250514",
		Format: "claude",
	}

	for i := 0; i < 10; i++ {
		result, err := sel.Select(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 2, result.Provider.ID, "should avoid circuit-broken provider")
	}
}

func TestSelector_FormatTypeFiltering(t *testing.T) {
	// A Claude provider and an OpenAI-compatible provider.
	pClaude := newClaudeProvider(1, "http://claude.test")
	pOpenAI := newOpenAIProvider(2, "http://openai.test")

	sel := selector.New(
		NewMockProviderRepo(pClaude, pOpenAI),
		&MockGroupRepo{},
		&MockCircuitBreaker{},
		NewMockSessionService(),
		&MockCostLimitChecker{},
		nil,
	)

	ctx := context.Background()

	// OpenAI format should only get pOpenAI.
	req := selector.SelectRequest{
		Model:  "gpt-4",
		Format: "openai",
	}
	result, err := sel.Select(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Provider.ID)

	// Claude format should only get pClaude.
	req2 := selector.SelectRequest{
		Model:  "claude-sonnet-4-20250514",
		Format: "claude",
	}
	result2, err := sel.Select(ctx, req2)
	require.NoError(t, err)
	assert.Equal(t, 1, result2.Provider.ID)
}

// =========================================================================
// 3. Forwarding + Response Tests
// =========================================================================

func TestForwarder_NonStreamingSuccess(t *testing.T) {
	body := claudeResponse(100, 50)
	upstream := newMockUpstream(http.StatusOK, "application/json", body)
	defer upstream.Close()

	tm := forwarder.NewTransportManager(forwarder.TransportConfig{})
	fwd := forwarder.New(tm, forwarder.DefaultTimeoutConfig())

	p := newClaudeProvider(1, upstream.URL)
	result, err := fwd.Forward(context.Background(), &forwarder.ForwardRequest{
		Provider: p,
		Method:   "POST",
		Path:     "/v1/messages",
		Headers:  map[string]string{"Content-Type": "application/json"},
		Body:     []byte(`{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"hi"}]}`),
	})

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.StatusCode)
	assert.NotEmpty(t, result.Body)

	// Verify usage can be parsed from the response.
	usage, parseErr := response.ParseUsageFromClaude(result.Body)
	require.NoError(t, parseErr)
	assert.Equal(t, 100, usage.InputTokens)
	assert.Equal(t, 50, usage.OutputTokens)
}

func TestForwarder_StreamingForwardsSSE(t *testing.T) {
	sseBody := claudeSSEStream(200, 75)
	upstream := newMockUpstreamSSE(sseBody)
	defer upstream.Close()

	tm := forwarder.NewTransportManager(forwarder.TransportConfig{})
	fwd := forwarder.New(tm, forwarder.DefaultTimeoutConfig())

	p := newClaudeProvider(1, upstream.URL)
	result, err := fwd.Forward(context.Background(), &forwarder.ForwardRequest{
		Provider:  p,
		Method:    "POST",
		Path:      "/v1/messages",
		Headers:   map[string]string{"Content-Type": "application/json"},
		Body:      []byte(`{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"hi"}],"stream":true}`),
		Streaming: true,
	})

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.StatusCode)
	require.NotNil(t, result.Stream, "streaming result should have a stream channel")

	// Drain the stream.
	var allData []byte
	for chunk := range result.Stream {
		allData = append(allData, chunk...)
	}
	<-result.StreamDone

	// Verify the streamed data contains SSE events.
	assert.Contains(t, string(allData), "event: message_start")
	assert.Contains(t, string(allData), "event: message_delta")
}

func TestForwarder_RetryOn500(t *testing.T) {
	// First call returns 500, second call succeeds.
	responses := []seqResponse{
		{StatusCode: 500, ContentType: "application/json", Body: []byte(`{"error":"internal"}`)},
		{StatusCode: 200, ContentType: "application/json", Body: claudeResponse(50, 25)},
	}
	upstream := newMockUpstreamSequence(responses)
	defer upstream.Close()

	tm := forwarder.NewTransportManager(forwarder.TransportConfig{})
	fwd := forwarder.New(tm, forwarder.DefaultTimeoutConfig())
	strategy := forwarder.NewRetryStrategy(fwd, forwarder.RetryConfig{
		MaxSameProviderRetries: 1,
		MaxProviderSwitches:    3,
		RetryDelay:             0, // no delay for tests
	})

	p := newClaudeProvider(1, upstream.URL)
	result, err := strategy.ForwardWithRetry(context.Background(), &forwarder.ForwardRequest{
		Provider: p,
		Method:   "POST",
		Path:     "/v1/messages",
		Headers:  map[string]string{"Content-Type": "application/json"},
		Body:     []byte(`{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"hi"}]}`),
	}, nil)

	require.NoError(t, err, "should succeed on retry after 500")
	assert.Equal(t, http.StatusOK, result.Result.StatusCode)
	assert.Equal(t, 1, result.Provider.ID)
	assert.Equal(t, 2, result.TotalAttempts, "should have made 2 attempts")
}

func TestForwarder_CrossProviderFallback(t *testing.T) {
	// Provider 1 always returns 500; Provider 2 returns 200.
	upstream1 := newMockUpstream(500, "application/json", []byte(`{"error":"down"}`))
	defer upstream1.Close()
	upstream2 := newMockUpstream(200, "application/json", claudeResponse(30, 15))
	defer upstream2.Close()

	p1 := newClaudeProvider(1, upstream1.URL)
	p2 := newClaudeProvider(2, upstream2.URL)

	tm := forwarder.NewTransportManager(forwarder.TransportConfig{})
	fwd := forwarder.New(tm, forwarder.DefaultTimeoutConfig())
	strategy := forwarder.NewRetryStrategy(fwd, forwarder.RetryConfig{
		MaxSameProviderRetries: 0, // no same-provider retry
		MaxProviderSwitches:    5,
		RetryDelay:             0,
	})

	providerIdx := 0
	providers := []*model.Provider{p1, p2}
	nextProvider := func(excludeIDs []int) *model.Provider {
		for _, p := range providers {
			excluded := false
			for _, id := range excludeIDs {
				if p.ID == id {
					excluded = true
					break
				}
			}
			if !excluded {
				providerIdx++
				return p
			}
		}
		return nil
	}

	result, err := strategy.ForwardWithRetry(context.Background(), &forwarder.ForwardRequest{
		Provider: p1,
		Method:   "POST",
		Path:     "/v1/messages",
		Headers:  map[string]string{"Content-Type": "application/json"},
		Body:     []byte(`{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"hi"}]}`),
	}, nextProvider)

	require.NoError(t, err, "should succeed via fallback to provider 2")
	assert.Equal(t, 2, result.Provider.ID, "should be served by fallback provider")
	assert.GreaterOrEqual(t, result.ProviderSwitches, 1, "should have at least one provider switch")
}

func TestForwarder_ModelRedirectApplies(t *testing.T) {
	// Upstream echos the request body back so we can inspect what model was sent.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			w.WriteHeader(500)
			return
		}
		// Echo the model field as the response "model" field.
		resp := map[string]interface{}{
			"id":    "msg_test",
			"type":  "message",
			"model": reqBody["model"],
			"usage": map[string]interface{}{
				"input_tokens":  10,
				"output_tokens": 5,
			},
			"content": []map[string]interface{}{
				{"type": "text", "text": "echo"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	p := newClaudeProvider(1, upstream.URL)
	p.ModelRedirects = model.ProviderModelRedirectRules{
		{MatchType: "exact", Source: "claude-sonnet-4-20250514", Target: "my-custom-model"},
	}

	// Apply model redirect.
	redirect := forwarder.ApplyModelRedirect(p, "claude-sonnet-4-20250514")
	assert.True(t, redirect.WasRedirected)
	assert.Equal(t, "my-custom-model", redirect.RedirectedModel)
	assert.Equal(t, "claude-sonnet-4-20250514", redirect.OriginalModel)

	// Redirect model in body.
	originalBody := []byte(`{"model":"claude-sonnet-4-20250514","messages":[]}`)
	redirectedBody := forwarder.RedirectModelInBody(originalBody, redirect.OriginalModel, redirect.RedirectedModel)
	assert.Contains(t, string(redirectedBody), `"model":"my-custom-model"`)

	// Forward with the redirected body.
	tm := forwarder.NewTransportManager(forwarder.TransportConfig{})
	fwd := forwarder.New(tm, forwarder.DefaultTimeoutConfig())
	result, err := fwd.Forward(context.Background(), &forwarder.ForwardRequest{
		Provider: p,
		Method:   "POST",
		Path:     "/v1/messages",
		Headers:  map[string]string{"Content-Type": "application/json"},
		Body:     redirectedBody,
	})

	require.NoError(t, err)
	assert.Equal(t, 200, result.StatusCode)

	// Verify the upstream saw the redirected model.
	var respBody map[string]interface{}
	require.NoError(t, json.Unmarshal(result.Body, &respBody))
	assert.Equal(t, "my-custom-model", respBody["model"])
}

func TestForwarder_UpstreamErrorPreservesBody(t *testing.T) {
	errorBody := []byte(`{"error":{"type":"overloaded_error","message":"Service overloaded"}}`)
	upstream := newMockUpstream(529, "application/json", errorBody)
	defer upstream.Close()

	tm := forwarder.NewTransportManager(forwarder.TransportConfig{})
	fwd := forwarder.New(tm, forwarder.DefaultTimeoutConfig())

	p := newClaudeProvider(1, upstream.URL)
	result, err := fwd.Forward(context.Background(), &forwarder.ForwardRequest{
		Provider: p,
		Method:   "POST",
		Path:     "/v1/messages",
		Headers:  map[string]string{"Content-Type": "application/json"},
		Body:     []byte(`{"model":"claude-sonnet-4-20250514","messages":[]}`),
	})

	require.Error(t, err, "should return error for 529")
	require.NotNil(t, result)
	assert.Equal(t, 529, result.StatusCode)
	assert.Contains(t, string(result.Body), "overloaded_error")
}

// =========================================================================
// 4. Endpoint Catalog Tests
// =========================================================================

func TestCatalog_ClaudeMessages(t *testing.T) {
	fam := proxy.MatchEndpoint("POST", "/v1/messages")
	require.NotNil(t, fam)
	assert.Equal(t, "claude-messages", fam.ID)
	assert.Equal(t, proxy.SurfaceClaude, fam.Surface)
	assert.Equal(t, proxy.AccountingRequired, fam.AccountingTier)
}

func TestCatalog_ClaudeCountTokens(t *testing.T) {
	fam := proxy.MatchEndpoint("POST", "/v1/messages/count_tokens")
	require.NotNil(t, fam)
	assert.Equal(t, "claude-count-tokens", fam.ID)
	assert.Equal(t, proxy.SurfaceClaude, fam.Surface)
	assert.True(t, fam.RawPassthrough)
}

func TestCatalog_OpenAIChatCompletions(t *testing.T) {
	fam := proxy.MatchEndpoint("POST", "/v1/chat/completions")
	require.NotNil(t, fam)
	assert.Equal(t, "openai-chat-completions", fam.ID)
	assert.Equal(t, proxy.SurfaceOpenAI, fam.Surface)
	assert.Equal(t, proxy.AccountingRequired, fam.AccountingTier)
	assert.True(t, fam.ModelRequired)
}

func TestCatalog_CodexResponses(t *testing.T) {
	fam := proxy.MatchEndpoint("POST", "/v1/responses")
	require.NotNil(t, fam)
	assert.Equal(t, "response-execution", fam.ID)
	assert.Equal(t, proxy.SurfaceCodex, fam.Surface)
}

func TestCatalog_CodexResponsesCompact(t *testing.T) {
	fam := proxy.MatchEndpoint("POST", "/v1/responses/compact")
	require.NotNil(t, fam)
	assert.Equal(t, "response-compact", fam.ID)
	assert.True(t, fam.RawPassthrough)
}

func TestCatalog_GeminiGenerateContent(t *testing.T) {
	fam := proxy.MatchEndpoint("POST", "/v1beta/models/gemini-2.0-flash:generateContent")
	require.NotNil(t, fam)
	assert.Equal(t, "gemini-generate-content", fam.ID)
	assert.Equal(t, proxy.SurfaceGemini, fam.Surface)
}

func TestCatalog_GeminiStreamGenerateContent(t *testing.T) {
	fam := proxy.MatchEndpoint("POST", "/v1beta/models/gemini-pro:streamGenerateContent")
	require.NotNil(t, fam)
	assert.Equal(t, "gemini-stream-generate-content", fam.ID)
}

func TestCatalog_GeminiCLI(t *testing.T) {
	fam := proxy.MatchEndpoint("POST", "/v1internal/models/gemini-2.0-flash:generateContent")
	require.NotNil(t, fam)
	assert.Equal(t, "gemini-cli-generate-content", fam.ID)
	assert.Equal(t, proxy.SurfaceGeminiCLI, fam.Surface)
}

func TestCatalog_OpenAIModels(t *testing.T) {
	fam := proxy.MatchEndpoint("GET", "/v1/models")
	require.NotNil(t, fam)
	assert.Equal(t, "openai-models", fam.ID)
}

func TestCatalog_UnknownEndpoint(t *testing.T) {
	fam := proxy.MatchEndpoint("GET", "/v99/unknown/endpoint")
	assert.Nil(t, fam, "unknown path should return nil")
}

func TestCatalog_NormalizationStripsQuery(t *testing.T) {
	// Path with query string should still match.
	fam := proxy.MatchEndpoint("POST", "/v1/messages?stream=true")
	require.NotNil(t, fam)
	assert.Equal(t, "claude-messages", fam.ID)
}

func TestCatalog_NormalizationStripsTrailingSlash(t *testing.T) {
	fam := proxy.MatchEndpoint("POST", "/v1/chat/completions/")
	require.NotNil(t, fam)
	assert.Equal(t, "openai-chat-completions", fam.ID)
}

func TestCatalog_DetectSurface(t *testing.T) {
	assert.Equal(t, proxy.SurfaceClaude, proxy.DetectSurface("/v1/messages"))
	assert.Equal(t, proxy.SurfaceOpenAI, proxy.DetectSurface("/v1/chat/completions"))
	assert.Equal(t, proxy.SurfaceCodex, proxy.DetectSurface("/v1/responses"))
	assert.Equal(t, proxy.Surface(""), proxy.DetectSurface("/unknown"))
}

func TestCatalog_IsGeminiGenerationEndpoint(t *testing.T) {
	assert.True(t, proxy.IsGeminiGenerationEndpoint("/v1beta/models/gemini-pro:generateContent"))
	assert.True(t, proxy.IsGeminiGenerationEndpoint("/v1beta/models/gemini-pro:streamGenerateContent"))
	assert.False(t, proxy.IsGeminiGenerationEndpoint("/v1beta/models/gemini-pro:countTokens"))
	assert.False(t, proxy.IsGeminiGenerationEndpoint("/v1/messages"))
}

func TestCatalog_GeminiVertexPattern(t *testing.T) {
	fam := proxy.MatchEndpoint("POST", "/v1/publishers/google/models/gemini-1.5-pro:generateContent")
	require.NotNil(t, fam)
	assert.Equal(t, "gemini-generate-content", fam.ID)
}

// =========================================================================
// 5. Usage Parsing Tests
// =========================================================================

func TestUsageParsing_ClaudeFormat(t *testing.T) {
	body := claudeResponse(150, 80)
	usage, err := response.ParseUsageFromClaude(body)
	require.NoError(t, err)
	assert.Equal(t, 150, usage.InputTokens)
	assert.Equal(t, 80, usage.OutputTokens)
	assert.Equal(t, 230, usage.TotalTokens)
}

func TestUsageParsing_OpenAIFormat(t *testing.T) {
	body := openAIResponse(200, 100)
	usage, err := response.ParseUsageFromOpenAI(body)
	require.NoError(t, err)
	assert.Equal(t, 200, usage.InputTokens)
	assert.Equal(t, 100, usage.OutputTokens)
	assert.Equal(t, 300, usage.TotalTokens)
}

func TestUsageParsing_CodexFormat(t *testing.T) {
	body := codexResponse(120, 60)
	usage, err := response.ParseUsageFromCodex(body)
	require.NoError(t, err)
	assert.Equal(t, 120, usage.InputTokens)
	assert.Equal(t, 60, usage.OutputTokens)
	assert.Equal(t, 180, usage.TotalTokens)
}

func TestUsageParsing_GeminiFormat(t *testing.T) {
	body := geminiResponse(300, 150)
	usage, err := response.ParseUsageFromGemini(body)
	require.NoError(t, err)
	assert.Equal(t, 300, usage.InputTokens)
	assert.Equal(t, 150, usage.OutputTokens)
	assert.Equal(t, 450, usage.TotalTokens)
}

func TestUsageParsing_SSEStreamClaude(t *testing.T) {
	sseData := claudeSSEStream(250, 90)
	events, err := sse.ParseBytes(sseData)
	require.NoError(t, err)
	require.NotEmpty(t, events)

	usage, err := response.ParseUsageFromSSEEvents(events, "claude")
	require.NoError(t, err)
	// The message_delta event carries output_tokens.
	assert.Equal(t, 90, usage.OutputTokens)
}

func TestUsageParsing_SSEStreamOpenAI(t *testing.T) {
	sseData := openAISSEStream(300, 150)
	events, err := sse.ParseBytes(sseData)
	require.NoError(t, err)
	require.NotEmpty(t, events)

	usage, err := response.ParseUsageFromSSEEvents(events, "openai")
	require.NoError(t, err)
	assert.Equal(t, 300, usage.InputTokens)
	assert.Equal(t, 150, usage.OutputTokens)
}

func TestUsageParsing_MissingUsageFields(t *testing.T) {
	// A response without a usage block.
	body := []byte(`{"id":"msg_test","type":"message","content":[{"type":"text","text":"no usage"}]}`)
	usage, err := response.ParseUsageFromClaude(body)
	require.NoError(t, err)
	assert.Equal(t, 0, usage.InputTokens)
	assert.Equal(t, 0, usage.OutputTokens)
	assert.Equal(t, 0, usage.TotalTokens)
}

func TestUsageParsing_EmptyBody(t *testing.T) {
	// ParseUsageFromClaude on empty body returns a JSON parse error.
	_, err := response.ParseUsageFromClaude([]byte{})
	assert.Error(t, err)

	// ParseUsageFromOpenAI on empty string also returns a JSON parse error.
	_, err = response.ParseUsageFromOpenAI([]byte(""))
	assert.Error(t, err)
}

func TestUsageParsing_InvalidJSON(t *testing.T) {
	body := []byte(`this is not json`)
	_, err := response.ParseUsageFromClaude(body)
	assert.Error(t, err)
}

func TestUsageParsing_ClaudeCacheTokens(t *testing.T) {
	body := []byte(`{
		"id": "msg_cache",
		"type": "message",
		"usage": {
			"input_tokens": 100,
			"output_tokens": 50,
			"cache_creation_input_tokens": 1000,
			"cache_read_input_tokens": 500
		}
	}`)
	usage, err := response.ParseUsageFromClaude(body)
	require.NoError(t, err)
	assert.Equal(t, 100, usage.InputTokens)
	assert.Equal(t, 50, usage.OutputTokens)
	assert.Equal(t, 1000, usage.CacheCreationTokens)
	assert.Equal(t, 500, usage.CacheReadTokens)
	assert.Equal(t, 1650, usage.TotalTokens, "total should include cache tokens")
}

func TestUsageParsing_SSEStreamNoUsageEvents(t *testing.T) {
	// SSE with only [DONE] -- no usage data.
	events := []*sse.Event{
		{Data: "[DONE]"},
	}
	usage, err := response.ParseUsageFromSSEEvents(events, "openai")
	require.NoError(t, err)
	assert.Equal(t, 0, usage.TotalTokens, "no usage events should yield empty usage")
}

// =========================================================================
// 6. StreamProcessor Tests (integration-level)
// =========================================================================

func TestStreamProcessor_ForwardsToDownstream(t *testing.T) {
	sseBody := claudeSSEStream(100, 50)

	// Use a pipe to simulate the upstream reader.
	reader := strings.NewReader(string(sseBody))

	recorder := httptest.NewRecorder()

	sp := response.NewStreamProcessor(0, 0)
	sp.Format = "claude"

	var capturedUsage *response.Usage
	sp.OnUsage = func(u response.Usage) {
		capturedUsage = &u
	}

	err := sp.Process(context.Background(), reader, recorder)
	require.NoError(t, err)

	// Verify downstream received data.
	assert.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
	body := recorder.Body.String()
	assert.Contains(t, body, "message_start")
	assert.Contains(t, body, "message_delta")

	// Verify usage was extracted.
	require.NotNil(t, capturedUsage)
	assert.Equal(t, 50, capturedUsage.OutputTokens)
}

// =========================================================================
// 7. Response Fixer Tests
// =========================================================================

func TestFixTruncatedJSON(t *testing.T) {
	truncated := []byte(`{"usage":{"input_tokens":100,"output_tokens":50`)
	fixed := response.FixTruncatedJSON(truncated)
	var parsed map[string]interface{}
	err := json.Unmarshal(fixed, &parsed)
	assert.NoError(t, err, "fixed JSON should be parseable")
}

func TestFixEncoding_ValidUTF8PassesThrough(t *testing.T) {
	original := []byte("Hello, world!")
	fixed := response.FixEncoding(original)
	assert.Equal(t, original, fixed)
}

func TestFixEncoding_InvalidBytesReplaced(t *testing.T) {
	invalid := []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f, 0xFF, 0xFE}
	fixed := response.FixEncoding(invalid)
	assert.True(t, len(fixed) > 0)
	// The invalid bytes should be replaced with replacement chars.
	assert.Contains(t, string(fixed), "Hello")
}

// =========================================================================
// 8. Model Redirect Tests
// =========================================================================

func TestModelRedirect_ExactMatch(t *testing.T) {
	p := &model.Provider{
		ModelRedirects: model.ProviderModelRedirectRules{
			{MatchType: "exact", Source: "claude-sonnet-4-20250514", Target: "sonnet-custom"},
		},
	}
	r := forwarder.ApplyModelRedirect(p, "claude-sonnet-4-20250514")
	assert.True(t, r.WasRedirected)
	assert.Equal(t, "sonnet-custom", r.RedirectedModel)
}

func TestModelRedirect_NoMatch(t *testing.T) {
	p := &model.Provider{
		ModelRedirects: model.ProviderModelRedirectRules{
			{MatchType: "exact", Source: "claude-opus-4-20250514", Target: "opus-custom"},
		},
	}
	r := forwarder.ApplyModelRedirect(p, "claude-sonnet-4-20250514")
	assert.False(t, r.WasRedirected)
	assert.Equal(t, "claude-sonnet-4-20250514", r.RedirectedModel)
}

func TestModelRedirect_EmptyRules(t *testing.T) {
	p := &model.Provider{}
	r := forwarder.ApplyModelRedirect(p, "claude-sonnet-4-20250514")
	assert.False(t, r.WasRedirected)
}

func TestModelRedirect_BodyReplacement(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-20250514","messages":[]}`)
	result := forwarder.RedirectModelInBody(body, "claude-sonnet-4-20250514", "custom-model")
	assert.Contains(t, string(result), `"model":"custom-model"`)
	assert.NotContains(t, string(result), `"model":"claude-sonnet-4-20250514"`)
}

// =========================================================================
// 9. Full pipeline integration (Guard -> Select -> Forward -> Parse)
// =========================================================================

func TestFullPipeline_ClaudeRequest(t *testing.T) {
	// 1. Set up a mock upstream.
	body := claudeResponse(500, 250)
	upstream := newMockUpstream(http.StatusOK, "application/json", body)
	defer upstream.Close()

	// 2. Create provider.
	p := newClaudeProvider(1, upstream.URL)

	// 3. Guard chain.
	chain := guard.NewChain(
		guard.NewProbeGuard(),
		guard.NewModelGuard(),
		guard.NewSensitiveWordGuard(&MockSensitiveWordRepo{}),
	)

	guardReq := &guard.Request{
		User: &model.User{
			ID:   1,
			Name: "testuser",
		},
		APIKey: &model.Key{
			ID:  1,
			Key: "sk-test",
		},
		Model: "claude-sonnet-4-20250514",
		Messages: []guard.RequestMessage{
			{Role: "user", Content: "Hello, world!"},
		},
	}

	err := chain.Execute(context.Background(), guardReq)
	require.NoError(t, err, "guard chain should pass")

	// 4. Provider selection.
	sel := selector.New(
		NewMockProviderRepo(p),
		&MockGroupRepo{},
		&MockCircuitBreaker{},
		NewMockSessionService(),
		&MockCostLimitChecker{},
		nil,
	)

	selResult, err := sel.Select(context.Background(), selector.SelectRequest{
		Model:  "claude-sonnet-4-20250514",
		Format: "claude",
	})
	require.NoError(t, err)
	require.Equal(t, p.ID, selResult.Provider.ID)

	// 5. Forward request.
	tm := forwarder.NewTransportManager(forwarder.TransportConfig{})
	fwd := forwarder.New(tm, forwarder.DefaultTimeoutConfig())

	fwdResult, err := fwd.Forward(context.Background(), &forwarder.ForwardRequest{
		Provider: selResult.Provider,
		Method:   "POST",
		Path:     "/v1/messages",
		Headers:  map[string]string{"Content-Type": "application/json"},
		Body:     []byte(`{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hello, world!"}]}`),
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, fwdResult.StatusCode)

	// 6. Parse usage.
	usage, err := response.ParseUsageFromClaude(fwdResult.Body)
	require.NoError(t, err)
	assert.Equal(t, 500, usage.InputTokens)
	assert.Equal(t, 250, usage.OutputTokens)

	// 7. Update session.
	session := proxy.NewSession(context.Background())
	session.SetUser(guardReq.User)
	session.SetAPIKey(guardReq.APIKey)
	session.SetProvider(selResult.Provider)
	session.SetRequest(&proxy.Request{
		Path:   "/v1/messages",
		Model:  "claude-sonnet-4-20250514",
		Client: "claude",
	})
	session.SetResponse(&proxy.Response{
		StatusCode:       fwdResult.StatusCode,
		Body:             fwdResult.Body,
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		TotalTokens:      usage.TotalTokens,
	})

	assert.Equal(t, "claude-sonnet-4-20250514", session.GetModel())
	assert.Equal(t, "claude", session.GetClient())
	assert.Equal(t, 1, session.GetUserID())
	assert.Equal(t, 1, session.GetProviderID())
}

func TestFullPipeline_ProbeShortCircuit(t *testing.T) {
	// A probe request should never reach the forwarder.
	chain := guard.NewChain(
		guard.NewProbeGuard(),
		guard.NewModelGuard(),
	)

	guardReq := &guard.Request{
		Messages: []guard.RequestMessage{
			{Role: "user", Content: "foo"},
		},
	}

	err := chain.Execute(context.Background(), guardReq)
	require.ErrorIs(t, err, guard.ErrProbeDetected)
	// In the real pipeline, this would return a canned probe response
	// without forwarding to any upstream.
}

// =========================================================================
// Mock auth service support
// =========================================================================

// mockKeyLookup implements the proxyKeyLookup interface for auth.Service.
type mockKeyLookup struct {
	keys map[string]*model.Key
}

func (m *mockKeyLookup) GetByKeyWithUser(_ context.Context, key string) (*model.Key, error) {
	if k, ok := m.keys[key]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("not found")
}
