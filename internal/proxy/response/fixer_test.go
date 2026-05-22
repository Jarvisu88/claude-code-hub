package response

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------
// FixTruncatedJSON
// ---------------------------------------------------------------

func TestFixTruncatedJSON_AlreadyValid(t *testing.T) {
	input := []byte(`{"usage":{"input_tokens":10}}`)
	result := FixTruncatedJSON(input)
	assert.Equal(t, `{"usage":{"input_tokens":10}}`, string(result))
}

func TestFixTruncatedJSON_UnclosedBrace(t *testing.T) {
	input := []byte(`{"usage":{"input_tokens":10}`)
	result := FixTruncatedJSON(input)
	assert.Equal(t, `{"usage":{"input_tokens":10}}`, string(result))
}

func TestFixTruncatedJSON_UnclosedBracket(t *testing.T) {
	input := []byte(`{"items":[1,2,3`)
	result := FixTruncatedJSON(input)
	assert.Equal(t, `{"items":[1,2,3]}`, string(result))
}

func TestFixTruncatedJSON_UnclosedString(t *testing.T) {
	input := []byte(`{"key":"value`)
	result := FixTruncatedJSON(input)
	// Should close the string and the brace.
	assert.Contains(t, string(result), `"value"`)
	assert.Contains(t, string(result), "}")
}

func TestFixTruncatedJSON_TrailingComma(t *testing.T) {
	input := []byte(`{"a":1,`)
	result := FixTruncatedJSON(input)
	assert.Equal(t, `{"a":1}`, string(result))
}

func TestFixTruncatedJSON_NestedUnclosed(t *testing.T) {
	input := []byte(`{"a":{"b":[1,{"c":2`)
	result := FixTruncatedJSON(input)
	// Should close }, ], } in order.
	expected := `{"a":{"b":[1,{"c":2}]}}`
	assert.Equal(t, expected, string(result))
}

func TestFixTruncatedJSON_Empty(t *testing.T) {
	assert.Equal(t, []byte(nil), FixTruncatedJSON([]byte("")))
}

func TestFixTruncatedJSON_WhitespaceOnly(t *testing.T) {
	assert.Equal(t, []byte(nil), FixTruncatedJSON([]byte("   ")))
}

func TestFixTruncatedJSON_ValidArray(t *testing.T) {
	input := []byte(`[1,2,3]`)
	result := FixTruncatedJSON(input)
	assert.Equal(t, `[1,2,3]`, string(result))
}

func TestFixTruncatedJSON_EscapedQuotes(t *testing.T) {
	input := []byte(`{"key":"val\"ue"}`)
	result := FixTruncatedJSON(input)
	assert.Equal(t, `{"key":"val\"ue"}`, string(result))
}

// ---------------------------------------------------------------
// FixSSEFormat
// ---------------------------------------------------------------

func TestFixSSEFormat_AlreadyValid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"data prefix", "data: {\"hello\":true}"},
		{"event prefix", "event: message_start"},
		{"id prefix", "id: 123"},
		{"retry prefix", "retry: 5000"},
		{"comment", ": this is a comment"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FixSSEFormat([]byte(tc.input))
			assert.Equal(t, tc.input, string(result))
		})
	}
}

func TestFixSSEFormat_MissingPrefix(t *testing.T) {
	input := []byte(`{"type":"message_delta"}`)
	result := FixSSEFormat(input)
	assert.Equal(t, `data: {"type":"message_delta"}`, string(result))
}

func TestFixSSEFormat_TrailingNewline(t *testing.T) {
	input := []byte("data: hello\r\n")
	result := FixSSEFormat(input)
	assert.Equal(t, "data: hello", string(result))
}

func TestFixSSEFormat_Empty(t *testing.T) {
	assert.Equal(t, []byte{}, FixSSEFormat([]byte("")))
}

// ---------------------------------------------------------------
// FixEncoding
// ---------------------------------------------------------------

func TestFixEncoding_ValidUTF8(t *testing.T) {
	input := []byte("hello world")
	result := FixEncoding(input)
	assert.Equal(t, "hello world", string(result))
}

func TestFixEncoding_ValidUTF8WithChinese(t *testing.T) {
	input := []byte("hello world")
	result := FixEncoding(input)
	assert.Equal(t, "hello world", string(result))
}

func TestFixEncoding_InvalidBytes(t *testing.T) {
	// 0xFF is not valid UTF-8.
	input := []byte{0x68, 0x65, 0xFF, 0x6C, 0x6F}
	result := FixEncoding(input)
	assert.True(t, len(result) > 0)
	// The invalid byte should be replaced.
	assert.NotContains(t, result, []byte{0xFF})
}

func TestFixEncoding_EmptyInput(t *testing.T) {
	result := FixEncoding([]byte{})
	assert.Equal(t, []byte{}, result)
}

func TestFixEncoding_AllInvalid(t *testing.T) {
	input := []byte{0xFF, 0xFE, 0xFD}
	result := FixEncoding(input)
	// All bytes should become replacement characters.
	assert.NotContains(t, result, []byte{0xFF})
	assert.NotContains(t, result, []byte{0xFE})
	assert.NotContains(t, result, []byte{0xFD})
}
