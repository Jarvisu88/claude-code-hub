package repository

import (
	"context"
	"database/sql"
	"math"
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
	UpdateTerminal(ctx context.Context, id int, update MessageRequestTerminalUpdate) error

	// ListRecent 获取最近的请求日志
	ListRecent(ctx context.Context, limit int) ([]*model.MessageRequest, error)

	// ListFiltered 获取按条件过滤的请求日志
	ListFiltered(ctx context.Context, limit int, filters MessageRequestQueryFilters) ([]*model.MessageRequest, error)

	// ListPaginatedFiltered 获取按条件过滤的分页请求日志
	ListPaginatedFiltered(ctx context.Context, page, pageSize int, filters MessageRequestQueryFilters) (MessageRequestListResult, error)

	// ListBatch 获取按条件过滤的批量请求日志（游标分页）
	ListBatch(ctx context.Context, filters MessageRequestBatchFilters) (MessageRequestBatchResult, error)

	// FindLatestBySessionIDs 获取多个 session 的最新请求
	FindLatestBySessionIDs(ctx context.Context, sessionIDs []string, limit int) ([]*model.MessageRequest, error)

	// FindLatestBySessionID 获取会话的最新请求日志
	FindLatestBySessionID(ctx context.Context, sessionID string) (*model.MessageRequest, error)

	// FindSessionOriginChain 获取会话来源链对应的最早请求
	FindSessionOriginChain(ctx context.Context, sessionID string) (*model.MessageRequest, error)

	// GetSummary 获取日志汇总
	GetSummary(ctx context.Context, filters MessageRequestQueryFilters) (MessageRequestSummary, error)

	// GetOverviewMetrics 获取概览面板指标
	GetOverviewMetrics(ctx context.Context, now time.Time, location *time.Location) (MessageRequestOverviewMetrics, error)

	// GetCurrentProviderStatus 获取 provider 当前可用性窗口聚合
	GetCurrentProviderStatus(ctx context.Context, providerIDs []int, now time.Time, window time.Duration) (map[int]ProviderCurrentStatus, error)

	// ListAvailabilityRows 获取可用性聚合原始请求行
	ListAvailabilityRows(ctx context.Context, startTime, endTime time.Time, providerIDs []int) ([]AvailabilityRequestRow, error)

	// ListLeaderboardRows 获取排行榜聚合原始请求行
	ListLeaderboardRows(ctx context.Context, startTime, endTime time.Time) ([]LeaderboardRequestRow, error)

	// GetFilterOptions 获取最小筛选选项
	GetFilterOptions(ctx context.Context) (MessageRequestFilterOptions, error)

	// FindSessionIDSuggestions 获取 sessionId 联想
	FindSessionIDSuggestions(ctx context.Context, filters MessageRequestSessionIDSuggestionFilters) ([]string, error)
}

type MessageRequestSummary struct {
	TotalRequests              int     `json:"totalRequests"`
	TotalRows                  int     `json:"totalRows"`
	TotalCost                  float64 `json:"totalCost"`
	TotalTokens                int     `json:"totalTokens"`
	TotalInputTokens           int     `json:"totalInputTokens"`
	TotalOutputTokens          int     `json:"totalOutputTokens"`
	TotalCacheCreationTokens   int     `json:"totalCacheCreationTokens"`
	TotalCacheReadTokens       int     `json:"totalCacheReadTokens"`
	TotalCacheCreation5mTokens int     `json:"totalCacheCreation5mTokens"`
	TotalCacheCreation1hTokens int     `json:"totalCacheCreation1hTokens"`
}

type MessageRequestOverviewMetrics struct {
	TodayRequests                      int     `json:"todayRequests"`
	TodayCost                          float64 `json:"todayCost"`
	AvgResponseTime                    int     `json:"avgResponseTime"`
	TodayErrorRate                     float64 `json:"todayErrorRate"`
	YesterdaySamePeriodRequests        int     `json:"yesterdaySamePeriodRequests"`
	YesterdaySamePeriodCost            float64 `json:"yesterdaySamePeriodCost"`
	YesterdaySamePeriodAvgResponseTime int     `json:"yesterdaySamePeriodAvgResponseTime"`
	RecentMinuteRequests               int     `json:"recentMinuteRequests"`
	ConcurrentSessions                 int     `json:"concurrentSessions"`
}

type ProviderCurrentStatus struct {
	ProviderID    int        `json:"providerId"`
	GreenCount    int        `json:"greenCount"`
	RedCount      int        `json:"redCount"`
	LastRequestAt *time.Time `json:"lastRequestAt,omitempty"`
}

type AvailabilityRequestRow struct {
	ProviderID int       `json:"providerId"`
	CreatedAt  time.Time `json:"createdAt"`
	StatusCode int       `json:"statusCode"`
	DurationMs *int      `json:"durationMs,omitempty"`
}

type LeaderboardRequestRow struct {
	UserID                   int              `json:"userId"`
	UserName                 string           `json:"userName"`
	ProviderID               int              `json:"providerId"`
	ProviderName             string           `json:"providerName"`
	ProviderType             string           `json:"providerType"`
	Model                    string           `json:"model"`
	StatusCode               int              `json:"statusCode"`
	CostUSD                  udecimal.Decimal `json:"costUsd"`
	DurationMs               *int             `json:"durationMs,omitempty"`
	TtfbMs                   *int             `json:"ttfbMs,omitempty"`
	InputTokens              *int             `json:"inputTokens,omitempty"`
	OutputTokens             *int             `json:"outputTokens,omitempty"`
	CacheCreationInputTokens *int             `json:"cacheCreationInputTokens,omitempty"`
	CacheReadInputTokens     *int             `json:"cacheReadInputTokens,omitempty"`
}

type MessageRequestTerminalUpdate struct {
	StatusCode                 int                       `json:"statusCode"`
	DurationMs                 int                       `json:"durationMs"`
	ErrorMessage               *string                   `json:"errorMessage,omitempty"`
	ProviderChain              []model.ProviderChainItem `json:"providerChain,omitempty"`
	InputTokens                *int                      `json:"inputTokens,omitempty"`
	OutputTokens               *int                      `json:"outputTokens,omitempty"`
	CacheCreationInputTokens   *int                      `json:"cacheCreationInputTokens,omitempty"`
	CacheReadInputTokens       *int                      `json:"cacheReadInputTokens,omitempty"`
	CacheCreation5mInputTokens *int                      `json:"cacheCreation5mInputTokens,omitempty"`
	CacheCreation1hInputTokens *int                      `json:"cacheCreation1hInputTokens,omitempty"`
}

type MessageRequestFilterOptions struct {
	Models      []string `json:"models"`
	StatusCodes []int    `json:"statusCodes"`
	Endpoints   []string `json:"endpoints"`
}

type MessageRequestQueryFilters struct {
	UserID               *int       `json:"userId,omitempty"`
	KeyID                *int       `json:"keyId,omitempty"`
	ProviderID           *int       `json:"providerId,omitempty"`
	SessionID            string     `json:"sessionId,omitempty"`
	StartTime            *time.Time `json:"startTime,omitempty"`
	EndTime              *time.Time `json:"endTime,omitempty"`
	StatusCode           *int       `json:"statusCode,omitempty"`
	ExcludeStatusCode200 bool       `json:"excludeStatusCode200,omitempty"`
	Model                string     `json:"model,omitempty"`
	Endpoint             string     `json:"endpoint,omitempty"`
	MinRetryCount        *int       `json:"minRetryCount,omitempty"`
}

type MessageRequestSessionIDSuggestionFilters struct {
	Term       string `json:"term"`
	UserID     *int   `json:"userId,omitempty"`
	KeyID      *int   `json:"keyId,omitempty"`
	ProviderID *int   `json:"providerId,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

type MessageRequestBatchCursor struct {
	CreatedAt string `json:"createdAt"`
	ID        int    `json:"id"`
}

type MessageRequestBatchFilters struct {
	MessageRequestQueryFilters
	Cursor *MessageRequestBatchCursor `json:"cursor,omitempty"`
	Limit  int                        `json:"limit,omitempty"`
}

type MessageRequestListResult struct {
	Logs     []*model.MessageRequest `json:"logs"`
	Total    int                     `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"pageSize"`
}

type MessageRequestBatchResult struct {
	Logs       []*model.MessageRequest    `json:"logs"`
	NextCursor *MessageRequestBatchCursor `json:"nextCursor"`
	HasMore    bool                       `json:"hasMore"`
}

type messageRequestRepository struct {
	*BaseRepository
}

const excludeWarmupMessageRequestCondition = "(blocked_by IS NULL OR blocked_by <> 'warmup')"
const retryCountExpression = `(
	SELECT
		CASE
			WHEN COALESCE(
				bool_or(
					(elem->>'reason') IN (
						'hedge_triggered',
						'hedge_launched',
						'hedge_winner',
						'hedge_loser_cancelled'
					)
				),
				false
			)
			THEN 0
			ELSE GREATEST(
				COALESCE(
					sum(
						CASE
							WHEN (
								(elem->>'reason') IN (
									'concurrent_limit_failed',
									'retry_failed',
									'system_error',
									'resource_not_found',
									'client_error_non_retryable',
									'endpoint_pool_exhausted',
									'vendor_type_all_timeout',
									'client_abort',
									'http2_fallback'
								)
								OR (
									(elem->>'reason') IN ('request_success', 'retry_success')
									AND (elem->>'statusCode') IS NOT NULL
								)
							)
							THEN 1
							ELSE 0
						END
					),
					0
				) - 1,
				0
			)
		END
	FROM jsonb_array_elements(COALESCE(mr.provider_chain, '[]'::jsonb)) AS elem
)`

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
		ColumnExpr("u.name AS user_name").
		ColumnExpr("k.name AS key_name").
		ColumnExpr("p.name AS provider_name").
		Join("LEFT JOIN users AS u ON u.id = mr.user_id AND u.deleted_at IS NULL").
		Join("LEFT JOIN keys AS k ON k.key = mr.key AND k.deleted_at IS NULL").
		Join("LEFT JOIN providers AS p ON p.id = mr.provider_id AND p.deleted_at IS NULL").
		Where("mr.id = ?", id).
		Where("mr.deleted_at IS NULL").
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewNotFoundError("MessageRequest")
		}
		return nil, errors.NewDatabaseError(err)
	}
	return log, nil
}

func (r *messageRequestRepository) UpdateTerminal(ctx context.Context, id int, update MessageRequestTerminalUpdate) error {
	updateFields := map[string]any{
		"status_code": update.StatusCode,
		"duration_ms": update.DurationMs,
		"updated_at":  time.Now(),
	}
	if update.ErrorMessage != nil {
		updateFields["error_message"] = *update.ErrorMessage
	} else {
		updateFields["error_message"] = nil
	}
	if update.InputTokens != nil {
		updateFields["input_tokens"] = *update.InputTokens
	}
	if update.OutputTokens != nil {
		updateFields["output_tokens"] = *update.OutputTokens
	}
	if update.CacheCreationInputTokens != nil {
		updateFields["cache_creation_input_tokens"] = *update.CacheCreationInputTokens
	}
	if update.CacheReadInputTokens != nil {
		updateFields["cache_read_input_tokens"] = *update.CacheReadInputTokens
	}
	if update.CacheCreation5mInputTokens != nil {
		updateFields["cache_creation_5m_input_tokens"] = *update.CacheCreation5mInputTokens
	}
	if update.CacheCreation1hInputTokens != nil {
		updateFields["cache_creation_1h_input_tokens"] = *update.CacheCreation1hInputTokens
	}
	if update.ProviderChain != nil {
		updateFields["provider_chain"] = update.ProviderChain
	}

	query := r.db.NewUpdate().
		Table("message_request").
		Set("status_code = ?", updateFields["status_code"]).
		Set("duration_ms = ?", updateFields["duration_ms"]).
		Set("error_message = ?", updateFields["error_message"]).
		Set("updated_at = ?", updateFields["updated_at"])
	if inputTokens, ok := updateFields["input_tokens"]; ok {
		query = query.Set("input_tokens = ?", inputTokens)
	}
	if outputTokens, ok := updateFields["output_tokens"]; ok {
		query = query.Set("output_tokens = ?", outputTokens)
	}
	if cacheCreationInputTokens, ok := updateFields["cache_creation_input_tokens"]; ok {
		query = query.Set("cache_creation_input_tokens = ?", cacheCreationInputTokens)
	}
	if cacheReadInputTokens, ok := updateFields["cache_read_input_tokens"]; ok {
		query = query.Set("cache_read_input_tokens = ?", cacheReadInputTokens)
	}
	if cacheCreation5mInputTokens, ok := updateFields["cache_creation_5m_input_tokens"]; ok {
		query = query.Set("cache_creation_5m_input_tokens = ?", cacheCreation5mInputTokens)
	}
	if cacheCreation1hInputTokens, ok := updateFields["cache_creation_1h_input_tokens"]; ok {
		query = query.Set("cache_creation_1h_input_tokens = ?", cacheCreation1hInputTokens)
	}
	if providerChain, ok := updateFields["provider_chain"]; ok {
		query = query.Set("provider_chain = ?", providerChain)
	}

	_, err := query.Where("id = ?", id).Exec(ctx)
	if err != nil {
		return errors.NewDatabaseError(err)
	}
	return nil
}

func (r *messageRequestRepository) ListRecent(ctx context.Context, limit int) ([]*model.MessageRequest, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var logs []*model.MessageRequest
	err := r.db.NewSelect().
		Model(&logs).
		ColumnExpr("u.name AS user_name").
		ColumnExpr("k.name AS key_name").
		ColumnExpr("p.name AS provider_name").
		Join("LEFT JOIN users AS u ON u.id = mr.user_id AND u.deleted_at IS NULL").
		Join("LEFT JOIN keys AS k ON k.key = mr.key AND k.deleted_at IS NULL").
		Join("LEFT JOIN providers AS p ON p.id = mr.provider_id AND p.deleted_at IS NULL").
		Where("mr.deleted_at IS NULL").
		Order("mr.updated_at DESC").
		Limit(limit).
		Scan(ctx)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return logs, nil
}

func (r *messageRequestRepository) ListFiltered(ctx context.Context, limit int, filters MessageRequestQueryFilters) ([]*model.MessageRequest, error) {
	result, err := r.ListPaginatedFiltered(ctx, 1, limit, filters)
	if err != nil {
		return nil, err
	}
	return result.Logs, nil
}

func applyMessageRequestQueryFilters(query *bun.SelectQuery, filters MessageRequestQueryFilters, excludeWarmup bool) *bun.SelectQuery {
	if excludeWarmup {
		query = query.Where(excludeWarmupMessageRequestCondition)
	}
	if filters.KeyID != nil {
		query = query.Join("JOIN keys AS k ON k.key = mr.key").
			Where("k.id = ?", *filters.KeyID)
	}
	if filters.UserID != nil {
		query = query.Where("mr.user_id = ?", *filters.UserID)
	}
	if filters.ProviderID != nil {
		query = query.Where("mr.provider_id = ?", *filters.ProviderID)
	}
	if sessionID := strings.TrimSpace(filters.SessionID); sessionID != "" {
		query = query.Where("mr.session_id = ?", sessionID)
	}
	if filters.StartTime != nil {
		query = query.Where("mr.created_at >= ?", *filters.StartTime)
	}
	if filters.EndTime != nil {
		query = query.Where("mr.created_at < ?", *filters.EndTime)
	}
	if filters.StatusCode != nil {
		query = query.Where("mr.status_code = ?", *filters.StatusCode)
	} else if filters.ExcludeStatusCode200 {
		query = query.Where("(mr.status_code IS NULL OR mr.status_code <> ?)", 200)
	}
	if modelName := strings.TrimSpace(filters.Model); modelName != "" {
		query = query.Where("mr.model = ?", modelName)
	}
	if endpoint := strings.TrimSpace(filters.Endpoint); endpoint != "" {
		query = query.Where("mr.endpoint = ?", endpoint)
	}
	if filters.MinRetryCount != nil && *filters.MinRetryCount > 0 {
		query = query.Where(retryCountExpression+" >= ?", *filters.MinRetryCount)
	}
	return query
}

func (r *messageRequestRepository) ListPaginatedFiltered(ctx context.Context, page, pageSize int, filters MessageRequestQueryFilters) (MessageRequestListResult, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := r.db.NewSelect().
		Model((*model.MessageRequest)(nil)).
		Where("mr.deleted_at IS NULL")
	query = applyMessageRequestQueryFilters(query, filters, false)

	total, err := query.Count(ctx)
	if err != nil {
		return MessageRequestListResult{}, errors.NewDatabaseError(err)
	}

	var logs []*model.MessageRequest
	err = query.
		ColumnExpr("u.name AS user_name").
		ColumnExpr("k2.name AS key_name").
		ColumnExpr("p.name AS provider_name").
		Join("LEFT JOIN users AS u ON u.id = mr.user_id AND u.deleted_at IS NULL").
		Join("LEFT JOIN keys AS k2 ON k2.key = mr.key AND k2.deleted_at IS NULL").
		Join("LEFT JOIN providers AS p ON p.id = mr.provider_id AND p.deleted_at IS NULL").
		Order("mr.created_at DESC").
		Limit(pageSize).
		Offset((page-1)*pageSize).
		Scan(ctx, &logs)
	if err != nil {
		return MessageRequestListResult{}, errors.NewDatabaseError(err)
	}
	return MessageRequestListResult{
		Logs:     logs,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (r *messageRequestRepository) ListBatch(ctx context.Context, filters MessageRequestBatchFilters) (MessageRequestBatchResult, error) {
	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	query := r.db.NewSelect().
		Model((*model.MessageRequest)(nil)).
		Where("mr.deleted_at IS NULL")
	query = applyMessageRequestQueryFilters(query, filters.MessageRequestQueryFilters, false)

	if filters.Cursor != nil && strings.TrimSpace(filters.Cursor.CreatedAt) != "" && filters.Cursor.ID > 0 {
		createdAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(filters.Cursor.CreatedAt))
		if err != nil {
			return MessageRequestBatchResult{}, errors.NewInvalidRequest("cursor.createdAt 不是合法时间")
		}
		query = query.Where("(mr.created_at, mr.id) < (?, ?)", createdAt, filters.Cursor.ID)
	}

	fetchLimit := limit + 1
	var logs []*model.MessageRequest
	err := query.
		ColumnExpr("u.name AS user_name").
		ColumnExpr("k2.name AS key_name").
		ColumnExpr("p.name AS provider_name").
		Join("LEFT JOIN users AS u ON u.id = mr.user_id AND u.deleted_at IS NULL").
		Join("LEFT JOIN keys AS k2 ON k2.key = mr.key AND k2.deleted_at IS NULL").
		Join("LEFT JOIN providers AS p ON p.id = mr.provider_id AND p.deleted_at IS NULL").
		Order("mr.created_at DESC, mr.id DESC").
		Limit(fetchLimit).
		Scan(ctx, &logs)
	if err != nil {
		return MessageRequestBatchResult{}, errors.NewDatabaseError(err)
	}

	hasMore := len(logs) > limit
	if hasMore {
		logs = logs[:limit]
	}

	var nextCursor *MessageRequestBatchCursor
	if hasMore && len(logs) > 0 {
		last := logs[len(logs)-1]
		nextCursor = &MessageRequestBatchCursor{
			CreatedAt: last.CreatedAt.UTC().Format(time.RFC3339Nano),
			ID:        last.ID,
		}
	}

	return MessageRequestBatchResult{
		Logs:       logs,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func (r *messageRequestRepository) FindLatestBySessionIDs(ctx context.Context, sessionIDs []string, limit int) ([]*model.MessageRequest, error) {
	trimmed := make([]string, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		if cleaned := strings.TrimSpace(sessionID); cleaned != "" {
			trimmed = append(trimmed, cleaned)
		}
	}
	if len(trimmed) == 0 {
		return []*model.MessageRequest{}, nil
	}
	if limit <= 0 {
		limit = len(trimmed)
	}
	if limit > len(trimmed) {
		limit = len(trimmed)
	}

	var logs []*model.MessageRequest
	query := r.db.NewSelect().
		Model((*model.MessageRequest)(nil)).
		ColumnExpr("DISTINCT ON (mr.session_id) mr.*").
		ColumnExpr("u.name AS user_name").
		ColumnExpr("k2.name AS key_name").
		ColumnExpr("p.name AS provider_name").
		Join("LEFT JOIN users AS u ON u.id = mr.user_id AND u.deleted_at IS NULL").
		Join("LEFT JOIN keys AS k2 ON k2.key = mr.key AND k2.deleted_at IS NULL").
		Join("LEFT JOIN providers AS p ON p.id = mr.provider_id AND p.deleted_at IS NULL").
		Where("mr.deleted_at IS NULL").
		Where("mr.session_id IN (?)", bun.In(trimmed)).
		OrderExpr("mr.session_id, mr.created_at DESC, mr.id DESC").
		Limit(limit)
	if err := query.Scan(ctx, &logs); err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return logs, nil
}

func (r *messageRequestRepository) FindLatestBySessionID(ctx context.Context, sessionID string) (*model.MessageRequest, error) {
	log := new(model.MessageRequest)
	err := r.db.NewSelect().
		Model(log).
		Where("mr.session_id = ?", sessionID).
		Where("mr.deleted_at IS NULL").
		Order("mr.created_at DESC").
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

func (r *messageRequestRepository) FindSessionOriginChain(ctx context.Context, sessionID string) (*model.MessageRequest, error) {
	log := new(model.MessageRequest)
	err := r.db.NewSelect().
		Model(log).
		Where("mr.session_id = ?", sessionID).
		Where("mr.deleted_at IS NULL").
		Where(excludeWarmupMessageRequestCondition).
		Where("mr.provider_chain IS NOT NULL").
		Where("mr.provider_chain @> ?::jsonb", `[{"reason":"initial_selection"}]`).
		Order("mr.request_sequence ASC").
		Limit(1).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, errors.NewDatabaseError(err)
	}
	if len(log.ProviderChain) == 0 {
		return nil, nil
	}
	return log, nil
}

func (r *messageRequestRepository) GetSummary(ctx context.Context, filters MessageRequestQueryFilters) (MessageRequestSummary, error) {
	query := r.db.NewSelect().
		Model((*model.MessageRequest)(nil)).
		ColumnExpr("COUNT(*) AS total_rows").
		ColumnExpr("COUNT(*) FILTER (WHERE " + excludeWarmupMessageRequestCondition + ") AS total_requests").
		ColumnExpr("COALESCE(SUM(cost_usd) FILTER (WHERE " + excludeWarmupMessageRequestCondition + "), 0) AS total_cost").
		ColumnExpr("COALESCE(SUM(COALESCE(input_tokens, 0)) FILTER (WHERE " + excludeWarmupMessageRequestCondition + "), 0) AS total_input_tokens").
		ColumnExpr("COALESCE(SUM(COALESCE(output_tokens, 0)) FILTER (WHERE " + excludeWarmupMessageRequestCondition + "), 0) AS total_output_tokens").
		ColumnExpr("COALESCE(SUM(COALESCE(cache_creation_input_tokens, 0)) FILTER (WHERE " + excludeWarmupMessageRequestCondition + "), 0) AS total_cache_creation_tokens").
		ColumnExpr("COALESCE(SUM(COALESCE(cache_read_input_tokens, 0)) FILTER (WHERE " + excludeWarmupMessageRequestCondition + "), 0) AS total_cache_read_tokens").
		ColumnExpr("COALESCE(SUM(COALESCE(cache_creation_5m_input_tokens, 0)) FILTER (WHERE " + excludeWarmupMessageRequestCondition + "), 0) AS total_cache_creation_5m_tokens").
		ColumnExpr("COALESCE(SUM(COALESCE(cache_creation_1h_input_tokens, 0)) FILTER (WHERE " + excludeWarmupMessageRequestCondition + "), 0) AS total_cache_creation_1h_tokens").
		ColumnExpr("COALESCE(SUM(COALESCE(input_tokens, 0) + COALESCE(output_tokens, 0) + COALESCE(cache_creation_input_tokens, 0) + COALESCE(cache_read_input_tokens, 0)) FILTER (WHERE " + excludeWarmupMessageRequestCondition + "), 0) AS total_tokens").
		Where("mr.deleted_at IS NULL")

	query = applyMessageRequestQueryFilters(query, filters, false)

	var result struct {
		TotalRows                  int              `bun:"total_rows"`
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
		TotalRows:                  result.TotalRows,
		TotalRequests:              result.TotalRequests,
		TotalCost:                  roundCost6(result.TotalCost.InexactFloat64()),
		TotalTokens:                result.TotalTokens,
		TotalInputTokens:           result.TotalInputTokens,
		TotalOutputTokens:          result.TotalOutputTokens,
		TotalCacheCreationTokens:   result.TotalCacheCreationTokens,
		TotalCacheReadTokens:       result.TotalCacheReadTokens,
		TotalCacheCreation5mTokens: result.TotalCacheCreation5mTokens,
		TotalCacheCreation1hTokens: result.TotalCacheCreation1hTokens,
	}, nil
}

func (r *messageRequestRepository) GetOverviewMetrics(ctx context.Context, now time.Time, location *time.Location) (MessageRequestOverviewMetrics, error) {
	if location == nil {
		location = time.Local
	}
	localNow := now.In(location)
	todayStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	tomorrowStart := todayStart.Add(24 * time.Hour)
	yesterdayStart := todayStart.Add(-24 * time.Hour)
	yesterdayEnd := yesterdayStart.Add(localNow.Sub(todayStart))
	recentMinuteStart := now.Add(-time.Minute)

	type overviewAggregate struct {
		RequestCount int              `bun:"request_count"`
		TotalCost    udecimal.Decimal `bun:"total_cost"`
		AvgDuration  float64          `bun:"avg_duration"`
		ErrorCount   int              `bun:"error_count"`
	}

	queryAggregate := func(start, end time.Time) (overviewAggregate, error) {
		var result overviewAggregate
		err := r.db.NewSelect().
			Model((*model.MessageRequest)(nil)).
			ColumnExpr("COUNT(*) AS request_count").
			ColumnExpr("COALESCE(SUM(cost_usd), 0) AS total_cost").
			ColumnExpr("COALESCE(AVG(duration_ms), 0) AS avg_duration").
			ColumnExpr("COUNT(*) FILTER (WHERE status_code IS NULL OR status_code >= 400) AS error_count").
			Where("mr.deleted_at IS NULL").
			Where(excludeWarmupMessageRequestCondition).
			Where("mr.duration_ms IS NOT NULL").
			Where("mr.created_at >= ?", start).
			Where("mr.created_at < ?", end).
			Scan(ctx, &result)
		if err != nil {
			return overviewAggregate{}, errors.NewDatabaseError(err)
		}
		return result, nil
	}

	today, err := queryAggregate(todayStart, tomorrowStart)
	if err != nil {
		return MessageRequestOverviewMetrics{}, err
	}
	yesterday, err := queryAggregate(yesterdayStart, yesterdayEnd)
	if err != nil {
		return MessageRequestOverviewMetrics{}, err
	}

	var recentMinute struct {
		RequestCount int `bun:"request_count"`
	}
	if err := r.db.NewSelect().
		Model((*model.MessageRequest)(nil)).
		ColumnExpr("COUNT(*) AS request_count").
		Where("mr.deleted_at IS NULL").
		Where(excludeWarmupMessageRequestCondition).
		Where("mr.duration_ms IS NOT NULL").
		Where("mr.created_at >= ?", recentMinuteStart).
		Scan(ctx, &recentMinute); err != nil {
		return MessageRequestOverviewMetrics{}, errors.NewDatabaseError(err)
	}

	var concurrent struct {
		SessionCount int `bun:"session_count"`
	}
	if err := r.db.NewSelect().
		Model((*model.MessageRequest)(nil)).
		ColumnExpr("COUNT(DISTINCT session_id) AS session_count").
		Where("mr.deleted_at IS NULL").
		Where(excludeWarmupMessageRequestCondition).
		Where("mr.duration_ms IS NULL").
		Where("mr.session_id IS NOT NULL").
		Where("mr.session_id != ''").
		Scan(ctx, &concurrent); err != nil {
		return MessageRequestOverviewMetrics{}, errors.NewDatabaseError(err)
	}

	todayErrorRate := 0.0
	if today.RequestCount > 0 {
		todayErrorRate = math.Round((float64(today.ErrorCount)/float64(today.RequestCount))*10000) / 100
	}

	return MessageRequestOverviewMetrics{
		TodayRequests:                      today.RequestCount,
		TodayCost:                          roundCost6(today.TotalCost.InexactFloat64()),
		AvgResponseTime:                    int(math.Round(today.AvgDuration)),
		TodayErrorRate:                     todayErrorRate,
		YesterdaySamePeriodRequests:        yesterday.RequestCount,
		YesterdaySamePeriodCost:            roundCost6(yesterday.TotalCost.InexactFloat64()),
		YesterdaySamePeriodAvgResponseTime: int(math.Round(yesterday.AvgDuration)),
		RecentMinuteRequests:               recentMinute.RequestCount,
		ConcurrentSessions:                 concurrent.SessionCount,
	}, nil
}

func (r *messageRequestRepository) GetCurrentProviderStatus(ctx context.Context, providerIDs []int, now time.Time, window time.Duration) (map[int]ProviderCurrentStatus, error) {
	filteredIDs := make([]int, 0, len(providerIDs))
	for _, providerID := range providerIDs {
		if providerID > 0 {
			filteredIDs = append(filteredIDs, providerID)
		}
	}
	if len(filteredIDs) == 0 {
		return map[int]ProviderCurrentStatus{}, nil
	}
	if window <= 0 {
		window = 15 * time.Minute
	}
	windowStart := now.Add(-window)

	var rows []struct {
		ProviderID    int        `bun:"provider_id"`
		GreenCount    int        `bun:"green_count"`
		RedCount      int        `bun:"red_count"`
		LastRequestAt *time.Time `bun:"last_request_at"`
	}
	query := r.db.NewSelect().
		Table("message_request AS mr").
		ColumnExpr("mr.provider_id").
		ColumnExpr("COUNT(*) FILTER (WHERE mr.status_code >= 200 AND mr.status_code < 400) AS green_count").
		ColumnExpr("COUNT(*) FILTER (WHERE mr.status_code < 200 OR mr.status_code >= 400) AS red_count").
		ColumnExpr("MAX(mr.created_at) AS last_request_at").
		Where("mr.deleted_at IS NULL").
		Where("mr.provider_id IN (?)", bun.In(filteredIDs)).
		Where("mr.created_at >= ?", windowStart).
		Where("mr.created_at <= ?", now).
		Where("mr.status_code IS NOT NULL").
		Group("mr.provider_id")
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, errors.NewDatabaseError(err)
	}

	results := make(map[int]ProviderCurrentStatus, len(filteredIDs))
	for _, row := range rows {
		results[row.ProviderID] = ProviderCurrentStatus{
			ProviderID:    row.ProviderID,
			GreenCount:    row.GreenCount,
			RedCount:      row.RedCount,
			LastRequestAt: row.LastRequestAt,
		}
	}
	return results, nil
}

func (r *messageRequestRepository) ListAvailabilityRows(ctx context.Context, startTime, endTime time.Time, providerIDs []int) ([]AvailabilityRequestRow, error) {
	filteredIDs := make([]int, 0, len(providerIDs))
	for _, providerID := range providerIDs {
		if providerID > 0 {
			filteredIDs = append(filteredIDs, providerID)
		}
	}

	var rows []AvailabilityRequestRow
	query := r.db.NewSelect().
		Table("message_request AS mr").
		ColumnExpr("mr.provider_id").
		ColumnExpr("mr.created_at").
		ColumnExpr("mr.status_code").
		ColumnExpr("mr.duration_ms").
		Where("mr.deleted_at IS NULL").
		Where("mr.status_code IS NOT NULL").
		Where("mr.created_at >= ?", startTime).
		Where("mr.created_at <= ?", endTime)
	if len(filteredIDs) > 0 {
		query = query.Where("mr.provider_id IN (?)", bun.In(filteredIDs))
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return rows, nil
}

func (r *messageRequestRepository) ListLeaderboardRows(ctx context.Context, startTime, endTime time.Time) ([]LeaderboardRequestRow, error) {
	var rows []LeaderboardRequestRow
	query := r.db.NewSelect().
		Table("message_request AS mr").
		ColumnExpr("mr.user_id").
		ColumnExpr("COALESCE(u.name, CONCAT('User #', mr.user_id::text)) AS user_name").
		ColumnExpr("mr.provider_id").
		ColumnExpr("COALESCE(p.name, 'unknown') AS provider_name").
		ColumnExpr("COALESCE(p.provider_type, '') AS provider_type").
		ColumnExpr("COALESCE(NULLIF(mr.original_model, ''), NULLIF(mr.model, ''), 'Unknown') AS model").
		ColumnExpr("mr.status_code").
		ColumnExpr("mr.cost_usd").
		ColumnExpr("mr.duration_ms").
		ColumnExpr("mr.ttfb_ms").
		ColumnExpr("mr.input_tokens").
		ColumnExpr("mr.output_tokens").
		ColumnExpr("mr.cache_creation_input_tokens").
		ColumnExpr("mr.cache_read_input_tokens").
		Join("LEFT JOIN users AS u ON u.id = mr.user_id AND u.deleted_at IS NULL").
		Join("LEFT JOIN providers AS p ON p.id = mr.provider_id AND p.deleted_at IS NULL").
		Where("mr.deleted_at IS NULL").
		Where(excludeWarmupMessageRequestCondition).
		Where("mr.status_code IS NOT NULL").
		Where("mr.created_at >= ?", startTime).
		Where("mr.created_at <= ?", endTime)
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return rows, nil
}

func roundCost6(value float64) float64 {
	return math.Round(value*1_000_000) / 1_000_000
}

func (r *messageRequestRepository) GetFilterOptions(ctx context.Context) (MessageRequestFilterOptions, error) {
	var models []string
	if err := r.db.NewSelect().
		Model((*model.MessageRequest)(nil)).
		ColumnExpr("DISTINCT model").
		Where("mr.deleted_at IS NULL").
		Where(excludeWarmupMessageRequestCondition).
		Where("mr.model IS NOT NULL").
		Where("mr.model != ''").
		Order("mr.model ASC").
		Scan(ctx, &models); err != nil {
		return MessageRequestFilterOptions{}, errors.NewDatabaseError(err)
	}

	var endpoints []string
	if err := r.db.NewSelect().
		Model((*model.MessageRequest)(nil)).
		ColumnExpr("DISTINCT endpoint").
		Where("mr.deleted_at IS NULL").
		Where(excludeWarmupMessageRequestCondition).
		Where("mr.endpoint IS NOT NULL").
		Where("mr.endpoint != ''").
		Order("mr.endpoint ASC").
		Scan(ctx, &endpoints); err != nil {
		return MessageRequestFilterOptions{}, errors.NewDatabaseError(err)
	}

	var statusCodes []int
	if err := r.db.NewSelect().
		Model((*model.MessageRequest)(nil)).
		ColumnExpr("DISTINCT status_code").
		Where("mr.deleted_at IS NULL").
		Where(excludeWarmupMessageRequestCondition).
		Where("mr.status_code IS NOT NULL").
		Order("mr.status_code ASC").
		Scan(ctx, &statusCodes); err != nil {
		return MessageRequestFilterOptions{}, errors.NewDatabaseError(err)
	}

	return MessageRequestFilterOptions{
		Models:      models,
		StatusCodes: statusCodes,
		Endpoints:   endpoints,
	}, nil
}

func (r *messageRequestRepository) FindSessionIDSuggestions(ctx context.Context, filters MessageRequestSessionIDSuggestionFilters) ([]string, error) {
	term := strings.TrimSpace(filters.Term)
	if term == "" {
		return []string{}, nil
	}
	limit := filters.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	var sessionIDs []string
	query := r.db.NewSelect().
		Model((*model.MessageRequest)(nil)).
		ColumnExpr("mr.session_id").
		ColumnExpr("MIN(mr.created_at) AS first_seen").
		Where("mr.deleted_at IS NULL").
		Where("("+excludeWarmupMessageRequestCondition+")").
		Where("mr.session_id IS NOT NULL").
		Where("mr.session_id != ''").
		Where("mr.session_id LIKE ?", term+"%")
	if filters.KeyID != nil {
		query = query.Join("JOIN keys AS k ON k.key = mr.key").
			Where("k.id = ?", *filters.KeyID)
	}
	if filters.UserID != nil {
		query = query.Where("mr.user_id = ?", *filters.UserID)
	}
	if filters.ProviderID != nil {
		query = query.Where("mr.provider_id = ?", *filters.ProviderID)
	}
	err := query.
		Group("mr.session_id").
		OrderExpr("MIN(mr.created_at) DESC").
		Limit(limit).
		Scan(ctx, &sessionIDs)
	if err != nil {
		return nil, errors.NewDatabaseError(err)
	}
	return sessionIDs, nil
}
