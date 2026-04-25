package api

import (
	"context"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

type leaderboardLogStore interface {
	ListLeaderboardRows(ctx context.Context, startTime, endTime time.Time) ([]repository.LeaderboardRequestRow, error)
}

type leaderboardAuthenticator interface {
	AuthenticateAdminToken(token string) (*authsvc.AuthResult, error)
	AuthenticateProxy(ctx context.Context, input authsvc.ProxyAuthInput) (*authsvc.AuthResult, error)
}

type LeaderboardHandler struct {
	auth     leaderboardAuthenticator
	settings systemSettingsStore
	logs     leaderboardLogStore
	now      func() time.Time
}

func NewLeaderboardHandler(auth leaderboardAuthenticator, settings systemSettingsStore, logs leaderboardLogStore) *LeaderboardHandler {
	return &LeaderboardHandler{auth: auth, settings: settings, logs: logs, now: time.Now}
}

func (h *LeaderboardHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/api/leaderboard", h.getLeaderboard)
}

func (h *LeaderboardHandler) getLeaderboard(c *gin.Context) {
	if h == nil || h.auth == nil || h.logs == nil || h.settings == nil {
		writeAdminError(c, appErrors.NewInternalError("排行榜服务未初始化"))
		return
	}
	if !h.ensureAuthorized(c) {
		return
	}
	options, err := decodeLeaderboardQuery(c, h.now())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	rows, err := h.logs.ListLeaderboardRows(c.Request.Context(), options.startTime, options.endTime)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	if options.scope == "user" || options.scope == "userCacheHitRate" {
		rows = filterLeaderboardRowsByUser(rows, options.userTagFilters, options.userGroupFilters)
	}

	switch options.scope {
	case "user":
		c.JSON(http.StatusOK, buildUserLeaderboard(rows, options.includeUserModelStats))
	case "provider":
		c.JSON(http.StatusOK, buildProviderLeaderboard(rows, options.providerTypeFilter, options.includeModelStats))
	case "userCacheHitRate":
		c.JSON(http.StatusOK, buildUserCacheHitRateLeaderboard(rows, options.includeUserModelStats))
	case "providerCacheHitRate":
		c.JSON(http.StatusOK, buildProviderCacheHitRateLeaderboard(rows, options.providerTypeFilter))
	case "model":
		c.JSON(http.StatusOK, buildModelLeaderboard(rows))
	default:
		writeAdminError(c, appErrors.NewInvalidRequest("scope 不支持"))
	}
}

func (h *LeaderboardHandler) ensureAuthorized(c *gin.Context) bool {
	token := resolveAdminToken(c)
	if token == "" {
		writeAdminError(c, appErrors.NewAuthenticationError("未授权，请先登录", appErrors.CodeUnauthorized))
		return false
	}
	if authResult, err := h.auth.AuthenticateAdminToken(token); err == nil && authResult != nil && authResult.IsAdmin {
		return true
	}
	authResult, err := h.auth.AuthenticateProxy(c.Request.Context(), authsvc.ProxyAuthInput{APIKeyHeader: token})
	if err != nil || authResult == nil {
		writeAdminError(c, appErrors.NewAuthenticationError("未授权，请先登录", appErrors.CodeUnauthorized))
		return false
	}
	settings, settingsErr := h.settings.Get(c.Request.Context())
	if settingsErr != nil {
		writeAdminError(c, settingsErr)
		return false
	}
	if settings != nil && settings.AllowGlobalUsageView {
		return true
	}
	writeAdminError(c, appErrors.NewPermissionDenied("无权限访问排行榜，请联系管理员开启全站使用量查看权限", appErrors.CodePermissionDenied))
	return false
}

type leaderboardQueryOptions struct {
	scope                 string
	startTime             time.Time
	endTime               time.Time
	includeModelStats     bool
	includeUserModelStats bool
	providerTypeFilter    string
	userTagFilters        []string
	userGroupFilters      []string
}

func decodeLeaderboardQuery(c *gin.Context, now time.Time) (options leaderboardQueryOptions, err error) {
	options.scope = strings.TrimSpace(c.DefaultQuery("scope", "user"))
	if options.scope != "user" && options.scope != "provider" && options.scope != "model" && options.scope != "userCacheHitRate" && options.scope != "providerCacheHitRate" {
		return options, appErrors.NewInvalidRequest("scope 不支持")
	}
	period := strings.TrimSpace(c.DefaultQuery("period", "daily"))
	location, locErr := time.LoadLocation(repository.DefaultTimezone)
	if locErr != nil {
		location = time.Local
	}
	localNow := now.In(location)
	switch period {
	case "daily":
		options.startTime = time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
		options.endTime = options.startTime.Add(24 * time.Hour)
	case "weekly":
		weekday := int(localNow.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		options.startTime = time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location).AddDate(0, 0, -(weekday - 1))
		options.endTime = options.startTime.AddDate(0, 0, 7)
	case "monthly":
		options.startTime = time.Date(localNow.Year(), localNow.Month(), 1, 0, 0, 0, 0, location)
		options.endTime = options.startTime.AddDate(0, 1, 0)
	case "allTime":
		options.startTime = time.Unix(0, 0).In(location)
		options.endTime = localNow.Add(24 * time.Hour)
	case "custom":
		startDate := strings.TrimSpace(c.Query("startDate"))
		endDate := strings.TrimSpace(c.Query("endDate"))
		if startDate == "" || endDate == "" {
			return options, appErrors.NewInvalidRequest("custom period 需要 startDate 和 endDate")
		}
		startParsed, parseErr := time.ParseInLocation("2006-01-02", startDate, location)
		if parseErr != nil {
			return options, appErrors.NewInvalidRequest("startDate 格式必须为 YYYY-MM-DD")
		}
		endParsed, parseErr := time.ParseInLocation("2006-01-02", endDate, location)
		if parseErr != nil {
			return options, appErrors.NewInvalidRequest("endDate 格式必须为 YYYY-MM-DD")
		}
		if endParsed.Before(startParsed) {
			return options, appErrors.NewInvalidRequest("startDate 不能晚于 endDate")
		}
		options.startTime = startParsed
		options.endTime = endParsed.Add(24 * time.Hour)
	default:
		return options, appErrors.NewInvalidRequest("period 不支持")
	}
	if options.scope == "provider" || options.scope == "providerCacheHitRate" {
		options.includeModelStats = parseTruthyQuery(c.Query("includeModelStats"))
		providerType := strings.TrimSpace(c.Query("providerType"))
		if providerType != "" && providerType != "all" {
			switch providerType {
			case "claude", "claude-auth", "codex", "gemini", "gemini-cli", "openai-compatible":
				options.providerTypeFilter = providerType
			default:
				return options, appErrors.NewInvalidRequest("providerType 不支持")
			}
		}
	}
	if options.scope == "user" || options.scope == "userCacheHitRate" {
		options.includeUserModelStats = parseTruthyQuery(c.Query("includeUserModelStats"))
		options.userTagFilters = parseCSVFilter(c.Query("userTags"))
		options.userGroupFilters = parseCSVFilter(c.Query("userGroups"))
	}
	return options, nil
}

func leaderboardTotalTokens(row repository.LeaderboardRequestRow) int {
	total := 0
	if row.InputTokens != nil {
		total += *row.InputTokens
	}
	if row.OutputTokens != nil {
		total += *row.OutputTokens
	}
	if row.CacheCreationInputTokens != nil {
		total += *row.CacheCreationInputTokens
	}
	if row.CacheReadInputTokens != nil {
		total += *row.CacheReadInputTokens
	}
	return total
}

func buildUserLeaderboard(rows []repository.LeaderboardRequestRow, includeModelStats bool) []gin.H {
	type aggregate struct {
		userID        int
		userName      string
		totalRequests int
		totalCost     float64
		totalTokens   int
		modelStats    map[string]*aggregate
	}
	aggregates := map[int]*aggregate{}
	for _, row := range rows {
		entry := aggregates[row.UserID]
		if entry == nil {
			entry = &aggregate{userID: row.UserID, userName: row.UserName, modelStats: map[string]*aggregate{}}
			aggregates[row.UserID] = entry
		}
		entry.totalRequests++
		entry.totalCost += row.CostUSD.InexactFloat64()
		entry.totalTokens += leaderboardTotalTokens(row)
		if includeModelStats {
			modelName := strings.TrimSpace(row.Model)
			if modelName == "" {
				modelName = "Unknown"
			}
			modelEntry := entry.modelStats[modelName]
			if modelEntry == nil {
				modelEntry = &aggregate{userName: modelName}
				entry.modelStats[modelName] = modelEntry
			}
			modelEntry.totalRequests++
			modelEntry.totalCost += row.CostUSD.InexactFloat64()
			modelEntry.totalTokens += leaderboardTotalTokens(row)
		}
	}
	result := make([]gin.H, 0, len(aggregates))
	for _, entry := range aggregates {
		row := gin.H{
			"userId":        entry.userID,
			"userName":      entry.userName,
			"totalRequests": entry.totalRequests,
			"totalCost":     leaderboardRound6(entry.totalCost),
			"totalTokens":   entry.totalTokens,
		}
		if includeModelStats {
			modelStats := make([]gin.H, 0, len(entry.modelStats))
			for modelName, modelEntry := range entry.modelStats {
				modelStats = append(modelStats, gin.H{
					"model":         modelName,
					"totalRequests": modelEntry.totalRequests,
					"totalCost":     leaderboardRound6(modelEntry.totalCost),
					"totalTokens":   modelEntry.totalTokens,
				})
			}
			sort.Slice(modelStats, func(i, j int) bool {
				if modelStats[i]["totalCost"].(float64) != modelStats[j]["totalCost"].(float64) {
					return modelStats[i]["totalCost"].(float64) > modelStats[j]["totalCost"].(float64)
				}
				return modelStats[i]["totalRequests"].(int) > modelStats[j]["totalRequests"].(int)
			})
			row["modelStats"] = modelStats
		}
		result = append(result, row)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i]["totalCost"].(float64) != result[j]["totalCost"].(float64) {
			return result[i]["totalCost"].(float64) > result[j]["totalCost"].(float64)
		}
		return result[i]["totalRequests"].(int) > result[j]["totalRequests"].(int)
	})
	return result
}

func buildProviderLeaderboard(rows []repository.LeaderboardRequestRow, providerTypeFilter string, includeModelStats bool) []gin.H {
	type aggregate struct {
		providerID    int
		providerName  string
		totalRequests int
		totalCost     float64
		totalTokens   int
		successCount  int
		ttfbTotal     int
		ttfbCount     int
		tpsTotal      float64
		tpsCount      int
		modelStats    map[string]*aggregate
	}
	aggregates := map[int]*aggregate{}
	for _, row := range rows {
		if row.ProviderID <= 0 {
			continue
		}
		if providerTypeFilter != "" && row.ProviderType != providerTypeFilter {
			continue
		}
		entry := aggregates[row.ProviderID]
		if entry == nil {
			entry = &aggregate{providerID: row.ProviderID, providerName: row.ProviderName, modelStats: map[string]*aggregate{}}
			aggregates[row.ProviderID] = entry
		}
		totalTokens := leaderboardTotalTokens(row)
		entry.totalRequests++
		entry.totalCost += row.CostUSD.InexactFloat64()
		entry.totalTokens += totalTokens
		if row.StatusCode >= 200 && row.StatusCode < 300 {
			entry.successCount++
		}
		if row.TtfbMs != nil {
			entry.ttfbTotal += *row.TtfbMs
			entry.ttfbCount++
		}
		if row.DurationMs != nil && *row.DurationMs > 0 && totalTokens > 0 {
			entry.tpsTotal += float64(totalTokens) / (float64(*row.DurationMs) / 1000.0)
			entry.tpsCount++
		}
		if includeModelStats {
			modelName := strings.TrimSpace(row.Model)
			if modelName == "" {
				modelName = "Unknown"
			}
			modelEntry := entry.modelStats[modelName]
			if modelEntry == nil {
				modelEntry = &aggregate{providerName: modelName}
				entry.modelStats[modelName] = modelEntry
			}
			modelEntry.totalRequests++
			modelEntry.totalCost += row.CostUSD.InexactFloat64()
			modelEntry.totalTokens += totalTokens
			if row.StatusCode >= 200 && row.StatusCode < 300 {
				modelEntry.successCount++
			}
			if row.TtfbMs != nil {
				modelEntry.ttfbTotal += *row.TtfbMs
				modelEntry.ttfbCount++
			}
			if row.DurationMs != nil && *row.DurationMs > 0 && totalTokens > 0 {
				modelEntry.tpsTotal += float64(totalTokens) / (float64(*row.DurationMs) / 1000.0)
				modelEntry.tpsCount++
			}
		}
	}
	result := make([]gin.H, 0, len(aggregates))
	for _, entry := range aggregates {
		successRate := 0.0
		avgTtfb := 0
		avgTps := 0.0
		var avgCostPerRequest any = nil
		var avgCostPerMillion any = nil
		if entry.totalRequests > 0 {
			successRate = float64(entry.successCount) / float64(entry.totalRequests)
			avgCostPerRequest = leaderboardRound6(entry.totalCost / float64(entry.totalRequests))
		}
		if entry.ttfbCount > 0 {
			avgTtfb = entry.ttfbTotal / entry.ttfbCount
		}
		if entry.tpsCount > 0 {
			avgTps = leaderboardRound6(entry.tpsTotal / float64(entry.tpsCount))
		}
		if entry.totalTokens > 0 {
			avgCostPerMillion = leaderboardRound6(entry.totalCost * 1_000_000 / float64(entry.totalTokens))
		}
		row := gin.H{
			"providerId":              entry.providerID,
			"providerName":            entry.providerName,
			"totalRequests":           entry.totalRequests,
			"totalCost":               leaderboardRound6(entry.totalCost),
			"totalTokens":             entry.totalTokens,
			"successRate":             successRate,
			"avgTtfbMs":               avgTtfb,
			"avgTokensPerSecond":      avgTps,
			"avgCostPerRequest":       avgCostPerRequest,
			"avgCostPerMillionTokens": avgCostPerMillion,
		}
		if includeModelStats {
			modelStats := make([]gin.H, 0, len(entry.modelStats))
			for modelName, modelEntry := range entry.modelStats {
				modelSuccess := 0.0
				modelAvgTtfb := 0
				modelAvgTps := 0.0
				var modelAvgCostPerRequest any = nil
				var modelAvgCostPerMillion any = nil
				if modelEntry.totalRequests > 0 {
					modelSuccess = float64(modelEntry.successCount) / float64(modelEntry.totalRequests)
					modelAvgCostPerRequest = leaderboardRound6(modelEntry.totalCost / float64(modelEntry.totalRequests))
				}
				if modelEntry.ttfbCount > 0 {
					modelAvgTtfb = modelEntry.ttfbTotal / modelEntry.ttfbCount
				}
				if modelEntry.tpsCount > 0 {
					modelAvgTps = leaderboardRound6(modelEntry.tpsTotal / float64(modelEntry.tpsCount))
				}
				if modelEntry.totalTokens > 0 {
					modelAvgCostPerMillion = leaderboardRound6(modelEntry.totalCost * 1_000_000 / float64(modelEntry.totalTokens))
				}
				modelStats = append(modelStats, gin.H{
					"model":                   modelName,
					"totalRequests":           modelEntry.totalRequests,
					"totalCost":               leaderboardRound6(modelEntry.totalCost),
					"totalTokens":             modelEntry.totalTokens,
					"successRate":             modelSuccess,
					"avgTtfbMs":               modelAvgTtfb,
					"avgTokensPerSecond":      modelAvgTps,
					"avgCostPerRequest":       modelAvgCostPerRequest,
					"avgCostPerMillionTokens": modelAvgCostPerMillion,
				})
			}
			sort.Slice(modelStats, func(i, j int) bool {
				if modelStats[i]["totalCost"].(float64) != modelStats[j]["totalCost"].(float64) {
					return modelStats[i]["totalCost"].(float64) > modelStats[j]["totalCost"].(float64)
				}
				return modelStats[i]["totalRequests"].(int) > modelStats[j]["totalRequests"].(int)
			})
			row["modelStats"] = modelStats
		}
		result = append(result, row)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i]["totalCost"].(float64) != result[j]["totalCost"].(float64) {
			return result[i]["totalCost"].(float64) > result[j]["totalCost"].(float64)
		}
		return result[i]["totalRequests"].(int) > result[j]["totalRequests"].(int)
	})
	return result
}

func parseTruthyQuery(value string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	return trimmed == "1" || trimmed == "true" || trimmed == "yes"
}

func parseCSVFilter(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		cleaned := strings.TrimSpace(part)
		if cleaned == "" {
			continue
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		result = append(result, cleaned)
	}
	return result
}

func filterLeaderboardRowsByUser(rows []repository.LeaderboardRequestRow, tagFilters, groupFilters []string) []repository.LeaderboardRequestRow {
	if len(tagFilters) == 0 && len(groupFilters) == 0 {
		return rows
	}
	tagSet := map[string]struct{}{}
	for _, tag := range tagFilters {
		tagSet[tag] = struct{}{}
	}
	groupSet := map[string]struct{}{}
	for _, group := range groupFilters {
		groupSet[group] = struct{}{}
	}
	filtered := make([]repository.LeaderboardRequestRow, 0, len(rows))
	for _, row := range rows {
		tagMatch := len(tagSet) == 0
		for _, tag := range row.UserTags {
			if _, ok := tagSet[strings.TrimSpace(tag)]; ok {
				tagMatch = true
				break
			}
		}
		groupMatch := len(groupSet) == 0
		if row.UserProviderGroup != nil {
			for _, part := range strings.Split(*row.UserProviderGroup, ",") {
				if _, ok := groupSet[strings.TrimSpace(part)]; ok {
					groupMatch = true
					break
				}
			}
		}
		if tagMatch && groupMatch {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func buildModelLeaderboard(rows []repository.LeaderboardRequestRow) []gin.H {
	type aggregate struct {
		model         string
		totalRequests int
		totalCost     float64
		totalTokens   int
		successCount  int
	}
	aggregates := map[string]*aggregate{}
	for _, row := range rows {
		modelName := strings.TrimSpace(row.Model)
		if modelName == "" {
			modelName = "Unknown"
		}
		entry := aggregates[modelName]
		if entry == nil {
			entry = &aggregate{model: modelName}
			aggregates[modelName] = entry
		}
		entry.totalRequests++
		entry.totalCost += row.CostUSD.InexactFloat64()
		entry.totalTokens += leaderboardTotalTokens(row)
		if row.StatusCode >= 200 && row.StatusCode < 300 {
			entry.successCount++
		}
	}
	result := make([]gin.H, 0, len(aggregates))
	for _, entry := range aggregates {
		successRate := 0.0
		if entry.totalRequests > 0 {
			successRate = float64(entry.successCount) / float64(entry.totalRequests)
		}
		result = append(result, gin.H{
			"model":         entry.model,
			"totalRequests": entry.totalRequests,
			"totalCost":     leaderboardRound6(entry.totalCost),
			"totalTokens":   entry.totalTokens,
			"successRate":   successRate,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i]["totalCost"].(float64) != result[j]["totalCost"].(float64) {
			return result[i]["totalCost"].(float64) > result[j]["totalCost"].(float64)
		}
		return result[i]["totalRequests"].(int) > result[j]["totalRequests"].(int)
	})
	return result
}

func cacheHitTotalInputTokens(row repository.LeaderboardRequestRow) int {
	total := 0
	if row.InputTokens != nil {
		total += *row.InputTokens
	}
	if row.CacheCreationInputTokens != nil {
		total += *row.CacheCreationInputTokens
	}
	if row.CacheReadInputTokens != nil {
		total += *row.CacheReadInputTokens
	}
	return total
}

func buildUserCacheHitRateLeaderboard(rows []repository.LeaderboardRequestRow, includeModelStats bool) []gin.H {
	type aggregate struct {
		userID           int
		userName         string
		totalRequests    int
		cacheReadTokens  int
		totalInputTokens int
		totalCost        float64
		modelStats       map[string]*aggregate
	}
	aggregates := map[int]*aggregate{}
	for _, row := range rows {
		entry := aggregates[row.UserID]
		if entry == nil {
			entry = &aggregate{userID: row.UserID, userName: row.UserName, modelStats: map[string]*aggregate{}}
			aggregates[row.UserID] = entry
		}
		entry.totalRequests++
		if row.CacheReadInputTokens != nil {
			entry.cacheReadTokens += *row.CacheReadInputTokens
		}
		entry.totalInputTokens += cacheHitTotalInputTokens(row)
		entry.totalCost += row.CostUSD.InexactFloat64()
		if includeModelStats {
			modelName := strings.TrimSpace(row.Model)
			if modelName == "" {
				modelName = "Unknown"
			}
			modelEntry := entry.modelStats[modelName]
			if modelEntry == nil {
				modelEntry = &aggregate{userName: modelName}
				entry.modelStats[modelName] = modelEntry
			}
			modelEntry.totalRequests++
			if row.CacheReadInputTokens != nil {
				modelEntry.cacheReadTokens += *row.CacheReadInputTokens
			}
			modelEntry.totalInputTokens += cacheHitTotalInputTokens(row)
		}
	}
	result := make([]gin.H, 0, len(aggregates))
	for _, entry := range aggregates {
		cacheHitRate := 0.0
		if entry.totalInputTokens > 0 {
			cacheHitRate = float64(entry.cacheReadTokens) / float64(entry.totalInputTokens)
		}
		row := gin.H{
			"userId":            entry.userID,
			"userName":          entry.userName,
			"totalRequests":     entry.totalRequests,
			"cacheReadTokens":   entry.cacheReadTokens,
			"totalInputTokens":  entry.totalInputTokens,
			"totalTokens":       entry.totalInputTokens,
			"totalCost":         leaderboardRound6(entry.totalCost),
			"cacheCreationCost": 0,
			"cacheHitRate":      cacheHitRate,
		}
		if includeModelStats {
			modelStats := make([]gin.H, 0, len(entry.modelStats))
			for modelName, modelEntry := range entry.modelStats {
				modelRate := 0.0
				if modelEntry.totalInputTokens > 0 {
					modelRate = float64(modelEntry.cacheReadTokens) / float64(modelEntry.totalInputTokens)
				}
				modelStats = append(modelStats, gin.H{
					"model":            modelName,
					"totalRequests":    modelEntry.totalRequests,
					"cacheReadTokens":  modelEntry.cacheReadTokens,
					"totalInputTokens": modelEntry.totalInputTokens,
					"cacheHitRate":     modelRate,
				})
			}
			sort.Slice(modelStats, func(i, j int) bool {
				if modelStats[i]["cacheHitRate"].(float64) != modelStats[j]["cacheHitRate"].(float64) {
					return modelStats[i]["cacheHitRate"].(float64) > modelStats[j]["cacheHitRate"].(float64)
				}
				return modelStats[i]["totalRequests"].(int) > modelStats[j]["totalRequests"].(int)
			})
			row["modelStats"] = modelStats
		}
		result = append(result, row)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i]["cacheHitRate"].(float64) != result[j]["cacheHitRate"].(float64) {
			return result[i]["cacheHitRate"].(float64) > result[j]["cacheHitRate"].(float64)
		}
		return result[i]["totalRequests"].(int) > result[j]["totalRequests"].(int)
	})
	return result
}

func buildProviderCacheHitRateLeaderboard(rows []repository.LeaderboardRequestRow, providerTypeFilter string) []gin.H {
	type aggregate struct {
		providerID       int
		providerName     string
		totalRequests    int
		cacheReadTokens  int
		totalInputTokens int
		totalCost        float64
		modelStats       map[string]*aggregate
	}
	aggregates := map[int]*aggregate{}
	for _, row := range rows {
		if row.ProviderID <= 0 {
			continue
		}
		if providerTypeFilter != "" && row.ProviderType != providerTypeFilter {
			continue
		}
		entry := aggregates[row.ProviderID]
		if entry == nil {
			entry = &aggregate{providerID: row.ProviderID, providerName: row.ProviderName, modelStats: map[string]*aggregate{}}
			aggregates[row.ProviderID] = entry
		}
		entry.totalRequests++
		if row.CacheReadInputTokens != nil {
			entry.cacheReadTokens += *row.CacheReadInputTokens
		}
		entry.totalInputTokens += cacheHitTotalInputTokens(row)
		entry.totalCost += row.CostUSD.InexactFloat64()
		modelName := strings.TrimSpace(row.Model)
		if modelName == "" {
			modelName = "Unknown"
		}
		modelEntry := entry.modelStats[modelName]
		if modelEntry == nil {
			modelEntry = &aggregate{providerName: modelName}
			entry.modelStats[modelName] = modelEntry
		}
		modelEntry.totalRequests++
		if row.CacheReadInputTokens != nil {
			modelEntry.cacheReadTokens += *row.CacheReadInputTokens
		}
		modelEntry.totalInputTokens += cacheHitTotalInputTokens(row)
	}
	result := make([]gin.H, 0, len(aggregates))
	for _, entry := range aggregates {
		cacheHitRate := 0.0
		if entry.totalInputTokens > 0 {
			cacheHitRate = float64(entry.cacheReadTokens) / float64(entry.totalInputTokens)
		}
		modelStats := make([]gin.H, 0, len(entry.modelStats))
		for modelName, modelEntry := range entry.modelStats {
			modelRate := 0.0
			if modelEntry.totalInputTokens > 0 {
				modelRate = float64(modelEntry.cacheReadTokens) / float64(modelEntry.totalInputTokens)
			}
			modelStats = append(modelStats, gin.H{
				"model":            modelName,
				"totalRequests":    modelEntry.totalRequests,
				"cacheReadTokens":  modelEntry.cacheReadTokens,
				"totalInputTokens": modelEntry.totalInputTokens,
				"cacheHitRate":     modelRate,
			})
		}
		sort.Slice(modelStats, func(i, j int) bool {
			if modelStats[i]["cacheHitRate"].(float64) != modelStats[j]["cacheHitRate"].(float64) {
				return modelStats[i]["cacheHitRate"].(float64) > modelStats[j]["cacheHitRate"].(float64)
			}
			return modelStats[i]["totalRequests"].(int) > modelStats[j]["totalRequests"].(int)
		})
		result = append(result, gin.H{
			"providerId":        entry.providerID,
			"providerName":      entry.providerName,
			"totalRequests":     entry.totalRequests,
			"cacheReadTokens":   entry.cacheReadTokens,
			"totalInputTokens":  entry.totalInputTokens,
			"totalTokens":       entry.totalInputTokens,
			"totalCost":         leaderboardRound6(entry.totalCost),
			"cacheCreationCost": 0,
			"cacheHitRate":      cacheHitRate,
			"modelStats":        modelStats,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i]["cacheHitRate"].(float64) != result[j]["cacheHitRate"].(float64) {
			return result[i]["cacheHitRate"].(float64) > result[j]["cacheHitRate"].(float64)
		}
		return result[i]["totalRequests"].(int) > result[j]["totalRequests"].(int)
	})
	return result
}

func leaderboardRound6(value float64) float64 {
	return math.Round(value*1_000_000) / 1_000_000
}
