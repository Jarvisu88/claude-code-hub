package repository

import (
	"context"
	"fmt"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/uptrace/bun"
)

// NotificationTargetBindingRepository notification_target_bindings data access.
type NotificationTargetBindingRepository interface {
	Repository
	List(ctx context.Context, notificationType string) ([]*model.NotificationTargetBinding, error)
	ListAll(ctx context.Context) ([]*model.NotificationTargetBinding, error)
	ReplaceByNotificationType(ctx context.Context, notificationType string, bindings []*model.NotificationTargetBinding) ([]*model.NotificationTargetBinding, error)
}

type notificationTargetBindingRepository struct {
	*BaseRepository
}

func NewNotificationTargetBindingRepository(db *bun.DB) NotificationTargetBindingRepository {
	return &notificationTargetBindingRepository{BaseRepository: NewBaseRepository(db)}
}

func (r *notificationTargetBindingRepository) List(ctx context.Context, notificationType string) ([]*model.NotificationTargetBinding, error) {
	return r.scanBindings(ctx, r.db.NewSelect().Where("notification_type = ?", notificationType))
}

func (r *notificationTargetBindingRepository) ListAll(ctx context.Context) ([]*model.NotificationTargetBinding, error) {
	return r.scanBindings(ctx, r.db.NewSelect())
}

func (r *notificationTargetBindingRepository) scanBindings(ctx context.Context, query *bun.SelectQuery) ([]*model.NotificationTargetBinding, error) {
	var bindings []*model.NotificationTargetBinding
	if err := query.
		Model(&bindings).
		Relation("Target").
		Order("ntb.notification_type ASC").
		Order("ntb.id ASC").
		Scan(ctx); err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return bindings, nil
}

func (r *notificationTargetBindingRepository) ReplaceByNotificationType(ctx context.Context, notificationType string, bindings []*model.NotificationTargetBinding) ([]*model.NotificationTargetBinding, error) {
	seenTargetIDs := make(map[int]struct{}, len(bindings))
	for _, binding := range bindings {
		if binding == nil {
			return nil, errors.NewInvalidRequest("bindings contains nil item")
		}
		if binding.TargetID <= 0 {
			return nil, errors.NewInvalidRequest("targetId must be greater than 0")
		}
		if _, exists := seenTargetIDs[binding.TargetID]; exists {
			return nil, errors.NewInvalidRequest(fmt.Sprintf("duplicate targetId %d", binding.TargetID))
		}
		seenTargetIDs[binding.TargetID] = struct{}{}
	}

	err := RunInTransaction(ctx, r.db, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewDelete().
			Model((*model.NotificationTargetBinding)(nil)).
			Where("notification_type = ?", notificationType).
			Exec(ctx); err != nil {
			return errors.NewDatabaseError(err)
		}

		for _, binding := range bindings {
			count, err := tx.NewSelect().
				Model((*model.WebhookTarget)(nil)).
				Where("id = ?", binding.TargetID).
				Count(ctx)
			if err != nil {
				return errors.NewDatabaseError(err)
			}
			if count == 0 {
				return errors.NewInvalidRequest(fmt.Sprintf("targetId %d not found", binding.TargetID))
			}
			binding.ID = 0
			binding.NotificationType = notificationType
			binding.ScheduleTimezone = ValidateTimezone(binding.ScheduleTimezone)
			if _, err := tx.NewInsert().Model(binding).Exec(ctx); err != nil {
				return errors.NewDatabaseError(err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.List(ctx, notificationType)
}
