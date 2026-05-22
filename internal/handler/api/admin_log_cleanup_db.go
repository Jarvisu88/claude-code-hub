package api

import (
	"context"
	"time"

	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/uptrace/bun"
)

type DBLogCleanupRunner struct {
	db *bun.DB
}

func NewDBLogCleanupRunner(db *bun.DB) *DBLogCleanupRunner {
	return &DBLogCleanupRunner{db: db}
}

func (r *DBLogCleanupRunner) Run(ctx context.Context, beforeDate time.Time, dryRun bool, now time.Time) (gin.H, error) {
	if r == nil || r.db == nil {
		return nil, appErrors.NewInternalError("日志清理数据库未初始化")
	}
	countQuery := r.db.NewSelect().
		Table("message_request").
		Where("deleted_at IS NULL").
		Where("created_at < ?", beforeDate)
	totalDeleted, err := countQuery.Count(ctx)
	if err != nil {
		return nil, appErrors.NewDatabaseError(err)
	}
	if !dryRun && totalDeleted > 0 {
		_, err = r.db.NewUpdate().
			Table("message_request").
			Set("deleted_at = ?", now).
			Where("deleted_at IS NULL").
			Where("created_at < ?", beforeDate).
			Exec(ctx)
		if err != nil {
			return nil, appErrors.NewDatabaseError(err)
		}
	}
	return gin.H{
		"success":           true,
		"totalDeleted":      totalDeleted,
		"batchCount":        1,
		"softDeletedPurged": 0,
		"vacuumPerformed":   false,
		"error":             nil,
	}, nil
}
