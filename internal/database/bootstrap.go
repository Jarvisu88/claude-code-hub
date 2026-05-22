package database

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/logger"
	"github.com/quagmt/udecimal"
	"github.com/uptrace/bun"
)

func AutoMigrate(ctx context.Context, db *bun.DB) error {
	models := []any{
		(*model.User)(nil),
		(*model.Key)(nil),
		(*model.Provider)(nil),
		(*model.ProviderGroup)(nil),
		(*model.ProviderVendor)(nil),
		(*model.ProviderEndpoint)(nil),
		(*model.ProviderEndpointProbeLog)(nil),
		(*model.UsageLedger)(nil),
		(*model.AuditLog)(nil),
		(*model.SystemSettings)(nil),
		(*model.ModelPrice)(nil),
		(*model.MessageRequest)(nil),
		(*model.ErrorRule)(nil),
		(*model.RequestFilter)(nil),
		(*model.SensitiveWord)(nil),
		(*model.NotificationSettings)(nil),
		(*model.WebhookTarget)(nil),
		(*model.NotificationTargetBinding)(nil),
	}

	for _, item := range models {
		if _, err := db.NewCreateTable().Model(item).IfNotExists().Exec(ctx); err != nil {
			return err
		}
	}
	return ensureParityColumns(ctx, db)
}

func ensureParityColumns(ctx context.Context, db *bun.DB) error {
	statements := []string{
		`ALTER TABLE request_filters ADD COLUMN IF NOT EXISTS rule_mode text NOT NULL DEFAULT 'simple'`,
		`ALTER TABLE request_filters ADD COLUMN IF NOT EXISTS execution_phase text NOT NULL DEFAULT 'guard'`,
		`ALTER TABLE request_filters ADD COLUMN IF NOT EXISTS operations jsonb`,
		`ALTER TABLE request_filters ADD COLUMN IF NOT EXISTS deleted_at timestamptz`,
		`ALTER TABLE sensitive_words ADD COLUMN IF NOT EXISTS deleted_at timestamptz`,
		`ALTER TABLE error_rules ADD COLUMN IF NOT EXISTS deleted_at timestamptz`,
	}
	for _, stmt := range statements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func SeedLocalDevData(ctx context.Context, db *bun.DB, appURL string) error {
	if !shouldSeedLocalDevData() {
		return nil
	}

	enabled := true
	now := time.Now()
	var userCount int
	if count, err := db.NewSelect().Model((*model.User)(nil)).Count(ctx); err == nil {
		userCount = count
	}
	if userCount == 0 {
		user := &model.User{
			Name:           "local-dev-user",
			Role:           "user",
			IsEnabled:      &enabled,
			CreatedAt:      now,
			UpdatedAt:      now,
			DailyResetMode: string(model.DailyResetModeFixed),
			DailyResetTime: "00:00",
		}
		_, _ = db.NewInsert().Model(user).Exec(ctx)
		key := &model.Key{
			UserID:         user.ID,
			Key:            "proxy-key",
			Name:           "local-dev-key",
			IsEnabled:      &enabled,
			CanLoginWebUi:  &enabled,
			CreatedAt:      now,
			UpdatedAt:      now,
			DailyResetMode: string(model.DailyResetModeFixed),
			DailyResetTime: "00:00",
		}
		_, _ = db.NewInsert().Model(key).Exec(ctx)
	}

	if strings.TrimSpace(appURL) != "" {
		seedLocalMockProviders(ctx, db, appURL, enabled, now)
	}

	seedDefaultProviderGroup(ctx, db, now)
	seedDefaultVendor(ctx, db, appURL, now)
	seedDefaultSystemSettings(ctx, db, now)
	seedDefaultModelPrices(ctx, db, now)

	logger.Info().Msg("Local dev bootstrap completed")
	return nil
}

func seedLocalMockProviders(ctx context.Context, db *bun.DB, appURL string, enabled bool, now time.Time) {
	base := strings.TrimRight(strings.TrimSpace(appURL), "/")
	mockWebsiteURL := "https://local.mock"
	priority := 1
	priority2 := 2

	providers := []model.Provider{
		{
			Name: "local-claude-mock", URL: base + "/__mock__/v1/messages",
			WebsiteUrl: &mockWebsiteURL, Key: "provider-secret", IsEnabled: &enabled,
			Priority: &priority, ProviderType: string(model.ProviderTypeClaude),
			AllowedModels: model.ExactAllowedModelRules("claude-sonnet-4"),
			CreatedAt: now, UpdatedAt: now,
			DailyResetMode: string(model.DailyResetModeFixed), DailyResetTime: "00:00",
		},
		{
			Name: "local-codex-mock", URL: base + "/__mock__/v1/responses",
			WebsiteUrl: &mockWebsiteURL, Key: "provider-secret", IsEnabled: &enabled,
			Priority: &priority, ProviderType: string(model.ProviderTypeCodex),
			AllowedModels: model.ExactAllowedModelRules("gpt-5.4"),
			CreatedAt: now, UpdatedAt: now,
			DailyResetMode: string(model.DailyResetModeFixed), DailyResetTime: "00:00",
		},
		{
			Name: "local-openai-mock", URL: base + "/__mock__/v1/chat/completions",
			WebsiteUrl: &mockWebsiteURL, Key: "provider-secret-2", IsEnabled: &enabled,
			Priority: &priority2, ProviderType: string(model.ProviderTypeOpenAICompatible),
			AllowedModels: model.ExactAllowedModelRules("gpt-4o-mini"),
			CreatedAt: now, UpdatedAt: now,
			DailyResetMode: string(model.DailyResetModeFixed), DailyResetTime: "00:00",
		},
	}

	for _, provider := range providers {
		existing := new(model.Provider)
		err := db.NewSelect().Model(existing).Where("name = ?", provider.Name).Limit(1).Scan(ctx)
		if err == nil {
			existing.URL = provider.URL
			existing.Key = provider.Key
			existing.IsEnabled = provider.IsEnabled
			existing.Priority = provider.Priority
			existing.ProviderType = provider.ProviderType
			existing.UpdatedAt = now
			_, _ = db.NewUpdate().Model(existing).WherePK().Exec(ctx)
			continue
		}
		p := provider
		_, _ = db.NewInsert().Model(&p).Exec(ctx)
	}
}

func seedDefaultProviderGroup(ctx context.Context, db *bun.DB, now time.Time) {
	existing := new(model.ProviderGroup)
	err := db.NewSelect().Model(existing).Where("name = ?", model.DefaultProviderGroupName).Limit(1).Scan(ctx)
	if err != nil {
		group := &model.ProviderGroup{
			Name:           model.DefaultProviderGroupName,
			CostMultiplier: udecimal.MustParse("1.0"),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		_, _ = db.NewInsert().Model(group).Exec(ctx)
	}
}

func seedDefaultVendor(ctx context.Context, db *bun.DB, appURL string, now time.Time) {
	existing := new(model.ProviderVendor)
	err := db.NewSelect().Model(existing).Where("website_domain = ?", "local.mock").Limit(1).Scan(ctx)
	if err != nil {
		displayName := "Local Mock Vendor"
		existing = &model.ProviderVendor{
			WebsiteDomain: "local.mock",
			DisplayName:   &displayName,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		_, _ = db.NewInsert().Model(existing).Exec(ctx)
	}
	if existing.ID > 0 && strings.TrimSpace(appURL) != "" {
		base := strings.TrimRight(strings.TrimSpace(appURL), "/")
		endpoints := []model.ProviderEndpoint{
			{VendorID: existing.ID, ProviderType: string(model.ProviderTypeClaude), URL: base + "/__mock__/v1/messages", IsEnabled: true, CreatedAt: now, UpdatedAt: now},
			{VendorID: existing.ID, ProviderType: string(model.ProviderTypeCodex), URL: base + "/__mock__/v1/responses", IsEnabled: true, CreatedAt: now, UpdatedAt: now},
			{VendorID: existing.ID, ProviderType: string(model.ProviderTypeOpenAICompatible), URL: base + "/__mock__/v1/chat/completions", IsEnabled: true, CreatedAt: now, UpdatedAt: now},
		}
		for _, ep := range endpoints {
			e := new(model.ProviderEndpoint)
			err := db.NewSelect().Model(e).
				Where("vendor_id = ? AND provider_type = ? AND url = ?", ep.VendorID, ep.ProviderType, ep.URL).
				Limit(1).Scan(ctx)
			if err != nil {
				item := ep
				_, _ = db.NewInsert().Model(&item).Exec(ctx)
			}
		}
	}
}

func seedDefaultSystemSettings(ctx context.Context, db *bun.DB, now time.Time) {
	var count int
	if c, err := db.NewSelect().Model((*model.SystemSettings)(nil)).Count(ctx); err == nil {
		count = c
	}
	if count == 0 {
		retention := 30
		batchSize := 10000
		settings := &model.SystemSettings{
			SiteTitle:                           "Claude Code Hub",
			CurrencyDisplay:                     "USD",
			BillingModelSource:                  "original",
			CodexPriorityBillingSource:          "requested",
			CleanupRetentionDays:                &retention,
			CleanupSchedule:                     "0 2 * * *",
			CleanupBatchSize:                    &batchSize,
			EnableThinkingSignatureRectifier:    true,
			EnableThinkingBudgetRectifier:       true,
			EnableBillingHeaderRectifier:        true,
			EnableResponseInputRectifier:        true,
			EnableCodexSessionIDCompletion:      true,
			EnableClaudeMetadataUserIDInjection: true,
			EnableResponseFixer:                 true,
			IpGeoLookupEnabled:                  true,
			CreatedAt:                           now,
			UpdatedAt:                           now,
		}
		_, _ = db.NewInsert().Model(settings).Exec(ctx)
	}
}

func seedDefaultModelPrices(ctx context.Context, db *bun.DB, now time.Time) {
	var count int
	if c, err := db.NewSelect().Model((*model.ModelPrice)(nil)).Count(ctx); err == nil {
		count = c
	}
	if count == 0 {
		modeResponses := "responses"
		modeChat := "chat"
		_, _ = db.NewInsert().Model(&model.ModelPrice{
			ModelName: "gpt-5.4",
			PriceData: model.PriceData{Mode: &modeResponses},
			Source:    "manual",
			CreatedAt: now, UpdatedAt: now,
		}).Exec(ctx)
		_, _ = db.NewInsert().Model(&model.ModelPrice{
			ModelName: "gpt-4o-mini",
			PriceData: model.PriceData{Mode: &modeChat},
			Source:    "manual",
			CreatedAt: now, UpdatedAt: now,
		}).Exec(ctx)
	}
}

func shouldSeedLocalDevData() bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv("BOOTSTRAP_DEV_SEED")))
	return raw == "1" || raw == "true" || raw == "yes" || raw == "on"
}

func ResolveBootstrapAppURL() string {
	if v := strings.TrimSpace(os.Getenv("BOOTSTRAP_PROVIDER_BASE_URL")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("APP_URL")); v != "" {
		return v
	}
	port := 23000
	if raw := strings.TrimSpace(os.Getenv("PORT")); raw != "" {
		if p, err := strconv.Atoi(raw); err == nil && p > 0 {
			port = p
		}
	}
	return "http://127.0.0.1:" + strconv.Itoa(port)
}
