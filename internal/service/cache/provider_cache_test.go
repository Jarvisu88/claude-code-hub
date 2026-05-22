package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
)

// mockProviderRepository 模拟供应商仓库
type mockProviderRepository struct {
	activeProviders     []*model.Provider
	providersById       map[int]*model.Provider
	providersByGroupTag map[string][]*model.Provider
	claudePoolProviders []*model.Provider
	callCount           int
	err                 error
}

func newMockProviderRepository() *mockProviderRepository {
	return &mockProviderRepository{
		providersById:       make(map[int]*model.Provider),
		providersByGroupTag: make(map[string][]*model.Provider),
	}
}

func (m *mockProviderRepository) GetActiveProviders(ctx context.Context) ([]*model.Provider, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	return m.activeProviders, nil
}

func (m *mockProviderRepository) GetByID(ctx context.Context, id int) (*model.Provider, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	if provider, ok := m.providersById[id]; ok {
		return provider, nil
	}
	return nil, errors.New("provider not found")
}

func (m *mockProviderRepository) GetByGroupTag(ctx context.Context, groupTag string) ([]*model.Provider, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	if providers, ok := m.providersByGroupTag[groupTag]; ok {
		return providers, nil
	}
	return []*model.Provider{}, nil
}

func (m *mockProviderRepository) GetClaudePoolProviders(ctx context.Context) ([]*model.Provider, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	return m.claudePoolProviders, nil
}

// TestGetActiveProviders_Cache 测试活跃供应商缓存
func TestGetActiveProviders_Cache(t *testing.T) {
	repo := newMockProviderRepository()
	enabled := true
	repo.activeProviders = []*model.Provider{
		{ID: 1, Name: "Provider 1", IsEnabled: &enabled},
		{ID: 2, Name: "Provider 2", IsEnabled: &enabled},
	}

	cache := NewProviderCache(repo, 100*time.Millisecond)
	ctx := context.Background()

	// 第一次调用 - 应该查询数据库
	providers1, err := cache.GetActiveProviders(ctx)
	if err != nil {
		t.Fatalf("GetActiveProviders() error = %v", err)
	}
	if len(providers1) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(providers1))
	}
	if repo.callCount != 1 {
		t.Errorf("Expected 1 database call, got %d", repo.callCount)
	}

	// 第二次调用 - 应该使用缓存
	providers2, err := cache.GetActiveProviders(ctx)
	if err != nil {
		t.Fatalf("GetActiveProviders() error = %v", err)
	}
	if len(providers2) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(providers2))
	}
	if repo.callCount != 1 {
		t.Errorf("Expected still 1 database call (cached), got %d", repo.callCount)
	}

	// 等待缓存过期
	time.Sleep(150 * time.Millisecond)

	// 第三次调用 - 缓存过期，应该重新查询
	providers3, err := cache.GetActiveProviders(ctx)
	if err != nil {
		t.Fatalf("GetActiveProviders() error = %v", err)
	}
	if len(providers3) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(providers3))
	}
	if repo.callCount != 2 {
		t.Errorf("Expected 2 database calls (cache expired), got %d", repo.callCount)
	}
}

// TestGetByID_Cache 测试按 ID 获取供应商缓存
func TestGetByID_Cache(t *testing.T) {
	repo := newMockProviderRepository()
	enabled := true
	repo.providersById[1] = &model.Provider{ID: 1, Name: "Provider 1", IsEnabled: &enabled}
	repo.providersById[2] = &model.Provider{ID: 2, Name: "Provider 2", IsEnabled: &enabled}

	cache := NewProviderCache(repo, 100*time.Millisecond)
	ctx := context.Background()

	// 获取 Provider 1
	provider1, err := cache.GetByID(ctx, 1)
	if err != nil {
		t.Fatalf("GetByID(1) error = %v", err)
	}
	if provider1.ID != 1 {
		t.Errorf("Expected provider ID 1, got %d", provider1.ID)
	}
	if repo.callCount != 1 {
		t.Errorf("Expected 1 database call, got %d", repo.callCount)
	}

	// 再次获取 Provider 1 - 应该使用缓存
	provider1Again, err := cache.GetByID(ctx, 1)
	if err != nil {
		t.Fatalf("GetByID(1) error = %v", err)
	}
	if provider1Again.ID != 1 {
		t.Errorf("Expected provider ID 1, got %d", provider1Again.ID)
	}
	if repo.callCount != 1 {
		t.Errorf("Expected still 1 database call (cached), got %d", repo.callCount)
	}

	// 获取 Provider 2 - 不同的 ID，应该查询数据库
	provider2, err := cache.GetByID(ctx, 2)
	if err != nil {
		t.Fatalf("GetByID(2) error = %v", err)
	}
	if provider2.ID != 2 {
		t.Errorf("Expected provider ID 2, got %d", provider2.ID)
	}
	if repo.callCount != 2 {
		t.Errorf("Expected 2 database calls, got %d", repo.callCount)
	}
}

// TestGetByGroupTag_Cache 测试按 GroupTag 获取供应商缓存
func TestGetByGroupTag_Cache(t *testing.T) {
	repo := newMockProviderRepository()
	enabled := true
	repo.providersByGroupTag["premium"] = []*model.Provider{
		{ID: 1, Name: "Premium 1", IsEnabled: &enabled},
		{ID: 2, Name: "Premium 2", IsEnabled: &enabled},
	}
	repo.providersByGroupTag["standard"] = []*model.Provider{
		{ID: 3, Name: "Standard 1", IsEnabled: &enabled},
	}

	cache := NewProviderCache(repo, 100*time.Millisecond)
	ctx := context.Background()

	// 获取 premium 组
	premiumProviders, err := cache.GetByGroupTag(ctx, "premium")
	if err != nil {
		t.Fatalf("GetByGroupTag(premium) error = %v", err)
	}
	if len(premiumProviders) != 2 {
		t.Errorf("Expected 2 premium providers, got %d", len(premiumProviders))
	}
	if repo.callCount != 1 {
		t.Errorf("Expected 1 database call, got %d", repo.callCount)
	}

	// 再次获取 premium 组 - 应该使用缓存
	premiumProvidersAgain, err := cache.GetByGroupTag(ctx, "premium")
	if err != nil {
		t.Fatalf("GetByGroupTag(premium) error = %v", err)
	}
	if len(premiumProvidersAgain) != 2 {
		t.Errorf("Expected 2 premium providers, got %d", len(premiumProvidersAgain))
	}
	if repo.callCount != 1 {
		t.Errorf("Expected still 1 database call (cached), got %d", repo.callCount)
	}

	// 获取 standard 组 - 不同的 tag，应该查询数据库
	standardProviders, err := cache.GetByGroupTag(ctx, "standard")
	if err != nil {
		t.Fatalf("GetByGroupTag(standard) error = %v", err)
	}
	if len(standardProviders) != 1 {
		t.Errorf("Expected 1 standard provider, got %d", len(standardProviders))
	}
	if repo.callCount != 2 {
		t.Errorf("Expected 2 database calls, got %d", repo.callCount)
	}
}

// TestGetClaudePoolProviders_Cache 测试 Claude Pool 供应商缓存
func TestGetClaudePoolProviders_Cache(t *testing.T) {
	repo := newMockProviderRepository()
	enabled := true
	repo.claudePoolProviders = []*model.Provider{
		{ID: 1, Name: "Claude Pool 1", IsEnabled: &enabled, JoinClaudePool: true},
		{ID: 2, Name: "Claude Pool 2", IsEnabled: &enabled, JoinClaudePool: true},
	}

	cache := NewProviderCache(repo, 100*time.Millisecond)
	ctx := context.Background()

	// 第一次调用
	providers1, err := cache.GetClaudePoolProviders(ctx)
	if err != nil {
		t.Fatalf("GetClaudePoolProviders() error = %v", err)
	}
	if len(providers1) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(providers1))
	}
	if repo.callCount != 1 {
		t.Errorf("Expected 1 database call, got %d", repo.callCount)
	}

	// 第二次调用 - 应该使用缓存
	providers2, err := cache.GetClaudePoolProviders(ctx)
	if err != nil {
		t.Fatalf("GetClaudePoolProviders() error = %v", err)
	}
	if len(providers2) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(providers2))
	}
	if repo.callCount != 1 {
		t.Errorf("Expected still 1 database call (cached), got %d", repo.callCount)
	}
}

// TestInvalidate 测试缓存失效
func TestInvalidate(t *testing.T) {
	repo := newMockProviderRepository()
	enabled := true
	repo.activeProviders = []*model.Provider{
		{ID: 1, Name: "Provider 1", IsEnabled: &enabled},
	}

	cache := NewProviderCache(repo, 1*time.Hour) // 长 TTL
	ctx := context.Background()

	// 第一次调用
	_, err := cache.GetActiveProviders(ctx)
	if err != nil {
		t.Fatalf("GetActiveProviders() error = %v", err)
	}
	if repo.callCount != 1 {
		t.Errorf("Expected 1 database call, got %d", repo.callCount)
	}

	// 第二次调用 - 应该使用缓存
	_, err = cache.GetActiveProviders(ctx)
	if err != nil {
		t.Fatalf("GetActiveProviders() error = %v", err)
	}
	if repo.callCount != 1 {
		t.Errorf("Expected still 1 database call (cached), got %d", repo.callCount)
	}

	// 使缓存失效
	cache.Invalidate()

	// 第三次调用 - 缓存已失效，应该重新查询
	_, err = cache.GetActiveProviders(ctx)
	if err != nil {
		t.Fatalf("GetActiveProviders() error = %v", err)
	}
	if repo.callCount != 2 {
		t.Errorf("Expected 2 database calls (cache invalidated), got %d", repo.callCount)
	}
}

// TestInvalidateProvider 测试单个供应商缓存失效
func TestInvalidateProvider(t *testing.T) {
	repo := newMockProviderRepository()
	enabled := true
	repo.providersById[1] = &model.Provider{ID: 1, Name: "Provider 1", IsEnabled: &enabled}

	cache := NewProviderCache(repo, 1*time.Hour) // 长 TTL
	ctx := context.Background()

	// 第一次调用
	_, err := cache.GetByID(ctx, 1)
	if err != nil {
		t.Fatalf("GetByID(1) error = %v", err)
	}
	if repo.callCount != 1 {
		t.Errorf("Expected 1 database call, got %d", repo.callCount)
	}

	// 第二次调用 - 应该使用缓存
	_, err = cache.GetByID(ctx, 1)
	if err != nil {
		t.Fatalf("GetByID(1) error = %v", err)
	}
	if repo.callCount != 1 {
		t.Errorf("Expected still 1 database call (cached), got %d", repo.callCount)
	}

	// 使 Provider 1 的缓存失效
	cache.InvalidateProvider(1)

	// 第三次调用 - 缓存已失效，应该重新查询
	_, err = cache.GetByID(ctx, 1)
	if err != nil {
		t.Fatalf("GetByID(1) error = %v", err)
	}
	if repo.callCount != 2 {
		t.Errorf("Expected 2 database calls (cache invalidated), got %d", repo.callCount)
	}
}

// TestGetStats 测试缓存统计
func TestGetStats(t *testing.T) {
	repo := newMockProviderRepository()
	enabled := true
	repo.activeProviders = []*model.Provider{
		{ID: 1, Name: "Provider 1", IsEnabled: &enabled},
	}
	repo.providersById[1] = &model.Provider{ID: 1, Name: "Provider 1", IsEnabled: &enabled}
	repo.providersById[2] = &model.Provider{ID: 2, Name: "Provider 2", IsEnabled: &enabled}
	repo.providersByGroupTag["premium"] = []*model.Provider{
		{ID: 1, Name: "Premium 1", IsEnabled: &enabled},
	}

	cache := NewProviderCache(repo, 100*time.Millisecond)
	ctx := context.Background()

	// 填充缓存
	_, _ = cache.GetActiveProviders(ctx)
	_, _ = cache.GetByID(ctx, 1)
	_, _ = cache.GetByID(ctx, 2)
	_, _ = cache.GetByGroupTag(ctx, "premium")

	// 获取统计信息
	stats := cache.GetStats()

	if stats.TTL != 100*time.Millisecond {
		t.Errorf("Expected TTL 100ms, got %v", stats.TTL)
	}
	if !stats.ActiveProvidersCached {
		t.Error("Expected ActiveProvidersCached to be true")
	}
	if stats.ValidProviderByIDCount != 2 {
		t.Errorf("Expected 2 valid provider by ID, got %d", stats.ValidProviderByIDCount)
	}
	if stats.ValidGroupTagCount != 1 {
		t.Errorf("Expected 1 valid group tag, got %d", stats.ValidGroupTagCount)
	}
}

// TestConcurrentAccess 测试并发访问
func TestConcurrentAccess(t *testing.T) {
	repo := newMockProviderRepository()
	enabled := true
	repo.activeProviders = []*model.Provider{
		{ID: 1, Name: "Provider 1", IsEnabled: &enabled},
	}

	cache := NewProviderCache(repo, 1*time.Second)
	ctx := context.Background()

	// 并发访问
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = cache.GetActiveProviders(ctx)
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 应该只查询一次数据库（其他请求使用缓存）
	if repo.callCount > 2 {
		t.Errorf("Expected at most 2 database calls (due to race), got %d", repo.callCount)
	}
}

// TestDefaultTTL 测试默认 TTL
func TestDefaultTTL(t *testing.T) {
	repo := newMockProviderRepository()
	cache := NewProviderCache(repo, 0) // TTL = 0

	stats := cache.GetStats()
	if stats.TTL != 30*time.Second {
		t.Errorf("Expected default TTL 30s, got %v", stats.TTL)
	}
}

// BenchmarkGetActiveProviders 性能基准测试
func BenchmarkGetActiveProviders(b *testing.B) {
	repo := newMockProviderRepository()
	enabled := true
	repo.activeProviders = []*model.Provider{
		{ID: 1, Name: "Provider 1", IsEnabled: &enabled},
		{ID: 2, Name: "Provider 2", IsEnabled: &enabled},
	}

	cache := NewProviderCache(repo, 1*time.Hour)
	ctx := context.Background()

	// 预热缓存
	_, _ = cache.GetActiveProviders(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.GetActiveProviders(ctx)
	}
}
