package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	livechainsvc "github.com/ding113/claude-code-hub/internal/service/livechain"
	"github.com/gin-gonic/gin"
	"github.com/quagmt/udecimal"
)

type usageLogsStore interface {
	ListRecent(ctx context.Context, limit int) ([]*model.MessageRequest, error)
	ListFiltered(ctx context.Context, limit int, filters repository.MessageRequestQueryFilters) ([]*model.MessageRequest, error)
	ListPaginatedFiltered(ctx context.Context, page, pageSize int, filters repository.MessageRequestQueryFilters) (repository.MessageRequestListResult, error)
	ListBatch(ctx context.Context, filters repository.MessageRequestBatchFilters) (repository.MessageRequestBatchResult, error)
	GetByID(ctx context.Context, id int) (*model.MessageRequest, error)
	GetSummary(ctx context.Context, filters repository.MessageRequestQueryFilters) (repository.MessageRequestSummary, error)
	GetOverviewMetrics(ctx context.Context, now time.Time, location *time.Location) (repository.MessageRequestOverviewMetrics, error)
	GetFilterOptions(ctx context.Context) (repository.MessageRequestFilterOptions, error)
	FindSessionIDSuggestions(ctx context.Context, filters repository.MessageRequestSessionIDSuggestionFilters) ([]string, error)
}

type UsageLogsActionHandler struct {
	auth  adminAuthenticator
	store usageLogsStore
}

const usageLogsFilterOptionsCacheTTL = 5 * time.Minute

var usageLogsFilterOptionsNow = time.Now

type usageLogsFilterOptionsCacheEntry struct {
	options   repository.MessageRequestFilterOptions
	expiresAt time.Time
}

type usageLogsFilterOptionsCacheStore struct {
	mu    sync.Mutex
	entry *usageLogsFilterOptionsCacheEntry
}

var defaultUsageLogsFilterOptionsCache = &usageLogsFilterOptionsCacheStore{}

func (s *usageLogsFilterOptionsCacheStore) get() (repository.MessageRequestFilterOptions, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entry == nil {
		return repository.MessageRequestFilterOptions{}, false
	}
	if !s.entry.expiresAt.After(usageLogsFilterOptionsNow()) {
		s.entry = nil
		return repository.MessageRequestFilterOptions{}, false
	}
	return s.entry.options, true
}

func (s *usageLogsFilterOptionsCacheStore) set(options repository.MessageRequestFilterOptions) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entry = &usageLogsFilterOptionsCacheEntry{
		options:   options,
		expiresAt: usageLogsFilterOptionsNow().Add(usageLogsFilterOptionsCacheTTL),
	}
}

type usageLogsFilterInput struct {
	Page                 int                                   `json:"page"`
	PageSize             int                                   `json:"pageSize"`
	Limit                int                                   `json:"limit"`
	UserID               *int                                  `json:"userId"`
	KeyID                *int                                  `json:"keyId"`
	ProviderID           *int                                  `json:"providerId"`
	MinRetryCount        *int                                  `json:"minRetryCount"`
	SessionID            string                                `json:"sessionId"`
	StartTime            *int64                                `json:"startTime"`
	EndTime              *int64                                `json:"endTime"`
	StatusCode           *int                                  `json:"statusCode"`
	ExcludeStatusCode200 bool                                  `json:"excludeStatusCode200"`
	Model                string                                `json:"model"`
	Endpoint             string                                `json:"endpoint"`
	Cursor               *repository.MessageRequestBatchCursor `json:"cursor"`
}

type usageLogSessionIDSuggestionInput struct {
	Term       string `json:"term"`
	Limit      *int   `json:"limit"`
	UserID     *int   `json:"userId"`
	KeyID      *int   `json:"keyId"`
	ProviderID *int   `json:"providerId"`
}

func NewUsageLogsActionHandler(auth adminAuthenticator, store usageLogsStore) *UsageLogsActionHandler {
	return &UsageLogsActionHandler{auth: auth, store: store}
}

func (h *UsageLogsActionHandler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("/usage-logs")
	protected.Use((&Handler{auth: h.auth}).AdminAuthMiddleware())
	protected.GET("", h.list)
	protected.POST("/getUsageLogs", h.list)
	protected.POST("/getUsageLogsBatch", h.batch)
	protected.POST("/startUsageLogsExport", h.startExport)
	protected.POST("/getUsageLogsExportStatus", h.exportStatus)
	protected.POST("/downloadUsageLogsExport", h.downloadExport)
	protected.GET("/models", h.models)
	protected.POST("/getModelList", h.models)
	protected.GET("/status-codes", h.statusCodes)
	protected.POST("/getStatusCodeList", h.statusCodes)
	protected.GET("/endpoints", h.endpoints)
	protected.POST("/getEndpointList", h.endpoints)
	protected.GET("/summary", h.summary)
	protected.POST("/getUsageLogsStats", h.summary)
	protected.GET("/filter-options", h.filterOptions)
	protected.POST("/getFilterOptions", h.filterOptions)
	protected.GET("/session-id-suggestions", h.sessionIDSuggestions)
	protected.POST("/getUsageLogSessionIdSuggestions", h.sessionIDSuggestionsAction)
	protected.GET("/:id", h.detail)
}

func (h *UsageLogsActionHandler) list(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("日志仓储未初始化"))
		return
	}
	input, err := decodeUsageLogsFilterInput(c)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	page := input.Page
	if page <= 0 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize <= 0 {
		if input.Limit > 0 {
			pageSize = input.Limit
		} else {
			pageSize = 50
		}
	}
	filters := input.toRepositoryFilters()
	result, err := h.store.ListPaginatedFiltered(c.Request.Context(), page, pageSize, filters)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	summary, err := h.store.GetSummary(c.Request.Context(), filters)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	logs := make([]gin.H, 0, len(result.Logs))
	for _, log := range result.Logs {
		logs = append(logs, buildUsageLogResponse(log))
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{
		"logs":     logs,
		"count":    len(logs),
		"total":    result.Total,
		"page":     result.Page,
		"pageSize": result.PageSize,
		"summary":  summary,
		"filters": gin.H{
			"userId":               filters.UserID,
			"keyId":                filters.KeyID,
			"providerId":           filters.ProviderID,
			"minRetryCount":        filters.MinRetryCount,
			"model":                filters.Model,
			"endpoint":             filters.Endpoint,
			"sessionId":            filters.SessionID,
			"statusCode":           filters.StatusCode,
			"excludeStatusCode200": filters.ExcludeStatusCode200,
			"startTime":            input.StartTime,
			"endTime":              input.EndTime,
		},
	}})
}

func (h *UsageLogsActionHandler) detail(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("日志仓储未初始化"))
		return
	}
	id, ok := parseIntParam(c, "id")
	if !ok {
		return
	}
	log, err := h.store.GetByID(c.Request.Context(), id)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": buildUsageLogResponse(log)})
}

func (h *UsageLogsActionHandler) batch(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("日志仓储未初始化"))
		return
	}
	input, err := decodeUsageLogsFilterInput(c)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	result, err := h.store.ListBatch(c.Request.Context(), repository.MessageRequestBatchFilters{
		MessageRequestQueryFilters: input.toRepositoryFilters(),
		Cursor:                     input.Cursor,
		Limit:                      normalizeUsageLogsBatchLimit(input.Limit),
	})
	if err != nil {
		writeAdminError(c, err)
		return
	}
	liveSnapshots := map[string]livechainsvc.Snapshot{}
	keys := make([]livechainsvc.Key, 0, len(result.Logs))
	for _, log := range result.Logs {
		if log == nil || log.SessionID == nil || *log.SessionID == "" || log.RequestSequence <= 0 {
			continue
		}
		if log.DurationMs != nil || log.StatusCode != nil {
			continue
		}
		keys = append(keys, livechainsvc.Key{SessionID: *log.SessionID, RequestSequence: log.RequestSequence})
	}
	if len(keys) > 0 {
		if snapshots, snapshotErr := livechainsvc.ReadBatch(c.Request.Context(), keys); snapshotErr == nil {
			liveSnapshots = snapshots
		}
	}
	logs := make([]gin.H, 0, len(result.Logs))
	for _, log := range result.Logs {
		payload := buildUsageLogResponse(log)
		if log != nil && log.SessionID != nil && *log.SessionID != "" && log.RequestSequence > 0 {
			if snapshot, ok := liveSnapshots[*log.SessionID+":"+strconv.Itoa(log.RequestSequence)]; ok {
				payload["_liveChain"] = snapshot
			}
		}
		logs = append(logs, payload)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{
		"logs":       logs,
		"nextCursor": result.NextCursor,
		"hasMore":    result.HasMore,
	}})
}

func (h *UsageLogsActionHandler) models(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("日志仓储未初始化"))
		return
	}
	options, err := h.getFilterOptions(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": options.Models})
}

func (h *UsageLogsActionHandler) statusCodes(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("日志仓储未初始化"))
		return
	}
	options, err := h.getFilterOptions(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": options.StatusCodes})
}

func (h *UsageLogsActionHandler) endpoints(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("日志仓储未初始化"))
		return
	}
	options, err := h.getFilterOptions(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": options.Endpoints})
}

func (h *UsageLogsActionHandler) summary(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("日志仓储未初始化"))
		return
	}
	input, err := decodeUsageLogsFilterInput(c)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	summary, err := h.store.GetSummary(c.Request.Context(), input.toRepositoryFilters())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": summary})
}

func (h *UsageLogsActionHandler) filterOptions(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("日志仓储未初始化"))
		return
	}
	options, err := h.getFilterOptions(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": options})
}

func (h *UsageLogsActionHandler) sessionIDSuggestions(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("日志仓储未初始化"))
		return
	}
	input, err := decodeUsageLogSessionIDSuggestionInput(c)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	h.respondSessionIDSuggestions(c, input)
}

func (h *UsageLogsActionHandler) sessionIDSuggestionsAction(c *gin.Context) {
	input, err := decodeUsageLogSessionIDSuggestionInput(c)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	h.respondSessionIDSuggestions(c, input)
}

func (h *UsageLogsActionHandler) respondSessionIDSuggestions(c *gin.Context, input usageLogSessionIDSuggestionInput) {
	term := strings.TrimSpace(input.Term)
	if len(term) < 2 {
		c.JSON(http.StatusOK, gin.H{"ok": true, "data": []string{}})
		return
	}
	limit := 20
	if input.Limit != nil {
		limit = *input.Limit
	}
	results, err := h.store.FindSessionIDSuggestions(c.Request.Context(), repository.MessageRequestSessionIDSuggestionFilters{
		Term:       term,
		Limit:      limit,
		UserID:     input.UserID,
		KeyID:      input.KeyID,
		ProviderID: input.ProviderID,
	})
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": results})
}

func decodeUsageLogsFilterInput(c *gin.Context) (usageLogsFilterInput, error) {
	var input usageLogsFilterInput
	if c.Request.Method == http.MethodPost {
		if err := bindOptionalJSON(c, &input); err != nil {
			return input, err
		}
		if input.Limit == 0 {
			input.Limit = parseOptionalIntQuery(c, "limit", input.Limit)
		}
		if input.Page == 0 {
			input.Page = parseOptionalIntQuery(c, "page", input.Page)
		}
		if input.PageSize == 0 {
			input.PageSize = parseOptionalIntQuery(c, "pageSize", input.PageSize)
		}
		return input, nil
	}
	var err error
	input.Limit, err = parseOptionalIntQueryStrict(c, "limit", 50)
	if err != nil {
		return input, err
	}
	input.Page, err = parseOptionalIntQueryStrict(c, "page", 1)
	if err != nil {
		return input, err
	}
	input.PageSize, err = parseOptionalIntQueryStrict(c, "pageSize", 0)
	if err != nil {
		return input, err
	}
	input.UserID, err = parseOptionalIntPointerQuery(c, "userId")
	if err != nil {
		return input, err
	}
	input.KeyID, err = parseOptionalIntPointerQuery(c, "keyId")
	if err != nil {
		return input, err
	}
	input.ProviderID, err = parseOptionalIntPointerQuery(c, "providerId")
	if err != nil {
		return input, err
	}
	input.MinRetryCount, err = parseOptionalIntPointerQuery(c, "minRetryCount")
	if err != nil {
		return input, err
	}
	input.StartTime, err = parseOptionalInt64PointerQuery(c, "startTime")
	if err != nil {
		return input, err
	}
	input.EndTime, err = parseOptionalInt64PointerQuery(c, "endTime")
	if err != nil {
		return input, err
	}
	input.StatusCode, err = parseOptionalIntPointerQuery(c, "statusCode")
	if err != nil {
		return input, err
	}
	input.ExcludeStatusCode200, err = parseOptionalBoolQuery(c, "excludeStatusCode200")
	if err != nil {
		return input, err
	}
	input.Model = c.Query("model")
	input.Endpoint = c.Query("endpoint")
	input.SessionID = c.Query("sessionId")
	return input, nil
}

func decodeUsageLogSessionIDSuggestionInput(c *gin.Context) (usageLogSessionIDSuggestionInput, error) {
	var input usageLogSessionIDSuggestionInput
	if c.Request.Method == http.MethodPost {
		if err := bindOptionalJSON(c, &input); err != nil {
			return input, err
		}
		if input.Term == "" {
			input.Term = c.Query("term")
		}
		if input.Limit == nil {
			if limit, err := parseOptionalIntPointerQuery(c, "limit"); err != nil {
				return input, err
			} else {
				input.Limit = limit
			}
		}
		return input, nil
	}
	var err error
	input.Term = c.Query("term")
	input.Limit, err = parseOptionalIntPointerQuery(c, "limit")
	if err != nil {
		return input, err
	}
	input.UserID, err = parseOptionalIntPointerQuery(c, "userId")
	if err != nil {
		return input, err
	}
	input.KeyID, err = parseOptionalIntPointerQuery(c, "keyId")
	if err != nil {
		return input, err
	}
	input.ProviderID, err = parseOptionalIntPointerQuery(c, "providerId")
	if err != nil {
		return input, err
	}
	return input, nil
}

func bindOptionalJSON(c *gin.Context, input any) error {
	if c.Request.ContentLength == 0 {
		return nil
	}
	if err := c.ShouldBindJSON(input); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return appErrors.NewInvalidRequest("请求体不是合法 JSON")
	}
	return nil
}

func parseOptionalIntQuery(c *gin.Context, key string, fallback int) int {
	if raw := c.Query(key); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			return value
		}
	}
	return fallback
}

func parseOptionalIntQueryStrict(c *gin.Context, key string, fallback int) (int, error) {
	if raw := c.Query(key); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return 0, appErrors.NewInvalidRequest(key + " 必须是整数")
		}
		return value, nil
	}
	return fallback, nil
}

func parseOptionalIntPointerQuery(c *gin.Context, key string) (*int, error) {
	if raw := c.Query(key); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return nil, appErrors.NewInvalidRequest(key + " 必须是整数")
		}
		return &value, nil
	}
	return nil, nil
}

func parseOptionalInt64PointerQuery(c *gin.Context, key string) (*int64, error) {
	if raw := c.Query(key); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, appErrors.NewInvalidRequest(key + " 必须是整数")
		}
		return &value, nil
	}
	return nil, nil
}

func parseOptionalBoolQuery(c *gin.Context, key string) (bool, error) {
	if raw := c.Query(key); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return false, appErrors.NewInvalidRequest(key + " 必须是布尔值")
		}
		return value, nil
	}
	return false, nil
}

func (i usageLogsFilterInput) toRepositoryFilters() repository.MessageRequestQueryFilters {
	filters := repository.MessageRequestQueryFilters{
		UserID:               i.UserID,
		KeyID:                i.KeyID,
		ProviderID:           i.ProviderID,
		SessionID:            strings.TrimSpace(i.SessionID),
		StatusCode:           i.StatusCode,
		ExcludeStatusCode200: i.ExcludeStatusCode200,
		Model:                strings.TrimSpace(i.Model),
		Endpoint:             strings.TrimSpace(i.Endpoint),
		MinRetryCount:        i.MinRetryCount,
	}
	if i.StartTime != nil {
		start := time.UnixMilli(*i.StartTime)
		filters.StartTime = &start
	}
	if i.EndTime != nil {
		end := time.UnixMilli(*i.EndTime)
		filters.EndTime = &end
	}
	return filters
}

func normalizeUsageLogsBatchLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func (h *UsageLogsActionHandler) getFilterOptions(ctx context.Context) (repository.MessageRequestFilterOptions, error) {
	if options, ok := defaultUsageLogsFilterOptionsCache.get(); ok {
		return options, nil
	}
	options, err := h.store.GetFilterOptions(ctx)
	if err != nil {
		return repository.MessageRequestFilterOptions{}, err
	}
	defaultUsageLogsFilterOptionsCache.set(options)
	return options, nil
}

func buildUsageLogResponse(log *model.MessageRequest) gin.H {
	if log == nil {
		return gin.H{}
	}
	userName := ""
	if log.UserName != nil {
		userName = strings.TrimSpace(*log.UserName)
	}
	if userName == "" {
		userName = "User #" + strconv.Itoa(log.UserID)
	}

	keyName := ""
	if log.KeyName != nil {
		keyName = strings.TrimSpace(*log.KeyName)
	}
	if keyName == "" {
		keyName = log.Key
	}

	var providerName any = nil
	if log.ProviderName != nil && strings.TrimSpace(*log.ProviderName) != "" {
		providerName = strings.TrimSpace(*log.ProviderName)
	}
	var costMultiplier any = nil
	if log.CostMultiplier != nil {
		costMultiplier = log.CostMultiplier.String()
	}
	var groupCostMultiplier any = nil
	if log.GroupCostMultiplier != nil {
		groupCostMultiplier = log.GroupCostMultiplier.String()
	}
	var anthropicEffort any = nil
	for _, setting := range log.SpecialSettings {
		if strings.EqualFold(setting.Type, "anthropic_effort") && setting.Effort != nil && strings.TrimSpace(*setting.Effort) != "" {
			anthropicEffort = strings.TrimSpace(*setting.Effort)
			break
		}
	}
	var providerChain any = nil
	if len(log.ProviderChain) > 0 {
		providerChain = log.ProviderChain
	}
	var specialSettings any = nil
	if len(log.SpecialSettings) > 0 {
		specialSettings = log.SpecialSettings
	}

	return gin.H{
		"id":                         log.ID,
		"createdAt":                  log.CreatedAt,
		"updatedAt":                  log.UpdatedAt,
		"deletedAt":                  log.DeletedAt,
		"sessionId":                  log.SessionID,
		"requestSequence":            log.RequestSequence,
		"userId":                     log.UserID,
		"userName":                   userName,
		"key":                        log.Key,
		"keyName":                    keyName,
		"providerId":                 log.ProviderID,
		"providerName":               providerName,
		"model":                      log.Model,
		"originalModel":              log.OriginalModel,
		"endpoint":                   log.Endpoint,
		"statusCode":                 log.StatusCode,
		"inputTokens":                log.InputTokens,
		"outputTokens":               log.OutputTokens,
		"cacheCreationInputTokens":   log.CacheCreationInputTokens,
		"cacheReadInputTokens":       log.CacheReadInputTokens,
		"cacheCreation5mInputTokens": log.CacheCreation5mInputTokens,
		"cacheCreation1hInputTokens": log.CacheCreation1hInputTokens,
		"cacheTtlApplied":            log.CacheTtlApplied,
		"totalTokens":                log.TotalTokens(),
		"costUsd":                    formatUsageLogCost(log.CostUSD),
		"costMultiplier":             costMultiplier,
		"groupCostMultiplier":        groupCostMultiplier,
		"durationMs":                 log.DurationMs,
		"ttfbMs":                     log.TtfbMs,
		"costBreakdown":              log.CostBreakdown,
		"errorMessage":               log.ErrorMessage,
		"providerChain":              providerChain,
		"blockedBy":                  log.BlockedBy,
		"blockedReason":              log.BlockedReason,
		"userAgent":                  log.UserAgent,
		"clientIp":                   log.ClientIP,
		"messagesCount":              log.MessagesCount,
		"context1mApplied":           log.Context1mApplied,
		"swapCacheTtlApplied":        log.SwapCacheTtlApplied,
		"specialSettings":            specialSettings,
		"anthropicEffort":            anthropicEffort,
	}
}

func formatUsageLogCost(value interface {
	RoundHAZ(uint8) udecimal.Decimal
}) string {
	return value.RoundHAZ(6).StringFixed(6)
}
