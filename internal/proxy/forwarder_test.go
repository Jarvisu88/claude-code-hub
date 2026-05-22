package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
)

// TestNewForwarder 测试创建转发器
func TestNewForwarder(t *testing.T) {
	forwarder := NewForwarder(30 * time.Second)

	if forwarder == nil {
		t.Fatal("Expected forwarder to be created")
	}

	if forwarder.timeout != 30*time.Second {
		t.Errorf("Expected timeout = 30s, got %v", forwarder.timeout)
	}
}

// TestNewForwarder_DefaultTimeout 测试默认超时
func TestNewForwarder_DefaultTimeout(t *testing.T) {
	forwarder := NewForwarder(0)

	if forwarder.timeout != 60*time.Second {
		t.Errorf("Expected default timeout = 60s, got %v", forwarder.timeout)
	}
}

// TestForward_Success 测试成功转发
func TestForward_Success(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer server.Close()

	// 创建会话
	session := NewSession(context.Background())
	session.SetProvider(&model.Provider{
		ID:           1,
		URL:          server.URL,
		Key:          "test-key",
		ProviderType: "openai",
	})
	session.SetRequest(&Request{
		Method:  "POST",
		Path:    "/test",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{"test": "data"}`),
		Stream:  false,
	})

	// 转发请求
	forwarder := NewForwarder(10 * time.Second)
	err := forwarder.Forward(context.Background(), session)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if session.Response == nil {
		t.Fatal("Expected response to be set")
	}

	if session.Response.StatusCode != http.StatusOK {
		t.Errorf("Expected status code = 200, got %d", session.Response.StatusCode)
	}

	if string(session.Response.Body) != `{"message": "success"}` {
		t.Errorf("Expected body = success, got %s", session.Response.Body)
	}
}

// TestForward_ErrorResponse 测试错误响应
func TestForward_ErrorResponse(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()

	// 创建会话
	session := NewSession(context.Background())
	session.SetProvider(&model.Provider{
		ID:      1,
		URL: server.URL,
		Key:  "test-key",
	})
	session.SetRequest(&Request{
		Method: "POST",
		Path:   "/test",
		Body:   []byte(`{}`),
		Stream: false,
	})

	// 转发请求
	forwarder := NewForwarder(10 * time.Second)
	err := forwarder.Forward(context.Background(), session)

	if err == nil {
		t.Error("Expected error for 400 response")
	}

	if session.Response == nil {
		t.Fatal("Expected response to be set")
	}

	if session.Response.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status code = 400, got %d", session.Response.StatusCode)
	}
}

// TestForward_StreamResponse 测试流式响应
func TestForward_StreamResponse(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// 发送流式数据
		for i := 0; i < 3; i++ {
			w.Write([]byte("data: chunk\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	// 创建会话
	session := NewSession(context.Background())
	session.SetProvider(&model.Provider{
		ID:  1,
		URL: server.URL,
		Key: "test-key",
	})
	session.SetRequest(&Request{
		Method: "POST",
		Path:   "/test",
		Body:   []byte(``),
		Stream: true,
	})

	// 转发请求
	forwarder := NewForwarder(10 * time.Second)
	err := forwarder.Forward(context.Background(), session)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if session.Response == nil {
		t.Fatal("Expected response to be set")
	}

	if session.Response.Stream == nil {
		t.Fatal("Expected stream to be set")
	}

	// 给 goroutine 一点时间启动
	time.Sleep(50 * time.Millisecond)

	// 读取流数据（带超时）
	chunks := 0
	timeout := time.After(2 * time.Second)

	t.Logf("Starting to read stream...")
	for {
		select {
		case data, ok := <-session.Response.Stream:
			if !ok {
				// 流已关闭
				t.Logf("Stream closed, received %d chunks", chunks)
				goto done
			}
			if len(data) > 0 {
				chunks++
				t.Logf("Received chunk %d: %d bytes", chunks, len(data))
			}
		case <-timeout:
			t.Errorf("Timeout waiting for stream chunks, received %d chunks", chunks)
			goto done
		}
	}

done:
	if chunks == 0 {
		t.Error("Expected to receive stream chunks")
	}
}

// TestForward_NoProvider 测试无供应商
func TestForward_NoProvider(t *testing.T) {
	session := NewSession(context.Background())
	session.SetRequest(&Request{
		Method: "POST",
		Path:   "/test",
		Body:   []byte(`{}`),
	})

	forwarder := NewForwarder(10 * time.Second)
	err := forwarder.Forward(context.Background(), session)

	if err == nil {
		t.Error("Expected error when provider is not set")
	}
}

// TestForward_NoRequest 测试无请求
func TestForward_NoRequest(t *testing.T) {
	session := NewSession(context.Background())
	session.SetProvider(&model.Provider{
		ID:      1,
		URL: "http://localhost",
	})

	forwarder := NewForwarder(10 * time.Second)
	err := forwarder.Forward(context.Background(), session)

	if err == nil {
		t.Error("Expected error when request is not set")
	}
}

// TestForwardWithRetry_Success 测试重试成功
func TestForwardWithRetry_Success(t *testing.T) {
	attempts := 0

	// 创建测试服务器（第一次失败，第二次成功）
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer server.Close()

	// 创建会话
	session := NewSession(context.Background())
	session.SetProvider(&model.Provider{
		ID:      1,
		URL: server.URL,
		Key:  "test-key",
	})
	session.SetRequest(&Request{
		Method: "POST",
		Path:   "/test",
		Body:   []byte(`{}`),
		Stream: false,
	})

	// 转发请求（带重试）
	forwarder := NewForwarder(10 * time.Second)
	err := forwarder.ForwardWithRetry(context.Background(), session, 2)

	if err != nil {
		t.Fatalf("Expected no error after retry, got %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

// TestBuildUpstreamRequest_Anthropic 测试构建 Anthropic 请求
func TestBuildUpstreamRequest_Anthropic(t *testing.T) {
	session := NewSession(context.Background())
	session.SetProvider(&model.Provider{
		ID:      1,
		URL: "https://api.anthropic.com",
		Key:  "test-key",
		ProviderType:    "anthropic",
	})
	session.SetRequest(&Request{
		Method:  "POST",
		Path:    "/v1/messages",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{}`),
	})

	forwarder := NewForwarder(10 * time.Second)
	req, err := forwarder.buildUpstreamRequest(context.Background(), session)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if req.Header.Get("x-api-key") != "test-key" {
		t.Error("Expected x-api-key header to be set for Anthropic")
	}
}

// TestBuildUpstreamRequest_OpenAI 测试构建 OpenAI 请求
func TestBuildUpstreamRequest_OpenAI(t *testing.T) {
	session := NewSession(context.Background())
	session.SetProvider(&model.Provider{
		ID:      1,
		URL: "https://api.openai.com",
		Key:  "test-key",
		ProviderType:    "openai",
	})
	session.SetRequest(&Request{
		Method:  "POST",
		Path:    "/v1/chat/completions",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{}`),
	})

	forwarder := NewForwarder(10 * time.Second)
	req, err := forwarder.buildUpstreamRequest(context.Background(), session)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if req.Header.Get("Authorization") != "Bearer test-key" {
		t.Error("Expected Authorization header to be set for OpenAI")
	}
}

// TestHandleStreamResponse_ReadError 测试流读取错误
func TestHandleStreamResponse_ReadError(t *testing.T) {
	// 创建一个会立即关闭的服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// 立即关闭连接
	}))
	server.Close() // 立即关闭

	session := NewSession(context.Background())
	session.SetProvider(&model.Provider{
		ID:      1,
		URL: server.URL,
		Key:  "test-key",
	})
	session.SetRequest(&Request{
		Method: "POST",
		Path:   "/test",
		Body:   []byte(`{}`),
		Stream: true,
	})

	forwarder := NewForwarder(1 * time.Second)
	err := forwarder.Forward(context.Background(), session)

	// 应该返回错误（连接被拒绝）
	if err == nil {
		t.Error("Expected error when server is closed")
	}
}
