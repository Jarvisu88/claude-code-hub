package forwarder

import (
	"context"
	"fmt"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/rs/zerolog/log"
)

// MaxProviderSwitches is the absolute maximum number of cross-provider fallback attempts.
const MaxProviderSwitches = 20

// RetryConfig holds retry/fallback configuration.
type RetryConfig struct {
	// MaxSameProviderRetries is the default max retries on the same provider.
	// Overridden by provider.MaxRetryAttempts if set.
	MaxSameProviderRetries int

	// MaxProviderSwitches is the max cross-provider fallback attempts. Capped at 20.
	MaxProviderSwitches int

	// RetryDelay is the base delay between same-provider retries.
	RetryDelay time.Duration
}

// DefaultRetryConfig returns sensible defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxSameProviderRetries: 1,
		MaxProviderSwitches:    MaxProviderSwitches,
		RetryDelay:             500 * time.Millisecond,
	}
}

// ProviderSnapshotFunc returns the next fallback provider, excluding the given IDs.
// Returns nil when no more providers are available.
type ProviderSnapshotFunc func(excludeIDs []int) *model.Provider

// RetryStrategy manages same-provider retry and cross-provider fallback.
type RetryStrategy struct {
	config    RetryConfig
	forwarder *Forwarder
}

// NewRetryStrategy creates a retry strategy.
func NewRetryStrategy(f *Forwarder, cfg RetryConfig) *RetryStrategy {
	if cfg.MaxProviderSwitches <= 0 || cfg.MaxProviderSwitches > MaxProviderSwitches {
		cfg.MaxProviderSwitches = MaxProviderSwitches
	}
	if cfg.MaxSameProviderRetries < 0 {
		cfg.MaxSameProviderRetries = 0
	}
	return &RetryStrategy{
		config:    cfg,
		forwarder: f,
	}
}

// RetryResult holds the final result of a retry-aware forwarding attempt.
type RetryResult struct {
	// Result is the upstream response (may be non-nil even on error, e.g. 4xx body).
	Result *ForwardResult

	// Provider is the provider that ultimately served the request.
	Provider *model.Provider

	// TotalAttempts is the total number of forwarding attempts made.
	TotalAttempts int

	// ProviderSwitches is the number of cross-provider fallback switches.
	ProviderSwitches int
}

// ForwardWithRetry attempts the request with same-provider retry and cross-provider fallback.
//
// Error handling strategy:
//   - Client errors (4xx except 429): NOT retryable, return immediately
//   - Rate limit (429): switch to next provider
//   - Server errors (5xx): retryable, try same provider first, then fallback
//   - Transport/network errors: retryable, try same provider once, then fallback
func (rs *RetryStrategy) ForwardWithRetry(
	ctx context.Context,
	req *ForwardRequest,
	nextProvider ProviderSnapshotFunc,
) (*RetryResult, error) {
	excludeIDs := make([]int, 0)
	totalAttempts := 0
	providerSwitches := 0
	currentProvider := req.Provider

	for providerSwitches <= rs.config.MaxProviderSwitches {
		if currentProvider == nil {
			break
		}

		// Determine max same-provider retries for this provider
		maxRetries := rs.config.MaxSameProviderRetries
		if currentProvider.MaxRetryAttempts != nil && *currentProvider.MaxRetryAttempts >= 0 {
			maxRetries = *currentProvider.MaxRetryAttempts
		}

		// Same-provider retry loop
		var lastErr error
		var lastResult *ForwardResult

		for attempt := 0; attempt <= maxRetries; attempt++ {
			totalAttempts++

			if attempt > 0 {
				// Backoff between same-provider retries
				delay := rs.config.RetryDelay * time.Duration(attempt)
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
				}
			}

			// Build the request for the current provider
			fwdReq := &ForwardRequest{
				Provider:    currentProvider,
				Method:      req.Method,
				Path:        req.Path,
				Headers:     req.Headers,
				Body:        req.Body,
				Streaming:   req.Streaming,
				EndpointURL: req.EndpointURL,
			}

			result, err := rs.forwarder.Forward(ctx, fwdReq)
			if err == nil {
				return &RetryResult{
					Result:           result,
					Provider:         currentProvider,
					TotalAttempts:    totalAttempts,
					ProviderSwitches: providerSwitches,
				}, nil
			}

			lastErr = err
			lastResult = result

			// Check for client abort first (highest priority)
			if errors.IsClientAbortError(err) {
				return &RetryResult{
					Result:           result,
					Provider:         currentProvider,
					TotalAttempts:    totalAttempts,
					ProviderSwitches: providerSwitches,
				}, err
			}

			// Check the HTTP status code for more precise classification.
			// CategorizeError treats all non-404 ProxyErrors as CategoryProviderError,
			// but we need to distinguish 4xx client errors from 5xx server errors.
			if proxyErr, ok := errors.AsProxyError(err); ok {
				statusCode := proxyErr.StatusCode

				switch {
				case statusCode == 429:
					// Rate limit - switch to next provider immediately
					log.Debug().
						Int("providerID", currentProvider.ID).
						Msg("forwarder: rate limited, switching provider")
					goto nextProvider

				case statusCode == 404:
					// Not found - switch to next provider
					log.Debug().
						Int("providerID", currentProvider.ID).
						Msg("forwarder: resource not found, switching provider")
					goto nextProvider

				case statusCode >= 400 && statusCode < 500:
					// Other client errors (400, 401, 403, etc.) - non-retryable
					return &RetryResult{
						Result:           result,
						Provider:         currentProvider,
						TotalAttempts:    totalAttempts,
						ProviderSwitches: providerSwitches,
					}, err

				case statusCode >= 500:
					// Server errors - retry on same provider, then fallback
					log.Debug().
						Int("providerID", currentProvider.ID).
						Int("attempt", attempt).
						Int("statusCode", statusCode).
						Err(err).
						Msg("forwarder: server error, will retry")
					continue
				}
			}

			// Non-ProxyError (transport/network error)
			category := errors.CategorizeError(err)
			switch category {
			case errors.CategoryNonRetryableClientError:
				return &RetryResult{
					Result:           result,
					Provider:         currentProvider,
					TotalAttempts:    totalAttempts,
					ProviderSwitches: providerSwitches,
				}, err

			case errors.CategorySystemError:
				// Network error: retry once on same provider, then fallback
				if attempt == 0 {
					log.Debug().
						Int("providerID", currentProvider.ID).
						Err(err).
						Msg("forwarder: system error, retrying once on same provider")
					continue
				}
				log.Debug().
					Int("providerID", currentProvider.ID).
					Err(err).
					Msg("forwarder: system error after retry, switching provider")
				goto nextProvider

			default:
				// Provider error or unknown - retry on same provider
				log.Debug().
					Int("providerID", currentProvider.ID).
					Int("attempt", attempt).
					Err(err).
					Msg("forwarder: error, will retry")
				continue
			}
		}

		// Exhausted same-provider retries, try fallback
		_ = lastErr
		_ = lastResult

	nextProvider:
		excludeIDs = append(excludeIDs, currentProvider.ID)
		providerSwitches++

		if nextProvider == nil {
			break
		}

		currentProvider = nextProvider(excludeIDs)
		if currentProvider == nil {
			break
		}

		log.Debug().
			Int("newProviderID", currentProvider.ID).
			Int("switchCount", providerSwitches).
			Msg("forwarder: switching to fallback provider")
	}

	return nil, fmt.Errorf("forwarder: exhausted all providers after %d attempts (%d switches)", totalAttempts, providerSwitches)
}

// ClassifyHTTPError categorizes an HTTP status code.
func ClassifyHTTPError(statusCode int) errors.ErrorCategory {
	switch {
	case statusCode == 429:
		return errors.CategoryProviderError
	case statusCode >= 400 && statusCode < 500:
		return errors.CategoryNonRetryableClientError
	case statusCode >= 500:
		return errors.CategoryProviderError
	default:
		return errors.CategorySystemError
	}
}
