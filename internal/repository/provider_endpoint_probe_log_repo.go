package repository

import (
	"context"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/uptrace/bun"
)

type ProviderEndpointProbeLogRepository interface {
	Repository
	ListByEndpoint(ctx context.Context, endpointID int, limit int) ([]*model.ProviderEndpointProbeLog, error)
	Create(ctx context.Context, log *model.ProviderEndpointProbeLog) (*model.ProviderEndpointProbeLog, error)
	DeleteOlderThan(ctx context.Context, cutoff time.Time) error
}

type providerEndpointProbeLogRepository struct {
	*BaseRepository
}

func NewProviderEndpointProbeLogRepository(db *bun.DB) ProviderEndpointProbeLogRepository {
	return &providerEndpointProbeLogRepository{BaseRepository: NewBaseRepository(db)}
}

func (r *providerEndpointProbeLogRepository) ListByEndpoint(ctx context.Context, endpointID int, limit int) ([]*model.ProviderEndpointProbeLog, error) {
	if limit <= 0 {
		limit = 50
	}
	var items []*model.ProviderEndpointProbeLog
	if err := r.db.NewSelect().
		Model(&items).
		Where("endpoint_id = ?", endpointID).
		Order("created_at DESC", "id DESC").
		Limit(limit).
		Scan(ctx); err != nil {
		return nil, appErrors.NewDatabaseError(err)
	}
	return items, nil
}

func (r *providerEndpointProbeLogRepository) Create(ctx context.Context, log *model.ProviderEndpointProbeLog) (*model.ProviderEndpointProbeLog, error) {
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	if _, err := r.db.NewInsert().Model(log).Returning("*").Exec(ctx); err != nil {
		return nil, appErrors.NewDatabaseError(err)
	}
	return log, nil
}

func (r *providerEndpointProbeLogRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) error {
	if _, err := r.db.NewDelete().
		Model((*model.ProviderEndpointProbeLog)(nil)).
		Where("created_at < ?", cutoff).
		Exec(ctx); err != nil {
		return appErrors.NewDatabaseError(err)
	}
	return nil
}
