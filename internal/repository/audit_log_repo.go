package repository

import (
	"context"
	"database/sql"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/uptrace/bun"
)

type AuditLogListOptions struct {
	Filter   *model.AuditLogFilter
	Cursor   *model.AuditLogCursor
	PageSize int
}

type AuditLogRepository interface {
	Repository
	Create(ctx context.Context, entry *model.AuditLog) (*model.AuditLog, error)
	CreateAsync(ctx context.Context, entry *model.AuditLog) error
	GetByID(ctx context.Context, id int) (*model.AuditLog, error)
	List(ctx context.Context, options *AuditLogListOptions) ([]*model.AuditLog, *model.AuditLogCursor, error)
	Count(ctx context.Context, filter *model.AuditLogFilter) (int, error)
}

type auditLogRepository struct {
	*BaseRepository
}

func NewAuditLogRepository(db *bun.DB) AuditLogRepository {
	return &auditLogRepository{BaseRepository: NewBaseRepository(db)}
}

func (r *auditLogRepository) Create(ctx context.Context, entry *model.AuditLog) (*model.AuditLog, error) {
	if _, err := r.db.NewInsert().Model(entry).Returning("*").Exec(ctx); err != nil {
		return nil, appErrors.NewDatabaseError(err)
	}
	return entry, nil
}

func (r *auditLogRepository) CreateAsync(ctx context.Context, entry *model.AuditLog) error {
	_, err := r.Create(ctx, entry)
	if err != nil {
		return err
	}
	return nil
}

func (r *auditLogRepository) GetByID(ctx context.Context, id int) (*model.AuditLog, error) {
	item := new(model.AuditLog)
	if err := r.db.NewSelect().Model(item).Where("id = ?", id).Limit(1).Scan(ctx); err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.NewNotFoundError("AuditLog")
		}
		return nil, appErrors.NewDatabaseError(err)
	}
	return item, nil
}

func (r *auditLogRepository) List(ctx context.Context, options *AuditLogListOptions) ([]*model.AuditLog, *model.AuditLogCursor, error) {
	pageSize := 50
	if options != nil && options.PageSize > 0 {
		pageSize = options.PageSize
	}
	if pageSize > 500 {
		pageSize = 500
	}
	query := r.db.NewSelect().Model((*model.AuditLog)(nil))
	if options != nil && options.Filter != nil {
		query = applyAuditLogFilter(query, options.Filter)
	}
	if options != nil && options.Cursor != nil {
		query = query.WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.WhereOr("created_at < ?", options.Cursor.CreatedAt).
				WhereOr("created_at = ? AND id < ?", options.Cursor.CreatedAt, options.Cursor.ID)
		})
	}
	query = query.Order("created_at DESC", "id DESC").Limit(pageSize + 1)
	var rows []*model.AuditLog
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, nil, appErrors.NewDatabaseError(err)
	}
	var nextCursor *model.AuditLogCursor
	if len(rows) > pageSize {
		last := rows[pageSize-1]
		nextCursor = &model.AuditLogCursor{CreatedAt: last.CreatedAt, ID: last.ID}
		rows = rows[:pageSize]
	}
	return rows, nextCursor, nil
}

func (r *auditLogRepository) Count(ctx context.Context, filter *model.AuditLogFilter) (int, error) {
	query := r.db.NewSelect().Model((*model.AuditLog)(nil))
	if filter != nil {
		query = applyAuditLogFilter(query, filter)
	}
	count, err := query.Count(ctx)
	if err != nil {
		return 0, appErrors.NewDatabaseError(err)
	}
	return count, nil
}

func applyAuditLogFilter(query *bun.SelectQuery, filter *model.AuditLogFilter) *bun.SelectQuery {
	if filter == nil {
		return query
	}
	if filter.Category != nil && *filter.Category != "" {
		query = query.Where("action_category = ?", *filter.Category)
	}
	if filter.ActionType != nil && *filter.ActionType != "" {
		query = query.Where("action_type = ?", *filter.ActionType)
	}
	if filter.OperatorUserID != nil {
		query = query.Where("operator_user_id = ?", *filter.OperatorUserID)
	}
	if filter.OperatorIP != nil && *filter.OperatorIP != "" {
		query = query.Where("operator_ip = ?", *filter.OperatorIP)
	}
	if filter.TargetType != nil && *filter.TargetType != "" {
		query = query.Where("target_type = ?", *filter.TargetType)
	}
	if filter.TargetID != nil && *filter.TargetID != "" {
		query = query.Where("target_id = ?", *filter.TargetID)
	}
	if filter.Success != nil {
		query = query.Where("success = ?", *filter.Success)
	}
	if filter.From != nil {
		query = query.Where("created_at >= ?", *filter.From)
	}
	if filter.To != nil {
		query = query.Where("created_at <= ?", *filter.To)
	}
	return query
}
