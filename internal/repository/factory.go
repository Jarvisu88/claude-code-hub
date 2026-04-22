package repository

import (
	"sync"

	"github.com/uptrace/bun"
)

// Factory Repository 工厂（依赖注入容器）
// 使用 sync.Once 保证并发安全的懒加载
type Factory struct {
	db *bun.DB

	// 缓存的 Repository 实例（懒加载）
	userRepo       UserRepository
	userRepoOnce   sync.Once
	keyRepo        KeyRepository
	keyRepoOnce    sync.Once
	providerRepo   ProviderRepository
	providerOnce   sync.Once
	systemRepo     SystemSettingsRepository
	systemOnce     sync.Once
	messageRepo    MessageRequestRepository
	messageOnce    sync.Once
	statisticsRepo StatisticsRepository
	statsOnce      sync.Once
	priceRepo      ModelPriceRepository
	priceOnce      sync.Once
}

// NewFactory 创建 Repository 工厂
func NewFactory(db *bun.DB) *Factory {
	return &Factory{db: db}
}

// User 获取 User Repository（并发安全）
func (f *Factory) User() UserRepository {
	f.userRepoOnce.Do(func() {
		f.userRepo = NewUserRepository(f.db)
	})
	return f.userRepo
}

// Key 获取 Key Repository（并发安全）
func (f *Factory) Key() KeyRepository {
	f.keyRepoOnce.Do(func() {
		f.keyRepo = NewKeyRepository(f.db)
	})
	return f.keyRepo
}

// Provider 获取 Provider Repository（并发安全）
func (f *Factory) Provider() ProviderRepository {
	f.providerOnce.Do(func() {
		f.providerRepo = NewProviderRepository(f.db)
	})
	return f.providerRepo
}

// SystemSettings 获取 SystemSettings Repository（并发安全）
func (f *Factory) SystemSettings() SystemSettingsRepository {
	f.systemOnce.Do(func() {
		f.systemRepo = NewSystemSettingsRepository(f.db)
	})
	return f.systemRepo
}

// MessageRequest 获取 MessageRequest Repository（并发安全）
func (f *Factory) MessageRequest() MessageRequestRepository {
	f.messageOnce.Do(func() {
		f.messageRepo = NewMessageRequestRepository(f.db)
	})
	return f.messageRepo
}

// Statistics 获取 Statistics Repository（并发安全）
func (f *Factory) Statistics() StatisticsRepository {
	f.statsOnce.Do(func() {
		f.statisticsRepo = NewStatisticsRepository(f.db)
	})
	return f.statisticsRepo
}

// ModelPrice 获取 ModelPrice Repository（并发安全）
func (f *Factory) ModelPrice() ModelPriceRepository {
	f.priceOnce.Do(func() {
		f.priceRepo = NewModelPriceRepository(f.db)
	})
	return f.priceRepo
}

// DB 获取数据库实例
func (f *Factory) DB() *bun.DB {
	return f.db
}
