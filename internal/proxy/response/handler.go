package response

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ding113/claude-code-hub/internal/proxy"
	"github.com/quagmt/udecimal"
)

// ResponseHandler orchestrates the full response processing pipeline:
// parse response, extract usage, calculate cost, persist records, and
// update rate-limit counters.
type ResponseHandler struct {
	costCalculator CostCalculator
	costRecorder   *CostRecorder
	sessionManager SessionManager
}

// NewResponseHandler creates a ResponseHandler.
func NewResponseHandler(
	costCalculator CostCalculator,
	costRecorder *CostRecorder,
	sessionManager SessionManager,
) *ResponseHandler {
	return &ResponseHandler{
		costCalculator: costCalculator,
		costRecorder:   costRecorder,
		sessionManager: sessionManager,
	}
}

// HandleNonStreaming processes a non-streaming upstream response. It reads
// the body that has already been captured in session.Response.Body, parses
// usage, calculates cost, persists everything and sets cost on the session.
func (h *ResponseHandler) HandleNonStreaming(ctx context.Context, session *proxy.Session) error {
	resp := session.Response
	if resp == nil {
		return fmt.Errorf("session has no response")
	}

	// 1. Parse usage from the response body
	usage, err := h.parseUsage(resp.Body, session.GetClient())
	if err != nil {
		// Usage parsing failure is non-fatal; we still want to record the request.
		usage = &Usage{}
	}

	// Update session response with parsed token counts.
	resp.PromptTokens = usage.InputTokens
	resp.CompletionTokens = usage.OutputTokens
	resp.TotalTokens = usage.TotalTokens

	// 2. Calculate cost
	costMultiplier := h.getCostMultiplier(session)
	costUSD, calcErr := h.costCalculator.CalculateCost(ctx, session.GetModel(), usage, costMultiplier)
	if calcErr != nil {
		// Cost calculation failure is non-fatal.
		costUSD = udecimal.Zero
	}

	// 3. Update session cost
	// We split into input/output for compatibility with session.SetCost.
	inputCostApprox, _ := udecimal.NewFromFloat64(0)
	session.SetCost(inputCostApprox, costUSD)

	// 4. Calculate duration
	durationMs := 0
	if !session.Timing.StartTime.IsZero() {
		durationMs = int(time.Since(session.Timing.StartTime).Milliseconds())
	}

	// 5. Persist cost data
	h.recordCost(ctx, session, usage, costUSD, costMultiplier, durationMs, 0)

	// 6. Bind session to provider on success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		h.bindSession(ctx, session)
	}

	return nil
}

// HandleStreaming processes a streaming upstream response. It sets up SSE
// headers, streams data to the client, and finalizes cost recording after
// the stream completes.
func (h *ResponseHandler) HandleStreaming(
	ctx context.Context,
	session *proxy.Session,
	upstream *http.Response,
	writer http.ResponseWriter,
) error {
	if upstream == nil {
		return fmt.Errorf("upstream response is nil")
	}
	defer upstream.Body.Close()

	// Determine timeouts from provider config.
	firstByteTimeout := time.Duration(0)
	idleTimeout := time.Duration(0)
	if session.Provider != nil {
		if session.Provider.FirstByteTimeoutStreamingMs != nil && *session.Provider.FirstByteTimeoutStreamingMs > 0 {
			firstByteTimeout = time.Duration(*session.Provider.FirstByteTimeoutStreamingMs) * time.Millisecond
		}
		if session.Provider.StreamingIdleTimeoutMs != nil && *session.Provider.StreamingIdleTimeoutMs > 0 {
			idleTimeout = time.Duration(*session.Provider.StreamingIdleTimeoutMs) * time.Millisecond
		}
	}

	sp := NewStreamProcessor(firstByteTimeout, idleTimeout)
	sp.Format = session.GetClient()

	// Track TTFB and final usage via callbacks.
	var ttfbMs int
	var finalUsage *Usage
	startTime := time.Now()

	sp.OnTTFB = func(d time.Duration) {
		ttfbMs = int(d.Milliseconds())
	}

	sp.OnUsage = func(u Usage) {
		finalUsage = &u
	}

	// Copy upstream headers to downstream before streaming.
	for key, values := range upstream.Header {
		for _, v := range values {
			writer.Header().Add(key, v)
		}
	}

	// Stream data.
	streamErr := sp.Process(ctx, upstream.Body, writer)

	// Calculate duration.
	durationMs := int(time.Since(startTime).Milliseconds())

	// Use the final usage or fall back to empty.
	usage := finalUsage
	if usage == nil {
		usage = &Usage{}
	}

	// Calculate cost.
	costMultiplier := h.getCostMultiplier(session)
	costUSD, _ := h.costCalculator.CalculateCost(ctx, session.GetModel(), usage, costMultiplier)

	// Update session.
	if session.Response == nil {
		session.SetResponse(&proxy.Response{
			StatusCode: upstream.StatusCode,
			Headers:    make(map[string]string),
		})
	}
	session.Response.PromptTokens = usage.InputTokens
	session.Response.CompletionTokens = usage.OutputTokens
	session.Response.TotalTokens = usage.TotalTokens

	inputCostApprox, _ := udecimal.NewFromFloat64(0)
	session.SetCost(inputCostApprox, costUSD)

	// Persist cost data (deferred finalization).
	statusCode := upstream.StatusCode
	h.recordCost(ctx, session, usage, costUSD, costMultiplier, durationMs, ttfbMs)

	// Bind session on success.
	if statusCode >= 200 && statusCode < 300 {
		h.bindSession(ctx, session)
	}

	return streamErr
}

// -------------------------------------------------------------------
// internal helpers
// -------------------------------------------------------------------

// parseUsage dispatches to the correct format parser based on client type.
func (h *ResponseHandler) parseUsage(body []byte, clientType string) (*Usage, error) {
	if len(body) == 0 {
		return &Usage{}, nil
	}

	// Try to fix truncated JSON before parsing.
	body = FixTruncatedJSON(body)
	body = FixEncoding(body)

	switch clientType {
	case "claude":
		return ParseUsageFromClaude(body)
	case "openai":
		return ParseUsageFromOpenAI(body)
	case "codex":
		return ParseUsageFromCodex(body)
	case "gemini":
		return ParseUsageFromGemini(body)
	default:
		// Try Claude first (most common), then OpenAI.
		u, err := ParseUsageFromClaude(body)
		if err == nil && u != nil && (u.InputTokens > 0 || u.OutputTokens > 0) {
			return u, nil
		}
		return ParseUsageFromOpenAI(body)
	}
}

// getCostMultiplier extracts the cost multiplier from the session provider.
func (h *ResponseHandler) getCostMultiplier(session *proxy.Session) udecimal.Decimal {
	if session.Provider != nil && session.Provider.CostMultiplier != nil {
		return *session.Provider.CostMultiplier
	}
	return udecimal.MustParse("1.0")
}

// recordCost is a fire-and-forget cost recording helper. Errors are logged
// but do not fail the request.
func (h *ResponseHandler) recordCost(
	ctx context.Context,
	session *proxy.Session,
	usage *Usage,
	costUSD udecimal.Decimal,
	costMultiplier udecimal.Decimal,
	durationMs int,
	ttfbMs int,
) {
	if h.costRecorder == nil {
		return
	}

	statusCode := 0
	if session.Response != nil {
		statusCode = session.Response.StatusCode
	}

	isSuccess := statusCode >= 200 && statusCode < 300

	keyID := 0
	keyName := ""
	if session.APIKey != nil {
		keyID = session.APIKey.ID
		keyName = session.APIKey.Name
	}

	providerID := session.GetProviderID()

	req := RecordCostRequest{
		MessageRequestID: 0, // filled by caller if available
		Usage:            usage,
		CostUSD:          costUSD,
		CostMultiplier:   costMultiplier,
		ProviderID:       providerID,
		FinalProviderID:  providerID,
		Model:            session.GetModel(),
		UserID:           session.GetUserID(),
		KeyID:            keyID,
		KeyName:          keyName,
		TTFBMs:           ttfbMs,
		DurationMs:       durationMs,
		StatusCode:       statusCode,
		IsSuccess:        isSuccess,
	}

	if session.Request != nil {
		req.Endpoint = session.Request.Path
		req.APIType = session.Request.Client
	}

	// Best-effort recording.
	_ = h.costRecorder.RecordCost(ctx, req)
}

// bindSession binds the session to the provider for session reuse.
func (h *ResponseHandler) bindSession(ctx context.Context, session *proxy.Session) {
	if h.sessionManager == nil || session.ID == "" || session.GetProviderID() == 0 {
		return
	}
	_ = h.sessionManager.BindProviderToSession(ctx, session.ID, session.GetProviderID())
}
