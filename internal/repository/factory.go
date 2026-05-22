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
	userRepo                     UserRepository
	userRepoOnce                 sync.Once
	keyRepo                      KeyRepository
	keyRepoOnce                  sync.Once
	providerRepo                 ProviderRepository
	providerOnce                 sync.Once
	providerGroupRepo            ProviderGroupRepository
	providerGroupRepoOnce        sync.Once
	providerVendorRepo           ProviderVendorRepository
	providerVendorRepoOnce       sync.Once
	providerEndpointRepo         ProviderEndpointRepository
	providerEndpointRepoOnce     sync.Once
	providerEndpointProbeLogRepo ProviderEndpointProbeLogRepository
	providerEndpointProbeLogOnce sync.Once
	usageLedgerRepo              UsageLedgerRepository
	usageLedgerRepoOnce          sync.Once
	auditLogRepo                 AuditLogRepository
	auditLogRepoOnce             sync.Once
	systemRepo                   SystemSettingsRepository
	systemOnce                   sync.Once
	messageRepo                  MessageRequestRepository
	messageOnce                  sync.Once
	statisticsRepo               StatisticsRepository
	statsOnce                    sync.Once
	priceRepo                    ModelPriceRepository
	priceOnce                    sync.Once
	requestFilterRepo            RequestFilterRepository
	requestFilterRepoOnce        sync.Once
	sensitiveWordRepo            SensitiveWordRepository
	sensitiveWordRepoOnce        sync.Once
	errorRuleRepo                ErrorRuleRepository
	errorRuleRepoOnce            sync.Once
	notificationSettingsRepo     NotificationSettingsRepository
	notificationSettingsRepoOnce sync.Once
	webhookTargetRepo            WebhookTargetRepository
	webhookTargetRepoOnce        sync.Once
	notificationBindingRepo      NotificationTargetBindingRepository
	notificationBindingRepoOnce  sync.Once
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

// ProviderGroup 获取 ProviderGroup Repository（并发安全）
func (f *Factory) ProviderGroup() ProviderGroupRepository {
	f.providerGroupRepoOnce.Do(func() {
		f.providerGroupRepo = NewProviderGroupRepository(f.db)
	})
	return f.providerGroupRepo
}

// ProviderVendor 获取 ProviderVendor Repository（并发安全）
func (f *Factory) ProviderVendor() ProviderVendorRepository {
	f.providerVendorRepoOnce.Do(func() {
		f.providerVendorRepo = NewProviderVendorRepository(f.db)
	})
	return f.providerVendorRepo
}

// ProviderEndpoint 获取 ProviderEndpoint Repository（并发安全）
func (f *Factory) ProviderEndpoint() ProviderEndpointRepository {
	f.providerEndpointRepoOnce.Do(func() {
		f.providerEndpointRepo = NewProviderEndpointRepository(f.db)
	})
	return f.providerEndpointRepo
}

// ProviderEndpointProbeLog 获取 ProviderEndpointProbeLog Repository（并发安全）
func (f *Factory) ProviderEndpointProbeLog() ProviderEndpointProbeLogRepository {
	f.providerEndpointProbeLogOnce.Do(func() {
		f.providerEndpointProbeLogRepo = NewProviderEndpointProbeLogRepository(f.db)
	})
	return f.providerEndpointProbeLogRepo
}

// UsageLedger 获取 UsageLedger Repository（并发安全）
func (f *Factory) UsageLedger() UsageLedgerRepository {
	f.usageLedgerRepoOnce.Do(func() {
		f.usageLedgerRepo = NewUsageLedgerRepository(f.db)
	})
	return f.usageLedgerRepo
}

// AuditLog 获取 AuditLog Repository（并发安全）
func (f *Factory) AuditLog() AuditLogRepository {
	f.auditLogRepoOnce.Do(func() {
		f.auditLogRepo = NewAuditLogRepository(f.db)
	})
	return f.auditLogRepo
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

// RequestFilter 获取请求过滤器 Repository（并发安全）
func (f *Factory) RequestFilter() RequestFilterRepository {
	f.requestFilterRepoOnce.Do(func() {
		f.requestFilterRepo = NewRequestFilterRepository(f.db)
	})
	return f.requestFilterRepo
}

// SensitiveWord 获取敏感词 Repository（并发安全）
func (f *Factory) SensitiveWord() SensitiveWordRepository {
	f.sensitiveWordRepoOnce.Do(func() {
		f.sensitiveWordRepo = NewSensitiveWordRepository(f.db)
	})
	return f.sensitiveWordRepo
}

// ErrorRule 获取错误规则 Repository（并发安全）
func (f *Factory) ErrorRule() ErrorRuleRepository {
	f.errorRuleRepoOnce.Do(func() {
		f.errorRuleRepo = NewErrorRuleRepository(f.db)
	})
	return f.errorRuleRepo
}

// NotificationSettings 获取通知设置 Repository（并发安全）
func (f *Factory) NotificationSettings() NotificationSettingsRepository {
	f.notificationSettingsRepoOnce.Do(func() {
		f.notificationSettingsRepo = NewNotificationSettingsRepository(f.db)
	})
	return f.notificationSettingsRepo
}

// WebhookTarget 获取 Webhook 目标 Repository（并发安全）
func (f *Factory) WebhookTarget() WebhookTargetRepository {
	f.webhookTargetRepoOnce.Do(func() {
		f.webhookTargetRepo = NewWebhookTargetRepository(f.db)
	})
	return f.webhookTargetRepo
}

// NotificationTargetBinding 获取通知绑定 Repository（并发安全）
func (f *Factory) NotificationTargetBinding() NotificationTargetBindingRepository {
	f.notificationBindingRepoOnce.Do(func() {
		f.notificationBindingRepo = NewNotificationTargetBindingRepository(f.db)
	})
	return f.notificationBindingRepo
}

// DB 获取数据库实例
func (f *Factory) DB() *bun.DB {
	return f.db
}
