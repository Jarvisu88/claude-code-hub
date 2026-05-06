package v1

import (
	"net/http"
	"testing"
)

func TestFixTruncatedJSON(t *testing.T) {
	body := []byte(`{"id":"resp_1","status":"completed",`)
	fixed := fixTruncatedJSON(body)
	if string(fixed) != `{"id":"resp_1","status":"completed"}` {
		t.Fatalf("unexpected fixed body: %s", string(fixed))
	}
}

func TestMaybeFixResponseBody(t *testing.T) {
	handler := &Handler{}
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	fixed, changed := handler.maybeFixResponseBody(headers, []byte(`{"ok":true,`))
	if !changed || string(fixed) != `{"ok":true}` {
		t.Fatalf("expected response fixer to repair truncated json, got changed=%v body=%s", changed, string(fixed))
	}
}
