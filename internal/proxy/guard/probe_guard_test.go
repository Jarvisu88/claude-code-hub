package guard

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProbeGuard_Name(t *testing.T) {
	guard := NewProbeGuard()
	assert.Equal(t, "ProbeGuard", guard.Name())
}

func TestProbeGuard_Check_NoMessages(t *testing.T) {
	guard := NewProbeGuard()
	req := &Request{Messages: nil}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
}

func TestProbeGuard_Check_MultipleMessages(t *testing.T) {
	guard := NewProbeGuard()
	req := &Request{
		Messages: []RequestMessage{
			{Role: "user", Content: "foo"},
			{Role: "assistant", Content: "bar"},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Probe requires exactly one message")
}

func TestProbeGuard_Check_FooDetected(t *testing.T) {
	guard := NewProbeGuard()
	req := &Request{
		Messages: []RequestMessage{
			{Role: "user", Content: "foo"},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.ErrorIs(t, err, ErrProbeDetected)
}

func TestProbeGuard_Check_CountDetected(t *testing.T) {
	guard := NewProbeGuard()
	req := &Request{
		Messages: []RequestMessage{
			{Role: "user", Content: "count"},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.ErrorIs(t, err, ErrProbeDetected)
}

func TestProbeGuard_Check_CaseInsensitive(t *testing.T) {
	guard := NewProbeGuard()

	for _, content := range []string{"FOO", "Foo", "fOo", "COUNT", "Count"} {
		req := &Request{
			Messages: []RequestMessage{
				{Role: "user", Content: content},
			},
		}
		err := guard.Check(context.Background(), req)
		assert.ErrorIs(t, err, ErrProbeDetected, "should detect probe for %q", content)
	}
}

func TestProbeGuard_Check_TrimmedWhitespace(t *testing.T) {
	guard := NewProbeGuard()
	req := &Request{
		Messages: []RequestMessage{
			{Role: "user", Content: "  foo  "},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.ErrorIs(t, err, ErrProbeDetected)
}

func TestProbeGuard_Check_NotProbe(t *testing.T) {
	guard := NewProbeGuard()
	req := &Request{
		Messages: []RequestMessage{
			{Role: "user", Content: "Hello, how are you?"},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
}

func TestProbeGuard_Check_ContentNotString(t *testing.T) {
	guard := NewProbeGuard()
	req := &Request{
		Messages: []RequestMessage{
			{Role: "user", Content: []ContentBlock{{Type: "text", Text: "foo"}}},
		},
	}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Probe detection only works on string content")
}
