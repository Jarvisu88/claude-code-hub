package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/quagmt/udecimal"
	"github.com/uptrace/bun"
)

// MessageRequestRepository message_request 数据访问接口
type MessageRequestRepository interface {
	Repository

	// Create 创建请求日志
	Create(ctx context.Context, messageRequest *model.MessageRequest) (*model.MessageRequest, error)

	// GetByID 根据 ID 获取请求日志
	GetByID(ctx context.Context, id int) (*model.MessageRequest, error)

	// UpdateTerminal 更新请求日志的终态字段
	UpdateTerminal(ctx context.Context, id int, statusCode int, durationMs int, errorMessage *string) error

	// ListRecent 获取最近的请求日志
	ListRecent(ctx context.Context, limit int) ([]*model.MessageRequest, error)

	// ListFiltered 获取按最小条件过滤的请求日志
	ListFiltered(ctx context.Context, limit int, modelName, endpoint, sessionID string, statusCode *int) ([]*model.MessageRequest, error)

	// FindLatestBySessionID 获取会话的最新请求日志
	FindLatestBySessionID(ctx context.Context, sessionID string) (*model.MessageRequest, error)

	// GetSummary 获取最近日志的最小汇总
	GetSummary(ctx context.Context, modelName, endpoint string, statusCode *int) (MessageRequestSummary, error)

	// GetFilterOptions 获取最小筛选选项
	GetFilterOptions(ctx context.Context) (MessageRequestFilterOptions, error)

	// FindSessionIDSuggestions 获取 sessionId 联想
	FindSessionIDSuggestions(ctx context.Context, term string, limit int) ([]string, error)
}

type MessageRequestSummary struct {
	TotalRequests              int    `json:"totalRequests"`
	TotalCost                  string `json:"totalCost"`
	TotalTokens                int    `json:"totalTokens"`
	TotalInputTokens           int    `json:"totalInputTokens"`
	TotalOutputTokens          int    `json:"totalOutputTokens"`
	TotalCacheCreationTokens   int    `json:"totalCacheCreationTokens"`
	TotalCacheReadTokens       int    `json:"totalCacheReadTokens"`
	TotalCacheCreation5mTokens int    `json:"totalCacheCreation5mTokens"`
	TotalCacheCreation1hTokens int    `json:"totalCacheCreation1hTokens"`
}

type MessageRequestFilterOptions struct {
	Models      []string `json:"models"`
	StatusCodes []int    `json:"statusCodes"`
	Endpoints   []string `json:"endpoints"`
}

type messageRequestRepository struct {
	*BaseRepository
}

func NewMessageRequestRepository(db *bun.DB) MessageRequestRepository {
	return &messageRequestRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

func (r *messageRequestRepository) Create(ctx context.Context, messageRequest *model.MessageRequest) (*model.MessageRequest, error) {
	now := time.Now()
	messageRequest.CreatedAt = now
	messageRequest.UpdatedAt = now
	if messageRequest.RequestSequence <= 0 {
		messageRequest.RequestSequence = 1
	}
	if messageRequest.CostUSD.IsZero() {
		messageRequest.CostUSD = udecimal.Zero
	}

	_, err := r.db.NewInsert().
		Model(messageRequest).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	return messageRequest, nil
}

func (r *messageRequestRepository) GetByID(ctx context.Context, id int) (*model.MessageRequest, error) {
	log := new(model.MessageRequest)
	err := r.db.NewSelect().
		Model(log).
		Where("id = ?", id).
		Where("deleted_at IS NULL").
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewNotFoundError("MessageRequest")
		}
		return nil, errors.NewDatabaseError(err)
	}
	return log, nil
}

func (r *messageRequestRepository) UpdateTerminal(ctx context.Context, id int, statusCode int, durationMs int, errorMessage *string) error {
	update := map[string]any{
		"status_code": statusCode,
		"duration_ms": durationMs,
		"updated_at":  time.Now(),
	}
	if errorMessage != nil {
		update["error_message"] = *errorMessage
	} else {
		update["error_message"] = nil
	}

	_, err := r.db.NewUpdate().
		Table("message_request").
		Set("status_code = ?", update["status_code"]).
		Set("duration_ms = ?", update["duration_ms"]).
		Set("error_message = ?", update["error_message"]).
		Set("updated_at = ?", update["updated_at"]).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return errors.NewDatabaseError(err)
	}
	return nil
}

func (r *messageRequestRepository) ListRecent(ctx context.Context, limit int) ([]*model.MessageRequest, error) {
	return r.ListFiltered(ctx, limit, "", "", "", nil)
}

func (r *messageRequestRepository) ListFiltered(ctx context.Context, limit int, modelName, endpoint, sessionID string, statusCode *int) ([]*model.MessageRequest, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	query := r.db.NewSelect().
		Model((*model.MessageRequest)(nil)).
		Where("deleted_at IS NULL")

	if modelName != "" {
		query = query.Where("model = ?", modelName)
	}
	if endpoint != "" {
		query = query.Where("endpoint = ?", endpoint)
	}
	if sessionID != "" {
		query = query.Where("session_id = ?", sessionID)
	}
	if statusCode != nil {
		query = query.Where("status_code = ?", *statusCode)
	}

	var logs []*model.MessageRequest
	err := query.
		Order("created_at DESC").
		Limit(limit).
		Scan(ctx, &logs)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return logs, nil
}

func (r *messageRequestRepository) FindLatestBySessionID(ctx context.Context, sessionID string) (*model.MessageRequest, error) {
	log := new(model.MessageRequest)
	err := r.db.NewSelect().
		Model(log).
		Where("session_id = ?", sessionID).
		Where("deleted_at IS NULL").
		Order("created_at DESC").
		Limit(1).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewNotFoundError("MessageRequest")
		}
		return nil, errors.NewDatabaseError(err)
	}
	return log, nil
}

func (r *messageRequestRepository) GetSummary(ctx context.Context, modelName, endpoint string, statusCode *int) (MessageRequestSummary, error) {
	query := r.db.NewSelect().
		Model((*model.MessageRequest)(nil)).
		ColumnExpr("COUNT(*) AS total_requests").
		ColumnExpr("COALESCE(SUM(cost_usd), 0) AS total_cost").
		ColumnExpr("COALESCE(SUM(COALESCE(input_tokens, 0)), 0) AS total_input_tokens").
		ColumnExpr("COALESCE(SUM(COALESCE(output_tokens, 0)), 0) AS total_output_tokens").
		ColumnExpr("COALESCE(SUM(COALESCE(cache_creation_input_tokens, 0)), 0) AS total_cache_creation_tokens").
		ColumnExpr("COALESCE(SUM(COALESCE(cache_read_input_tokens, 0)), 0) AS total_cache_read_tokens").
		ColumnExpr("COALESCE(SUM(COALESCE(cache_creation_5m_input_tokens, 0)), 0) AS total_cache_creation_5m_tokens").
		ColumnExpr("COALESCE(SUM(COALESCE(cache_creation_1h_input_tokens, 0)), 0) AS total_cache_creation_1h_tokens").
		ColumnExpr("COALESCE(SUM(COALESCE(input_tokens, 0) + COALESCE(output_tokens, 0) + COALESCE(cache_creation_input_tokens, 0) + COALESCE(cache_read_input_tokens, 0)), 0) AS total_tokens").
		Where("deleted_at IS NULL")

	if modelName != "" {
		query = query.Where("model = ?", modelName)
	}
	if endpoint != "" {
		query = query.Where("endpoint = ?", endpoint)
	}
	if statusCode != nil {
		query = query.Where("status_code = ?", *statusCode)
	}

	var result struct {
		TotalRequests              int              `bun:"total_requests"`
		TotalCost                  udecimal.Decimal `bun:"total_cost"`
		TotalTokens                int              `bun:"total_tokens"`
		TotalInputTokens           int              `bun:"total_input_tokens"`
		TotalOutputTokens          int              `bun:"total_output_tokens"`
		TotalCacheCreationTokens   int              `bun:"total_cache_creation_tokens"`
		TotalCacheReadTokens       int              `bun:"total_cache_read_tokens"`
		TotalCacheCreation5mTokens int              `bun:"total_cache_creation_5m_tokens"`
		TotalCacheCreation1hTokens int              `bun:"total_cache_creation_1h_tokens"`
	}
	if err := query.Scan(ctx, &result); err != nil {
		return MessageRequestSummary{}, errors.NewDatabaseError(err)
	}
	return MessageRequestSummary{
		TotalRequests:              result.TotalRequests,
		TotalCost:                  result.TotalCost.String(),
		TotalTokens:                result.TotalTokens,
		TotalInputTokens:           result.TotalInputTokens,
		TotalOutputTokens:          result.TotalOutputTokens,
		TotalCacheCreationTokens:   result.TotalCacheCreationTokens,
		TotalCacheReadTokens:       result.TotalCacheReadTokens,
		TotalCacheCreation5mTokens: result.TotalCacheCreation5mTokens,
		TotalCacheCreation1hTokens: result.TotalCacheCreation1hTokens,
	}, nil
}

func (r *messageRequestRepository) GetFilterOptions(ctx context.Context) (MessageRequestFilterOptions, error) {
	var models []string
	if err := r.db.NewSelect().
		Model((*model.MessageRequest)(nil)).
		ColumnExpr("DISTINCT model").
		Where("deleted_at IS NULL").
		Where("model IS NOT NULL").
		Where("model != ''").
		Order("model ASC").
		Scan(ctx, &models); err != nil {
		return MessageRequestFilterOptions{}, errors.NewDatabaseError(err)
	}

	var endpoints []string
	if err := r.db.NewSelect().
		Model((*model.MessageRequest)(nil)).
		ColumnExpr("DISTINCT endpoint").
		Where("deleted_at IS NULL").
		Where("endpoint IS NOT NULL").
		Where("endpoint != ''").
		Order("endpoint ASC").
		Scan(ctx, &endpoints); err != nil {
		return MessageRequestFilterOptions{}, errors.NewDatabaseError(err)
	}

	var statusCodes []int
	if err := r.db.NewSelect().
		Model((*model.MessageRequest)(nil)).
		ColumnExpr("DISTINCT status_code").
		Where("deleted_at IS NULL").
		Where("status_code IS NOT NULL").
		Order("status_code ASC").
		Scan(ctx, &statusCodes); err != nil {
		return MessageRequestFilterOptions{}, errors.NewDatabaseError(err)
	}

	return MessageRequestFilterOptions{
		Models:      models,
		StatusCodes: statusCodes,
		Endpoints:   endpoints,
	}, nil
}

func (r *messageRequestRepository) FindSessionIDSuggestions(ctx context.Context, term string, limit int) ([]string, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return []string{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	var sessionIDs []string
	err := r.db.NewSelect().
		Model((*model.MessageRequest)(nil)).
		ColumnExpr("DISTINCT session_id").
		Where("deleted_at IS NULL").
		Where("session_id IS NOT NULL").
		Where("session_id != ''").
		Where("session_id LIKE ?", term+"%").
		Order("session_id ASC").
		Limit(limit).
		Scan(ctx, &sessionIDs)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return sessionIDs, nil
}
