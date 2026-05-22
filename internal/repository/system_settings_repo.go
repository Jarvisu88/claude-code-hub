package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/uptrace/bun"
)

// SystemSettingsRepository system_settings 数据访问接口
type SystemSettingsRepository interface {
	Repository

	// Get 获取当前系统设置；不存在时自动创建默认记录
	Get(ctx context.Context) (*model.SystemSettings, error)

	// UpdateFields 按字段更新系统设置
	UpdateFields(ctx context.Context, id int, fields map[string]any) (*model.SystemSettings, error)
}

type systemSettingsRepository struct {
	*BaseRepository
}

func NewSystemSettingsRepository(db *bun.DB) SystemSettingsRepository {
	return &systemSettingsRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

func (r *systemSettingsRepository) Get(ctx context.Context) (*model.SystemSettings, error) {
	settings := new(model.SystemSettings)
	err := r.db.NewSelect().
		Model(settings).
		Order("id ASC").
		Limit(1).
		Scan(ctx)
	if err == nil {
		return settings, nil
	}
	if err != sql.ErrNoRows {
		return nil, errors.NewDatabaseError(err)
	}

	now := time.Now()
	settings = &model.SystemSettings{
		SiteTitle:                           "Claude Code Hub",
		CurrencyDisplay:                     "USD",
		BillingModelSource:                  "original",
		CodexPriorityBillingSource:          "requested",
		EnableHighConcurrencyMode:           false,
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
	_, err = r.db.NewInsert().
		Model(settings).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return settings, nil
}

func (r *systemSettingsRepository) UpdateFields(ctx context.Context, id int, fields map[string]any) (*model.SystemSettings, error) {
	if len(fields) == 0 {
		return r.Get(ctx)
	}

	fields["updated_at"] = time.Now()
	query := r.db.NewUpdate().
		Model((*model.SystemSettings)(nil)).
		Where("id = ?", id)

	for column, value := range fields {
		query = query.Set(column+" = ?", value)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, errors.NewNotFoundError("SystemSettings")
	}

	settings := new(model.SystemSettings)
	err = r.db.NewSelect().
		Model(settings).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewNotFoundError("SystemSettings")
		}
		return nil, errors.NewDatabaseError(err)
	}
	return settings, nil
}
