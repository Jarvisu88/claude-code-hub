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

type ProviderVendorRepository interface {
	Repository
	List(ctx context.Context) ([]*model.ProviderVendor, error)
	GetByID(ctx context.Context, id int) (*model.ProviderVendor, error)
	GetByWebsiteDomain(ctx context.Context, domain string) (*model.ProviderVendor, error)
	Create(ctx context.Context, vendor *model.ProviderVendor) (*model.ProviderVendor, error)
	UpdateFields(ctx context.Context, id int, fields map[string]any) (*model.ProviderVendor, error)
	Delete(ctx context.Context, id int) error
}

type providerVendorRepository struct {
	*BaseRepository
}

func NewProviderVendorRepository(db *bun.DB) ProviderVendorRepository {
	return &providerVendorRepository{BaseRepository: NewBaseRepository(db)}
}

func (r *providerVendorRepository) List(ctx context.Context) ([]*model.ProviderVendor, error) {
	var items []*model.ProviderVendor
	if err := r.db.NewSelect().Model(&items).Order("website_domain ASC").Scan(ctx); err != nil {
		return nil, appErrors.NewDatabaseError(err)
	}
	return items, nil
}

func (r *providerVendorRepository) GetByID(ctx context.Context, id int) (*model.ProviderVendor, error) {
	item := new(model.ProviderVendor)
	if err := r.db.NewSelect().Model(item).Where("id = ?", id).Limit(1).Scan(ctx); err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.NewNotFoundError("ProviderVendor")
		}
		return nil, appErrors.NewDatabaseError(err)
	}
	return item, nil
}

func (r *providerVendorRepository) GetByWebsiteDomain(ctx context.Context, domain string) (*model.ProviderVendor, error) {
	item := new(model.ProviderVendor)
	if err := r.db.NewSelect().Model(item).Where("website_domain = ?", strings.TrimSpace(domain)).Limit(1).Scan(ctx); err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.NewNotFoundError("ProviderVendor")
		}
		return nil, appErrors.NewDatabaseError(err)
	}
	return item, nil
}

func (r *providerVendorRepository) Create(ctx context.Context, vendor *model.ProviderVendor) (*model.ProviderVendor, error) {
	now := time.Now()
	vendor.WebsiteDomain = strings.TrimSpace(vendor.WebsiteDomain)
	vendor.CreatedAt = now
	vendor.UpdatedAt = now
	if _, err := r.db.NewInsert().Model(vendor).Returning("*").Exec(ctx); err != nil {
		return nil, appErrors.NewDatabaseError(err)
	}
	return vendor, nil
}

func (r *providerVendorRepository) UpdateFields(ctx context.Context, id int, fields map[string]any) (*model.ProviderVendor, error) {
	if len(fields) == 0 {
		return r.GetByID(ctx, id)
	}
	fields["updated_at"] = time.Now()
	query := r.db.NewUpdate().Model((*model.ProviderVendor)(nil)).Where("id = ?", id)
	for column, value := range fields {
		query = query.Set(column+" = ?", value)
	}
	result, err := query.Exec(ctx)
	if err != nil {
		return nil, appErrors.NewDatabaseError(err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, appErrors.NewNotFoundError("ProviderVendor")
	}
	return r.GetByID(ctx, id)
}

func (r *providerVendorRepository) Delete(ctx context.Context, id int) error {
	result, err := r.db.NewDelete().Model((*model.ProviderVendor)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return appErrors.NewDatabaseError(err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return appErrors.NewNotFoundError("ProviderVendor")
	}
	return nil
}
