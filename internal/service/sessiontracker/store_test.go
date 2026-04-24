package sessiontracker

import "testing"

func TestSortSessionIDsByLastSeen(t *testing.T) {
	got := sortSessionIDsByLastSeen(map[string]int64{
		"sess-b": 200,
		"sess-a": 300,
		"sess-c": 200,
	}, 2)
	if len(got) != 2 || got[0] != "sess-a" || got[1] != "sess-b" {
		t.Fatalf("unexpected sorted session ids: %+v", got)
	}
}

func TestSortSessionIDsByLastSeenUnlimitedWhenLimitNonPositive(t *testing.T) {
	got := sortSessionIDsByLastSeen(map[string]int64{
		"sess-b": 200,
		"sess-a": 300,
		"sess-c": 100,
	}, 0)
	if len(got) != 3 || got[0] != "sess-a" || got[1] != "sess-b" || got[2] != "sess-c" {
		t.Fatalf("unexpected unlimited sorted session ids: %+v", got)
	}
}

func TestParseSessionID(t *testing.T) {
	if got := parseSessionID("session:sess_123:last_seen"); got != "sess_123" {
		t.Fatalf("expected sess_123, got %q", got)
	}
	if got := parseSessionID("session:sess_123:key"); got != "" {
		t.Fatalf("expected empty parse result for non-last_seen key, got %q", got)
	}
}
