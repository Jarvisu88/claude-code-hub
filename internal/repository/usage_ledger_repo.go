package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/quagmt/udecimal"
	"github.com/uptrace/bun"
)

type UsageLedgerRepository interface {
	Repository
	Create(ctx context.Context, entry *model.UsageLedger) (*model.UsageLedger, error)
	GetByRequestID(ctx context.Context, requestID int) (*model.UsageLedger, error)
	List(ctx context.Context, opts *ListOptions) ([]*model.UsageLedger, error)
	SumUserCost(ctx context.Context, userID int, startTime, endTime *time.Time) (udecimal.Decimal, error)
	SumKeyCost(ctx context.Context, key string, startTime, endTime *time.Time) (udecimal.Decimal, error)
	SumProviderCost(ctx context.Context, finalProviderID int, startTime, endTime *time.Time) (udecimal.Decimal, error)
}

type usageLedgerRepository struct {
	*BaseRepository
}

func NewUsageLedgerRepository(db *bun.DB) UsageLedgerRepository {
	return &usageLedgerRepository{BaseRepository: NewBaseRepository(db)}
}

func (r *usageLedgerRepository) Create(ctx context.Context, entry *model.UsageLedger) (*model.UsageLedger, error) {
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	if _, err := r.db.NewInsert().Model(entry).Returning("*").Exec(ctx); err != nil {
		return nil, appErrors.NewDatabaseError(err)
	}
	return entry, nil
}

func (r *usageLedgerRepository) GetByRequestID(ctx context.Context, requestID int) (*model.UsageLedger, error) {
	item := new(model.UsageLedger)
	if err := r.db.NewSelect().Model(item).Where("request_id = ?", requestID).Limit(1).Scan(ctx); err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.NewNotFoundError("UsageLedger")
		}
		return nil, appErrors.NewDatabaseError(err)
	}
	return item, nil
}

func (r *usageLedgerRepository) List(ctx context.Context, opts *ListOptions) ([]*model.UsageLedger, error) {
	if opts == nil {
		opts = NewListOptions()
	}
	query := r.db.NewSelect().Model((*model.UsageLedger)(nil))
	if opts.OrderBy != "" {
		query = query.Order(opts.OrderBy)
	} else {
		query = query.Order("created_at DESC", "id DESC")
	}
	if opts.Pagination != nil {
		query = query.Limit(opts.Pagination.GetLimit()).Offset(opts.Pagination.GetOffset())
	}
	var items []*model.UsageLedger
	if err := query.Scan(ctx, &items); err != nil {
		return nil, appErrors.NewDatabaseError(err)
	}
	return items, nil
}

func (r *usageLedgerRepository) SumUserCost(ctx context.Context, userID int, startTime, endTime *time.Time) (udecimal.Decimal, error) {
	return r.sumCost(ctx, "user_id = ?", userID, startTime, endTime)
}

func (r *usageLedgerRepository) SumKeyCost(ctx context.Context, key string, startTime, endTime *time.Time) (udecimal.Decimal, error) {
	return r.sumCost(ctx, "key = ?", key, startTime, endTime)
}

func (r *usageLedgerRepository) SumProviderCost(ctx context.Context, finalProviderID int, startTime, endTime *time.Time) (udecimal.Decimal, error) {
	return r.sumCost(ctx, "final_provider_id = ?", finalProviderID, startTime, endTime)
}

func (r *usageLedgerRepository) sumCost(ctx context.Context, clause string, value any, startTime, endTime *time.Time) (udecimal.Decimal, error) {
	type sumRow struct {
		Total udecimal.Decimal `bun:"total"`
	}
	row := new(sumRow)
	query := r.db.NewSelect().
		Model((*model.UsageLedger)(nil)).
		ColumnExpr("COALESCE(SUM(cost_usd), 0) AS total").
		Where("blocked_by IS NULL").
		Where(clause, value)
	if startTime != nil {
		query = query.Where("created_at >= ?", *startTime)
	}
	if endTime != nil {
		query = query.Where("created_at < ?", *endTime)
	}
	if err := query.Scan(ctx, row); err != nil {
		return udecimal.Zero, appErrors.NewDatabaseError(err)
	}
	return row.Total, nil
}
