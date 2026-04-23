package api

import (
	"context"
	"net/http"
	"time"

	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	"github.com/gin-gonic/gin"
)

type countStore interface {
	Count(ctx context.Context, includeDeleted bool) (int, error)
}

type OverviewActionHandler struct {
	auth      adminAuthenticator
	users     countStore
	keys      countStore
	providers countStore
	logs      usageLogsStore
}

func NewOverviewActionHandler(auth adminAuthenticator, users countStore, keys countStore, providers countStore, logs usageLogsStore) *OverviewActionHandler {
	return &OverviewActionHandler{auth: auth, users: users, keys: keys, providers: providers, logs: logs}
}

func (h *OverviewActionHandler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("/overview")
	protected.Use((&Handler{auth: h.auth}).AdminAuthMiddleware())
	protected.POST("/getOverviewData", h.getOverviewData)
}

func (h *OverviewActionHandler) getOverviewData(c *gin.Context) {
	if h == nil || h.users == nil || h.keys == nil || h.providers == nil || h.logs == nil {
		writeAdminError(c, appErrors.NewInternalError("概览仓储未初始化"))
		return
	}
	userCount, err := h.users.Count(c.Request.Context(), false)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	keyCount, err := h.keys.Count(c.Request.Context(), false)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	providerCount, err := h.providers.Count(c.Request.Context(), false)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	summary, err := h.logs.GetSummary(c.Request.Context(), repository.MessageRequestQueryFilters{})
	if err != nil {
		writeAdminError(c, err)
		return
	}
	location, locErr := time.LoadLocation(repository.DefaultTimezone)
	if locErr != nil {
		location = time.Local
	}
	metrics, err := h.logs.GetOverviewMetrics(c.Request.Context(), time.Now(), location)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{
		"totalUsers":                         userCount,
		"totalKeys":                          keyCount,
		"totalProviders":                     providerCount,
		"totalRequests":                      summary.TotalRequests,
		"totalCost":                          summary.TotalCost,
		"concurrentSessions":                 metrics.ConcurrentSessions,
		"todayRequests":                      metrics.TodayRequests,
		"todayCost":                          metrics.TodayCost,
		"avgResponseTime":                    metrics.AvgResponseTime,
		"todayErrorRate":                     metrics.TodayErrorRate,
		"yesterdaySamePeriodRequests":        metrics.YesterdaySamePeriodRequests,
		"yesterdaySamePeriodCost":            metrics.YesterdaySamePeriodCost,
		"yesterdaySamePeriodAvgResponseTime": metrics.YesterdaySamePeriodAvgResponseTime,
		"recentMinuteRequests":               metrics.RecentMinuteRequests,
	}})
}
