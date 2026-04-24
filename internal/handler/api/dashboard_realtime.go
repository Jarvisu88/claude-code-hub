package api

import (
	"context"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	providertrackersvc "github.com/ding113/claude-code-hub/internal/service/providertracker"
	sessiontrackersvc "github.com/ding113/claude-code-hub/internal/service/sessiontracker"
	"github.com/gin-gonic/gin"
)

type dashboardRealtimeStatsStore interface {
	GetUserStatistics(ctx context.Context, timeRange repository.TimeRange, timezone string) ([]*repository.UserStatRow, error)
	GetActiveUsers(ctx context.Context) ([]*repository.ActiveUserItem, error)
}

type dashboardRealtimeProviderStore interface {
	GetActiveProviders(ctx context.Context) ([]*model.Provider, error)
}

type DashboardRealtimeActionHandler struct {
	auth      adminAuthenticator
	logs      usageLogsStore
	stats     dashboardRealtimeStatsStore
	providers dashboardRealtimeProviderStore
}

var dashboardRealtimeNow = time.Now

func NewDashboardRealtimeActionHandler(auth adminAuthenticator, logs usageLogsStore, stats dashboardRealtimeStatsStore, providers dashboardRealtimeProviderStore) *DashboardRealtimeActionHandler {
	return &DashboardRealtimeActionHandler{auth: auth, logs: logs, stats: stats, providers: providers}
}

func (h *DashboardRealtimeActionHandler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("/dashboard-realtime")
	protected.Use((&Handler{auth: h.auth}).AdminAuthMiddleware())
	protected.POST("/getDashboardRealtimeData", h.getDashboardRealtimeData)
}

func (h *DashboardRealtimeActionHandler) getDashboardRealtimeData(c *gin.Context) {
	if h == nil || h.logs == nil || h.stats == nil || h.providers == nil {
		writeAdminError(c, appErrors.NewInternalError("数据大盘服务未初始化"))
		return
	}

	location, err := time.LoadLocation(repository.DefaultTimezone)
	if err != nil {
		location = time.Local
	}
	now := dashboardRealtimeNow()
	metrics, err := h.logs.GetOverviewMetrics(c.Request.Context(), now, location)
	if err != nil {
		writeAdminError(c, err)
		return
	}

	activeSessionIDs, err := sessiontrackersvc.ActiveSessionIDs(c.Request.Context(), 20)
	if err != nil {
		activeSessionIDs = nil
	}
	activeSessionLogs, err := h.logs.FindLatestBySessionIDs(c.Request.Context(), activeSessionIDs, 20)
	if err != nil {
		activeSessionLogs = nil
	}

	recentLogs, err := h.logs.ListRecent(c.Request.Context(), 200)
	if err != nil {
		recentLogs = nil
	}
	activityLogs := mergeDashboardLogs(activeSessionLogs, recentLogs)

	statsRows, err := h.stats.GetUserStatistics(c.Request.Context(), repository.TimeRangeToday, repository.DefaultTimezone)
	if err != nil {
		statsRows = nil
	}
	activeUsers, err := h.stats.GetActiveUsers(c.Request.Context())
	if err != nil {
		activeUsers = nil
	}
	activeProviders, err := h.providers.GetActiveProviders(c.Request.Context())
	if err != nil {
		activeProviders = nil
	}
	statsPayload := buildUserStatisticsResponse(repository.TimeRangeToday, statsRows, activeUsers)
	trendData := buildDashboardTrendData(statsPayload["chartData"])
	todayCompletedLogs := filterDashboardLogsForToday(recentLogs, now, location)
	userRankings := buildDashboardUserRankings(todayCompletedLogs)
	providerRankings := buildDashboardProviderRankings(todayCompletedLogs)
	modelDistribution := buildDashboardModelDistribution(todayCompletedLogs)

	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{
		"metrics": gin.H{
			"concurrentSessions":                 metrics.ConcurrentSessions,
			"todayRequests":                      metrics.TodayRequests,
			"todayCost":                          metrics.TodayCost,
			"avgResponseTime":                    metrics.AvgResponseTime,
			"todayErrorRate":                     metrics.TodayErrorRate,
			"yesterdaySamePeriodRequests":        metrics.YesterdaySamePeriodRequests,
			"yesterdaySamePeriodCost":            metrics.YesterdaySamePeriodCost,
			"yesterdaySamePeriodAvgResponseTime": metrics.YesterdaySamePeriodAvgResponseTime,
			"recentMinuteRequests":               metrics.RecentMinuteRequests,
		},
		"activityStream":    buildDashboardActivityStream(activityLogs),
		"userRankings":      limitDashboardRows(userRankings, 5),
		"providerRankings":  limitDashboardRows(providerRankings, 5),
		"providerSlots":     buildDashboardProviderSlots(c.Request.Context(), recentLogs, activeProviders, providerRankings),
		"modelDistribution": limitDashboardRows(modelDistribution, 10),
		"trendData":         trendData,
	}})
}

func buildDashboardActivityStream(logs []*model.MessageRequest) []gin.H {
	now := dashboardRealtimeNow()
	unique := map[string]gin.H{}
	for _, log := range logs {
		if log == nil || isWarmupProxyStatusRequest(log) {
			continue
		}
		latency := int(now.Sub(log.CreatedAt).Milliseconds())
		if log.DurationMs != nil {
			latency = *log.DurationMs
		}
		status := 0
		isFinalized := log.DurationMs != nil || log.StatusCode != nil
		if log.DurationMs != nil || log.StatusCode != nil {
			status = 200
			if log.StatusCode != nil {
				status = *log.StatusCode
			}
		}
		user := ""
		if log.UserName != nil {
			user = *log.UserName
		}
		if user == "" {
			user = "Unknown"
		}
		provider := ""
		if isFinalized && log.ProviderName != nil {
			provider = *log.ProviderName
		}
		if isFinalized && provider == "" {
			provider = "Unknown"
		}
		modelName := log.Model
		if log.OriginalModel != nil && *log.OriginalModel != "" {
			modelName = *log.OriginalModel
		}
		if modelName == "" {
			modelName = "Unknown"
		}
		cost := 0.0
		if !log.CostUSD.IsZero() {
			cost = dashboardRound6(log.CostUSD.InexactFloat64())
		}
		id := "req-" + strconv.Itoa(log.ID)
		if log.SessionID != nil && *log.SessionID != "" {
			id = *log.SessionID
		}
		dedupeKey := id
		candidate := gin.H{
			"id":        id,
			"user":      user,
			"model":     modelName,
			"provider":  provider,
			"latency":   latency,
			"status":    status,
			"cost":      cost,
			"startTime": log.CreatedAt.UnixMilli(),
		}
		if existing, ok := unique[dedupeKey]; ok {
			if existingStartTime, ok := existing["startTime"].(int64); ok && existingStartTime >= candidate["startTime"].(int64) {
				continue
			}
		}
		unique[dedupeKey] = candidate
	}
	stream := make([]gin.H, 0, len(unique))
	for _, item := range unique {
		stream = append(stream, item)
	}
	sort.Slice(stream, func(i, j int) bool {
		return stream[i]["startTime"].(int64) > stream[j]["startTime"].(int64)
	})
	if len(stream) > 20 {
		stream = stream[:20]
	}
	return stream
}

func filterDashboardLogsForToday(logs []*model.MessageRequest, now time.Time, location *time.Location) []*model.MessageRequest {
	if location == nil {
		location = time.Local
	}
	localNow := now.In(location)
	todayStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	tomorrowStart := todayStart.Add(24 * time.Hour)

	filtered := make([]*model.MessageRequest, 0, len(logs))
	for _, log := range logs {
		if log == nil || log.DurationMs == nil || isWarmupProxyStatusRequest(log) {
			continue
		}
		createdAt := log.CreatedAt.In(location)
		if createdAt.Before(todayStart) || !createdAt.Before(tomorrowStart) {
			continue
		}
		filtered = append(filtered, log)
	}
	return filtered
}

func mergeDashboardLogs(primary []*model.MessageRequest, secondary []*model.MessageRequest) []*model.MessageRequest {
	merged := make([]*model.MessageRequest, 0, len(primary)+len(secondary))
	seen := map[int]struct{}{}
	appendLogs := func(items []*model.MessageRequest) {
		for _, log := range items {
			if log == nil {
				continue
			}
			if _, ok := seen[log.ID]; ok {
				continue
			}
			seen[log.ID] = struct{}{}
			merged = append(merged, log)
		}
	}
	appendLogs(primary)
	appendLogs(secondary)
	return merged
}

func buildDashboardUserRankings(logs []*model.MessageRequest) []gin.H {
	type aggregate struct {
		userID        int
		userName      string
		totalRequests int
		totalCost     float64
		totalTokens   int
	}
	aggregates := map[int]*aggregate{}
	for _, log := range logs {
		if log == nil || isWarmupProxyStatusRequest(log) || log.DurationMs == nil {
			continue
		}
		userName := "User" + strconv.Itoa(log.UserID)
		if log.UserName != nil && *log.UserName != "" {
			userName = *log.UserName
		}
		entry, ok := aggregates[log.UserID]
		if !ok {
			entry = &aggregate{userID: log.UserID, userName: userName}
			aggregates[log.UserID] = entry
		}
		entry.totalRequests++
		entry.totalCost += log.CostUSD.InexactFloat64()
		entry.totalTokens += log.TotalTokens()
	}
	rankings := make([]gin.H, 0, len(aggregates))
	for _, entry := range aggregates {
		rankings = append(rankings, gin.H{
			"userId":        entry.userID,
			"userName":      entry.userName,
			"totalRequests": entry.totalRequests,
			"totalCost":     dashboardRound6(entry.totalCost),
			"totalTokens":   entry.totalTokens,
		})
	}
	sort.Slice(rankings, func(i, j int) bool {
		if rankings[i]["totalCost"].(float64) != rankings[j]["totalCost"].(float64) {
			return rankings[i]["totalCost"].(float64) > rankings[j]["totalCost"].(float64)
		}
		return rankings[i]["totalRequests"].(int) > rankings[j]["totalRequests"].(int)
	})
	return rankings
}

func buildDashboardProviderRankings(logs []*model.MessageRequest) []gin.H {
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
	}
	aggregates := map[int]*aggregate{}
	for _, log := range logs {
		if log == nil || isWarmupProxyStatusRequest(log) || log.DurationMs == nil {
			continue
		}
		if log.ProviderID == 0 {
			continue
		}
		providerName := "unknown"
		if log.ProviderName != nil && *log.ProviderName != "" {
			providerName = *log.ProviderName
		}
		entry, ok := aggregates[log.ProviderID]
		if !ok {
			entry = &aggregate{providerID: log.ProviderID, providerName: providerName}
			aggregates[log.ProviderID] = entry
		}
		entry.totalRequests++
		entry.totalCost += log.CostUSD.InexactFloat64()
		entry.totalTokens += log.TotalTokens()
		if log.IsSuccess() {
			entry.successCount++
		}
		if log.TtfbMs != nil {
			entry.ttfbTotal += *log.TtfbMs
			entry.ttfbCount++
		}
		if log.DurationMs != nil && *log.DurationMs > 0 && log.TotalTokens() > 0 {
			entry.tpsTotal += float64(log.TotalTokens()) / (float64(*log.DurationMs) / 1000.0)
			entry.tpsCount++
		}
	}
	rankings := make([]gin.H, 0, len(aggregates))
	for _, entry := range aggregates {
		successRate := 0.0
		avgCostPerRequest := any(nil)
		avgCostPerMillionTokens := any(nil)
		avgTtfbMs := 0
		avgTokensPerSecond := 0.0
		if entry.totalRequests > 0 {
			successRate = float64(entry.successCount) / float64(entry.totalRequests)
			avgCostPerRequest = entry.totalCost / float64(entry.totalRequests)
		}
		if entry.ttfbCount > 0 {
			avgTtfbMs = entry.ttfbTotal / entry.ttfbCount
		}
		if entry.tpsCount > 0 {
			avgTokensPerSecond = entry.tpsTotal / float64(entry.tpsCount)
		}
		if entry.totalTokens > 0 {
			avgCostPerMillionTokens = entry.totalCost * 1_000_000 / float64(entry.totalTokens)
		}
		rankings = append(rankings, gin.H{
			"providerId":              entry.providerID,
			"providerName":            entry.providerName,
			"totalRequests":           entry.totalRequests,
			"totalCost":               dashboardRound6(entry.totalCost),
			"totalTokens":             entry.totalTokens,
			"successRate":             successRate,
			"avgTtfbMs":               avgTtfbMs,
			"avgTokensPerSecond":      dashboardRound6(avgTokensPerSecond),
			"avgCostPerRequest":       dashboardMaybeRound6(avgCostPerRequest),
			"avgCostPerMillionTokens": dashboardMaybeRound6(avgCostPerMillionTokens),
		})
	}
	sort.Slice(rankings, func(i, j int) bool {
		if rankings[i]["totalCost"].(float64) != rankings[j]["totalCost"].(float64) {
			return rankings[i]["totalCost"].(float64) > rankings[j]["totalCost"].(float64)
		}
		return rankings[i]["totalRequests"].(int) > rankings[j]["totalRequests"].(int)
	})
	return rankings
}

func buildDashboardModelDistribution(logs []*model.MessageRequest) []gin.H {
	type aggregate struct {
		model         string
		totalRequests int
		totalCost     float64
		totalTokens   int
		successCount  int
	}
	aggregates := map[string]*aggregate{}
	for _, log := range logs {
		if log == nil || isWarmupProxyStatusRequest(log) || log.DurationMs == nil {
			continue
		}
		if log.Model == "" && (log.OriginalModel == nil || *log.OriginalModel == "") {
			continue
		}
		modelName := log.Model
		if log.OriginalModel != nil && *log.OriginalModel != "" {
			modelName = *log.OriginalModel
		}
		if modelName == "" {
			modelName = "Unknown"
		}
		entry, ok := aggregates[modelName]
		if !ok {
			entry = &aggregate{model: modelName}
			aggregates[modelName] = entry
		}
		entry.totalRequests++
		entry.totalCost += log.CostUSD.InexactFloat64()
		entry.totalTokens += log.TotalTokens()
		if log.IsSuccess() {
			entry.successCount++
		}
	}
	distribution := make([]gin.H, 0, len(aggregates))
	for _, entry := range aggregates {
		successRate := 0.0
		if entry.totalRequests > 0 {
			successRate = float64(entry.successCount) / float64(entry.totalRequests)
		}
		distribution = append(distribution, gin.H{
			"model":         entry.model,
			"totalRequests": entry.totalRequests,
			"totalCost":     dashboardRound6(entry.totalCost),
			"totalTokens":   entry.totalTokens,
			"successRate":   successRate,
		})
	}
	sort.Slice(distribution, func(i, j int) bool {
		if distribution[i]["totalRequests"].(int) != distribution[j]["totalRequests"].(int) {
			return distribution[i]["totalRequests"].(int) > distribution[j]["totalRequests"].(int)
		}
		return distribution[i]["totalCost"].(float64) > distribution[j]["totalCost"].(float64)
	})
	return distribution
}

func buildDashboardTrendData(raw any) []gin.H {
	items, ok := raw.([]gin.H)
	if !ok {
		items = []gin.H{}
	}
	valuesByHour := map[int]int{}
	for _, item := range items {
		dateStr, _ := item["date"].(string)
		t, err := time.Parse(time.RFC3339, dateStr)
		if err != nil {
			continue
		}
		value := 0
		for key, field := range item {
			if len(key) > 6 && key[len(key)-6:] == "_calls" {
				switch v := field.(type) {
				case int:
					value += v
				case float64:
					value += int(v)
				}
			}
		}
		valuesByHour[t.UTC().Hour()] += value
	}
	trend := make([]gin.H, 0, 24)
	for hour := 0; hour < 24; hour++ {
		trend = append(trend, gin.H{"hour": hour, "value": valuesByHour[hour]})
	}
	return trend
}

func limitDashboardRows(rows []gin.H, limit int) []gin.H {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	limited := make([]gin.H, limit)
	copy(limited, rows[:limit])
	return limited
}

func dashboardRound6(value float64) float64 {
	return math.Round(value*1_000_000) / 1_000_000
}

func dashboardMaybeRound6(value any) any {
	if value == nil {
		return nil
	}
	if number, ok := value.(float64); ok {
		return dashboardRound6(number)
	}
	return value
}

func buildDashboardProviderSlots(ctx context.Context, logs []*model.MessageRequest, providers []*model.Provider, providerRankings []gin.H) []gin.H {
	activeSessionsByProvider := map[int]map[string]struct{}{}
	for _, log := range logs {
		if log == nil || isWarmupProxyStatusRequest(log) || log.DurationMs != nil {
			continue
		}
		if log.ProviderID == 0 || log.SessionID == nil || *log.SessionID == "" {
			continue
		}
		sessions, ok := activeSessionsByProvider[log.ProviderID]
		if !ok {
			sessions = map[string]struct{}{}
			activeSessionsByProvider[log.ProviderID] = sessions
		}
		sessions[*log.SessionID] = struct{}{}
	}

	volumeByProvider := map[int]float64{}
	for _, ranking := range providerRankings {
		providerID, _ := ranking["providerId"].(int)
		if totalTokens, ok := ranking["totalTokens"].(int); ok {
			volumeByProvider[providerID] = float64(totalTokens)
		}
	}

	slots := make([]gin.H, 0, len(providers))
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		totalSlots := 0
		if provider.LimitConcurrentSessions != nil {
			totalSlots = *provider.LimitConcurrentSessions
		}
		if totalSlots <= 0 {
			continue
		}
		usedSlots := len(activeSessionsByProvider[provider.ID])
		slots = append(slots, gin.H{
			"providerId":  provider.ID,
			"name":        provider.Name,
			"usedSlots":   usedSlots,
			"totalSlots":  totalSlots,
			"totalVolume": volumeByProvider[provider.ID],
		})
	}
	liveCounts, err := providertrackersvc.Count(ctx)
	if err == nil {
		for _, slot := range slots {
			providerID, _ := slot["providerId"].(int)
			slot["usedSlots"] = liveCounts[providerID]
		}
	}
	sort.Slice(slots, func(i, j int) bool {
		leftUsage := float64(slots[i]["usedSlots"].(int)) / float64(slots[i]["totalSlots"].(int))
		rightUsage := float64(slots[j]["usedSlots"].(int)) / float64(slots[j]["totalSlots"].(int))
		if leftUsage != rightUsage {
			return leftUsage > rightUsage
		}
		return slots[i]["providerId"].(int) < slots[j]["providerId"].(int)
	})
	if len(slots) > 3 {
		slots = slots[:3]
	}
	return slots
}
