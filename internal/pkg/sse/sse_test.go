package sse

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// TestParseSimpleEvent 测试解析简单事件
func TestParseSimpleEvent(t *testing.T) {
	input := "data: hello world\n\n"
	events, err := ParseString(input)

	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].Data != "hello world" {
		t.Errorf("Expected data 'hello world', got '%s'", events[0].Data)
	}
}

// TestParseMultiLineData 测试解析多行数据
func TestParseMultiLineData(t *testing.T) {
	input := "data: line 1\ndata: line 2\ndata: line 3\n\n"
	events, err := ParseString(input)

	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	expected := "line 1\nline 2\nline 3"
	if events[0].Data != expected {
		t.Errorf("Expected data '%s', got '%s'", expected, events[0].Data)
	}
}

// TestParseEventWithType 测试解析带类型的事件
func TestParseEventWithType(t *testing.T) {
	input := "event: message\ndata: hello\n\n"
	events, err := ParseString(input)

	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].Event != "message" {
		t.Errorf("Expected event 'message', got '%s'", events[0].Event)
	}

	if events[0].Data != "hello" {
		t.Errorf("Expected data 'hello', got '%s'", events[0].Data)
	}
}

// TestParseEventWithID 测试解析带 ID 的事件
func TestParseEventWithID(t *testing.T) {
	input := "id: 123\ndata: hello\n\n"
	events, err := ParseString(input)

	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].ID != "123" {
		t.Errorf("Expected id '123', got '%s'", events[0].ID)
	}
}

// TestParseEventWithRetry 测试解析带重连时间的事件
func TestParseEventWithRetry(t *testing.T) {
	input := "retry: 5000\ndata: hello\n\n"
	events, err := ParseString(input)

	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].Retry != 5000 {
		t.Errorf("Expected retry 5000, got %d", events[0].Retry)
	}
}

// TestParseMultipleEvents 测试解析多个事件
func TestParseMultipleEvents(t *testing.T) {
	input := "data: event 1\n\ndata: event 2\n\ndata: event 3\n\n"
	events, err := ParseString(input)

	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("Expected 3 events, got %d", len(events))
	}

	for i, event := range events {
		expected := "event " + string(rune('1'+i))
		if event.Data != expected {
			t.Errorf("Event %d: expected data '%s', got '%s'", i, expected, event.Data)
		}
	}
}

// TestParseComments 测试解析注释
func TestParseComments(t *testing.T) {
	input := ": this is a comment\ndata: hello\n\n"
	events, err := ParseString(input)

	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].Data != "hello" {
		t.Errorf("Expected data 'hello', got '%s'", events[0].Data)
	}
}

// TestParseEmptyLines 测试解析空行
func TestParseEmptyLines(t *testing.T) {
	input := "\n\ndata: hello\n\n\n"
	events, err := ParseString(input)

	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].Data != "hello" {
		t.Errorf("Expected data 'hello', got '%s'", events[0].Data)
	}
}

// TestParseFieldWithoutColon 测试解析没有冒号的字段
func TestParseFieldWithoutColon(t *testing.T) {
	input := "data\n\n"
	events, err := ParseString(input)

	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].Data != "" {
		t.Errorf("Expected empty data, got '%s'", events[0].Data)
	}
}

// TestParseFieldWithSpace 测试解析带空格的字段
func TestParseFieldWithSpace(t *testing.T) {
	input := "data: hello world\n\n"
	events, err := ParseString(input)

	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].Data != "hello world" {
		t.Errorf("Expected data 'hello world', got '%s'", events[0].Data)
	}
}

// TestParseEmptyData 测试解析空数据
func TestParseEmptyData(t *testing.T) {
	input := "data:\n\n"
	events, err := ParseString(input)

	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].Data != "" {
		t.Errorf("Expected empty data, got '%s'", events[0].Data)
	}
}

// TestParseInvalidRetry 测试解析无效的重连时间
func TestParseInvalidRetry(t *testing.T) {
	input := "retry: invalid\ndata: hello\n\n"
	events, err := ParseString(input)

	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].Retry != 0 {
		t.Errorf("Expected retry 0 (invalid), got %d", events[0].Retry)
	}
}

// TestWriteSimpleEvent 测试写入简单事件
func TestWriteSimpleEvent(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	event := &Event{Data: "hello world"}
	if err := writer.WriteEvent(event); err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}

	expected := "data: hello world\n\n"
	if buf.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, buf.String())
	}
}

// TestWriteEventWithType 测试写入带类型的事件
func TestWriteEventWithType(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	event := &Event{
		Event: "message",
		Data:  "hello",
	}
	if err := writer.WriteEvent(event); err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}

	expected := "event: message\ndata: hello\n\n"
	if buf.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, buf.String())
	}
}

// TestWriteEventWithID 测试写入带 ID 的事件
func TestWriteEventWithID(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	event := &Event{
		ID:   "123",
		Data: "hello",
	}
	if err := writer.WriteEvent(event); err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}

	expected := "id: 123\ndata: hello\n\n"
	if buf.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, buf.String())
	}
}

// TestWriteEventWithRetry 测试写入带重连时间的事件
func TestWriteEventWithRetry(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	event := &Event{
		Retry: 5000,
		Data:  "hello",
	}
	if err := writer.WriteEvent(event); err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}

	expected := "retry: 5000\ndata: hello\n\n"
	if buf.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, buf.String())
	}
}

// TestWriteMultiLineData 测试写入多行数据
func TestWriteMultiLineData(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	event := &Event{Data: "line 1\nline 2\nline 3"}
	if err := writer.WriteEvent(event); err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}

	expected := "data: line 1\ndata: line 2\ndata: line 3\n\n"
	if buf.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, buf.String())
	}
}

// TestWriteComment 测试写入注释
func TestWriteComment(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	if err := writer.WriteComment("this is a comment"); err != nil {
		t.Fatalf("WriteComment() error = %v", err)
	}

	expected := ": this is a comment\n\n"
	if buf.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, buf.String())
	}
}

// TestWriteData 测试写入简单数据
func TestWriteData(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	if err := writer.WriteData("hello"); err != nil {
		t.Fatalf("WriteData() error = %v", err)
	}

	expected := "data: hello\n\n"
	if buf.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, buf.String())
	}
}

// TestRoundTrip 测试往返转换
func TestRoundTrip(t *testing.T) {
	original := &Event{
		Event: "message",
		ID:    "123",
		Retry: 5000,
		Data:  "hello\nworld",
	}

	// 写入
	var buf bytes.Buffer
	writer := NewWriter(&buf)
	if err := writer.WriteEvent(original); err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}

	// 读取
	parser := NewParser(&buf)
	parsed, err := parser.ReadEvent()
	if err != nil {
		t.Fatalf("ReadEvent() error = %v", err)
	}

	// 比较
	if parsed.Event != original.Event {
		t.Errorf("Event mismatch: expected '%s', got '%s'", original.Event, parsed.Event)
	}
	if parsed.ID != original.ID {
		t.Errorf("ID mismatch: expected '%s', got '%s'", original.ID, parsed.ID)
	}
	if parsed.Retry != original.Retry {
		t.Errorf("Retry mismatch: expected %d, got %d", original.Retry, parsed.Retry)
	}
	if parsed.Data != original.Data {
		t.Errorf("Data mismatch: expected '%s', got '%s'", original.Data, parsed.Data)
	}
}

// TestParseEOF 测试 EOF 处理
func TestParseEOF(t *testing.T) {
	input := "data: hello"
	parser := NewParser(strings.NewReader(input))

	event, err := parser.ReadEvent()
	if err != nil {
		t.Fatalf("ReadEvent() error = %v", err)
	}

	if event.Data != "hello" {
		t.Errorf("Expected data 'hello', got '%s'", event.Data)
	}

	// 第二次读取应该返回 EOF
	_, err = parser.ReadEvent()
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}
}

// BenchmarkParse 性能基准测试
func BenchmarkParse(b *testing.B) {
	input := "event: message\nid: 123\ndata: hello world\n\n"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseString(input)
	}
}

// BenchmarkWrite 性能基准测试
func BenchmarkWrite(b *testing.B) {
	event := &Event{
		Event: "message",
		ID:    "123",
		Data:  "hello world",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		writer := NewWriter(&buf)
		_ = writer.WriteEvent(event)
	}
}

// TestParseBytes 测试解析字节数组
func TestParseBytes(t *testing.T) {
	input := []byte("data: hello\n\n")
	events, err := ParseBytes(input)

	if err != nil {
		t.Fatalf("ParseBytes() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].Data != "hello" {
		t.Errorf("Expected data 'hello', got '%s'", events[0].Data)
	}
}

// TestParseComplexEvent 测试解析复杂事件
func TestParseComplexEvent(t *testing.T) {
	input := "event: update\nid: 456\nretry: 3000\ndata: line 1\ndata: line 2\n\n"
	events, err := ParseString(input)

	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.Event != "update" {
		t.Errorf("Expected event 'update', got '%s'", event.Event)
	}
	if event.ID != "456" {
		t.Errorf("Expected id '456', got '%s'", event.ID)
	}
	if event.Retry != 3000 {
		t.Errorf("Expected retry 3000, got %d", event.Retry)
	}
	if event.Data != "line 1\nline 2" {
		t.Errorf("Expected data 'line 1\\nline 2', got '%s'", event.Data)
	}
}

// TestWriteComplexEvent 测试写入复杂事件
func TestWriteComplexEvent(t *testing.T) {
	var buf bytes.Buffer
	writer := NewWriter(&buf)

	event := &Event{
		Event: "update",
		ID:    "456",
		Retry: 3000,
		Data:  "line 1\nline 2",
	}
	if err := writer.WriteEvent(event); err != nil {
		t.Fatalf("WriteEvent() error = %v", err)
	}

	expected := "event: update\nid: 456\nretry: 3000\ndata: line 1\ndata: line 2\n\n"
	if buf.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, buf.String())
	}
}
