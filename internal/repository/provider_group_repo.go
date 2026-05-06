package repository

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/quagmt/udecimal"
	"github.com/uptrace/bun"
)

type ProviderGroupRepository interface {
	Repository
	List(ctx context.Context) ([]*model.ProviderGroup, error)
	GetByID(ctx context.Context, id int) (*model.ProviderGroup, error)
	GetByName(ctx context.Context, name string) (*model.ProviderGroup, error)
	Create(ctx context.Context, group *model.ProviderGroup) (*model.ProviderGroup, error)
	UpdateFields(ctx context.Context, id int, fields map[string]any) (*model.ProviderGroup, error)
	Delete(ctx context.Context, id int) error
	EnsureExists(ctx context.Context, names []string) error
	GetCostMultiplier(ctx context.Context, rawGroupString string) (udecimal.Decimal, error)
	InvalidateCache()
}

type providerGroupRepository struct {
	*BaseRepository

	multiplierMu    sync.RWMutex
	multiplierCache map[string]cacheEntry
}

type cacheEntry struct {
	value     udecimal.Decimal
	expiresAt time.Time
}

const providerGroupCacheTTL = 60 * time.Second

func NewProviderGroupRepository(db *bun.DB) ProviderGroupRepository {
	return &providerGroupRepository{
		BaseRepository:  NewBaseRepository(db),
		multiplierCache: make(map[string]cacheEntry),
	}
}

func (r *providerGroupRepository) List(ctx context.Context) ([]*model.ProviderGroup, error) {
	var items []*model.ProviderGroup
	if err := r.db.NewSelect().
		Model(&items).
		OrderExpr("CASE WHEN name = ? THEN 0 ELSE 1 END", model.DefaultProviderGroupName).
		Order("name ASC").
		Scan(ctx); err != nil {
		return nil, appErrors.NewDatabaseError(err)
	}
	return items, nil
}

func (r *providerGroupRepository) GetByID(ctx context.Context, id int) (*model.ProviderGroup, error) {
	item := new(model.ProviderGroup)
	if err := r.db.NewSelect().Model(item).Where("id = ?", id).Limit(1).Scan(ctx); err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.NewNotFoundError("ProviderGroup")
		}
		return nil, appErrors.NewDatabaseError(err)
	}
	return item, nil
}

func (r *providerGroupRepository) GetByName(ctx context.Context, name string) (*model.ProviderGroup, error) {
	item := new(model.ProviderGroup)
	if err := r.db.NewSelect().Model(item).Where("name = ?", strings.TrimSpace(name)).Limit(1).Scan(ctx); err != nil {
		if err == sql.ErrNoRows {
			return nil, appErrors.NewNotFoundError("ProviderGroup")
		}
		return nil, appErrors.NewDatabaseError(err)
	}
	return item, nil
}

func (r *providerGroupRepository) Create(ctx context.Context, group *model.ProviderGroup) (*model.ProviderGroup, error) {
	group.Normalize()
	now := time.Now()
	group.Name = strings.TrimSpace(group.Name)
	group.CreatedAt = now
	group.UpdatedAt = now
	if _, err := r.db.NewInsert().Model(group).Returning("*").Exec(ctx); err != nil {
		return nil, appErrors.NewDatabaseError(err)
	}
	r.InvalidateCache()
	return group, nil
}

func (r *providerGroupRepository) UpdateFields(ctx context.Context, id int, fields map[string]any) (*model.ProviderGroup, error) {
	if len(fields) == 0 {
		return r.GetByID(ctx, id)
	}
	fields["updated_at"] = time.Now()
	query := r.db.NewUpdate().Model((*model.ProviderGroup)(nil)).Where("id = ?", id)
	for column, value := range fields {
		query = query.Set(column+" = ?", value)
	}
	result, err := query.Exec(ctx)
	if err != nil {
		return nil, appErrors.NewDatabaseError(err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, appErrors.NewNotFoundError("ProviderGroup")
	}
	r.InvalidateCache()
	return r.GetByID(ctx, id)
}

func (r *providerGroupRepository) Delete(ctx context.Context, id int) error {
	item, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if item.Name == model.DefaultProviderGroupName {
		return appErrors.NewInvalidRequest("cannot delete the default provider group")
	}
	result, err := r.db.NewDelete().Model((*model.ProviderGroup)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return appErrors.NewDatabaseError(err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return appErrors.NewNotFoundError("ProviderGroup")
	}
	r.InvalidateCache()
	return nil
}

func (r *providerGroupRepository) EnsureExists(ctx context.Context, names []string) error {
	unique := make([]*model.ProviderGroup, 0)
	seen := map[string]struct{}{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		unique = append(unique, &model.ProviderGroup{Name: name, CostMultiplier: udecimal.MustParse("1.0")})
	}
	if len(unique) == 0 {
		return nil
	}
	if _, err := r.db.NewInsert().Model(&unique).Ignore().Exec(ctx); err != nil {
		return appErrors.NewDatabaseError(err)
	}
	r.InvalidateCache()
	return nil
}

func (r *providerGroupRepository) GetCostMultiplier(ctx context.Context, rawGroupString string) (udecimal.Decimal, error) {
	rawGroupString = strings.TrimSpace(rawGroupString)
	if rawGroupString == "" {
		return udecimal.MustParse("1.0"), nil
	}
	if value, ok := r.getCachedMultiplier(rawGroupString); ok {
		return value, nil
	}

	parsedGroups := parseProviderGroups(rawGroupString)
	if len(parsedGroups) == 0 {
		return udecimal.MustParse("1.0"), nil
	}

	var rows []*model.ProviderGroup
	if err := r.db.NewSelect().Model(&rows).Where("name IN (?)", bun.In(parsedGroups)).Scan(ctx); err != nil {
		return udecimal.Zero, appErrors.NewDatabaseError(err)
	}
	byName := make(map[string]udecimal.Decimal, len(rows))
	for _, row := range rows {
		byName[row.Name] = row.CostMultiplier
	}
	for _, name := range parsedGroups {
		if value, ok := byName[name]; ok {
			r.setCachedMultiplier(rawGroupString, value)
			return value, nil
		}
	}
	return udecimal.MustParse("1.0"), nil
}

func (r *providerGroupRepository) InvalidateCache() {
	r.multiplierMu.Lock()
	defer r.multiplierMu.Unlock()
	r.multiplierCache = make(map[string]cacheEntry)
}

func (r *providerGroupRepository) getCachedMultiplier(key string) (udecimal.Decimal, bool) {
	r.multiplierMu.RLock()
	entry, ok := r.multiplierCache[key]
	r.multiplierMu.RUnlock()
	if !ok {
		return udecimal.Zero, false
	}
	if time.Now().After(entry.expiresAt) {
		r.multiplierMu.Lock()
		delete(r.multiplierCache, key)
		r.multiplierMu.Unlock()
		return udecimal.Zero, false
	}
	return entry.value, true
}

func (r *providerGroupRepository) setCachedMultiplier(key string, value udecimal.Decimal) {
	r.multiplierMu.Lock()
	defer r.multiplierMu.Unlock()
	r.multiplierCache[key] = cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(providerGroupCacheTTL),
	}
}

func parseProviderGroups(raw string) []string {
	replacer := strings.NewReplacer("\n", ",", "\r", ",", "\t", ",")
	raw = replacer.Replace(raw)
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}
