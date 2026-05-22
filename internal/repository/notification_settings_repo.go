package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/quagmt/udecimal"
	"github.com/uptrace/bun"
)

// NotificationSettingsRepository notification_settings 数据访问接口。
type NotificationSettingsRepository interface {
	Repository
	Get(ctx context.Context) (*model.NotificationSettings, error)
	UpdateFields(ctx context.Context, id int, fields map[string]any) (*model.NotificationSettings, error)
}

type notificationSettingsRepository struct {
	*BaseRepository
}

func NewNotificationSettingsRepository(db *bun.DB) NotificationSettingsRepository {
	return &notificationSettingsRepository{BaseRepository: NewBaseRepository(db)}
}

func (r *notificationSettingsRepository) Get(ctx context.Context) (*model.NotificationSettings, error) {
	settings := new(model.NotificationSettings)
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
	topN := 5
	checkInterval := 60
	settings = &model.NotificationSettings{
		Enabled:                false,
		UseLegacyMode:          false,
		DailyLeaderboardTime:   "09:00",
		DailyLeaderboardTopN:   &topN,
		CostAlertThreshold:     udecimal.MustParse("0.80"),
		CostAlertCheckInterval: &checkInterval,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	_, err = r.db.NewInsert().Model(settings).Returning("*").Exec(ctx)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return settings, nil
}

func (r *notificationSettingsRepository) UpdateFields(ctx context.Context, id int, fields map[string]any) (*model.NotificationSettings, error) {
	if len(fields) == 0 {
		return r.Get(ctx)
	}

	fields["updated_at"] = time.Now()
	query := r.db.NewUpdate().
		Model((*model.NotificationSettings)(nil)).
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
		return nil, errors.NewNotFoundError("NotificationSettings")
	}

	settings := new(model.NotificationSettings)
	err = r.db.NewSelect().
		Model(settings).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewNotFoundError("NotificationSettings")
		}
		return nil, errors.NewDatabaseError(err)
	}
	return settings, nil
}
