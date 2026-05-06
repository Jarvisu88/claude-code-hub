package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/uptrace/bun"
)

type ErrorRuleRepository interface {
	Repository
	Create(ctx context.Context, rule *model.ErrorRule) (*model.ErrorRule, error)
	GetByID(ctx context.Context, id int) (*model.ErrorRule, error)
	Update(ctx context.Context, rule *model.ErrorRule) (*model.ErrorRule, error)
	Delete(ctx context.Context, id int) error
	List(ctx context.Context, opts *ListOptions) ([]*model.ErrorRule, error)
	ListActive(ctx context.Context) ([]*model.ErrorRule, error)
	RefreshCache(ctx context.Context) (*CacheStats, error)
	GetCacheStats(ctx context.Context) (*CacheStats, error)
}

type errorRuleRepository struct {
	*BaseRepository
}

func NewErrorRuleRepository(db *bun.DB) ErrorRuleRepository {
	return &errorRuleRepository{BaseRepository: NewBaseRepository(db)}
}

func (r *errorRuleRepository) Create(ctx context.Context, rule *model.ErrorRule) (*model.ErrorRule, error) {
	now := time.Now()
	if rule.MatchType == "" {
		rule.MatchType = "regex"
	}
	rule.CreatedAt = now
	rule.UpdatedAt = now
	_, err := r.db.NewInsert().Model(rule).Returning("*").Exec(ctx)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return rule, nil
}

func (r *errorRuleRepository) GetByID(ctx context.Context, id int) (*model.ErrorRule, error) {
	rule := new(model.ErrorRule)
	err := r.db.NewSelect().Model(rule).Where("id = ?", id).Where("deleted_at IS NULL").Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewNotFoundError("ErrorRule")
		}
		return nil, errors.NewDatabaseError(err)
	}
	return rule, nil
}

func (r *errorRuleRepository) Update(ctx context.Context, rule *model.ErrorRule) (*model.ErrorRule, error) {
	rule.UpdatedAt = time.Now()
	result, err := r.db.NewUpdate().
		Model(rule).
		WherePK().
		Where("deleted_at IS NULL").
		Column(
			"pattern",
			"match_type",
			"category",
			"description",
			"override_response",
			"override_status_code",
			"is_enabled",
			"is_default",
			"priority",
			"updated_at",
		).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, errors.NewNotFoundError("ErrorRule")
	}
	return rule, nil
}

func (r *errorRuleRepository) Delete(ctx context.Context, id int) error {
	now := time.Now()
	result, err := r.db.NewUpdate().
		Model((*model.ErrorRule)(nil)).
		Set("deleted_at = ?", now).
		Set("updated_at = ?", now).
		Where("id = ?", id).
		Where("deleted_at IS NULL").
		Exec(ctx)
	if err != nil {
		return errors.NewDatabaseError(err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.NewNotFoundError("ErrorRule")
	}
	return nil
}

func (r *errorRuleRepository) List(ctx context.Context, opts *ListOptions) ([]*model.ErrorRule, error) {
	if opts == nil {
		opts = NewListOptions()
	}
	query := r.db.NewSelect().Model((*model.ErrorRule)(nil))
	if !opts.IncludeDeleted {
		query = query.Where("deleted_at IS NULL")
	}
	if opts.OrderBy != "" {
		query = query.Order(opts.OrderBy)
	} else {
		query = query.Order("created_at DESC")
	}
	if opts.Pagination != nil {
		query = query.Limit(opts.Pagination.GetLimit()).Offset(opts.Pagination.GetOffset())
	}
	var items []*model.ErrorRule
	if err := query.Scan(ctx, &items); err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return items, nil
}

func (r *errorRuleRepository) ListActive(ctx context.Context) ([]*model.ErrorRule, error) {
	var items []*model.ErrorRule
	if err := r.db.NewSelect().
		Model(&items).
		Where("deleted_at IS NULL").
		Where("is_enabled = ?", true).
		Order("priority ASC", "category ASC", "id ASC").
		Scan(ctx); err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return items, nil
}

func (r *errorRuleRepository) RefreshCache(ctx context.Context) (*CacheStats, error) {
	stats, err := r.GetCacheStats(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	stats.LastRefreshedAt = &now
	return stats, nil
}

func (r *errorRuleRepository) GetCacheStats(ctx context.Context) (*CacheStats, error) {
	total, err := r.db.NewSelect().Model((*model.ErrorRule)(nil)).Where("deleted_at IS NULL").Count(ctx)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	activeCount, err := r.db.NewSelect().Model((*model.ErrorRule)(nil)).Where("deleted_at IS NULL").Where("is_enabled = ?", true).Count(ctx)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return &CacheStats{
		Resource:         "error-rules",
		Total:            total,
		ActiveCount:      activeCount,
		CacheImplemented: false,
		Note:             "placeholder stats derived from database rows",
	}, nil
}
