package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/uptrace/bun"
)

type RequestFilterRepository interface {
	Repository
	Create(ctx context.Context, filter *model.RequestFilter) (*model.RequestFilter, error)
	GetByID(ctx context.Context, id int) (*model.RequestFilter, error)
	Update(ctx context.Context, filter *model.RequestFilter) (*model.RequestFilter, error)
	Delete(ctx context.Context, id int) error
	List(ctx context.Context, opts *ListOptions) ([]*model.RequestFilter, error)
	ListActive(ctx context.Context) ([]*model.RequestFilter, error)
	RefreshCache(ctx context.Context) (*CacheStats, error)
	GetCacheStats(ctx context.Context) (*CacheStats, error)
}

type requestFilterRepository struct {
	*BaseRepository
}

func NewRequestFilterRepository(db *bun.DB) RequestFilterRepository {
	return &requestFilterRepository{BaseRepository: NewBaseRepository(db)}
}

func (r *requestFilterRepository) Create(ctx context.Context, filter *model.RequestFilter) (*model.RequestFilter, error) {
	now := time.Now()
	if filter.BindingType == "" {
		filter.BindingType = "global"
	}
	if filter.RuleMode == "" {
		filter.RuleMode = "simple"
	}
	if filter.ExecutionPhase == "" {
		filter.ExecutionPhase = "guard"
	}
	filter.CreatedAt = now
	filter.UpdatedAt = now

	_, err := r.db.NewInsert().
		Model(filter).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return filter, nil
}

func (r *requestFilterRepository) GetByID(ctx context.Context, id int) (*model.RequestFilter, error) {
	filter := new(model.RequestFilter)
	err := r.db.NewSelect().
		Model(filter).
		Where("id = ?", id).
		Where("deleted_at IS NULL").
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewNotFoundError("RequestFilter")
		}
		return nil, errors.NewDatabaseError(err)
	}
	return filter, nil
}

func (r *requestFilterRepository) Update(ctx context.Context, filter *model.RequestFilter) (*model.RequestFilter, error) {
	filter.UpdatedAt = time.Now()
	result, err := r.db.NewUpdate().
		Model(filter).
		WherePK().
		Where("deleted_at IS NULL").
		Column(
			"name",
			"description",
			"scope",
			"action",
			"match_type",
			"target",
			"replacement",
			"priority",
			"is_enabled",
			"binding_type",
			"provider_ids",
			"group_tags",
			"rule_mode",
			"execution_phase",
			"operations",
			"updated_at",
		).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, errors.NewNotFoundError("RequestFilter")
	}
	return filter, nil
}

func (r *requestFilterRepository) Delete(ctx context.Context, id int) error {
	now := time.Now()
	result, err := r.db.NewUpdate().
		Model((*model.RequestFilter)(nil)).
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
		return errors.NewNotFoundError("RequestFilter")
	}
	return nil
}

func (r *requestFilterRepository) List(ctx context.Context, opts *ListOptions) ([]*model.RequestFilter, error) {
	if opts == nil {
		opts = NewListOptions()
	}
	query := r.db.NewSelect().Model((*model.RequestFilter)(nil))
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
	var items []*model.RequestFilter
	if err := query.Scan(ctx, &items); err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return items, nil
}

func (r *requestFilterRepository) ListActive(ctx context.Context) ([]*model.RequestFilter, error) {
	var items []*model.RequestFilter
	if err := r.db.NewSelect().
		Model(&items).
		Where("deleted_at IS NULL").
		Where("is_enabled = ?", true).
		Order("priority ASC", "id ASC").
		Scan(ctx); err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return items, nil
}

func (r *requestFilterRepository) RefreshCache(ctx context.Context) (*CacheStats, error) {
	stats, err := r.GetCacheStats(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	stats.LastRefreshedAt = &now
	return stats, nil
}

func (r *requestFilterRepository) GetCacheStats(ctx context.Context) (*CacheStats, error) {
	total, err := r.db.NewSelect().
		Model((*model.RequestFilter)(nil)).
		Where("deleted_at IS NULL").
		Count(ctx)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	activeCount, err := r.db.NewSelect().
		Model((*model.RequestFilter)(nil)).
		Where("deleted_at IS NULL").
		Where("is_enabled = ?", true).
		Count(ctx)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return &CacheStats{
		Resource:         "request-filters",
		Total:            total,
		ActiveCount:      activeCount,
		CacheImplemented: false,
		Note:             "placeholder stats derived from database rows",
	}, nil
}
