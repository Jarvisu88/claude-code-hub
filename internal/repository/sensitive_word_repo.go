package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/uptrace/bun"
)

type SensitiveWordRepository interface {
	Repository
	Create(ctx context.Context, word *model.SensitiveWord) (*model.SensitiveWord, error)
	GetByID(ctx context.Context, id int) (*model.SensitiveWord, error)
	Update(ctx context.Context, word *model.SensitiveWord) (*model.SensitiveWord, error)
	Delete(ctx context.Context, id int) error
	List(ctx context.Context, opts *ListOptions) ([]*model.SensitiveWord, error)
	ListActive(ctx context.Context) ([]*model.SensitiveWord, error)
	RefreshCache(ctx context.Context) (*CacheStats, error)
	GetCacheStats(ctx context.Context) (*CacheStats, error)
}

type sensitiveWordRepository struct {
	*BaseRepository
}

func NewSensitiveWordRepository(db *bun.DB) SensitiveWordRepository {
	return &sensitiveWordRepository{BaseRepository: NewBaseRepository(db)}
}

func (r *sensitiveWordRepository) Create(ctx context.Context, word *model.SensitiveWord) (*model.SensitiveWord, error) {
	now := time.Now()
	if word.MatchType == "" {
		word.MatchType = "contains"
	}
	word.CreatedAt = now
	word.UpdatedAt = now
	_, err := r.db.NewInsert().Model(word).Returning("*").Exec(ctx)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return word, nil
}

func (r *sensitiveWordRepository) GetByID(ctx context.Context, id int) (*model.SensitiveWord, error) {
	word := new(model.SensitiveWord)
	err := r.db.NewSelect().Model(word).Where("id = ?", id).Where("deleted_at IS NULL").Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewNotFoundError("SensitiveWord")
		}
		return nil, errors.NewDatabaseError(err)
	}
	return word, nil
}

func (r *sensitiveWordRepository) Update(ctx context.Context, word *model.SensitiveWord) (*model.SensitiveWord, error) {
	word.UpdatedAt = time.Now()
	result, err := r.db.NewUpdate().
		Model(word).
		WherePK().
		Where("deleted_at IS NULL").
		Column("word", "match_type", "description", "is_enabled", "updated_at").
		Returning("*").
		Exec(ctx)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, errors.NewNotFoundError("SensitiveWord")
	}
	return word, nil
}

func (r *sensitiveWordRepository) Delete(ctx context.Context, id int) error {
	now := time.Now()
	result, err := r.db.NewUpdate().
		Model((*model.SensitiveWord)(nil)).
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
		return errors.NewNotFoundError("SensitiveWord")
	}
	return nil
}

func (r *sensitiveWordRepository) List(ctx context.Context, opts *ListOptions) ([]*model.SensitiveWord, error) {
	if opts == nil {
		opts = NewListOptions()
	}
	query := r.db.NewSelect().Model((*model.SensitiveWord)(nil))
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
	var items []*model.SensitiveWord
	if err := query.Scan(ctx, &items); err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return items, nil
}

func (r *sensitiveWordRepository) ListActive(ctx context.Context) ([]*model.SensitiveWord, error) {
	var items []*model.SensitiveWord
	if err := r.db.NewSelect().
		Model(&items).
		Where("deleted_at IS NULL").
		Where("is_enabled = ?", true).
		Order("match_type ASC", "word ASC").
		Scan(ctx); err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return items, nil
}

func (r *sensitiveWordRepository) RefreshCache(ctx context.Context) (*CacheStats, error) {
	stats, err := r.GetCacheStats(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	stats.LastRefreshedAt = &now
	return stats, nil
}

func (r *sensitiveWordRepository) GetCacheStats(ctx context.Context) (*CacheStats, error) {
	total, err := r.db.NewSelect().Model((*model.SensitiveWord)(nil)).Where("deleted_at IS NULL").Count(ctx)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	activeCount, err := r.db.NewSelect().Model((*model.SensitiveWord)(nil)).Where("deleted_at IS NULL").Where("is_enabled = ?", true).Count(ctx)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return &CacheStats{
		Resource:         "sensitive-words",
		Total:            total,
		ActiveCount:      activeCount,
		CacheImplemented: false,
		Note:             "placeholder stats derived from database rows",
	}, nil
}
