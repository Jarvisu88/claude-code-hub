package cache

import (
	"context"
	"sync"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
)

// ProviderRepository 供应商数据访问接口
type ProviderRepository interface {
	GetActiveProviders(ctx context.Context) ([]*model.Provider, error)
	GetByID(ctx context.Context, id int) (*model.Provider, error)
	GetByGroupTag(ctx context.Context, groupTag string) ([]*model.Provider, error)
	GetClaudePoolProviders(ctx context.Context) ([]*model.Provider, error)
}

// ProviderCache 供应商缓存
type ProviderCache struct {
	repo ProviderRepository
	ttl  time.Duration

	// 缓存数据
	mu                    sync.RWMutex
	activeProviders       []*model.Provider
	activeProvidersExpiry time.Time

	providerByID       map[int]*model.Provider
	providerByIDExpiry map[int]time.Time

	providersByGroupTag       map[string][]*model.Provider
	providersByGroupTagExpiry map[string]time.Time

	claudePoolProviders       []*model.Provider
	claudePoolProvidersExpiry time.Time
}

// NewProviderCache 创建供应商缓存
func NewProviderCache(repo ProviderRepository, ttl time.Duration) *ProviderCache {
	if ttl <= 0 {
		ttl = 30 * time.Second // 默认 30 秒
	}

	return &ProviderCache{
		repo:                      repo,
		ttl:                       ttl,
		providerByID:              make(map[int]*model.Provider),
		providerByIDExpiry:        make(map[int]time.Time),
		providersByGroupTag:       make(map[string][]*model.Provider),
		providersByGroupTagExpiry: make(map[string]time.Time),
	}
}

// GetActiveProviders 获取活跃供应商（带缓存）
func (c *ProviderCache) GetActiveProviders(ctx context.Context) ([]*model.Provider, error) {
	// 检查缓存
	c.mu.RLock()
	if c.activeProviders != nil && time.Now().Before(c.activeProvidersExpiry) {
		providers := c.activeProviders
		c.mu.RUnlock()
		return providers, nil
	}
	c.mu.RUnlock()

	// 缓存未命中，从数据库加载
	c.mu.Lock()
	defer c.mu.Unlock()

	// 双重检查（避免并发重复查询）
	if c.activeProviders != nil && time.Now().Before(c.activeProvidersExpiry) {
		return c.activeProviders, nil
	}

	providers, err := c.repo.GetActiveProviders(ctx)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	c.activeProviders = providers
	c.activeProvidersExpiry = time.Now().Add(c.ttl)

	return providers, nil
}

// GetByID 根据 ID 获取供应商（带缓存）
func (c *ProviderCache) GetByID(ctx context.Context, id int) (*model.Provider, error) {
	// 检查缓存
	c.mu.RLock()
	if provider, ok := c.providerByID[id]; ok {
		if time.Now().Before(c.providerByIDExpiry[id]) {
			c.mu.RUnlock()
			return provider, nil
		}
	}
	c.mu.RUnlock()

	// 缓存未命中，从数据库加载
	c.mu.Lock()
	defer c.mu.Unlock()

	// 双重检查
	if provider, ok := c.providerByID[id]; ok {
		if time.Now().Before(c.providerByIDExpiry[id]) {
			return provider, nil
		}
	}

	provider, err := c.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	c.providerByID[id] = provider
	c.providerByIDExpiry[id] = time.Now().Add(c.ttl)

	return provider, nil
}

// GetByGroupTag 根据 GroupTag 获取供应商（带缓存）
func (c *ProviderCache) GetByGroupTag(ctx context.Context, groupTag string) ([]*model.Provider, error) {
	// 检查缓存
	c.mu.RLock()
	if providers, ok := c.providersByGroupTag[groupTag]; ok {
		if time.Now().Before(c.providersByGroupTagExpiry[groupTag]) {
			c.mu.RUnlock()
			return providers, nil
		}
	}
	c.mu.RUnlock()

	// 缓存未命中，从数据库加载
	c.mu.Lock()
	defer c.mu.Unlock()

	// 双重检查
	if providers, ok := c.providersByGroupTag[groupTag]; ok {
		if time.Now().Before(c.providersByGroupTagExpiry[groupTag]) {
			return providers, nil
		}
	}

	providers, err := c.repo.GetByGroupTag(ctx, groupTag)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	c.providersByGroupTag[groupTag] = providers
	c.providersByGroupTagExpiry[groupTag] = time.Now().Add(c.ttl)

	return providers, nil
}

// GetClaudePoolProviders 获取 Claude Pool 供应商（带缓存）
func (c *ProviderCache) GetClaudePoolProviders(ctx context.Context) ([]*model.Provider, error) {
	// 检查缓存
	c.mu.RLock()
	if c.claudePoolProviders != nil && time.Now().Before(c.claudePoolProvidersExpiry) {
		providers := c.claudePoolProviders
		c.mu.RUnlock()
		return providers, nil
	}
	c.mu.RUnlock()

	// 缓存未命中，从数据库加载
	c.mu.Lock()
	defer c.mu.Unlock()

	// 双重检查
	if c.claudePoolProviders != nil && time.Now().Before(c.claudePoolProvidersExpiry) {
		return c.claudePoolProviders, nil
	}

	providers, err := c.repo.GetClaudePoolProviders(ctx)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	c.claudePoolProviders = providers
	c.claudePoolProvidersExpiry = time.Now().Add(c.ttl)

	return providers, nil
}

// Invalidate 使所有缓存失效
func (c *ProviderCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.activeProviders = nil
	c.activeProvidersExpiry = time.Time{}

	c.providerByID = make(map[int]*model.Provider)
	c.providerByIDExpiry = make(map[int]time.Time)

	c.providersByGroupTag = make(map[string][]*model.Provider)
	c.providersByGroupTagExpiry = make(map[string]time.Time)

	c.claudePoolProviders = nil
	c.claudePoolProvidersExpiry = time.Time{}
}

// InvalidateProvider 使指定供应商的缓存失效
func (c *ProviderCache) InvalidateProvider(id int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 删除单个供应商缓存
	delete(c.providerByID, id)
	delete(c.providerByIDExpiry, id)

	// 清空列表缓存（因为可能包含该供应商）
	c.activeProviders = nil
	c.activeProvidersExpiry = time.Time{}

	c.providersByGroupTag = make(map[string][]*model.Provider)
	c.providersByGroupTagExpiry = make(map[string]time.Time)

	c.claudePoolProviders = nil
	c.claudePoolProvidersExpiry = time.Time{}
}

// GetStats 获取缓存统计信息
func (c *ProviderCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CacheStats{
		TTL:                     c.ttl,
		ActiveProvidersCached:   c.activeProviders != nil && time.Now().Before(c.activeProvidersExpiry),
		ProviderByIDCount:       len(c.providerByID),
		GroupTagCount:           len(c.providersByGroupTag),
		ClaudePoolProvidersCached: c.claudePoolProviders != nil && time.Now().Before(c.claudePoolProvidersExpiry),
	}

	// 统计有效的 ID 缓存数量
	validIDCount := 0
	for id, expiry := range c.providerByIDExpiry {
		if time.Now().Before(expiry) {
			validIDCount++
		} else {
			// 清理过期缓存
			delete(c.providerByID, id)
			delete(c.providerByIDExpiry, id)
		}
	}
	stats.ValidProviderByIDCount = validIDCount

	// 统计有效的 GroupTag 缓存数量
	validGroupTagCount := 0
	for tag, expiry := range c.providersByGroupTagExpiry {
		if time.Now().Before(expiry) {
			validGroupTagCount++
		} else {
			// 清理过期缓存
			delete(c.providersByGroupTag, tag)
			delete(c.providersByGroupTagExpiry, tag)
		}
	}
	stats.ValidGroupTagCount = validGroupTagCount

	return stats
}

// CacheStats 缓存统计信息
type CacheStats struct {
	TTL                       time.Duration
	ActiveProvidersCached     bool
	ProviderByIDCount         int
	ValidProviderByIDCount    int
	GroupTagCount             int
	ValidGroupTagCount        int
	ClaudePoolProvidersCached bool
}
