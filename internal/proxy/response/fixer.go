package response

import (
	"bytes"
	"unicode/utf8"
)

// FixTruncatedJSON attempts to repair a truncated JSON payload by closing
// any unclosed braces, brackets and strings. It is intentionally best-effort:
// the goal is to make the response parseable so we can still extract fields
// like usage from a partially-received upstream response.
func FixTruncatedJSON(body []byte) []byte {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return body
	}

	// Walk the bytes keeping a stack of open delimiters.
	type state struct {
		delim byte // '{' or '['
	}
	var stack []state
	inString := false
	escaped := false

	for i := 0; i < len(body); i++ {
		b := body[i]
		if escaped {
			escaped = false
			continue
		}
		if b == '\\' && inString {
			escaped = true
			continue
		}
		if b == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch b {
		case '{':
			stack = append(stack, state{'{'})
		case '[':
			stack = append(stack, state{'['})
		case '}':
			if len(stack) > 0 && stack[len(stack)-1].delim == '{' {
				stack = stack[:len(stack)-1]
			}
		case ']':
			if len(stack) > 0 && stack[len(stack)-1].delim == '[' {
				stack = stack[:len(stack)-1]
			}
		}
	}

	// If we ended inside a string, close it.
	if inString {
		body = append(body, '"')
	}

	// Remove trailing commas (invalid JSON).
	body = bytes.TrimRight(body, " \t\r\n,")

	// Close unclosed delimiters.
	for i := len(stack) - 1; i >= 0; i-- {
		switch stack[i].delim {
		case '{':
			body = append(body, '}')
		case '[':
			body = append(body, ']')
		}
	}

	return body
}

// FixSSEFormat normalises an SSE data line. Some providers send malformed SSE
// that is missing the "data: " prefix or has extra whitespace. This function
// ensures each non-empty line either starts with "data: ", "event: ", "id: ",
// "retry: " or ":" (comment), or gets wrapped with "data: ".
func FixSSEFormat(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	// Trim trailing whitespace / newlines.
	data = bytes.TrimRight(data, "\r\n \t")

	// Already has a valid SSE field prefix -- pass through.
	if bytes.HasPrefix(data, []byte("data:")) ||
		bytes.HasPrefix(data, []byte("event:")) ||
		bytes.HasPrefix(data, []byte("id:")) ||
		bytes.HasPrefix(data, []byte("retry:")) ||
		bytes.HasPrefix(data, []byte(":")) {
		return data
	}

	// Wrap with data: prefix.
	out := make([]byte, 0, len(data)+6)
	out = append(out, "data: "...)
	out = append(out, data...)
	return out
}

// FixEncoding replaces invalid UTF-8 sequences with the Unicode replacement
// character so downstream JSON parsers do not choke.
func FixEncoding(data []byte) []byte {
	if utf8.Valid(data) {
		return data
	}

	// Replace invalid bytes.
	var buf bytes.Buffer
	buf.Grow(len(data))
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && size <= 1 {
			buf.WriteRune('�')
			data = data[1:]
		} else {
			buf.WriteRune(r)
			data = data[size:]
		}
	}
	return buf.Bytes()
}
