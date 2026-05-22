package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Service 通知服务
type Service struct {
	client     *http.Client
	queue      chan *Notification
	workers    int
	maxRetries int
	retryDelay time.Duration
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	logger     zerolog.Logger
}

// Notification 通知消息
type Notification struct {
	ID          string                 `json:"id"`
	URL         string                 `json:"url"`
	Method      string                 `json:"method"`
	Headers     map[string]string      `json:"headers"`
	Body        map[string]interface{} `json:"body"`
	Retries     int                    `json:"retries"`
	MaxRetries  int                    `json:"max_retries"`
	CreatedAt   time.Time              `json:"created_at"`
	LastAttempt time.Time              `json:"last_attempt"`
}

// NotificationResult 通知结果
type NotificationResult struct {
	Success    bool      `json:"success"`
	StatusCode int       `json:"status_code"`
	Error      string    `json:"error,omitempty"`
	Duration   int64     `json:"duration_ms"`
	Timestamp  time.Time `json:"timestamp"`
}

// Config 通知服务配置
type Config struct {
	Workers      int           // 工作协程数量
	QueueSize    int           // 队列大小
	MaxRetries   int           // 最大重试次数
	RetryDelay   time.Duration // 重试延迟
	Timeout      time.Duration // 请求超时
	EnableLogger bool          // 是否启用日志
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Workers:      5,
		QueueSize:    1000,
		MaxRetries:   3,
		RetryDelay:   5 * time.Second,
		Timeout:      30 * time.Second,
		EnableLogger: true,
	}
}

// NewService 创建通知服务
func NewService(config *Config) *Service {
	if config == nil {
		config = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	service := &Service{
		client: &http.Client{
			Timeout: config.Timeout,
		},
		queue:      make(chan *Notification, config.QueueSize),
		workers:    config.Workers,
		maxRetries: config.MaxRetries,
		retryDelay: config.RetryDelay,
		ctx:        ctx,
		cancel:     cancel,
	}

	if config.EnableLogger {
		service.logger = zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()
	}

	// 启动工作协程
	service.start()

	return service
}

// start 启动工作协程
func (s *Service) start() {
	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}
}

// worker 工作协程
func (s *Service) worker(id int) {
	defer s.wg.Done()

	if s.logger.GetLevel() != zerolog.Disabled {
		s.logger.Info().Int("worker_id", id).Msg("Notification worker started")
	}

	for {
		select {
		case <-s.ctx.Done():
			if s.logger.GetLevel() != zerolog.Disabled {
				s.logger.Info().Int("worker_id", id).Msg("Notification worker stopped")
			}
			return
		case notification := <-s.queue:
			s.processNotification(notification)
		}
	}
}

// processNotification 处理通知
func (s *Service) processNotification(notification *Notification) {
	startTime := time.Now()

	// 发送通知
	result := s.sendNotification(notification)

	// 记录结果
	if s.logger.GetLevel() != zerolog.Disabled {
		if result.Success {
			s.logger.Info().
				Str("id", notification.ID).
				Str("url", notification.URL).
				Int("status_code", result.StatusCode).
				Int64("duration_ms", result.Duration).
				Int("retries", notification.Retries).
				Msg("Notification sent successfully")
		} else {
			s.logger.Error().
				Str("id", notification.ID).
				Str("url", notification.URL).
				Str("error", result.Error).
				Int("retries", notification.Retries).
				Int("max_retries", notification.MaxRetries).
				Msg("Notification failed")
		}
	}

	// 如果失败且未达到最大重试次数，重新入队
	if !result.Success && notification.Retries < notification.MaxRetries {
		notification.Retries++
		notification.LastAttempt = time.Now()

		// 延迟后重新入队
		time.Sleep(s.retryDelay)

		select {
		case s.queue <- notification:
			if s.logger.GetLevel() != zerolog.Disabled {
				s.logger.Info().
					Str("id", notification.ID).
					Int("retry", notification.Retries).
					Int("max_retries", notification.MaxRetries).
					Msg("Notification requeued for retry")
			}
		case <-s.ctx.Done():
			return
		default:
			if s.logger.GetLevel() != zerolog.Disabled {
				s.logger.Warn().
					Str("id", notification.ID).
					Msg("Notification queue full, dropping retry")
			}
		}
	}

	_ = startTime // 避免未使用变量警告
}

// sendNotification 发送通知
func (s *Service) sendNotification(notification *Notification) *NotificationResult {
	startTime := time.Now()

	result := &NotificationResult{
		Timestamp: startTime,
	}

	// 序列化请求体
	bodyBytes, err := json.Marshal(notification.Body)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to marshal body: %v", err)
		return result
	}

	// 创建请求
	req, err := http.NewRequestWithContext(s.ctx, notification.Method, notification.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create request: %v", err)
		return result
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	for key, value := range notification.Headers {
		req.Header.Set(key, value)
	}

	// 发送请求
	resp, err := s.client.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to send request: %v", err)
		return result
	}
	defer resp.Body.Close()

	// 记录结果
	result.StatusCode = resp.StatusCode
	result.Duration = time.Since(startTime).Milliseconds()

	// 检查状态码
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Success = true
	} else {
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return result
}

// Send 发送通知（异步）
func (s *Service) Send(notification *Notification) error {
	// 检查服务是否已关闭
	select {
	case <-s.ctx.Done():
		return fmt.Errorf("notification service is shutting down")
	default:
	}

	// 设置默认值
	if notification.ID == "" {
		notification.ID = generateID()
	}
	if notification.Method == "" {
		notification.Method = "POST"
	}
	if notification.MaxRetries == 0 {
		notification.MaxRetries = s.maxRetries
	}
	if notification.CreatedAt.IsZero() {
		notification.CreatedAt = time.Now()
	}

	// 入队
	select {
	case s.queue <- notification:
		return nil
	case <-s.ctx.Done():
		return fmt.Errorf("notification service is shutting down")
	default:
		return fmt.Errorf("notification queue is full")
	}
}

// SendSync 发送通知（同步）
func (s *Service) SendSync(notification *Notification) *NotificationResult {
	// 设置默认值
	if notification.ID == "" {
		notification.ID = generateID()
	}
	if notification.Method == "" {
		notification.Method = "POST"
	}
	if notification.CreatedAt.IsZero() {
		notification.CreatedAt = time.Now()
	}

	return s.sendNotification(notification)
}

// SendBatch 批量发送通知（异步）
func (s *Service) SendBatch(notifications []*Notification) error {
	for _, notification := range notifications {
		if err := s.Send(notification); err != nil {
			return err
		}
	}
	return nil
}

// QueueSize 获取队列大小
func (s *Service) QueueSize() int {
	return len(s.queue)
}

// Close 关闭通知服务
func (s *Service) Close() error {
	// 取消上下文
	s.cancel()

	// 等待所有工作协程完成
	s.wg.Wait()

	// 关闭队列
	close(s.queue)

	if s.logger.GetLevel() != zerolog.Disabled {
		s.logger.Info().Msg("Notification service closed")
	}

	return nil
}

// Helper functions

// generateID 生成通知 ID
func generateID() string {
	return fmt.Sprintf("notif_%d", time.Now().UnixNano())
}
