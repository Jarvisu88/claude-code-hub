package proxyerror

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------
// CategorizeUpstreamStatus
// ---------------------------------------------------------------

func TestCategorizeUpstreamStatus(t *testing.T) {
	tests := []struct {
		status   int
		expected Category
	}{
		{400, CategoryClientError},
		{401, CategoryClientError},
		{403, CategoryClientError},
		{404, CategoryResourceNotFound},
		{422, CategoryClientError},
		{429, CategoryRateLimit},
		{499, CategoryClientAbort},
		{500, CategoryServerError},
		{502, CategoryServerError},
		{503, CategoryServerError},
	}

	for _, tc := range tests {
		t.Run(string(rune(tc.status)), func(t *testing.T) {
			assert.Equal(t, tc.expected, CategorizeUpstreamStatus(tc.status))
		})
	}
}

// ---------------------------------------------------------------
// CategoryDefaultStatus
// ---------------------------------------------------------------

func TestCategoryDefaultStatus(t *testing.T) {
	assert.Equal(t, 400, CategoryDefaultStatus(CategoryClientError))
	assert.Equal(t, 502, CategoryDefaultStatus(CategoryServerError))
	assert.Equal(t, 502, CategoryDefaultStatus(CategoryTransportError))
	assert.Equal(t, 429, CategoryDefaultStatus(CategoryRateLimit))
	assert.Equal(t, 404, CategoryDefaultStatus(CategoryResourceNotFound))
	assert.Equal(t, 499, CategoryDefaultStatus(CategoryClientAbort))

	// Unknown category should fall back.
	assert.Equal(t, 502, CategoryDefaultStatus(Category("unknown_category")))
}
