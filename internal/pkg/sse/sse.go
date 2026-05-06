package sse

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// Event SSE 事件
type Event struct {
	// Event 事件类型
	Event string

	// Data 事件数据
	Data string

	// ID 事件 ID
	ID string

	// Retry 重连时间（毫秒）
	Retry int
}

// Parser SSE 解析器
type Parser struct {
	reader *bufio.Reader
}

// NewParser 创建 SSE 解析器
func NewParser(r io.Reader) *Parser {
	return &Parser{
		reader: bufio.NewReader(r),
	}
}

// ReadEvent 读取下一个 SSE 事件
// 返回 nil, io.EOF 表示流结束
// 返回 nil, nil 表示空事件（注释或空行）
func (p *Parser) ReadEvent() (*Event, error) {
	event := &Event{}
	var dataLines []string

	for {
		line, err := p.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF && len(line) > 0 {
				// 处理最后一行没有换行符的情况
				if err := p.parseLine(line, event, &dataLines); err != nil {
					return nil, err
				}
				// 返回事件
				if len(dataLines) > 0 || event.Event != "" || event.ID != "" {
					event.Data = strings.Join(dataLines, "\n")
					return event, nil
				}
				return nil, io.EOF
			}
			return nil, err
		}

		// 移除换行符
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		// 空行表示事件结束
		if line == "" {
			if len(dataLines) > 0 || event.Event != "" || event.ID != "" {
				event.Data = strings.Join(dataLines, "\n")
				return event, nil
			}
			// 空事件，继续读取
			continue
		}

		// 解析行
		if err := p.parseLine(line, event, &dataLines); err != nil {
			return nil, err
		}
	}
}

// parseLine 解析单行
func (p *Parser) parseLine(line string, event *Event, dataLines *[]string) error {
	// 注释行（以 : 开头）
	if strings.HasPrefix(line, ":") {
		return nil
	}

	// 查找冒号
	colonIndex := strings.Index(line, ":")
	if colonIndex == -1 {
		// 没有冒号，整行作为字段名，值为空
		field := line
		return p.processField(field, "", event, dataLines)
	}

	field := line[:colonIndex]
	value := line[colonIndex+1:]

	// 移除值开头的空格
	if len(value) > 0 && value[0] == ' ' {
		value = value[1:]
	}

	return p.processField(field, value, event, dataLines)
}

// processField 处理字段
func (p *Parser) processField(field, value string, event *Event, dataLines *[]string) error {
	switch field {
	case "event":
		event.Event = value
	case "data":
		*dataLines = append(*dataLines, value)
	case "id":
		event.ID = value
	case "retry":
		var retry int
		if _, err := fmt.Sscanf(value, "%d", &retry); err == nil {
			event.Retry = retry
		}
	}
	return nil
}

// ReadAll 读取所有事件
func (p *Parser) ReadAll() ([]*Event, error) {
	var events []*Event

	for {
		event, err := p.ReadEvent()
		if err == io.EOF {
			break
		}
		if err != nil {
			return events, err
		}
		if event != nil {
			events = append(events, event)
		}
	}

	return events, nil
}

// ParseString 解析字符串
func ParseString(s string) ([]*Event, error) {
	parser := NewParser(strings.NewReader(s))
	return parser.ReadAll()
}

// ParseBytes 解析字节数组
func ParseBytes(b []byte) ([]*Event, error) {
	parser := NewParser(bytes.NewReader(b))
	return parser.ReadAll()
}

// Writer SSE 写入器
type Writer struct {
	writer io.Writer
}

// NewWriter 创建 SSE 写入器
func NewWriter(w io.Writer) *Writer {
	return &Writer{writer: w}
}

// WriteEvent 写入事件
func (w *Writer) WriteEvent(event *Event) error {
	if event.Event != "" {
		if _, err := fmt.Fprintf(w.writer, "event: %s\n", event.Event); err != nil {
			return err
		}
	}

	if event.ID != "" {
		if _, err := fmt.Fprintf(w.writer, "id: %s\n", event.ID); err != nil {
			return err
		}
	}

	if event.Retry > 0 {
		if _, err := fmt.Fprintf(w.writer, "retry: %d\n", event.Retry); err != nil {
			return err
		}
	}

	// 写入数据（可能有多行）
	if event.Data != "" {
		lines := strings.Split(event.Data, "\n")
		for _, line := range lines {
			if _, err := fmt.Fprintf(w.writer, "data: %s\n", line); err != nil {
				return err
			}
		}
	}

	// 写入空行表示事件结束
	if _, err := fmt.Fprint(w.writer, "\n"); err != nil {
		return err
	}

	// 如果是 Flusher，立即刷新
	if flusher, ok := w.writer.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}

	return nil
}

// WriteComment 写入注释
func (w *Writer) WriteComment(comment string) error {
	if _, err := fmt.Fprintf(w.writer, ": %s\n\n", comment); err != nil {
		return err
	}

	if flusher, ok := w.writer.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}

	return nil
}

// WriteData 写入简单数据事件
func (w *Writer) WriteData(data string) error {
	return w.WriteEvent(&Event{Data: data})
}
