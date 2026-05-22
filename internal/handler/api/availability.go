package api

import (
	"context"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	"github.com/gin-gonic/gin"
)

const (
	availabilityDefaultRange      = 24 * time.Hour
	availabilityDefaultBucketMins = 15.0
	availabilityMinBucketMins     = 0.25
	availabilityMaxBucketMins     = 1440.0
	availabilityDefaultMaxBuckets = 100
	availabilityMaxBucketsHard    = 100
)

type availabilityProviderListStore interface {
	ListAll(ctx context.Context) ([]*model.Provider, error)
}

type availabilityRowStore interface {
	ListAvailabilityRows(ctx context.Context, startTime, endTime time.Time, providerIDs []int) ([]repository.AvailabilityRequestRow, error)
}

type AvailabilityHandler struct {
	auth      adminAuthenticator
	providers availabilityProviderListStore
	logs      availabilityRowStore
	now       func() time.Time
}

func NewAvailabilityHandler(auth adminAuthenticator, providers availabilityProviderListStore, logs availabilityRowStore) *AvailabilityHandler {
	return &AvailabilityHandler{
		auth:      auth,
		providers: providers,
		logs:      logs,
		now:       time.Now,
	}
}

func (h *AvailabilityHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/api/availability", h.getAvailability)
}

func (h *AvailabilityHandler) getAvailability(c *gin.Context) {
	if h == nil || h.auth == nil || h.providers == nil || h.logs == nil {
		writeAdminError(c, appErrors.NewInternalError("可用性服务未初始化"))
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

	options, err := decodeAvailabilityQuery(c, h.now())
	if err != nil {
		writeAdminError(c, err)
		return
	}

	providers, err := h.providers.ListAll(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	filteredProviders := filterAvailabilityProviders(providers, options.providerIDs, options.includeDisabled)
	providerIDs := make([]int, 0, len(filteredProviders))
	for _, provider := range filteredProviders {
		if provider != nil {
			providerIDs = append(providerIDs, provider.ID)
		}
	}

	rows, err := h.logs.ListAvailabilityRows(c.Request.Context(), options.startTime, options.endTime, providerIDs)
	if err != nil {
		writeAdminError(c, err)
		return
	}

	result := buildAvailabilityResponse(filteredProviders, rows, options)
	c.JSON(http.StatusOK, result)
}

type availabilityQueryOptions struct {
	startTime         time.Time
	endTime           time.Time
	providerIDs       []int
	bucketSizeMinutes float64
	includeDisabled   bool
	maxBuckets        int
}

func decodeAvailabilityQuery(c *gin.Context, now time.Time) (availabilityQueryOptions, error) {
	options := availabilityQueryOptions{
		startTime:         now.Add(-availabilityDefaultRange),
		endTime:           now,
		bucketSizeMinutes: availabilityDefaultBucketMins,
		maxBuckets:        availabilityDefaultMaxBuckets,
	}
	if raw := strings.TrimSpace(c.Query("startTime")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return options, appErrors.NewInvalidRequest("startTime 不是合法 ISO 时间")
		}
		options.startTime = parsed
	}
	if raw := strings.TrimSpace(c.Query("endTime")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return options, appErrors.NewInvalidRequest("endTime 不是合法 ISO 时间")
		}
		options.endTime = parsed
	}
	if !options.startTime.Before(options.endTime) {
		return options, appErrors.NewInvalidRequest("startTime 必须早于 endTime")
	}
	if raw := strings.TrimSpace(c.Query("providerIds")); raw != "" {
		parts := strings.Split(raw, ",")
		ids := make([]int, 0, len(parts))
		seen := map[int]struct{}{}
		for _, part := range parts {
			id, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil || id <= 0 {
				return options, appErrors.NewInvalidRequest("providerIds 必须是正整数列表")
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
		options.providerIDs = ids
	}
	if raw := strings.TrimSpace(c.Query("bucketSizeMinutes")); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || value < availabilityMinBucketMins || value > availabilityMaxBucketMins {
			return options, appErrors.NewInvalidRequest("bucketSizeMinutes 超出允许范围")
		}
		options.bucketSizeMinutes = value
	}
	if raw := strings.TrimSpace(c.Query("includeDisabled")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return options, appErrors.NewInvalidRequest("includeDisabled 必须是布尔值")
		}
		options.includeDisabled = value
	}
	if raw := strings.TrimSpace(c.Query("maxBuckets")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 || value > availabilityMaxBucketsHard {
			return options, appErrors.NewInvalidRequest("maxBuckets 超出允许范围")
		}
		options.maxBuckets = value
	}
	return options, nil
}

func filterAvailabilityProviders(providers []*model.Provider, providerIDs []int, includeDisabled bool) []*model.Provider {
	idSet := map[int]struct{}{}
	for _, providerID := range providerIDs {
		idSet[providerID] = struct{}{}
	}
	filtered := make([]*model.Provider, 0, len(providers))
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		if !includeDisabled && provider.IsEnabled != nil && !*provider.IsEnabled {
			continue
		}
		if len(idSet) > 0 {
			if _, ok := idSet[provider.ID]; !ok {
				continue
			}
		}
		filtered = append(filtered, provider)
	}
	return filtered
}

func buildAvailabilityResponse(providers []*model.Provider, rows []repository.AvailabilityRequestRow, options availabilityQueryOptions) gin.H {
	bucketSize := time.Duration(options.bucketSizeMinutes * float64(time.Minute))
	type bucketAggregate struct {
		start     time.Time
		total     int
		green     int
		red       int
		latencies []int
	}
	perProviderBuckets := map[int]map[time.Time]*bucketAggregate{}
	lastRequestAt := map[int]time.Time{}
	for _, row := range rows {
		if row.ProviderID <= 0 {
			continue
		}
		bucketStart := bucketFloor(row.CreatedAt.UTC(), bucketSize)
		providerBuckets := perProviderBuckets[row.ProviderID]
		if providerBuckets == nil {
			providerBuckets = map[time.Time]*bucketAggregate{}
			perProviderBuckets[row.ProviderID] = providerBuckets
		}
		aggregate := providerBuckets[bucketStart]
		if aggregate == nil {
			aggregate = &bucketAggregate{start: bucketStart}
			providerBuckets[bucketStart] = aggregate
		}
		aggregate.total++
		if row.StatusCode >= 200 && row.StatusCode < 400 {
			aggregate.green++
		} else {
			aggregate.red++
		}
		if row.DurationMs != nil && *row.DurationMs >= 0 {
			aggregate.latencies = append(aggregate.latencies, *row.DurationMs)
		}
		if latest, ok := lastRequestAt[row.ProviderID]; !ok || row.CreatedAt.After(latest) {
			lastRequestAt[row.ProviderID] = row.CreatedAt
		}
	}

	providerSummaries := make([]gin.H, 0, len(providers))
	totalRequestsAll := 0
	weightedAvailability := 0.0
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		buckets := perProviderBuckets[provider.ID]
		bucketList := make([]gin.H, 0, len(buckets))
		if len(buckets) > 0 {
			starts := make([]time.Time, 0, len(buckets))
			for start := range buckets {
				starts = append(starts, start)
			}
			sort.Slice(starts, func(i, j int) bool { return starts[i].Before(starts[j]) })
			if len(starts) > options.maxBuckets {
				starts = starts[len(starts)-options.maxBuckets:]
			}
			for _, start := range starts {
				aggregate := buckets[start]
				if aggregate == nil {
					continue
				}
				avgLatency, p50, p95, p99 := summarizeLatencies(aggregate.latencies)
				availabilityScore := 0.0
				if aggregate.total > 0 {
					availabilityScore = float64(aggregate.green) / float64(aggregate.total)
				}
				bucketList = append(bucketList, gin.H{
					"bucketStart":       start.Format(time.RFC3339Nano),
					"bucketEnd":         start.Add(bucketSize).Format(time.RFC3339Nano),
					"totalRequests":     aggregate.total,
					"greenCount":        aggregate.green,
					"redCount":          aggregate.red,
					"availabilityScore": availabilityScore,
					"avgLatencyMs":      avgLatency,
					"p50LatencyMs":      p50,
					"p95LatencyMs":      p95,
					"p99LatencyMs":      p99,
				})
			}
		}

		totalRequests := 0
		totalGreen := 0
		latencySamples := make([]int, 0)
		for _, bucket := range bucketList {
			totalRequests += bucket["totalRequests"].(int)
			totalGreen += bucket["greenCount"].(int)
			if avg, ok := bucket["avgLatencyMs"].(int); ok && avg > 0 {
				latencySamples = append(latencySamples, avg)
			}
		}
		currentAvailability := 0.0
		status := "unknown"
		if totalRequests > 0 {
			currentAvailability = float64(totalGreen) / float64(totalRequests)
			if currentAvailability >= 0.5 {
				status = "green"
			} else {
				status = "red"
			}
		}
		avgLatency := 0
		if len(latencySamples) > 0 {
			sum := 0
			for _, sample := range latencySamples {
				sum += sample
			}
			avgLatency = int(math.Round(float64(sum) / float64(len(latencySamples))))
		}
		var lastRequest any
		if ts, ok := lastRequestAt[provider.ID]; ok {
			lastRequest = ts.UTC().Format(time.RFC3339Nano)
		}
		isEnabled := provider.IsEnabled == nil || *provider.IsEnabled
		providerSummaries = append(providerSummaries, gin.H{
			"providerId":          provider.ID,
			"providerName":        provider.Name,
			"providerType":        provider.ProviderType,
			"isEnabled":           isEnabled,
			"currentStatus":       status,
			"currentAvailability": currentAvailability,
			"totalRequests":       totalRequests,
			"successRate":         currentAvailability,
			"avgLatencyMs":        avgLatency,
			"lastRequestAt":       lastRequest,
			"timeBuckets":         bucketList,
		})
		totalRequestsAll += totalRequests
		weightedAvailability += currentAvailability * float64(totalRequests)
	}
	systemAvailability := 0.0
	if totalRequestsAll > 0 {
		systemAvailability = weightedAvailability / float64(totalRequestsAll)
	}
	return gin.H{
		"queriedAt":          options.endTime.UTC().Format(time.RFC3339Nano),
		"startTime":          options.startTime.UTC().Format(time.RFC3339Nano),
		"endTime":            options.endTime.UTC().Format(time.RFC3339Nano),
		"bucketSizeMinutes":  options.bucketSizeMinutes,
		"providers":          providerSummaries,
		"systemAvailability": systemAvailability,
	}
}

func bucketFloor(t time.Time, size time.Duration) time.Time {
	if size <= 0 {
		return t
	}
	unixMs := t.UnixMilli()
	sizeMs := size.Milliseconds()
	return time.UnixMilli((unixMs / sizeMs) * sizeMs).UTC()
}

func summarizeLatencies(samples []int) (avg, p50, p95, p99 int) {
	if len(samples) == 0 {
		return 0, 0, 0, 0
	}
	sorted := append([]int(nil), samples...)
	sort.Ints(sorted)
	sum := 0
	for _, sample := range sorted {
		sum += sample
	}
	avg = int(math.Round(float64(sum) / float64(len(sorted))))
	p50 = percentile(sorted, 0.50)
	p95 = percentile(sorted, 0.95)
	p99 = percentile(sorted, 0.99)
	return avg, p50, p95, p99
}

func percentile(sorted []int, q float64) int {
	if len(sorted) == 0 {
		return 0
	}
	index := int(math.Ceil(q*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}
