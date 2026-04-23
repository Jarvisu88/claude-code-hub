package api

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/quagmt/udecimal"
)

type statisticsStore interface {
	GetUserStatistics(ctx context.Context, timeRange repository.TimeRange, timezone string) ([]*repository.UserStatRow, error)
	GetKeyStatistics(ctx context.Context, userID int, timeRange repository.TimeRange, timezone string) ([]*repository.KeyStatRow, error)
	GetMixedStatistics(ctx context.Context, userID int, timeRange repository.TimeRange, timezone string) (*repository.MixedStatistics, error)
	GetActiveUsers(ctx context.Context) ([]*repository.ActiveUserItem, error)
	GetActiveKeysForUser(ctx context.Context, userID int) ([]*repository.ActiveKeyItem, error)
}

type StatisticsActionHandler struct {
	auth  adminAuthenticator
	store statisticsStore
}

func NewStatisticsActionHandler(auth adminAuthenticator, store statisticsStore) *StatisticsActionHandler {
	return &StatisticsActionHandler{auth: auth, store: store}
}

func (h *StatisticsActionHandler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("/statistics")
	protected.Use((&Handler{auth: h.auth}).AdminAuthMiddleware())
	protected.POST("/getUserStatistics", h.getUserStatistics)
}

func (h *StatisticsActionHandler) getUserStatistics(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("统计仓储未初始化"))
		return
	}
	var input struct {
		TimeRange string `json:"timeRange"`
		Mode      string `json:"mode"`
		UserID    *int   `json:"userId"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	if input.TimeRange == "" {
		input.TimeRange = string(repository.TimeRangeToday)
	}
	timeRange := repository.TimeRange(input.TimeRange)
	mode := strings.TrimSpace(input.Mode)
	if mode == "" {
		mode = "users"
	}

	switch mode {
	case "users":
		rows, err := h.store.GetUserStatistics(c.Request.Context(), timeRange, repository.DefaultTimezone)
		if err != nil {
			writeAdminError(c, err)
			return
		}
		users, err := h.store.GetActiveUsers(c.Request.Context())
		if err != nil {
			writeAdminError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "data": buildUserStatisticsResponse(timeRange, rows, users)})
	case "keys":
		if input.UserID == nil {
			writeAdminError(c, appErrors.NewInvalidRequest("keys 模式需要 userId"))
			return
		}
		rows, err := h.store.GetKeyStatistics(c.Request.Context(), *input.UserID, timeRange, repository.DefaultTimezone)
		if err != nil {
			writeAdminError(c, err)
			return
		}
		keys, err := h.store.GetActiveKeysForUser(c.Request.Context(), *input.UserID)
		if err != nil {
			writeAdminError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "data": buildKeyStatisticsResponse(timeRange, rows, keys, "keys")})
	case "mixed":
		if input.UserID == nil {
			writeAdminError(c, appErrors.NewInvalidRequest("mixed 模式需要 userId"))
			return
		}
		mixed, err := h.store.GetMixedStatistics(c.Request.Context(), *input.UserID, timeRange, repository.DefaultTimezone)
		if err != nil {
			writeAdminError(c, err)
			return
		}
		keys, err := h.store.GetActiveKeysForUser(c.Request.Context(), *input.UserID)
		if err != nil {
			writeAdminError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "data": buildMixedStatisticsResponse(timeRange, mixed, keys)})
	default:
		writeAdminError(c, appErrors.NewInvalidRequest("不支持的 statistics mode"))
	}
}

func buildUserStatisticsResponse(timeRange repository.TimeRange, rows []*repository.UserStatRow, users []*repository.ActiveUserItem) gin.H {
	resolution := statisticsResolution(timeRange)
	chartDataList := buildStatisticsChartDataForUsers(rows, resolution, "user")

	userList := make([]gin.H, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		userList = append(userList, gin.H{
			"id":      user.ID,
			"name":    statisticsEntityName(user.Name, "User", user.ID),
			"dataKey": statisticsDataKey("user", user.ID),
		})
	}

	return gin.H{
		"chartData":  chartDataList,
		"users":      userList,
		"timeRange":  string(timeRange),
		"resolution": resolution,
		"mode":       "users",
	}
}

func buildKeyStatisticsResponse(timeRange repository.TimeRange, rows []*repository.KeyStatRow, keys []*repository.ActiveKeyItem, mode string) gin.H {
	resolution := statisticsResolution(timeRange)
	chartDataList := buildStatisticsChartDataForKeys(rows, resolution, "key")

	keyList := make([]gin.H, 0, len(keys))
	for _, key := range keys {
		if key == nil {
			continue
		}
		keyList = append(keyList, gin.H{
			"id":      key.ID,
			"name":    statisticsEntityName(key.Name, "Key", key.ID),
			"dataKey": statisticsDataKey("key", key.ID),
		})
	}

	return gin.H{
		"chartData":  chartDataList,
		"users":      keyList,
		"timeRange":  string(timeRange),
		"resolution": resolution,
		"mode":       mode,
	}
}

func buildMixedStatisticsResponse(timeRange repository.TimeRange, mixed *repository.MixedStatistics, keys []*repository.ActiveKeyItem) gin.H {
	resolution := statisticsResolution(timeRange)
	chartMap := map[string]gin.H{}
	mergeStatisticsChartData(chartMap, buildStatisticsChartMapForKeys(mixed.OwnKeys, resolution, "key"))
	mergeStatisticsChartData(chartMap, buildStatisticsChartMapForUsers(mixed.OthersAggregate, resolution, "key"))

	entities := make([]gin.H, 0, len(keys)+1)
	for _, key := range keys {
		if key == nil {
			continue
		}
		entities = append(entities, gin.H{
			"id":      key.ID,
			"name":    statisticsEntityName(key.Name, "Key", key.ID),
			"dataKey": statisticsDataKey("key", key.ID),
		})
	}
	entities = append(entities, gin.H{
		"id":      -1,
		"name":    "__others__",
		"dataKey": statisticsDataKey("key", -1),
	})

	return gin.H{
		"chartData":  chartMapToSortedList(chartMap),
		"users":      entities,
		"timeRange":  string(timeRange),
		"resolution": resolution,
		"mode":       "mixed",
	}
}

func statisticsResolution(timeRange repository.TimeRange) string {
	resolution := "day"
	if timeRange == repository.TimeRangeToday {
		resolution = "hour"
	}
	return resolution
}

func buildStatisticsChartDataForUsers(rows []*repository.UserStatRow, resolution string, prefix string) []gin.H {
	return chartMapToSortedList(buildStatisticsChartMapForUsers(rows, resolution, prefix))
}

func buildStatisticsChartMapForUsers(rows []*repository.UserStatRow, resolution string, prefix string) map[string]gin.H {
	chartData := make(map[string]gin.H)
	for _, row := range rows {
		if row == nil {
			continue
		}
		dateKey := formatStatisticsDate(row.Date, resolution)
		entry, ok := chartData[dateKey]
		if !ok {
			entry = gin.H{"date": dateKey}
			chartData[dateKey] = entry
		}
		dataKey := statisticsDataKey(prefix, row.UserID)
		entry[dataKey+"_cost"] = formatStatisticsCost(row.TotalCost)
		entry[dataKey+"_calls"] = row.APICalls
	}
	return chartData
}

func buildStatisticsChartDataForKeys(rows []*repository.KeyStatRow, resolution string, prefix string) []gin.H {
	return chartMapToSortedList(buildStatisticsChartMapForKeys(rows, resolution, prefix))
}

func buildStatisticsChartMapForKeys(rows []*repository.KeyStatRow, resolution string, prefix string) map[string]gin.H {
	chartData := make(map[string]gin.H)
	for _, row := range rows {
		if row == nil {
			continue
		}
		dateKey := formatStatisticsDate(row.Date, resolution)
		entry, ok := chartData[dateKey]
		if !ok {
			entry = gin.H{"date": dateKey}
			chartData[dateKey] = entry
		}
		dataKey := statisticsDataKey(prefix, row.KeyID)
		entry[dataKey+"_cost"] = formatStatisticsCost(row.TotalCost)
		entry[dataKey+"_calls"] = row.APICalls
	}
	return chartData
}

func formatStatisticsCost(value udecimal.Decimal) string {
	return value.RoundHAZ(6).StringFixed(6)
}

func mergeStatisticsChartData(dst map[string]gin.H, src map[string]gin.H) {
	for dateKey, srcEntry := range src {
		dstEntry, ok := dst[dateKey]
		if !ok {
			dstEntry = gin.H{"date": dateKey}
			dst[dateKey] = dstEntry
		}
		for key, value := range srcEntry {
			if key == "date" {
				continue
			}
			dstEntry[key] = value
		}
	}
}

func chartMapToSortedList(chartData map[string]gin.H) []gin.H {
	sortedDates := make([]string, 0, len(chartData))
	for dateKey := range chartData {
		sortedDates = append(sortedDates, dateKey)
	}
	sort.Strings(sortedDates)
	chartDataList := make([]gin.H, 0, len(sortedDates))
	for _, dateKey := range sortedDates {
		chartDataList = append(chartDataList, chartData[dateKey])
	}
	return chartDataList
}

func statisticsDataKey(prefix string, id int) string {
	return prefix + "-" + strconv.Itoa(id)
}

func formatStatisticsDate(value time.Time, resolution string) string {
	if resolution == "hour" {
		return value.UTC().Format(time.RFC3339)
	}
	return value.Format("2006-01-02")
}

func statisticsEntityName(name string, fallbackPrefix string, id int) string {
	name = strings.TrimSpace(name)
	if name != "" {
		return name
	}
	return fallbackPrefix + strconv.Itoa(id)
}
