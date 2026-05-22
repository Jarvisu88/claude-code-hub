package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ding113/claude-code-hub/internal/database"
	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	"github.com/gin-gonic/gin"
)

const (
	usageLogsExportBatchSize = 500
	usageLogsExportJobTTL    = 15 * time.Minute
)

var usageLogsExportNow = time.Now

type usageLogsExportStatus struct {
	JobID           string `json:"jobId"`
	Status          string `json:"status"`
	ProcessedRows   int    `json:"processedRows"`
	TotalRows       int    `json:"totalRows"`
	ProgressPercent int    `json:"progressPercent"`
	Error           string `json:"error,omitempty"`
}

type usageLogsExportRecord struct {
	status    usageLogsExportStatus
	csv       string
	expiresAt time.Time
}

type usageLogsExportStatusStore interface {
	save(record usageLogsExportRecord)
	update(jobID string, mutate func(*usageLogsExportRecord))
	get(jobID string) (usageLogsExportRecord, bool)
}

type usageLogsExportStore struct {
	mu    sync.Mutex
	items map[string]usageLogsExportRecord
}

type redisUsageLogsExportStore struct {
	client *database.RedisClient
}

var defaultUsageLogsExportStore usageLogsExportStatusStore = &usageLogsExportStore{
	items: map[string]usageLogsExportRecord{},
}

func ConfigureUsageLogsExportStore(client *database.RedisClient) {
	if client == nil {
		defaultUsageLogsExportStore = &usageLogsExportStore{items: map[string]usageLogsExportRecord{}}
		return
	}
	defaultUsageLogsExportStore = &redisUsageLogsExportStore{client: client}
}

func (s *usageLogsExportStore) save(record usageLogsExportRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked()
	s.items[record.status.JobID] = record
}

func (s *usageLogsExportStore) update(jobID string, mutate func(*usageLogsExportRecord)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked()
	record, ok := s.items[jobID]
	if !ok {
		return
	}
	mutate(&record)
	record.expiresAt = usageLogsExportNow().Add(usageLogsExportJobTTL)
	s.items[jobID] = record
}

func (s *usageLogsExportStore) get(jobID string) (usageLogsExportRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked()
	record, ok := s.items[jobID]
	return record, ok
}

func (s *redisUsageLogsExportStore) save(record usageLogsExportRecord) {
	if s == nil || s.client == nil {
		return
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return
	}
	_ = s.client.Set(context.Background(), usageLogsExportRedisKey(record.status.JobID), payload, usageLogsExportJobTTL).Err()
}

func (s *redisUsageLogsExportStore) update(jobID string, mutate func(*usageLogsExportRecord)) {
	if s == nil || s.client == nil {
		return
	}
	record, ok := s.get(jobID)
	if !ok {
		return
	}
	mutate(&record)
	record.expiresAt = usageLogsExportNow().Add(usageLogsExportJobTTL)
	s.save(record)
}

func (s *redisUsageLogsExportStore) get(jobID string) (usageLogsExportRecord, bool) {
	if s == nil || s.client == nil {
		return usageLogsExportRecord{}, false
	}
	raw, err := s.client.Get(context.Background(), usageLogsExportRedisKey(jobID)).Result()
	if err != nil || raw == "" {
		return usageLogsExportRecord{}, false
	}
	var record usageLogsExportRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return usageLogsExportRecord{}, false
	}
	if !record.expiresAt.After(usageLogsExportNow()) {
		_ = s.client.Del(context.Background(), usageLogsExportRedisKey(jobID)).Err()
		return usageLogsExportRecord{}, false
	}
	return record, true
}

func usageLogsExportRedisKey(jobID string) string {
	return "cch:usage-logs:export:" + jobID
}

func (s *usageLogsExportStore) cleanupLocked() {
	now := usageLogsExportNow()
	for jobID, record := range s.items {
		if !record.expiresAt.After(now) {
			delete(s.items, jobID)
		}
	}
}

func newUsageLogsExportRecord(status usageLogsExportStatus, csv string) usageLogsExportRecord {
	return usageLogsExportRecord{
		status:    status,
		csv:       csv,
		expiresAt: usageLogsExportNow().Add(usageLogsExportJobTTL),
	}
}

func generateUsageLogsExportJobID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func (h *UsageLogsActionHandler) startExport(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("日志仓储未初始化"))
		return
	}
	input, err := decodeUsageLogsFilterInput(c)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	jobID, err := generateUsageLogsExportJobID()
	if err != nil {
		writeAdminError(c, appErrors.NewInternalError("导出任务创建失败"))
		return
	}

	initialStatus := usageLogsExportStatus{
		JobID:           jobID,
		Status:          "queued",
		ProcessedRows:   0,
		TotalRows:       0,
		ProgressPercent: 0,
	}
	defaultUsageLogsExportStore.save(newUsageLogsExportRecord(initialStatus, ""))
	go h.runUsageLogsExportJob(jobID, input.toRepositoryFilters())

	c.JSON(200, gin.H{"ok": true, "data": gin.H{"jobId": jobID}})
}

func (h *UsageLogsActionHandler) exportStatus(c *gin.Context) {
	jobID := extractUsageLogsExportJobID(c)
	if jobID == "" {
		writeAdminError(c, appErrors.NewInvalidRequest("jobId 不能为空"))
		return
	}
	record, ok := defaultUsageLogsExportStore.get(jobID)
	if !ok {
		writeAdminError(c, appErrors.NewNotFoundError("UsageLogsExportJob"))
		return
	}
	c.JSON(200, gin.H{"ok": true, "data": record.status})
}

func (h *UsageLogsActionHandler) downloadExport(c *gin.Context) {
	jobID := extractUsageLogsExportJobID(c)
	if jobID == "" {
		writeAdminError(c, appErrors.NewInvalidRequest("jobId 不能为空"))
		return
	}
	record, ok := defaultUsageLogsExportStore.get(jobID)
	if !ok {
		writeAdminError(c, appErrors.NewNotFoundError("UsageLogsExportJob"))
		return
	}
	if record.status.Status == "failed" {
		writeAdminError(c, appErrors.NewInvalidRequest(strings.TrimSpace(record.status.Error)))
		return
	}
	if record.status.Status != "completed" {
		writeAdminError(c, appErrors.NewInvalidRequest("导出尚未完成"))
		return
	}
	c.JSON(200, gin.H{"ok": true, "data": record.csv})
}

func extractUsageLogsExportJobID(c *gin.Context) string {
	if raw := strings.TrimSpace(c.Query("jobId")); raw != "" {
		return raw
	}
	var body struct {
		JobID string `json:"jobId"`
	}
	if err := bindOptionalJSON(c, &body); err == nil {
		return strings.TrimSpace(body.JobID)
	}
	return ""
}

func (h *UsageLogsActionHandler) runUsageLogsExportJob(jobID string, filters repository.MessageRequestQueryFilters) {
	defaultUsageLogsExportStore.update(jobID, func(record *usageLogsExportRecord) {
		record.status.Status = "running"
		record.status.Error = ""
		record.status.ProgressPercent = 0
		record.csv = ""
	})

	csv, finalStatus := h.buildUsageLogsExport(context.Background(), jobID, filters, func(progress usageLogsExportStatus) {
		defaultUsageLogsExportStore.update(jobID, func(record *usageLogsExportRecord) {
			record.status = progress
		})
	})
	defaultUsageLogsExportStore.update(jobID, func(record *usageLogsExportRecord) {
		record.status = finalStatus
		record.csv = csv
	})
}

func (h *UsageLogsActionHandler) buildUsageLogsExport(ctx context.Context, jobID string, filters repository.MessageRequestQueryFilters, onProgress func(usageLogsExportStatus)) (string, usageLogsExportStatus) {
	summary, err := h.store.GetSummary(ctx, filters)
	if err != nil {
		return "", usageLogsExportStatus{
			JobID:           jobID,
			Status:          "failed",
			ProcessedRows:   0,
			TotalRows:       0,
			ProgressPercent: 0,
			Error:           err.Error(),
		}
	}

	lines := []string{strings.Join([]string{
		"Time", "User", "Key", "Provider", "Model", "Original Model", "Endpoint", "Status Code",
		"Input Tokens", "Output Tokens", "Cache Write 5m", "Cache Write 1h", "Cache Read",
		"Total Tokens", "Cost (USD)", "Duration (ms)", "Session ID", "Retry Count",
	}, ",")}

	totalRows := summary.TotalRows
	cursor := (*repository.MessageRequestBatchCursor)(nil)
	processedRows := 0

	for {
		batch, batchErr := h.store.ListBatch(ctx, repository.MessageRequestBatchFilters{
			MessageRequestQueryFilters: filters,
			Cursor:                     cursor,
			Limit:                      usageLogsExportBatchSize,
		})
		if batchErr != nil {
			return "", usageLogsExportStatus{
				JobID:           jobID,
				Status:          "failed",
				ProcessedRows:   processedRows,
				TotalRows:       totalRows,
				ProgressPercent: usageLogsExportProgress(processedRows, totalRows, false),
				Error:           batchErr.Error(),
			}
		}
		for _, log := range batch.Logs {
			lines = append(lines, buildUsageLogCSVRow(log))
		}
		processedRows += len(batch.Logs)
		if onProgress != nil {
			onProgress(usageLogsExportStatus{
				JobID:           jobID,
				Status:          "running",
				ProcessedRows:   processedRows,
				TotalRows:       maxInt(totalRows, processedRows),
				ProgressPercent: usageLogsExportProgress(processedRows, totalRows, batch.HasMore),
			})
		}
		if !batch.HasMore || batch.NextCursor == nil {
			break
		}
		cursor = batch.NextCursor
	}

	return "\uFEFF" + strings.Join(lines, "\n"), usageLogsExportStatus{
		JobID:           jobID,
		Status:          "completed",
		ProcessedRows:   processedRows,
		TotalRows:       maxInt(totalRows, processedRows),
		ProgressPercent: 100,
	}
}

func usageLogsExportProgress(processedRows, totalRows int, hasMore bool) int {
	effectiveTotalRows := totalRows
	if effectiveTotalRows < processedRows {
		effectiveTotalRows = processedRows
	}
	if effectiveTotalRows <= 0 {
		if hasMore {
			return 99
		}
		return 100
	}
	if !hasMore {
		return 100
	}
	progress := (processedRows * 100) / effectiveTotalRows
	if progress > 99 {
		return 99
	}
	return progress
}

func buildUsageLogCSVRow(log *model.MessageRequest) string {
	if log == nil {
		return ""
	}
	return strings.Join([]string{
		escapeUsageLogsCSVField(log.CreatedAt.UTC().Format(time.RFC3339)),
		escapeUsageLogsCSVField(usageLogCSVUserName(log)),
		escapeUsageLogsCSVField(usageLogCSVKeyName(log)),
		escapeUsageLogsCSVField(usageLogCSVOptional(log.ProviderName)),
		escapeUsageLogsCSVField(log.Model),
		escapeUsageLogsCSVField(usageLogCSVOptional(log.OriginalModel)),
		escapeUsageLogsCSVField(usageLogCSVOptional(log.Endpoint)),
		escapeUsageLogsCSVField(usageLogCSVIntPointer(log.StatusCode)),
		escapeUsageLogsCSVField(usageLogCSVIntPointer(log.InputTokens)),
		escapeUsageLogsCSVField(usageLogCSVIntPointer(log.OutputTokens)),
		escapeUsageLogsCSVField(usageLogCSVIntPointer(log.CacheCreation5mInputTokens)),
		escapeUsageLogsCSVField(usageLogCSVIntPointer(log.CacheCreation1hInputTokens)),
		escapeUsageLogsCSVField(usageLogCSVIntPointer(log.CacheReadInputTokens)),
		escapeUsageLogsCSVField(usageLogCSVInt(log.TotalTokens())),
		escapeUsageLogsCSVField(formatUsageLogCost(log.CostUSD)),
		escapeUsageLogsCSVField(usageLogCSVIntPointer(log.DurationMs)),
		escapeUsageLogsCSVField(usageLogCSVOptional(log.SessionID)),
		escapeUsageLogsCSVField(usageLogCSVInt(usageLogRetryCount(log))),
	}, ",")
}

func usageLogRetryCount(log *model.MessageRequest) int {
	if log == nil || len(log.ProviderChain) == 0 {
		return 0
	}
	hasHedge := false
	actualRequests := 0
	for _, item := range log.ProviderChain {
		reason := ""
		if item.Reason != nil {
			reason = strings.TrimSpace(*item.Reason)
		}
		switch reason {
		case "hedge_triggered", "hedge_launched", "hedge_winner", "hedge_loser_cancelled":
			hasHedge = true
		case "concurrent_limit_failed", "retry_failed", "system_error", "resource_not_found", "client_error_non_retryable", "endpoint_pool_exhausted", "vendor_type_all_timeout", "client_abort", "http2_fallback":
			actualRequests++
		case "request_success", "retry_success":
			if item.StatusCode != nil {
				actualRequests++
			}
		}
	}
	if hasHedge || actualRequests <= 1 {
		return 0
	}
	return actualRequests - 1
}

func usageLogCSVUserName(log *model.MessageRequest) string {
	if log.UserName != nil && strings.TrimSpace(*log.UserName) != "" {
		return strings.TrimSpace(*log.UserName)
	}
	return "User #" + usageLogCSVInt(log.UserID)
}

func usageLogCSVKeyName(log *model.MessageRequest) string {
	if log.KeyName != nil && strings.TrimSpace(*log.KeyName) != "" {
		return strings.TrimSpace(*log.KeyName)
	}
	return log.Key
}

func usageLogCSVOptional[T ~string](value *T) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(string(*value))
}

func usageLogCSVIntPointer(value *int) string {
	if value == nil {
		return ""
	}
	return usageLogCSVInt(*value)
}

func usageLogCSVInt(value int) string {
	return strconv.Itoa(value)
}

func escapeUsageLogsCSVField(field string) string {
	trimmed := strings.TrimLeft(field, " \t")
	if trimmed != "" {
		switch trimmed[0] {
		case '=', '+', '-', '@':
			field = "'" + field
		}
	}
	if strings.ContainsAny(field, ",\"\n\r") {
		return `"` + strings.ReplaceAll(field, `"`, `""`) + `"`
	}
	return field
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
