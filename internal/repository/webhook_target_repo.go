package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/uptrace/bun"
)

// WebhookTargetRepository webhook_targets 数据访问接口。
type WebhookTargetRepository interface {
	Repository
	List(ctx context.Context) ([]*model.WebhookTarget, error)
	GetByID(ctx context.Context, id int) (*model.WebhookTarget, error)
	Create(ctx context.Context, target *model.WebhookTarget) (*model.WebhookTarget, error)
	UpdateFields(ctx context.Context, id int, fields map[string]any) (*model.WebhookTarget, error)
	Delete(ctx context.Context, id int) error
	UpdateTestResult(ctx context.Context, id int, result *model.WebhookTestResult, testedAt time.Time) (*model.WebhookTarget, error)
}

type webhookTargetRepository struct {
	*BaseRepository
}

func NewWebhookTargetRepository(db *bun.DB) WebhookTargetRepository {
	return &webhookTargetRepository{BaseRepository: NewBaseRepository(db)}
}

func (r *webhookTargetRepository) List(ctx context.Context) ([]*model.WebhookTarget, error) {
	var targets []*model.WebhookTarget
	if err := r.db.NewSelect().
		Model(&targets).
		Order("id ASC").
		Scan(ctx); err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return targets, nil
}

func (r *webhookTargetRepository) GetByID(ctx context.Context, id int) (*model.WebhookTarget, error) {
	target := new(model.WebhookTarget)
	if err := r.db.NewSelect().
		Model(target).
		Where("id = ?", id).
		Limit(1).
		Scan(ctx); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewNotFoundError("WebhookTarget")
		}
		return nil, errors.NewDatabaseError(err)
	}
	return target, nil
}

func (r *webhookTargetRepository) Create(ctx context.Context, target *model.WebhookTarget) (*model.WebhookTarget, error) {
	now := time.Now()
	target.CreatedAt = now
	target.UpdatedAt = now
	if _, err := r.db.NewInsert().Model(target).Returning("*").Exec(ctx); err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return target, nil
}

func (r *webhookTargetRepository) UpdateFields(ctx context.Context, id int, fields map[string]any) (*model.WebhookTarget, error) {
	if len(fields) == 0 {
		return r.GetByID(ctx, id)
	}
	fields["updated_at"] = time.Now()
	query := r.db.NewUpdate().
		Model((*model.WebhookTarget)(nil)).
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
		return nil, errors.NewNotFoundError("WebhookTarget")
	}
	return r.GetByID(ctx, id)
}

func (r *webhookTargetRepository) Delete(ctx context.Context, id int) error {
	return RunInTransaction(ctx, r.db, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewDelete().
			Model((*model.NotificationTargetBinding)(nil)).
			Where("target_id = ?", id).
			Exec(ctx); err != nil {
			return errors.NewDatabaseError(err)
		}
		result, err := tx.NewDelete().
			Model((*model.WebhookTarget)(nil)).
			Where("id = ?", id).
			Exec(ctx)
		if err != nil {
			return errors.NewDatabaseError(err)
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			return errors.NewNotFoundError("WebhookTarget")
		}
		return nil
	})
}

func (r *webhookTargetRepository) UpdateTestResult(ctx context.Context, id int, result *model.WebhookTestResult, testedAt time.Time) (*model.WebhookTarget, error) {
	return r.UpdateFields(ctx, id, map[string]any{
		"last_test_at":     testedAt,
		"last_test_result": result,
	})
}
