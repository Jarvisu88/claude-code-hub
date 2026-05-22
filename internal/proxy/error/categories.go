package proxyerror

import "net/http"

// Category describes how an error should be handled by the retry/fallback logic.
type Category string

const (
	// CategoryClientError indicates a 4xx-class error caused by the client
	// input (bad request, missing fields, etc.). These should NOT be retried.
	CategoryClientError Category = "client_error"

	// CategoryServerError indicates a 5xx-class upstream error. These are
	// eligible for retry and provider fallback.
	CategoryServerError Category = "server_error"

	// CategoryTransportError indicates a network-level failure (DNS, TLS,
	// connection reset, timeout). Eligible for retry with a different provider.
	CategoryTransportError Category = "transport_error"

	// CategoryRateLimit indicates the upstream provider returned a 429 or
	// equivalent. May be retried with a different provider.
	CategoryRateLimit Category = "rate_limit"

	// CategoryResourceNotFound indicates the upstream returned 404. The model
	// or resource may not exist on this provider but could on another.
	CategoryResourceNotFound Category = "resource_not_found"

	// CategoryClientAbort indicates the downstream client closed the connection.
	// No retry is needed.
	CategoryClientAbort Category = "client_abort"
)

// CategoryDefinition holds the default HTTP status code and a human-readable
// label for each error category.
type CategoryDefinition struct {
	Category          Category
	DefaultStatusCode int
	Label             string
}

// Categories is the ordered catalog of error categories and their defaults.
var Categories = []CategoryDefinition{
	{CategoryClientError, http.StatusBadRequest, "Client Error"},
	{CategoryServerError, http.StatusBadGateway, "Server Error"},
	{CategoryTransportError, http.StatusBadGateway, "Transport Error"},
	{CategoryRateLimit, http.StatusTooManyRequests, "Rate Limit"},
	{CategoryResourceNotFound, http.StatusNotFound, "Resource Not Found"},
	{CategoryClientAbort, 499, "Client Abort"},
}

// CategoryDefaultStatus returns the default HTTP status code for a category.
func CategoryDefaultStatus(cat Category) int {
	for _, def := range Categories {
		if def.Category == cat {
			return def.DefaultStatusCode
		}
	}
	return http.StatusBadGateway
}

// CategorizeUpstreamStatus maps an upstream HTTP status code to an error category.
func CategorizeUpstreamStatus(statusCode int) Category {
	switch {
	case statusCode == 429:
		return CategoryRateLimit
	case statusCode == 404:
		return CategoryResourceNotFound
	case statusCode == 499:
		return CategoryClientAbort
	case statusCode >= 400 && statusCode < 500:
		return CategoryClientError
	case statusCode >= 500:
		return CategoryServerError
	default:
		return CategoryServerError
	}
}
