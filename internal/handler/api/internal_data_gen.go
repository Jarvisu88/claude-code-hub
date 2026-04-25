package api

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	"github.com/gin-gonic/gin"
)

type internalDataGenLogStore interface {
	ListFiltered(ctx context.Context, limit int, filters repository.MessageRequestQueryFilters) ([]*model.MessageRequest, error)
}

type InternalDataGenHandler struct {
	auth adminAuthenticator
	logs internalDataGenLogStore
}

func NewInternalDataGenHandler(auth adminAuthenticator, logs internalDataGenLogStore) *InternalDataGenHandler {
	return &InternalDataGenHandler{auth: auth, logs: logs}
}

func (h *InternalDataGenHandler) RegisterRoutes(router gin.IRouter) {
	router.POST("/api/internal/data-gen", h.generate)
}

func (h *InternalDataGenHandler) generate(c *gin.Context) {
	if h == nil || h.auth == nil || h.logs == nil {
		writeAdminError(c, appErrors.NewInternalError("数据生成服务未初始化"))
		return
	}
	authResult, err := h.auth.AuthenticateAdminToken(resolveAdminToken(c))
	if err != nil {
		writeAdminError(c, err)
		return
	}
	if authResult == nil || !authResult.IsAdmin {
		writeAdminError(c, appErrors.NewPermissionDenied("权限不足", appErrors.CodePermissionDenied))
		return
	}

	var input struct {
		Mode         string   `json:"mode"`
		ServiceName  string   `json:"serviceName"`
		StartDate    string   `json:"startDate"`
		EndDate      string   `json:"endDate"`
		TotalRecords *int     `json:"totalRecords"`
		Models       []string `json:"models"`
		UserIDs      []int    `json:"userIds"`
		ProviderIDs  []int    `json:"providerIds"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	if strings.TrimSpace(input.StartDate) == "" || strings.TrimSpace(input.EndDate) == "" {
		writeAdminError(c, appErrors.NewInvalidRequest("startDate and endDate are required"))
		return
	}
	startDate, err := parseInternalDataGenTime(input.StartDate)
	if err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("startDate 不是合法时间"))
		return
	}
	endDate, err := parseInternalDataGenTime(input.EndDate)
	if err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("endDate 不是合法时间"))
		return
	}
	if !startDate.Before(endDate) {
		writeAdminError(c, appErrors.NewInvalidRequest("startDate 必须早于 endDate"))
		return
	}

	logs, err := h.logs.ListFiltered(c.Request.Context(), 5000, repository.MessageRequestQueryFilters{
		StartTime: &startDate,
		EndTime:   &endDate,
	})
	if err != nil {
		writeAdminError(c, err)
		return
	}
	logs = filterInternalDataGenLogs(logs, input.Models, input.UserIDs, input.ProviderIDs)
	if input.TotalRecords != nil && *input.TotalRecords > 0 && len(logs) > *input.TotalRecords {
		logs = logs[:*input.TotalRecords]
	}

	if strings.TrimSpace(input.Mode) == "userBreakdown" {
		serviceName := strings.TrimSpace(input.ServiceName)
		if serviceName == "" {
			serviceName = "AI大模型推理服务"
		}
		c.JSON(http.StatusOK, buildUserBreakdownResponse(logs, serviceName))
		return
	}
	c.JSON(http.StatusOK, buildGeneratedUsageResponse(logs))
}

func parseInternalDataGenTime(raw string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed, nil
	}
	return time.Parse("2006-01-02T15:04", raw)
}

func filterInternalDataGenLogs(logs []*model.MessageRequest, models []string, userIDs []int, providerIDs []int) []*model.MessageRequest {
	modelSet := map[string]struct{}{}
	for _, modelName := range models {
		if trimmed := strings.TrimSpace(modelName); trimmed != "" {
			modelSet[trimmed] = struct{}{}
		}
	}
	userSet := map[int]struct{}{}
	for _, userID := range userIDs {
		if userID > 0 {
			userSet[userID] = struct{}{}
		}
	}
	providerSet := map[int]struct{}{}
	for _, providerID := range providerIDs {
		if providerID > 0 {
			providerSet[providerID] = struct{}{}
		}
	}
	filtered := make([]*model.MessageRequest, 0, len(logs))
	for _, log := range logs {
		if log == nil {
			continue
		}
		if len(modelSet) > 0 {
			if _, ok := modelSet[strings.TrimSpace(log.Model)]; !ok {
				continue
			}
		}
		if len(userSet) > 0 {
			if _, ok := userSet[log.UserID]; !ok {
				continue
			}
		}
		if len(providerSet) > 0 {
			if _, ok := providerSet[log.ProviderID]; !ok {
				continue
			}
		}
		filtered = append(filtered, log)
	}
	return filtered
}

func buildGeneratedUsageResponse(logs []*model.MessageRequest) gin.H {
	totalCost := 0.0
	totalTokens := 0
	totalInputTokens := 0
	totalOutputTokens := 0
	totalCacheCreationTokens := 0
	totalCacheReadTokens := 0
	rows := make([]gin.H, 0, len(logs))
	for _, log := range logs {
		if log == nil {
			continue
		}
		inputTokens := 0
		outputTokens := 0
		cacheCreation := 0
		cacheRead := 0
		if log.InputTokens != nil {
			inputTokens = *log.InputTokens
		}
		if log.OutputTokens != nil {
			outputTokens = *log.OutputTokens
		}
		if log.CacheCreationInputTokens != nil {
			cacheCreation = *log.CacheCreationInputTokens
		}
		if log.CacheReadInputTokens != nil {
			cacheRead = *log.CacheReadInputTokens
		}
		total := inputTokens + outputTokens + cacheCreation + cacheRead
		totalCost += log.CostUSD.InexactFloat64()
		totalTokens += total
		totalInputTokens += inputTokens
		totalOutputTokens += outputTokens
		totalCacheCreationTokens += cacheCreation
		totalCacheReadTokens += cacheRead
		rows = append(rows, gin.H{
			"id":                       log.ID,
			"createdAt":                log.CreatedAt,
			"sessionId":                log.SessionID,
			"userName":                 optionalString(log.UserName, "Unknown"),
			"keyName":                  optionalString(log.KeyName, log.Key),
			"providerName":             optionalString(log.ProviderName, "unknown"),
			"model":                    log.Model,
			"statusCode":               derefInt(log.StatusCode),
			"inputTokens":              inputTokens,
			"outputTokens":             outputTokens,
			"cacheCreationInputTokens": cacheCreation,
			"cacheReadInputTokens":     cacheRead,
			"totalTokens":              total,
			"costUsd":                  formatUsageLogCost(log.CostUSD),
			"durationMs":               derefInt(log.DurationMs),
			"errorMessage":             log.ErrorMessage,
			"providerChain":            nullableProviderChain(log.ProviderChain),
			"blockedBy":                log.BlockedBy,
			"blockedReason":            log.BlockedReason,
		})
	}
	return gin.H{
		"logs": rows,
		"summary": gin.H{
			"totalRecords":             len(rows),
			"totalCost":                leaderboardRound6(totalCost),
			"totalTokens":              totalTokens,
			"totalInputTokens":         totalInputTokens,
			"totalOutputTokens":        totalOutputTokens,
			"totalCacheCreationTokens": totalCacheCreationTokens,
			"totalCacheReadTokens":     totalCacheReadTokens,
		},
	}
}

func buildUserBreakdownResponse(logs []*model.MessageRequest, serviceName string) gin.H {
	type aggregate struct {
		userName   string
		keyName    string
		model      string
		totalCalls int
		totalCost  float64
	}
	aggregates := map[string]*aggregate{}
	userSet := map[string]struct{}{}
	keySet := map[string]struct{}{}
	modelSet := map[string]struct{}{}
	for _, log := range logs {
		if log == nil {
			continue
		}
		userName := optionalString(log.UserName, "Unknown")
		keyName := optionalString(log.KeyName, log.Key)
		modelName := strings.TrimSpace(log.Model)
		if modelName == "" {
			modelName = "Unknown"
		}
		key := userName + "|" + keyName + "|" + modelName
		entry := aggregates[key]
		if entry == nil {
			entry = &aggregate{userName: userName, keyName: keyName, model: modelName}
			aggregates[key] = entry
		}
		entry.totalCalls++
		entry.totalCost += log.CostUSD.InexactFloat64()
		userSet[userName] = struct{}{}
		keySet[keyName] = struct{}{}
		modelSet[modelName] = struct{}{}
	}
	items := make([]gin.H, 0, len(aggregates))
	totalCalls := 0
	totalCost := 0.0
	for _, entry := range aggregates {
		totalCalls += entry.totalCalls
		totalCost += entry.totalCost
		items = append(items, gin.H{
			"userName":    entry.userName,
			"keyName":     entry.keyName,
			"model":       entry.model,
			"serviceName": serviceName,
			"totalCalls":  entry.totalCalls,
			"totalCost":   leaderboardRound6(entry.totalCost),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i]["totalCost"].(float64) != items[j]["totalCost"].(float64) {
			return items[i]["totalCost"].(float64) > items[j]["totalCost"].(float64)
		}
		return items[i]["totalCalls"].(int) > items[j]["totalCalls"].(int)
	})
	return gin.H{
		"items": items,
		"summary": gin.H{
			"totalCalls":   totalCalls,
			"totalCost":    leaderboardRound6(totalCost),
			"uniqueUsers":  len(userSet),
			"uniqueKeys":   len(keySet),
			"uniqueModels": len(modelSet),
		},
	}
}

func optionalString(value *string, fallback string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return fallback
	}
	return strings.TrimSpace(*value)
}

func derefInt(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func nullableProviderChain(chain []model.ProviderChainItem) any {
	if len(chain) == 0 {
		return nil
	}
	return chain
}
