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
	"github.com/gin-gonic/gin"
)

type leaderboardLogStore interface {
	ListLeaderboardRows(ctx context.Context, startTime, endTime time.Time) ([]repository.LeaderboardRequestRow, error)
}

type LeaderboardHandler struct {
	auth adminAuthenticator
	logs leaderboardLogStore
	now  func() time.Time
}

func NewLeaderboardHandler(auth adminAuthenticator, logs leaderboardLogStore) *LeaderboardHandler {
	return &LeaderboardHandler{auth: auth, logs: logs, now: time.Now}
}

func (h *LeaderboardHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/api/leaderboard", h.getLeaderboard)
}

func (h *LeaderboardHandler) getLeaderboard(c *gin.Context) {
	if h == nil || h.auth == nil || h.logs == nil {
		writeAdminError(c, appErrors.NewInternalError("排行榜服务未初始化"))
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
	scope, startTime, endTime, err := decodeLeaderboardQuery(c, h.now())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	rows, err := h.logs.ListLeaderboardRows(c.Request.Context(), startTime, endTime)
	if err != nil {
		writeAdminError(c, err)
		return
	}

	switch scope {
	case "user":
		c.JSON(http.StatusOK, buildUserLeaderboard(rows))
	case "provider":
		c.JSON(http.StatusOK, buildProviderLeaderboard(rows))
	case "model":
		c.JSON(http.StatusOK, buildModelLeaderboard(rows))
	default:
		writeAdminError(c, appErrors.NewInvalidRequest("scope 不支持"))
	}
}

func decodeLeaderboardQuery(c *gin.Context, now time.Time) (scope string, startTime, endTime time.Time, err error) {
	scope = strings.TrimSpace(c.DefaultQuery("scope", "user"))
	if scope != "user" && scope != "provider" && scope != "model" {
		return "", time.Time{}, time.Time{}, appErrors.NewInvalidRequest("scope 仅支持 user/provider/model")
	}
	period := strings.TrimSpace(c.DefaultQuery("period", "daily"))
	location, locErr := time.LoadLocation(repository.DefaultTimezone)
	if locErr != nil {
		location = time.Local
	}
	localNow := now.In(location)
	switch period {
	case "daily":
		startTime = time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
		endTime = startTime.Add(24 * time.Hour)
	case "weekly":
		weekday := int(localNow.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		startTime = time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location).AddDate(0, 0, -(weekday - 1))
		endTime = startTime.AddDate(0, 0, 7)
	case "monthly":
		startTime = time.Date(localNow.Year(), localNow.Month(), 1, 0, 0, 0, 0, location)
		endTime = startTime.AddDate(0, 1, 0)
	case "allTime":
		startTime = time.Unix(0, 0).In(location)
		endTime = localNow.Add(24 * time.Hour)
	case "custom":
		startDate := strings.TrimSpace(c.Query("startDate"))
		endDate := strings.TrimSpace(c.Query("endDate"))
		if startDate == "" || endDate == "" {
			return "", time.Time{}, time.Time{}, appErrors.NewInvalidRequest("custom period 需要 startDate 和 endDate")
		}
		startParsed, parseErr := time.ParseInLocation("2006-01-02", startDate, location)
		if parseErr != nil {
			return "", time.Time{}, time.Time{}, appErrors.NewInvalidRequest("startDate 格式必须为 YYYY-MM-DD")
		}
		endParsed, parseErr := time.ParseInLocation("2006-01-02", endDate, location)
		if parseErr != nil {
			return "", time.Time{}, time.Time{}, appErrors.NewInvalidRequest("endDate 格式必须为 YYYY-MM-DD")
		}
		if endParsed.Before(startParsed) {
			return "", time.Time{}, time.Time{}, appErrors.NewInvalidRequest("startDate 不能晚于 endDate")
		}
		startTime = startParsed
		endTime = endParsed.Add(24 * time.Hour)
	default:
		return "", time.Time{}, time.Time{}, appErrors.NewInvalidRequest("period 不支持")
	}
	return scope, startTime, endTime, nil
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

func buildUserLeaderboard(rows []repository.LeaderboardRequestRow) []gin.H {
	type aggregate struct {
		userID        int
		userName      string
		totalRequests int
		totalCost     float64
		totalTokens   int
	}
	aggregates := map[int]*aggregate{}
	for _, row := range rows {
		entry := aggregates[row.UserID]
		if entry == nil {
			entry = &aggregate{userID: row.UserID, userName: row.UserName}
			aggregates[row.UserID] = entry
		}
		entry.totalRequests++
		entry.totalCost += row.CostUSD.InexactFloat64()
		entry.totalTokens += leaderboardTotalTokens(row)
	}
	result := make([]gin.H, 0, len(aggregates))
	for _, entry := range aggregates {
		result = append(result, gin.H{
			"userId":        entry.userID,
			"userName":      entry.userName,
			"totalRequests": entry.totalRequests,
			"totalCost":     leaderboardRound6(entry.totalCost),
			"totalTokens":   entry.totalTokens,
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

func buildProviderLeaderboard(rows []repository.LeaderboardRequestRow) []gin.H {
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
	for _, row := range rows {
		if row.ProviderID <= 0 {
			continue
		}
		entry := aggregates[row.ProviderID]
		if entry == nil {
			entry = &aggregate{providerID: row.ProviderID, providerName: row.ProviderName}
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
		result = append(result, gin.H{
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

func leaderboardRound6(value float64) float64 {
	return math.Round(value*1_000_000) / 1_000_000
}
