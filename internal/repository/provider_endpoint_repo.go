package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/uptrace/bun"
)

type ProviderEndpointRepository interface {
	Repository
	List(ctx context.Context, opts *ListOptions) ([]*model.ProviderEndpoint, error)
	ListActiveByVendorAndType(ctx context.Context, vendorID int, providerType string) ([]*model.ProviderEndpoint, error)
	GetByID(ctx context.Context, id int) (*model.ProviderEndpoint, error)
	Create(ctx context.Context, endpoint *model.ProviderEndpoint) (*model.ProviderEndpoint, error)
	UpdateFields(ctx context.Context, id int, fields map[string]any) (*model.ProviderEndpoint, error)
	SoftDelete(ctx context.Context, id int) error
	UpdateProbeSnapshot(ctx context.Context, id int, log *model.ProviderEndpointProbeLog) (*model.ProviderEndpoint, error)
}

type providerEndpointRepository struct {
	*BaseRepository
}

func NewProviderEndpointRepository(db *bun.DB) ProviderEndpointRepository {
	return &providerEndpointRepository{BaseRepository: NewBaseRepository(db)}
}

func (r *providerEndpointRepository) List(ctx context.Context, opts *ListOptions) ([]*model.ProviderEndpoint, error) {
	if opts == nil {
		opts = NewListOptions()
	}
	query := r.db.NewSelect().Model((*model.ProviderEndpoint)(nil))
	if !opts.IncludeDeleted {
		query = query.Where("deleted_at IS NULL")
	}
	if opts.OrderBy != "" {
		query = query.Order(opts.OrderBy)
	} else {
		query = query.Order("vendor_id ASC", "provider_type ASC", "sort_order ASC", "id ASC")
	}
	if opts.Pagination != nil {
		query = query.Limit(opts.Pagination.GetLimit()).Offset(opts.Pagination.GetOffset())
	}
	var items []*model.ProviderEndpoint
	if err := query.Scan(ctx, &items); err != nil {
		return nil, appErrors.NewDatabaseError(err)
	}
	return items, nil
}

func (r *providerEndpointRepository) ListActiveByVendorAndType(ctx context.Context, vendorID int, providerType string) ([]*model.ProviderEndpoint, error) {
	var items []*model.ProviderEndpoint
	if err := r.db.NewSelect().
		Model(&items).
		Where("deleted_at IS NULL").
		Where("is_enabled = ?", true).
		Where("vendor_id = ?", vendorID).
		Where("provider_type = ?", strings.TrimSpace(providerType)).
		Order("sort_order ASC", "id ASC").
		Scan(ctx); err != nil {
		return nil, appErrors.NewDatabaseError(err)
	}
	return items, nil
}

func (r *providerEndpointRepository) GetByID(ctx context.Context, id int) (*model.ProviderEndpoint, error) {
	item := new(model.ProviderEndpoint)
	if err := r.db.NewSelect().Model(item).Where("id = ?", id).Limit(1).Scan(ctx); err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.NewNotFoundError("ProviderEndpoint")
		}
		return nil, appErrors.NewDatabaseError(err)
	}
	return item, nil
}

func (r *providerEndpointRepository) Create(ctx context.Context, endpoint *model.ProviderEndpoint) (*model.ProviderEndpoint, error) {
	now := time.Now()
	endpoint.URL = strings.TrimSpace(endpoint.URL)
	endpoint.ProviderType = strings.TrimSpace(endpoint.ProviderType)
	endpoint.CreatedAt = now
	endpoint.UpdatedAt = now
	if _, err := r.db.NewInsert().Model(endpoint).Returning("*").Exec(ctx); err != nil {
		return nil, appErrors.NewDatabaseError(err)
	}
	return endpoint, nil
}

func (r *providerEndpointRepository) UpdateFields(ctx context.Context, id int, fields map[string]any) (*model.ProviderEndpoint, error) {
	if len(fields) == 0 {
		return r.GetByID(ctx, id)
	}
	fields["updated_at"] = time.Now()
	query := r.db.NewUpdate().
		Model((*model.ProviderEndpoint)(nil)).
		Where("id = ?", id).
		Where("deleted_at IS NULL")
	for column, value := range fields {
		query = query.Set(column+" = ?", value)
	}
	result, err := query.Exec(ctx)
	if err != nil {
		return nil, appErrors.NewDatabaseError(err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, appErrors.NewNotFoundError("ProviderEndpoint")
	}
	return r.GetByID(ctx, id)
}

func (r *providerEndpointRepository) SoftDelete(ctx context.Context, id int) error {
	now := time.Now()
	result, err := r.db.NewUpdate().
		Model((*model.ProviderEndpoint)(nil)).
		Set("deleted_at = ?", now).
		Set("updated_at = ?", now).
		Where("id = ?", id).
		Where("deleted_at IS NULL").
		Exec(ctx)
	if err != nil {
		return appErrors.NewDatabaseError(err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return appErrors.NewNotFoundError("ProviderEndpoint")
	}
	return nil
}

func (r *providerEndpointRepository) UpdateProbeSnapshot(ctx context.Context, id int, log *model.ProviderEndpointProbeLog) (*model.ProviderEndpoint, error) {
	fields := map[string]any{
		"last_probed_at": time.Now(),
	}
	if log != nil {
		fields["last_probe_ok"] = log.Ok
		fields["last_probe_status_code"] = log.StatusCode
		fields["last_probe_latency_ms"] = log.LatencyMs
		fields["last_probe_error_type"] = log.ErrorType
		fields["last_probe_error_message"] = log.ErrorMessage
		if !log.CreatedAt.IsZero() {
			fields["last_probed_at"] = log.CreatedAt
		}
	}
	return r.UpdateFields(ctx, id, fields)
}
