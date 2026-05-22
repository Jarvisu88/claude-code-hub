package response

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// StreamProcessor reads SSE events from an upstream reader and writes them
// to a downstream http.ResponseWriter in real-time. It enforces configurable
// first-byte and idle timeouts and provides callbacks for TTFB and final usage.
type StreamProcessor struct {
	// FirstByteTimeout is how long to wait for the very first data byte
	// from upstream before declaring a timeout. Zero means no timeout.
	FirstByteTimeout time.Duration

	// IdleTimeout is how long to tolerate silence between successive data
	// chunks during streaming. Zero means no timeout.
	IdleTimeout time.Duration

	// OnUsage is called (if non-nil) when a final usage block is detected
	// among the streamed SSE events.
	OnUsage func(Usage)

	// OnTTFB is called (if non-nil) with the duration between Process start
	// and the first received data byte.
	OnTTFB func(time.Duration)

	// Format identifies the upstream SSE format so the processor can detect
	// the terminal event and extract usage. One of: "claude", "openai",
	// "codex", "gemini".
	Format string
}

// NewStreamProcessor returns a StreamProcessor with the given timeouts.
func NewStreamProcessor(firstByteTimeout, idleTimeout time.Duration) *StreamProcessor {
	return &StreamProcessor{
		FirstByteTimeout: firstByteTimeout,
		IdleTimeout:      idleTimeout,
	}
}

// Process reads SSE from upstream and streams to downstream. It returns nil
// on successful stream completion, or an error on timeout / read failure.
// The caller is responsible for closing upstream.
func (sp *StreamProcessor) Process(
	ctx context.Context,
	upstream io.Reader,
	downstream http.ResponseWriter,
) error {
	// Set SSE headers.
	downstream.Header().Set("Content-Type", "text/event-stream")
	downstream.Header().Set("Cache-Control", "no-cache")
	downstream.Header().Set("Connection", "keep-alive")
	downstream.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := downstream.(http.Flusher)
	if !ok {
		return fmt.Errorf("downstream does not support http.Flusher")
	}

	// Channel-based read loop so we can select on context + timeouts.
	type readResult struct {
		data []byte
		err  error
	}
	readCh := make(chan readResult, 1)

	go func() {
		defer close(readCh)
		reader := bufio.NewReaderSize(upstream, 8192)
		buf := make([]byte, 8192)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				readCh <- readResult{data: chunk}
			}
			if err != nil {
				readCh <- readResult{err: err}
				return
			}
		}
	}()

	startTime := time.Now()
	firstByteReceived := false
	var lastUsage *Usage

	// Accumulators for SSE line-based usage detection.
	var dataBuf strings.Builder

	for {
		// Determine the appropriate timeout.
		var timeout <-chan time.Time
		if !firstByteReceived && sp.FirstByteTimeout > 0 {
			timeout = time.After(sp.FirstByteTimeout)
		} else if firstByteReceived && sp.IdleTimeout > 0 {
			timeout = time.After(sp.IdleTimeout)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-timeout:
			if !firstByteReceived {
				return fmt.Errorf("first byte timeout after %v", sp.FirstByteTimeout)
			}
			return fmt.Errorf("idle timeout after %v", sp.IdleTimeout)

		case res, ok := <-readCh:
			if !ok {
				// Channel closed -- goroutine exited.
				sp.finalizeUsage(lastUsage)
				return nil
			}

			if res.err != nil {
				if res.err == io.EOF {
					sp.finalizeUsage(lastUsage)
					return nil
				}
				return fmt.Errorf("upstream read error: %w", res.err)
			}

			// TTFB tracking.
			if !firstByteReceived {
				firstByteReceived = true
				ttfb := time.Since(startTime)
				if sp.OnTTFB != nil {
					sp.OnTTFB(ttfb)
				}
			}

			// Write data downstream.
			if _, err := downstream.Write(res.data); err != nil {
				return fmt.Errorf("downstream write error: %w", err)
			}
			flusher.Flush()

			// Accumulate for usage detection.
			dataBuf.Write(res.data)
			if u := sp.tryExtractUsage(dataBuf.String()); u != nil {
				lastUsage = u
			}
		}
	}
}

// tryExtractUsage scans the accumulated SSE data for the latest usage block.
func (sp *StreamProcessor) tryExtractUsage(accumulated string) *Usage {
	// We scan for the last "data:" line that contains usage information.
	lines := strings.Split(accumulated, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimPrefix(line, "data:")
		payload = strings.TrimSpace(payload)
		if payload == "" || payload == "[DONE]" {
			continue
		}

		raw := []byte(payload)
		var u *Usage
		var err error

		switch sp.Format {
		case "claude":
			u, err = parseClaudeStreamEvent(raw)
		case "openai":
			u, err = ParseUsageFromOpenAI(raw)
		case "codex":
			u, err = ParseUsageFromCodex(raw)
		case "gemini":
			u, err = ParseUsageFromGemini(raw)
		default:
			// Try Claude then OpenAI as fallback.
			u, err = parseClaudeStreamEvent(raw)
			if err != nil || u == nil || (u.InputTokens == 0 && u.OutputTokens == 0) {
				u, err = ParseUsageFromOpenAI(raw)
			}
		}

		if err == nil && u != nil && (u.InputTokens > 0 || u.OutputTokens > 0) {
			return u
		}
	}
	return nil
}

// finalizeUsage invokes OnUsage with the last known usage if available.
func (sp *StreamProcessor) finalizeUsage(u *Usage) {
	if u != nil && sp.OnUsage != nil {
		sp.OnUsage(*u)
	}
}
