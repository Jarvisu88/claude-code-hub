package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ding113/claude-code-hub/internal/pkg/errors"
)

// Forwarder 请求转发器
type Forwarder struct {
	client  *http.Client
	timeout time.Duration
}

// NewForwarder 创建转发器
func NewForwarder(timeout time.Duration) *Forwarder {
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return &Forwarder{
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // 不自动跟随重定向
			},
		},
		timeout: timeout,
	}
}

// Forward 转发请求
func (f *Forwarder) Forward(ctx context.Context, session *Session) error {
	if session.Provider == nil {
		return errors.NewInternalError("Provider not set")
	}

	if session.Request == nil {
		return errors.NewInternalError("Request not set")
	}

	// 构建上游请求
	upstreamReq, err := f.buildUpstreamRequest(ctx, session)
	if err != nil {
		return err
	}

	// 发送请求
	session.MarkForwardStart()
	resp, err := f.client.Do(upstreamReq)
	if err != nil {
		session.MarkForwardEnd()
		return errors.NewProxyError(fmt.Sprintf("Failed to forward request: %v", err), 0, nil)
	}

	// 处理响应
	if session.IsStream() {
		// 流式响应：不在这里关闭 Body，由 handleStreamResponse 的 goroutine 负责
		return f.handleStreamResponse(session, resp)
	}

	// 非流式响应：在函数返回时关闭 Body
	defer resp.Body.Close()
	return f.handleNormalResponse(session, resp)
}

// buildUpstreamRequest 构建上游请求
func (f *Forwarder) buildUpstreamRequest(ctx context.Context, session *Session) (*http.Request, error) {
	provider := session.Provider
	req := session.Request

	// 构建 URL
	url := provider.URL + req.Path

	// 创建请求
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, url, bytes.NewReader(req.Body))
	if err != nil {
		return nil, errors.NewInternalError(fmt.Sprintf("Failed to create request: %v", err))
	}

	// 设置请求头
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// 设置供应商 API Key
	if provider.Key != "" {
		// 根据供应商类型设置不同的认证头
		switch provider.ProviderType {
		case "anthropic":
			httpReq.Header.Set("x-api-key", provider.Key)
		case "openai":
			httpReq.Header.Set("Authorization", "Bearer "+provider.Key)
		default:
			httpReq.Header.Set("Authorization", "Bearer "+provider.Key)
		}
	}

	return httpReq, nil
}

// handleNormalResponse 处理普通响应
func (f *Forwarder) handleNormalResponse(session *Session, resp *http.Response) error {
	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		session.MarkForwardEnd()
		return errors.NewProxyError(fmt.Sprintf("Failed to read response: %v", err), resp.StatusCode, nil)
	}

	// 设置响应
	response := &Response{
		StatusCode: resp.StatusCode,
		Headers:    make(map[string]string),
		Body:       body,
	}

	// 复制响应头
	for key, values := range resp.Header {
		if len(values) > 0 {
			response.Headers[key] = values[0]
		}
	}

	session.SetResponse(response)
	session.MarkForwardEnd()

	// 检查错误状态码
	if resp.StatusCode >= 400 {
		return errors.NewProxyError(fmt.Sprintf("Upstream returned error: %d", resp.StatusCode), resp.StatusCode, nil)
	}

	return nil
}

// handleStreamResponse 处理流式响应
func (f *Forwarder) handleStreamResponse(session *Session, resp *http.Response) error {
	// 创建流式响应
	response := &Response{
		StatusCode: resp.StatusCode,
		Headers:    make(map[string]string),
		Stream:     make(chan []byte, 100),
	}

	// 复制响应头
	for key, values := range resp.Header {
		if len(values) > 0 {
			response.Headers[key] = values[0]
		}
	}

	session.SetResponse(response)

	// 启动 goroutine 读取流
	go func() {
		defer close(response.Stream)
		defer session.MarkForwardEnd()
		defer resp.Body.Close() // 关闭响应体

		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				// 复制数据到新的切片
				data := make([]byte, n)
				copy(data, buf[:n])
				response.Stream <- data
			}

			if err != nil {
				if err != io.EOF {
					session.SetError(errors.NewProxyError(fmt.Sprintf("Stream read error: %v", err), 0, nil))
				}
				break
			}
		}
	}()

	return nil
}

// ForwardWithRetry 带重试的转发
func (f *Forwarder) ForwardWithRetry(ctx context.Context, session *Session, maxRetries int) error {
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			// 等待一段时间后重试
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(i) * time.Second):
			}
		}

		err := f.Forward(ctx, session)
		if err == nil {
			return nil
		}

		lastErr = err

		// 某些错误不应该重试
		if !f.shouldRetry(err) {
			break
		}
	}

	return lastErr
}

// shouldRetry 判断是否应该重试
func (f *Forwarder) shouldRetry(err error) bool {
	// 检查是否是可重试的错误
	// 例如：网络错误、超时、5xx 错误等
	if err == nil {
		return false
	}

	// 这里可以根据错误类型判断
	// 暂时简单处理：所有错误都可以重试
	return true
}
