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
		base := strings.TrimRight(strings.TrimSpace(appURL), "/")
		priority := 1
		priority2 := 2
		claude := model.Provider{
			Name:           "local-claude-mock",
			URL:            base + "/__mock__/v1/messages",
			Key:            "provider-secret",
			IsEnabled:      &enabled,
			Priority:       &priority,
			ProviderType:   string(model.ProviderTypeClaude),
			AllowedModels:  model.ExactAllowedModelRules("claude-sonnet-4"),
			CreatedAt:      now,
			UpdatedAt:      now,
			DailyResetMode: string(model.DailyResetModeFixed),
			DailyResetTime: "00:00",
		}
		codex := model.Provider{
			Name:           "local-codex-mock",
			URL:            base + "/__mock__/v1/responses",
			Key:            "provider-secret",
			IsEnabled:      &enabled,
			Priority:       &priority,
			ProviderType:   string(model.ProviderTypeCodex),
			AllowedModels:  model.ExactAllowedModelRules("gpt-5.4"),
			CreatedAt:      now,
			UpdatedAt:      now,
			DailyResetMode: string(model.DailyResetModeFixed),
			DailyResetTime: "00:00",
		}
		openai := model.Provider{
			Name:           "local-openai-mock",
			URL:            base + "/__mock__/v1/chat/completions",
			Key:            "provider-secret-2",
			IsEnabled:      &enabled,
			Priority:       &priority2,
			ProviderType:   string(model.ProviderTypeOpenAICompatible),
			AllowedModels:  model.ExactAllowedModelRules("gpt-4o-mini"),
			CreatedAt:      now,
			UpdatedAt:      now,
			DailyResetMode: string(model.DailyResetModeFixed),
			DailyResetTime: "00:00",
		}
		for _, provider := range []model.Provider{claude, codex, openai} {
			existing := new(model.Provider)
			err := db.NewSelect().Model(existing).Where("name = ?", provider.Name).Limit(1).Scan(ctx)
			if err == nil {
				existing.URL = provider.URL
				existing.Key = provider.Key
				existing.IsEnabled = provider.IsEnabled
				existing.Priority = provider.Priority
				existing.ProviderType = provider.ProviderType
				existing.AllowedModels = provider.AllowedModels
				existing.DailyResetMode = provider.DailyResetMode
				existing.DailyResetTime = provider.DailyResetTime
				existing.UpdatedAt = now
				_, _ = db.NewUpdate().Model(existing).WherePK().Exec(ctx)
				continue
			}
			p := provider
			_, _ = db.NewInsert().Model(&p).Exec(ctx)
		}
	}

	var settingsCount int
	if count, err := db.NewSelect().Model((*model.SystemSettings)(nil)).Count(ctx); err == nil {
		settingsCount = count
	}
	if settingsCount == 0 {
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
			ResponseFixerConfig: map[string]any{
				"fixTruncatedJson": true,
				"fixSseFormat":     true,
				"fixEncoding":      true,
				"maxJsonDepth":     200,
				"maxFixSize":       1024 * 1024,
			},
			IpGeoLookupEnabled: true,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		_, _ = db.NewInsert().Model(settings).Exec(ctx)
	}

	var priceCount int
	if count, err := db.NewSelect().Model((*model.ModelPrice)(nil)).Count(ctx); err == nil {
		priceCount = count
	}
	if priceCount == 0 {
		modeResponses := "responses"
		modeChat := "chat"
		priceDataCodex := model.PriceData{Mode: &modeResponses}
		priceDataOpenAI := model.PriceData{Mode: &modeChat}
		_, _ = db.NewInsert().Model(&model.ModelPrice{
			ModelName: "gpt-5.4",
			PriceData: priceDataCodex,
			Source:    "manual",
			CreatedAt: now,
			UpdatedAt: now,
		}).Exec(ctx)
		_, _ = db.NewInsert().Model(&model.ModelPrice{
			ModelName: "gpt-4o-mini",
			PriceData: priceDataOpenAI,
			Source:    "manual",
			CreatedAt: now,
			UpdatedAt: now,
		}).Exec(ctx)
		_ = udecimal.Zero
	}

	logger.Info().Msg("Local dev bootstrap completed")
	return nil
}

func shouldSeedLocalDevData() bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv("BOOTSTRAP_DEV_SEED")))
	switch raw {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func ResolveBootstrapAppURL() string {
	if value := strings.TrimSpace(os.Getenv("BOOTSTRAP_PROVIDER_BASE_URL")); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("APP_URL")); value != "" {
		return value
	}
	port := 23000
	if raw := strings.TrimSpace(os.Getenv("PORT")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			port = parsed
		}
	}
	return "http://127.0.0.1:" + strconv.Itoa(port)
}
