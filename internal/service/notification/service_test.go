package notification

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestNewService 测试创建服务
func TestNewService(t *testing.T) {
	service := NewService(nil)
	if service == nil {
		t.Fatal("Expected service to be created")
	}

	if service.workers != 5 {
		t.Errorf("Expected workers = 5, got %d", service.workers)
	}

	if service.maxRetries != 3 {
		t.Errorf("Expected maxRetries = 3, got %d", service.maxRetries)
	}

	// 清理
	service.Close()
}

// TestNewService_CustomConfig 测试自定义配置
func TestNewService_CustomConfig(t *testing.T) {
	config := &Config{
		Workers:    10,
		QueueSize:  500,
		MaxRetries: 5,
		RetryDelay: 10 * time.Second,
		Timeout:    60 * time.Second,
	}

	service := NewService(config)
	if service == nil {
		t.Fatal("Expected service to be created")
	}

	if service.workers != 10 {
		t.Errorf("Expected workers = 10, got %d", service.workers)
	}

	if service.maxRetries != 5 {
		t.Errorf("Expected maxRetries = 5, got %d", service.maxRetries)
	}

	// 清理
	service.Close()
}

// TestSend_Success 测试发送成功
func TestSend_Success(t *testing.T) {
	// 创建测试服务器
	received := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// 创建服务
	service := NewService(&Config{
		Workers:      1,
		QueueSize:    10,
		MaxRetries:   3,
		RetryDelay:   100 * time.Millisecond,
		Timeout:      5 * time.Second,
		EnableLogger: false,
	})
	defer service.Close()

	// 发送通知
	notification := &Notification{
		URL:    server.URL,
		Method: "POST",
		Body: map[string]interface{}{
			"message": "test",
		},
	}

	err := service.Send(notification)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	// 等待处理
	time.Sleep(500 * time.Millisecond)

	if !received {
		t.Error("Expected notification to be received")
	}
}

// TestSendSync_Success 测试同步发送成功
func TestSendSync_Success(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// 创建服务
	service := NewService(nil)
	defer service.Close()

	// 同步发送通知
	notification := &Notification{
		URL:    server.URL,
		Method: "POST",
		Body: map[string]interface{}{
			"message": "test",
		},
	}

	result := service.SendSync(notification)

	if !result.Success {
		t.Errorf("Expected success = true, got false: %s", result.Error)
	}

	if result.StatusCode != 200 {
		t.Errorf("Expected status code = 200, got %d", result.StatusCode)
	}
}

// TestSendSync_Error 测试同步发送失败
func TestSendSync_Error(t *testing.T) {
	// 创建测试服务器（返回错误）
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// 创建服务
	service := NewService(nil)
	defer service.Close()

	// 同步发送通知
	notification := &Notification{
		URL:    server.URL,
		Method: "POST",
		Body: map[string]interface{}{
			"message": "test",
		},
	}

	result := service.SendSync(notification)

	if result.Success {
		t.Error("Expected success = false, got true")
	}

	if result.StatusCode != 500 {
		t.Errorf("Expected status code = 500, got %d", result.StatusCode)
	}
}

// TestSendBatch 测试批量发送
func TestSendBatch(t *testing.T) {
	// 创建测试服务器
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// 创建服务
	service := NewService(&Config{
		Workers:      2,
		QueueSize:    10,
		MaxRetries:   3,
		RetryDelay:   100 * time.Millisecond,
		Timeout:      5 * time.Second,
		EnableLogger: false,
	})
	defer service.Close()

	// 批量发送通知
	notifications := []*Notification{
		{
			URL:    server.URL,
			Method: "POST",
			Body:   map[string]interface{}{"id": 1},
		},
		{
			URL:    server.URL,
			Method: "POST",
			Body:   map[string]interface{}{"id": 2},
		},
		{
			URL:    server.URL,
			Method: "POST",
			Body:   map[string]interface{}{"id": 3},
		},
	}

	err := service.SendBatch(notifications)
	if err != nil {
		t.Fatalf("SendBatch() error = %v", err)
	}

	// 等待处理
	time.Sleep(1 * time.Second)

	if count != 3 {
		t.Errorf("Expected 3 notifications to be received, got %d", count)
	}
}

// TestQueueSize 测试队列大小
func TestQueueSize(t *testing.T) {
	service := NewService(&Config{
		Workers:      1,
		QueueSize:    10,
		MaxRetries:   3,
		RetryDelay:   100 * time.Millisecond,
		Timeout:      5 * time.Second,
		EnableLogger: false,
	})
	defer service.Close()

	// 初始队列大小应该为 0
	if size := service.QueueSize(); size != 0 {
		t.Errorf("Expected queue size = 0, got %d", size)
	}

	// 发送通知
	notification := &Notification{
		URL:    "http://localhost:9999",
		Method: "POST",
		Body:   map[string]interface{}{"test": true},
	}

	service.Send(notification)

	// 队列大小应该增加
	time.Sleep(10 * time.Millisecond)
	// 注意：由于工作协程会立即处理，队列大小可能为 0 或 1
}

// TestClose 测试关闭服务
func TestClose(t *testing.T) {
	service := NewService(nil)

	err := service.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// 关闭后发送应该失败
	notification := &Notification{
		URL:    "http://localhost:9999",
		Method: "POST",
		Body:   map[string]interface{}{"test": true},
	}

	err = service.Send(notification)
	if err == nil {
		t.Error("Expected error when sending after close")
	}
}

// TestTemplateRenderer 测试模板渲染器
func TestTemplateRenderer(t *testing.T) {
	renderer := NewTemplateRenderer()

	// 测试默认模板
	templates := renderer.ListTemplates()
	if len(templates) == 0 {
		t.Error("Expected default templates to be registered")
	}

	// 测试渲染
	data := &TemplateData{
		UserID:       1,
		UserName:     "test_user",
		UserEmail:    "test@example.com",
		APIKeyID:     1,
		APIKeyName:   "test_key",
		RequestID:    "req_123",
		Model:        "claude-opus-4",
		Provider:     "anthropic",
		InputTokens:  100,
		OutputTokens: 200,
		TotalTokens:  300,
		Cost:         "$0.01",
		Duration:     1000,
		StatusCode:   200,
	}

	rendered, err := renderer.Render("request_success", data)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if rendered == "" {
		t.Error("Expected non-empty rendered output")
	}

	// 检查是否包含数据
	if !contains(rendered, "test_user") {
		t.Error("Expected rendered output to contain user name")
	}

	if !contains(rendered, "claude-opus-4") {
		t.Error("Expected rendered output to contain model")
	}
}

// TestTemplateRenderer_CustomTemplate 测试自定义模板
func TestTemplateRenderer_CustomTemplate(t *testing.T) {
	renderer := NewTemplateRenderer()

	// 注册自定义模板
	customTemplate := &Template{
		Name:    "custom",
		Subject: "Custom Notification",
		Body:    "Hello {{.UserName}}, your request {{.RequestID}} is complete.",
	}

	err := renderer.RegisterTemplate(customTemplate)
	if err != nil {
		t.Fatalf("RegisterTemplate() error = %v", err)
	}

	// 渲染自定义模板
	data := &TemplateData{
		UserName:  "Alice",
		RequestID: "req_456",
	}

	rendered, err := renderer.Render("custom", data)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "Hello Alice, your request req_456 is complete."
	if rendered != expected {
		t.Errorf("Expected rendered = %s, got %s", expected, rendered)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && containsMiddle(s, substr)
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
