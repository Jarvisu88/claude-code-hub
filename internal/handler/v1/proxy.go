package v1

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	stderrors "errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	circuitbreakersvc "github.com/ding113/claude-code-hub/internal/service/circuitbreaker"
	livechainsvc "github.com/ding113/claude-code-hub/internal/service/livechain"
	providertrackersvc "github.com/ding113/claude-code-hub/internal/service/providertracker"
	sessionsvc "github.com/ding113/claude-code-hub/internal/service/session"
	"github.com/gin-gonic/gin"
	"github.com/quagmt/udecimal"
)

const authResultContextKey = "proxy_auth_result"
const proxySessionIDContextKey = "proxy_session_id"

type sessionManager interface {
	ExtractClientSessionID(requestBody map[string]any, headers http.Header) sessionsvc.ClientSessionExtractionResult
	GetOrCreateSessionID(ctx context.Context, keyID int, messages any, clientSessionID string) string
	GetNextRequestSequence(ctx context.Context, sessionID string) int
	BindProvider(ctx context.Context, sessionID string, providerID int)
	UpdateCodexSessionWithPromptCacheKey(ctx context.Context, currentSessionID, promptCacheKey string, providerID int) string
	GetConcurrentCount(ctx context.Context, sessionID string) int
	IncrementConcurrentCount(ctx context.Context, sessionID string)
	DecrementConcurrentCount(ctx context.Context, sessionID string)
}

type providerRepository interface {
	GetActiveProviders(ctx context.Context) ([]*model.Provider, error)
}

type providerVendorStore interface {
	GetByWebsiteDomain(ctx context.Context, domain string) (*model.ProviderVendor, error)
}

type providerEndpointStore interface {
	ListActiveByVendorAndType(ctx context.Context, vendorID int, providerType string) ([]*model.ProviderEndpoint, error)
}

type messageRequestRepository interface {
	Create(ctx context.Context, messageRequest *model.MessageRequest) (*model.MessageRequest, error)
	UpdateTerminal(ctx context.Context, id int, update repository.MessageRequestTerminalUpdate) error
}

type proxySystemSettingsStore interface {
	Get(ctx context.Context) (*model.SystemSettings, error)
}

type proxyStatisticsStore interface {
	SumUserTotalCost(ctx context.Context, userID int, maxAgeDays int) (udecimal.Decimal, error)
	SumKeyTotalCost(ctx context.Context, keyStr string, maxAgeDays int) (udecimal.Decimal, error)
	SumUserCostInTimeRange(ctx context.Context, userID int, startTime, endTime time.Time) (udecimal.Decimal, error)
	SumKeyCostInTimeRangeByKeyString(ctx context.Context, keyStr string, startTime, endTime time.Time) (udecimal.Decimal, error)
	SumProviderCostInTimeRange(ctx context.Context, providerID int, startTime, endTime time.Time) (udecimal.Decimal, error)
	SumProviderTotalCost(ctx context.Context, providerID int, resetAt *time.Time) (udecimal.Decimal, error)
}

type proxyRequestFilterStore interface {
	ListActive(ctx context.Context) ([]*model.RequestFilter, error)
}

type proxySensitiveWordStore interface {
	ListActive(ctx context.Context) ([]*model.SensitiveWord, error)
}

type proxyErrorRuleStore interface {
	ListActive(ctx context.Context) ([]*model.ErrorRule, error)
}

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Handler 承载 /v1 代理入口的最小可用接线。
// 当前阶段先把鉴权链接入 Gin，后续再逐步替换 501 占位逻辑。
type Handler struct {
	auth              *authsvc.Service
	sessions          sessionManager
	providers         providerRepository
	providerVendors   providerVendorStore
	providerEndpoints providerEndpointStore
	requestLogs       messageRequestRepository
	settings          proxySystemSettingsStore
	requestFilters    proxyRequestFilterStore
	sensitiveWords    proxySensitiveWordStore
	errorRules        proxyErrorRuleStore
	stats             proxyStatisticsStore
	http              httpDoer
}

type proxyEndpointKind string

const (
	proxyEndpointMessages        proxyEndpointKind = "messages"
	proxyEndpointMessagesCount   proxyEndpointKind = "messages_count_tokens"
	proxyEndpointResponses       proxyEndpointKind = "responses"
	proxyEndpointChatCompletions proxyEndpointKind = "chat_completions"
)

func NewHandler(auth *authsvc.Service, sessions sessionManager, providers providerRepository, requestLogs messageRequestRepository, httpClient httpDoer, options ...any) *Handler {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	var settingsStore proxySystemSettingsStore
	var providerVendorRepo providerVendorStore
	var providerEndpointRepo providerEndpointStore
	var requestFilterStore proxyRequestFilterStore
	var sensitiveWordStore proxySensitiveWordStore
	var errorRuleStore proxyErrorRuleStore
	var statsStore proxyStatisticsStore
	for _, option := range options {
		switch typed := option.(type) {
		case proxySystemSettingsStore:
			if settingsStore == nil {
				settingsStore = typed
			}
		case providerVendorStore:
			if providerVendorRepo == nil {
				providerVendorRepo = typed
			}
		case providerEndpointStore:
			if providerEndpointRepo == nil {
				providerEndpointRepo = typed
			}
		case proxyRequestFilterStore:
			if requestFilterStore == nil {
				requestFilterStore = typed
			}
		case proxySensitiveWordStore:
			if sensitiveWordStore == nil {
				sensitiveWordStore = typed
			}
		case proxyErrorRuleStore:
			if errorRuleStore == nil {
				errorRuleStore = typed
			}
		case proxyStatisticsStore:
			if statsStore == nil {
				statsStore = typed
			}
		}
	}
	return &Handler{
		auth:              auth,
		sessions:          sessions,
		providers:         providers,
		providerVendors:   providerVendorRepo,
		providerEndpoints: providerEndpointRepo,
		requestLogs:       requestLogs,
		settings:          settingsStore,
		requestFilters:    requestFilterStore,
		sensitiveWords:    sensitiveWordStore,
		errorRules:        errorRuleStore,
		stats:             statsStore,
		http:              httpClient,
	}
}

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("")
	protected.Use(h.AuthMiddleware())
	protected.Use(h.SessionLifecycleMiddleware())
	protected.POST("/messages", h.messages)
	protected.POST("/messages/count_tokens", h.messagesCountTokens)
	protected.POST("/chat/completions", h.chatCompletions)
	protected.POST("/responses", h.responses)
	protected.GET("/models", h.models(""))
	protected.GET("/responses/models", h.models(proxyEndpointResponses))
	protected.GET("/chat/completions/models", h.models(proxyEndpointChatCompletions))
	protected.GET("/chat/models", h.models(proxyEndpointChatCompletions))
}

func (h *Handler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.auth == nil {
			appErr := appErrors.NewInternalError("代理鉴权服务未初始化")
			c.AbortWithStatusJSON(appErr.HTTPStatus, appErr.ToResponse())
			return
		}

		result, err := h.auth.AuthenticateProxy(c.Request.Context(), authsvc.ProxyAuthInput{
			AuthorizationHeader: c.GetHeader("Authorization"),
			APIKeyHeader:        c.GetHeader("x-api-key"),
			GeminiAPIKeyHeader:  c.GetHeader("x-goog-api-key"),
			GeminiAPIKeyQuery:   c.Query("key"),
		})
		if err != nil {
			writeAppError(c, err)
			return
		}

		c.Set(authResultContextKey, result)
		c.Next()
	}
}

func GetAuthResult(c *gin.Context) (*authsvc.AuthResult, bool) {
	if c == nil {
		return nil, false
	}
	value, ok := c.Get(authResultContextKey)
	if !ok {
		return nil, false
	}
	result, ok := value.(*authsvc.AuthResult)
	return result, ok && result != nil
}

func GetProxySessionID(c *gin.Context) (string, bool) {
	if c == nil {
		return "", false
	}
	value, ok := c.Get(proxySessionIDContextKey)
	if !ok {
		return "", false
	}
	sessionID, ok := value.(string)
	return sessionID, ok && sessionID != ""
}

func (h *Handler) SessionLifecycleMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !shouldTrackConcurrentRequests(c) || h == nil || h.sessions == nil {
			c.Next()
			return
		}

		authResult, ok := GetAuthResult(c)
		if !ok || authResult == nil || authResult.Key == nil {
			c.Next()
			return
		}

		requestBody, ok := decodeRequestBody(c)
		if !ok {
			c.Next()
			return
		}

		extracted := h.sessions.ExtractClientSessionID(requestBody, c.Request.Header)
		sessionID := h.sessions.GetOrCreateSessionID(c.Request.Context(), authResult.Key.ID, extractRequestMessages(requestBody), extracted.SessionID)
		if sessionID == "" {
			c.Next()
			return
		}
		if exceeded, limit, current := exceedsTotalCostLimit(c.Request.Context(), h.stats, authResult); exceeded {
			writeAppError(c, (&appErrors.AppError{
				Type:       appErrors.ErrorTypeRateLimitError,
				Message:    "总费用额度已达上限：当前 " + current.String() + "，上限 " + limit.String(),
				Code:       appErrors.CodeTotalLimitExceeded,
				HTTPStatus: http.StatusPaymentRequired,
			}).WithDetails(map[string]any{
				"current": current.String(),
				"limit":   limit.String(),
			}))
			return
		}
		if exceeded, limit, current := exceedsConcurrentSessionLimit(authResult, h.sessions.GetConcurrentCount(c.Request.Context(), sessionID)); exceeded {
			writeAppError(c, (&appErrors.AppError{
				Type:       appErrors.ErrorTypeRateLimitError,
				Message:    "并发会话数超出限制：当前 " + strconv.Itoa(current) + "，上限 " + strconv.Itoa(limit),
				Code:       appErrors.CodeConcurrentSessionsExceeded,
				HTTPStatus: http.StatusTooManyRequests,
			}).WithDetails(map[string]any{
				"current": current,
				"limit":   limit,
			}))
			return
		}
		if exceeded, limit, current := exceeds5hCostLimit(c.Request.Context(), h.stats, authResult); exceeded {
			writeAppError(c, (&appErrors.AppError{
				Type:       appErrors.ErrorTypeRateLimitError,
				Message:    "5小时费用额度已达上限：当前 " + current.String() + "，上限 " + limit.String(),
				Code:       appErrors.Code5HLimitExceeded,
				HTTPStatus: http.StatusPaymentRequired,
			}).WithDetails(map[string]any{
				"current": current.String(),
				"limit":   limit.String(),
			}))
			return
		}
		if exceeded, limit, current := exceedsDailyCostLimit(c.Request.Context(), h.stats, h.settings, authResult); exceeded {
			writeAppError(c, (&appErrors.AppError{
				Type:       appErrors.ErrorTypeRateLimitError,
				Message:    "每日费用额度已达上限：当前 " + current.String() + "，上限 " + limit.String(),
				Code:       appErrors.CodeDailyLimitExceeded,
				HTTPStatus: http.StatusPaymentRequired,
			}).WithDetails(map[string]any{
				"current": current.String(),
				"limit":   limit.String(),
			}))
			return
		}
		if exceeded, limit, current := exceedsWeeklyCostLimit(c.Request.Context(), h.stats, h.settings, authResult); exceeded {
			writeAppError(c, (&appErrors.AppError{
				Type:       appErrors.ErrorTypeRateLimitError,
				Message:    "每周费用额度已达上限：当前 " + current.String() + "，上限 " + limit.String(),
				Code:       appErrors.CodeWeeklyLimitExceeded,
				HTTPStatus: http.StatusPaymentRequired,
			}).WithDetails(map[string]any{
				"current": current.String(),
				"limit":   limit.String(),
			}))
			return
		}
		if exceeded, limit, current := exceedsMonthlyCostLimit(c.Request.Context(), h.stats, h.settings, authResult); exceeded {
			writeAppError(c, (&appErrors.AppError{
				Type:       appErrors.ErrorTypeRateLimitError,
				Message:    "每月费用额度已达上限：当前 " + current.String() + "，上限 " + limit.String(),
				Code:       appErrors.CodeMonthlyLimitExceeded,
				HTTPStatus: http.StatusPaymentRequired,
			}).WithDetails(map[string]any{
				"current": current.String(),
				"limit":   limit.String(),
			}))
			return
		}

		c.Set(proxySessionIDContextKey, sessionID)
		h.sessions.IncrementConcurrentCount(c.Request.Context(), sessionID)
		defer h.sessions.DecrementConcurrentCount(c.Request.Context(), sessionID)

		c.Next()
	}
}

func exceedsConcurrentSessionLimit(authResult *authsvc.AuthResult, currentCount int) (bool, int, int) {
	if authResult == nil {
		return false, 0, currentCount
	}
	limit := 0
	if authResult.Key != nil {
		limit = normalizeConcurrentSessionLimit(authResult.Key.LimitConcurrentSessions)
	}
	if limit <= 0 && authResult.User != nil {
		limit = normalizeConcurrentSessionLimit(authResult.User.LimitConcurrentSessions)
	}
	if limit <= 0 {
		return false, 0, currentCount
	}
	return currentCount >= limit, limit, currentCount
}

func normalizeConcurrentSessionLimit(value *int) int {
	if value == nil || *value <= 0 {
		return 0
	}
	return *value
}

func exceedsTotalCostLimit(ctx context.Context, stats proxyStatisticsStore, authResult *authsvc.AuthResult) (bool, udecimal.Decimal, udecimal.Decimal) {
	if stats == nil || authResult == nil || authResult.Key == nil || authResult.User == nil {
		return false, udecimal.Zero, udecimal.Zero
	}
	if authResult.Key.LimitTotalUSD != nil && authResult.Key.LimitTotalUSD.GreaterThan(udecimal.Zero) {
		current, err := stats.SumKeyTotalCost(ctx, authResult.Key.Key, 365)
		if err == nil && (current.GreaterThan(*authResult.Key.LimitTotalUSD) || current.Equal(*authResult.Key.LimitTotalUSD)) {
			return true, *authResult.Key.LimitTotalUSD, current
		}
		return false, udecimal.Zero, udecimal.Zero
	}
	if authResult.User.LimitTotalUSD != nil && authResult.User.LimitTotalUSD.GreaterThan(udecimal.Zero) {
		current, err := stats.SumUserTotalCost(ctx, authResult.User.ID, 365)
		if err == nil && (current.GreaterThan(*authResult.User.LimitTotalUSD) || current.Equal(*authResult.User.LimitTotalUSD)) {
			return true, *authResult.User.LimitTotalUSD, current
		}
	}
	return false, udecimal.Zero, udecimal.Zero
}

func exceeds5hCostLimit(ctx context.Context, stats proxyStatisticsStore, authResult *authsvc.AuthResult) (bool, udecimal.Decimal, udecimal.Decimal) {
	if stats == nil || authResult == nil || authResult.Key == nil || authResult.User == nil {
		return false, udecimal.Zero, udecimal.Zero
	}
	endTime := time.Now()
	startTime := endTime.Add(-5 * time.Hour)
	if authResult.Key.Limit5hUSD != nil && authResult.Key.Limit5hUSD.GreaterThan(udecimal.Zero) {
		current, err := stats.SumKeyCostInTimeRangeByKeyString(ctx, authResult.Key.Key, startTime, endTime)
		if err == nil && (current.GreaterThan(*authResult.Key.Limit5hUSD) || current.Equal(*authResult.Key.Limit5hUSD)) {
			return true, *authResult.Key.Limit5hUSD, current
		}
		return false, udecimal.Zero, udecimal.Zero
	}
	if authResult.User.Limit5hUSD != nil && authResult.User.Limit5hUSD.GreaterThan(udecimal.Zero) {
		current, err := stats.SumUserCostInTimeRange(ctx, authResult.User.ID, startTime, endTime)
		if err == nil && (current.GreaterThan(*authResult.User.Limit5hUSD) || current.Equal(*authResult.User.Limit5hUSD)) {
			return true, *authResult.User.Limit5hUSD, current
		}
	}
	return false, udecimal.Zero, udecimal.Zero
}

func exceedsDailyCostLimit(ctx context.Context, stats proxyStatisticsStore, settings proxySystemSettingsStore, authResult *authsvc.AuthResult) (bool, udecimal.Decimal, udecimal.Decimal) {
	if stats == nil || authResult == nil || authResult.Key == nil || authResult.User == nil {
		return false, udecimal.Zero, udecimal.Zero
	}
	startTime, endTime := dailyWindowBounds(settings, authResult)
	if authResult.Key.LimitDailyUSD != nil && authResult.Key.LimitDailyUSD.GreaterThan(udecimal.Zero) {
		current, err := lookupDailyCost(ctx, stats, true, authResult.Key.Key, authResult.User.ID, startTime, endTime)
		if err == nil && (current.GreaterThan(*authResult.Key.LimitDailyUSD) || current.Equal(*authResult.Key.LimitDailyUSD)) {
			return true, *authResult.Key.LimitDailyUSD, current
		}
		return false, udecimal.Zero, udecimal.Zero
	}
	if authResult.User.DailyLimitUSD != nil && authResult.User.DailyLimitUSD.GreaterThan(udecimal.Zero) {
		current, err := lookupDailyCost(ctx, stats, false, authResult.Key.Key, authResult.User.ID, startTime, endTime)
		if err == nil && (current.GreaterThan(*authResult.User.DailyLimitUSD) || current.Equal(*authResult.User.DailyLimitUSD)) {
			return true, *authResult.User.DailyLimitUSD, current
		}
	}
	return false, udecimal.Zero, udecimal.Zero
}

func exceedsWeeklyCostLimit(ctx context.Context, stats proxyStatisticsStore, settings proxySystemSettingsStore, authResult *authsvc.AuthResult) (bool, udecimal.Decimal, udecimal.Decimal) {
	if stats == nil || authResult == nil || authResult.Key == nil || authResult.User == nil {
		return false, udecimal.Zero, udecimal.Zero
	}
	startTime, endTime := weeklyWindowBounds(settings)
	if authResult.Key.LimitWeeklyUSD != nil && authResult.Key.LimitWeeklyUSD.GreaterThan(udecimal.Zero) {
		current, err := lookupDailyCost(ctx, stats, true, authResult.Key.Key, authResult.User.ID, startTime, endTime)
		if err == nil && (current.GreaterThan(*authResult.Key.LimitWeeklyUSD) || current.Equal(*authResult.Key.LimitWeeklyUSD)) {
			return true, *authResult.Key.LimitWeeklyUSD, current
		}
		return false, udecimal.Zero, udecimal.Zero
	}
	if authResult.User.LimitWeeklyUSD != nil && authResult.User.LimitWeeklyUSD.GreaterThan(udecimal.Zero) {
		current, err := lookupDailyCost(ctx, stats, false, authResult.Key.Key, authResult.User.ID, startTime, endTime)
		if err == nil && (current.GreaterThan(*authResult.User.LimitWeeklyUSD) || current.Equal(*authResult.User.LimitWeeklyUSD)) {
			return true, *authResult.User.LimitWeeklyUSD, current
		}
	}
	return false, udecimal.Zero, udecimal.Zero
}

func exceedsMonthlyCostLimit(ctx context.Context, stats proxyStatisticsStore, settings proxySystemSettingsStore, authResult *authsvc.AuthResult) (bool, udecimal.Decimal, udecimal.Decimal) {
	if stats == nil || authResult == nil || authResult.Key == nil || authResult.User == nil {
		return false, udecimal.Zero, udecimal.Zero
	}
	startTime, endTime := monthlyWindowBounds(settings)
	if authResult.Key.LimitMonthlyUSD != nil && authResult.Key.LimitMonthlyUSD.GreaterThan(udecimal.Zero) {
		current, err := lookupDailyCost(ctx, stats, true, authResult.Key.Key, authResult.User.ID, startTime, endTime)
		if err == nil && (current.GreaterThan(*authResult.Key.LimitMonthlyUSD) || current.Equal(*authResult.Key.LimitMonthlyUSD)) {
			return true, *authResult.Key.LimitMonthlyUSD, current
		}
		return false, udecimal.Zero, udecimal.Zero
	}
	if authResult.User.LimitMonthlyUSD != nil && authResult.User.LimitMonthlyUSD.GreaterThan(udecimal.Zero) {
		current, err := lookupDailyCost(ctx, stats, false, authResult.Key.Key, authResult.User.ID, startTime, endTime)
		if err == nil && (current.GreaterThan(*authResult.User.LimitMonthlyUSD) || current.Equal(*authResult.User.LimitMonthlyUSD)) {
			return true, *authResult.User.LimitMonthlyUSD, current
		}
	}
	return false, udecimal.Zero, udecimal.Zero
}

func lookupDailyCost(ctx context.Context, stats proxyStatisticsStore, keyScoped bool, keyString string, userID int, startTime, endTime time.Time) (udecimal.Decimal, error) {
	if keyScoped {
		return stats.SumKeyCostInTimeRangeByKeyString(ctx, keyString, startTime, endTime)
	}
	return stats.SumUserCostInTimeRange(ctx, userID, startTime, endTime)
}

func dailyWindowBounds(settings proxySystemSettingsStore, authResult *authsvc.AuthResult) (time.Time, time.Time) {
	mode := "fixed"
	resetTime := "00:00"
	if authResult != nil && authResult.Key != nil && authResult.Key.LimitDailyUSD != nil && authResult.Key.LimitDailyUSD.GreaterThan(udecimal.Zero) {
		mode = authResult.Key.DailyResetMode
		resetTime = authResult.Key.DailyResetTime
	} else if authResult != nil && authResult.User != nil {
		mode = authResult.User.DailyResetMode
		resetTime = authResult.User.DailyResetTime
	}
	return dailyWindowBoundsForMode(settings, mode, resetTime)
}

func dailyWindowBoundsForMode(settings proxySystemSettingsStore, mode string, resetTime string) (time.Time, time.Time) {
	now := time.Now()
	if strings.EqualFold(strings.TrimSpace(mode), string(model.DailyResetModeRolling)) {
		return now.Add(-24 * time.Hour), now
	}

	timezone := repository.DefaultTimezone
	if settings != nil {
		if systemSettings, err := settings.Get(context.Background()); err == nil && systemSettings != nil && systemSettings.Timezone != nil {
			timezone = repository.ValidateTimezone(strings.TrimSpace(*systemSettings.Timezone))
		}
	}
	location, _ := time.LoadLocation(repository.ValidateTimezone(timezone))
	zonedNow := now.In(location)
	hour, minute := parseDailyResetTime(resetTime)
	startOfWindow := time.Date(zonedNow.Year(), zonedNow.Month(), zonedNow.Day(), hour, minute, 0, 0, location)
	if zonedNow.Before(startOfWindow) {
		startOfWindow = startOfWindow.Add(-24 * time.Hour)
	}
	return startOfWindow.UTC(), now
}

func resolveSettingsTimezone(settings proxySystemSettingsStore) string {
	timezone := repository.DefaultTimezone
	if settings != nil {
		if systemSettings, err := settings.Get(context.Background()); err == nil && systemSettings != nil && systemSettings.Timezone != nil {
			timezone = repository.ValidateTimezone(strings.TrimSpace(*systemSettings.Timezone))
		}
	}
	return repository.ValidateTimezone(timezone)
}

func weeklyWindowBounds(settings proxySystemSettingsStore) (time.Time, time.Time) {
	now := time.Now()
	location, _ := time.LoadLocation(resolveSettingsTimezone(settings))
	zonedNow := now.In(location)
	delta := (int(zonedNow.Weekday()) + 6) % 7
	start := time.Date(zonedNow.Year(), zonedNow.Month(), zonedNow.Day(), 0, 0, 0, 0, location).AddDate(0, 0, -delta)
	return start.UTC(), now
}

func monthlyWindowBounds(settings proxySystemSettingsStore) (time.Time, time.Time) {
	now := time.Now()
	location, _ := time.LoadLocation(resolveSettingsTimezone(settings))
	zonedNow := now.In(location)
	start := time.Date(zonedNow.Year(), zonedNow.Month(), 1, 0, 0, 0, 0, location)
	return start.UTC(), now
}

func parseDailyResetTime(value string) (int, int) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, 0
	}
	hour, errHour := strconv.Atoi(parts[0])
	minute, errMinute := strconv.Atoi(parts[1])
	if errHour != nil || errMinute != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0
	}
	return hour, minute
}

func (h *Handler) notImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": gin.H{
			"type":    "not_implemented",
			"message": "This endpoint is not yet implemented",
		},
	})
}

func (h *Handler) messages(c *gin.Context) {
	h.proxyEndpoint(c, proxyEndpointMessages)
}

func (h *Handler) messagesCountTokens(c *gin.Context) {
	h.proxyEndpoint(c, proxyEndpointMessagesCount)
}

func (h *Handler) models(endpointKind proxyEndpointKind) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.providers == nil {
			h.notImplemented(c)
			return
		}

		providers, err := h.providers.GetActiveProviders(c.Request.Context())
		if err != nil {
			writeAppError(c, err)
			return
		}

		seen := map[string]struct{}{}
		models := make([]gin.H, 0)
		for _, provider := range providers {
			if provider == nil || !provider.IsActive() {
				continue
			}
			if endpointKind != "" && !supportsEndpointKind(provider.ProviderType, endpointKind) {
				continue
			}
			for _, modelName := range provider.AllowedModels.ExactModelNames() {
				modelName = strings.TrimSpace(modelName)
				if modelName == "" {
					continue
				}
				if _, ok := seen[modelName]; ok {
					continue
				}
				seen[modelName] = struct{}{}
				models = append(models, gin.H{
					"id":     modelName,
					"object": "model",
				})
			}
		}

		sort.Slice(models, func(i, j int) bool {
			return models[i]["id"].(string) < models[j]["id"].(string)
		})

		c.JSON(http.StatusOK, gin.H{"data": models, "object": "list"})
	}
}

func (h *Handler) responses(c *gin.Context) {
	h.proxyEndpoint(c, proxyEndpointResponses)
}

func (h *Handler) chatCompletions(c *gin.Context) {
	h.proxyEndpoint(c, proxyEndpointChatCompletions)
}

func (h *Handler) proxyEndpoint(c *gin.Context, endpointKind proxyEndpointKind) {
	if h == nil || h.providers == nil || h.http == nil {
		writeAppError(c, appErrors.NewInternalError("代理转发服务未初始化"))
		return
	}

	authResult, ok := GetAuthResult(c)
	if !ok || authResult == nil || authResult.Key == nil {
		writeAppError(c, appErrors.NewInternalError("代理鉴权上下文缺失"))
		return
	}

	requestBody, requestBodyBytes, ok := decodeRequestBodyBytes(c)
	if !ok {
		writeNormalizedProxyErrorResponse(c, http.StatusBadRequest, "请求体不是合法 JSON", "invalid_request_error", "")
		return
	}
	var err error
	if h.maybeRectifyRequestBody(requestBody, endpointKind) {
		requestBodyBytes, err = json.Marshal(requestBody)
		if err != nil {
			writeAppError(c, appErrors.NewInternalError("规范化请求体失败").WithError(err))
			return
		}
	}
	globalChanged, err := h.applyGlobalRequestFilters(c, requestBody)
	if err != nil {
		if appErr := appErrorAs(err); appErr != nil && appErr.Type == appErrors.ErrorTypeInvalidRequest {
			writeNormalizedProxyErrorResponse(c, appErr.HTTPStatus, appErr.Message, "invalid_request_error", "")
			return
		}
		writeAppError(c, err)
		return
	}
	if err := h.ensureNoSensitiveWords(requestBody); err != nil {
		if appErr := appErrorAs(err); appErr != nil && appErr.Type == appErrors.ErrorTypeInvalidRequest {
			writeNormalizedProxyErrorResponse(c, appErr.HTTPStatus, appErr.Message, "invalid_request_error", "")
			return
		}
		writeAppError(c, err)
		return
	}
	if h.shouldInterceptWarmup(c.Request.Context(), endpointKind, requestBody) {
		h.logWarmupIntercept(c, authResult, requestBody)
		warmupBody, err := json.Marshal(buildWarmupResponseBody(requestStringValue(requestBody["model"])))
		if err != nil {
			writeAppError(c, appErrors.NewInternalError("构建 Warmup 响应失败").WithError(err))
			return
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("x-cch-intercepted", "warmup")
		c.Header("x-cch-intercepted-by", "claude-code-hub")
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write(warmupBody)
		return
	}

	originalModel := requestStringValue(requestBody["model"])
	candidates, err := h.selectProvidersForEndpoint(c.Request.Context(), endpointKind, originalModel)
	if err != nil {
		if appErr := appErrorAs(err); appErr != nil && appErr.Code == appErrors.CodeNoProviderAvailable {
			writeNormalizedProxyErrorResponse(c, appErr.HTTPStatus, appErr.Message, "no_available_providers", "")
			return
		}
		writeAppError(c, err)
		return
	}
	provider := candidates[0]
	baseProviderChain := buildInitialProviderChain(provider)
	effectiveModel := originalModel
	if redirectedModel := provider.GetRedirectedModel(originalModel); redirectedModel != "" && redirectedModel != originalModel {
		requestBody["model"] = redirectedModel
		requestBodyBytes, err = json.Marshal(requestBody)
		if err != nil {
			errorMessage := "重写上游模型请求失败"
			h.finalizeMessageRequest(c, 0, repository.MessageRequestTerminalUpdate{
				StatusCode:   http.StatusInternalServerError,
				DurationMs:   0,
				ErrorMessage: &errorMessage,
			})
			writeAppError(c, appErrors.NewInternalError("重写上游模型请求失败").WithError(err))
			return
		}
		effectiveModel = redirectedModel
	}
	providerChanged, err := h.applyProviderRequestFilters(c, requestBody, provider)
	if err != nil {
		writeAppError(c, err)
		return
	}
	if globalChanged || providerChanged {
		requestBodyBytes, err = json.Marshal(requestBody)
		if err != nil {
			writeAppError(c, appErrors.NewInternalError("构建过滤后的请求体失败").WithError(err))
			return
		}
	}

	sessionID, _ := GetProxySessionID(c)
	requestSequence := 1
	if sessionID != "" && h.sessions != nil {
		requestSequence = h.sessions.GetNextRequestSequence(c.Request.Context(), sessionID)
		h.sessions.BindProvider(c.Request.Context(), sessionID, provider.ID)
	}
	if sessionID != "" {
		_ = livechainsvc.Write(c.Request.Context(), sessionID, requestSequence, baseProviderChain)
		defer livechainsvc.Delete(context.Background(), sessionID, requestSequence)
	}
	startedAt := time.Now()
	messageRequestID := h.createMessageRequest(c, authResult, provider, baseProviderChain, requestBody, originalModel, effectiveModel, sessionID, requestSequence)

	var upstreamResp *http.Response
	var lastTransportErr error
	providerChain := append([]model.ProviderChainItem(nil), baseProviderChain...)
	for providerIndex, candidate := range candidates {
		provider = candidate
		if providerIndex > 0 {
			providerChain = append(providerChain, model.ProviderChainItem{
				ID:              provider.ID,
				Name:            provider.Name,
				ProviderType:    stringPointer(provider.ProviderType),
				EndpointURL:     stringPointer(provider.URL),
				Reason:          stringPointer("system_error"),
				SelectionMethod: stringPointer("fallback"),
				Priority:        provider.Priority,
				Weight:          provider.Weight,
				CostMultiplier:  providerCostMultiplier(provider),
				Timestamp:       int64Pointer(time.Now().UnixMilli()),
			})
			if sessionID != "" && h.sessions != nil {
				h.sessions.BindProvider(c.Request.Context(), sessionID, provider.ID)
			}
		}

		upstreamURL, err := buildProxyURL(provider.URL, c.Request.URL)
		if err != nil {
			errorMessage := "构建上游代理地址失败"
			h.finalizeMessageRequest(c, messageRequestID, repository.MessageRequestTerminalUpdate{
				StatusCode:    http.StatusInternalServerError,
				DurationMs:    int(time.Since(startedAt).Milliseconds()),
				ErrorMessage:  &errorMessage,
				ProviderChain: finalizeProviderChain(providerChain, http.StatusInternalServerError, &errorMessage),
			})
			writeAppError(c, appErrors.NewInternalError("构建上游代理地址失败").WithError(err))
			return
		}

		maxAttempts := 1
		if provider.MaxRetryAttempts != nil && *provider.MaxRetryAttempts > 1 {
			maxAttempts = *provider.MaxRetryAttempts
		}
		settingsSnapshot := h.currentProxySystemSettings()
		signatureRetried := false
		budgetRetried := false
		upstreamResp = nil
		lastTransportErr = nil
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			upstreamReq, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, upstreamURL, bytes.NewReader(requestBodyBytes))
			if err != nil {
				errorMessage := "构建上游请求失败"
				h.finalizeMessageRequest(c, messageRequestID, repository.MessageRequestTerminalUpdate{
					StatusCode:    http.StatusInternalServerError,
					DurationMs:    int(time.Since(startedAt).Milliseconds()),
					ErrorMessage:  &errorMessage,
					ProviderChain: finalizeProviderChain(providerChain, http.StatusInternalServerError, &errorMessage),
				})
				writeAppError(c, appErrors.NewInternalError("构建上游请求失败").WithError(err))
				return
			}

			copyProxyRequestHeaders(upstreamReq.Header, c.Request.Header)
			applyProviderAuthHeaders(upstreamReq.Header, provider, endpointKind)

			upstreamResp, err = h.http.Do(upstreamReq)
			if err == nil {
				if upstreamResp != nil && upstreamResp.StatusCode == http.StatusBadRequest &&
					endpointKind == proxyEndpointMessages &&
					(provider.ProviderType == string(model.ProviderTypeClaude) || provider.ProviderType == string(model.ProviderTypeClaudeAuth)) {
					responseBody, readErr := io.ReadAll(upstreamResp.Body)
					if readErr != nil {
						upstreamResp.Body.Close()
						lastTransportErr = readErr
						break
					}
					errorMessage := extractUpstreamErrorMessage(responseBody)
					retried := false
					if !signatureRetried && thinkingSignatureRectifierEnabled(settingsSnapshot) && detectThinkingSignatureRectifierTrigger(errorMessage) != "" {
						rectified := rectifyAnthropicRequestMessage(requestBody)
						if rectified.Applied {
							signatureRetried = true
							retried = true
						}
					}
					if !retried && !budgetRetried && thinkingBudgetRectifierEnabled(settingsSnapshot) && detectThinkingBudgetRectifierTrigger(errorMessage) {
						rectified := rectifyThinkingBudget(requestBody)
						if rectified.Applied {
							budgetRetried = true
							retried = true
						}
					}
					if retried {
						upstreamResp.Body.Close()
						requestBodyBytes, err = json.Marshal(requestBody)
						if err != nil {
							writeAppError(c, appErrors.NewInternalError("重试前重建请求体失败").WithError(err))
							return
						}
						if attempt == maxAttempts {
							maxAttempts++
						}
						providerChain = append(providerChain, model.ProviderChainItem{
							ID:              provider.ID,
							Name:            provider.Name,
							ProviderType:    stringPointer(provider.ProviderType),
							EndpointURL:     stringPointer(provider.URL),
							Reason:          stringPointer("rectifier_retry"),
							SelectionMethod: stringPointer("same_provider_retry"),
							Priority:        provider.Priority,
							Weight:          provider.Weight,
							CostMultiplier:  providerCostMultiplier(provider),
							Timestamp:       int64Pointer(time.Now().UnixMilli()),
							ErrorMessage:    stringPointer(errorMessage),
						})
						continue
					}
					upstreamResp.Body = io.NopCloser(bytes.NewReader(responseBody))
				}
				if upstreamResp != nil && providerIndex < len(candidates)-1 && !isStreamingResponse(upstreamResp.Header) {
					responseBody, readErr := io.ReadAll(upstreamResp.Body)
					if readErr != nil {
						upstreamResp.Body.Close()
						lastTransportErr = readErr
						break
					}
					upstreamResp.Body.Close()
					decision, decisionErr := h.evaluateUpstreamErrorDecision(c.Request.Context(), upstreamResp.StatusCode, responseBody)
					if decisionErr != nil {
						errorMessage := "应用错误规则失败"
						h.finalizeMessageRequest(c, messageRequestID, repository.MessageRequestTerminalUpdate{
							StatusCode:    http.StatusInternalServerError,
							DurationMs:    int(time.Since(startedAt).Milliseconds()),
							ErrorMessage:  &errorMessage,
							ProviderChain: finalizeProviderChain(providerChain, http.StatusInternalServerError, &errorMessage),
						})
						writeAppError(c, decisionErr)
						return
					}
					if decision.FallbackReason != "" && decision.StatusCode >= http.StatusBadRequest {
						if shouldCountFailureForCircuit(decision.StatusCode) {
							circuitbreakersvc.RecordFailure(provider, false)
						}
						providerChain = append(providerChain, model.ProviderChainItem{
							ID:              provider.ID,
							Name:            provider.Name,
							ProviderType:    stringPointer(provider.ProviderType),
							EndpointURL:     stringPointer(provider.URL),
							Reason:          stringPointer(decision.FallbackReason),
							SelectionMethod: stringPointer("fallback"),
							Priority:        provider.Priority,
							Weight:          provider.Weight,
							CostMultiplier:  providerCostMultiplier(provider),
							Timestamp:       int64Pointer(time.Now().UnixMilli()),
							StatusCode:      intPointer(decision.StatusCode),
							ErrorMessage:    stringPointer(decision.ErrorMessage),
						})
						upstreamResp = nil
						continue
					}
					upstreamResp = &http.Response{
						Status:        upstreamResp.Status,
						StatusCode:    upstreamResp.StatusCode,
						Proto:         upstreamResp.Proto,
						ProtoMajor:    upstreamResp.ProtoMajor,
						ProtoMinor:    upstreamResp.ProtoMinor,
						Header:        upstreamResp.Header.Clone(),
						Body:          io.NopCloser(bytes.NewReader(responseBody)),
						ContentLength: int64(len(responseBody)),
						Request:       upstreamResp.Request,
						TLS:           upstreamResp.TLS,
					}
				}
				if attempt > 1 {
					providerChain = append(providerChain, model.ProviderChainItem{
						ID:              provider.ID,
						Name:            provider.Name,
						ProviderType:    stringPointer(provider.ProviderType),
						EndpointURL:     stringPointer(provider.URL),
						Reason:          stringPointer("retry_success"),
						SelectionMethod: stringPointer("same_provider_retry"),
						Priority:        provider.Priority,
						Weight:          provider.Weight,
						CostMultiplier:  providerCostMultiplier(provider),
						Timestamp:       int64Pointer(time.Now().UnixMilli()),
					})
				}
				break
			}
			lastTransportErr = err
			circuitbreakersvc.RecordFailure(provider, true)
			if attempt < maxAttempts {
				providerChain = append(providerChain, model.ProviderChainItem{
					ID:              provider.ID,
					Name:            provider.Name,
					ProviderType:    stringPointer(provider.ProviderType),
					EndpointURL:     stringPointer(provider.URL),
					Reason:          stringPointer("retry_failed"),
					SelectionMethod: stringPointer("same_provider_retry"),
					Priority:        provider.Priority,
					Weight:          provider.Weight,
					CostMultiplier:  providerCostMultiplier(provider),
					Timestamp:       int64Pointer(time.Now().UnixMilli()),
					ErrorMessage:    stringPointer(err.Error()),
				})
			}
		}
		if lastTransportErr == nil && upstreamResp != nil && upstreamResp.StatusCode == http.StatusNotFound && providerIndex < len(candidates)-1 {
			responseBody, err := io.ReadAll(upstreamResp.Body)
			if err != nil {
				errorMessage := "读取上游响应失败"
				h.finalizeMessageRequest(c, messageRequestID, repository.MessageRequestTerminalUpdate{
					StatusCode:    http.StatusBadGateway,
					DurationMs:    int(time.Since(startedAt).Milliseconds()),
					ErrorMessage:  &errorMessage,
					ProviderChain: finalizeProviderChain(providerChain, http.StatusBadGateway, &errorMessage),
				})
				writeAppError(c, appErrors.NewProviderError(errorMessage, appErrors.CodeProviderError).WithError(err))
				return
			}
			providerChain = append(providerChain, model.ProviderChainItem{
				ID:              provider.ID,
				Name:            provider.Name,
				ProviderType:    stringPointer(provider.ProviderType),
				EndpointURL:     stringPointer(provider.URL),
				Reason:          stringPointer("resource_not_found"),
				SelectionMethod: stringPointer("fallback"),
				Priority:        provider.Priority,
				Weight:          provider.Weight,
				CostMultiplier:  providerCostMultiplier(provider),
				Timestamp:       int64Pointer(time.Now().UnixMilli()),
				StatusCode:      intPointer(http.StatusNotFound),
				ErrorMessage:    stringPointer(extractErrorMessageOrBody(responseBody)),
			})
			upstreamResp.Body.Close()
			upstreamResp = nil
			continue
		}
		if upstreamResp != nil {
			break
		}
	}
	if lastTransportErr != nil && upstreamResp == nil {
		errMsg := "上游 Responses 供应商请求失败"
		if endpointKind == proxyEndpointMessages {
			errMsg = "上游 Messages 供应商请求失败"
		}
		if endpointKind == proxyEndpointMessagesCount {
			errMsg = "上游 Count Tokens 供应商请求失败"
		}
		if endpointKind == proxyEndpointChatCompletions {
			errMsg = "上游 Chat Completions 供应商请求失败"
		}
		if isTimeoutError(lastTransportErr) {
			timeoutMessage := errMsg + "：请求超时"
			h.finalizeMessageRequest(c, messageRequestID, repository.MessageRequestTerminalUpdate{
				StatusCode:    http.StatusGatewayTimeout,
				DurationMs:    int(time.Since(startedAt).Milliseconds()),
				ErrorMessage:  &timeoutMessage,
				ProviderChain: finalizeProviderChain(providerChain, http.StatusGatewayTimeout, &timeoutMessage),
			})
			writeNormalizedProxyErrorResponse(c, http.StatusGatewayTimeout, timeoutMessage, "", "")
			return
		}
		h.finalizeMessageRequest(c, messageRequestID, repository.MessageRequestTerminalUpdate{
			StatusCode:    http.StatusBadGateway,
			DurationMs:    int(time.Since(startedAt).Milliseconds()),
			ErrorMessage:  &errMsg,
			ProviderChain: finalizeProviderChain(providerChain, http.StatusBadGateway, &errMsg),
		})
		writeNormalizedProxyErrorResponse(c, http.StatusBadGateway, errMsg, "", "")
		return
	}
	defer upstreamResp.Body.Close()

	copyProxyResponseHeaders(c.Writer.Header(), upstreamResp.Header)
	if isStreamingResponse(upstreamResp.Header) {
		var streamMirror bytes.Buffer
		reader := io.TeeReader(upstreamResp.Body, &streamMirror)
		c.Status(upstreamResp.StatusCode)
		if _, err := io.Copy(c.Writer, reader); err != nil {
			c.Error(err)
		}
		if endpointKind == proxyEndpointResponses && sessionID != "" && h.sessions != nil {
			if promptCacheKey := extractCodexPromptCacheKeyFromSSE(streamMirror.Bytes()); promptCacheKey != "" {
				h.sessions.UpdateCodexSessionWithPromptCacheKey(c.Request.Context(), sessionID, promptCacheKey, provider.ID)
			}
		}
		streamDecision, detectedStreamError, decisionErr := h.evaluateStreamingUpstreamDecision(c.Request.Context(), upstreamResp.StatusCode, streamMirror.Bytes())
		if decisionErr != nil {
			errorMessage := "应用流式错误规则失败"
			h.finalizeMessageRequest(c, messageRequestID, repository.MessageRequestTerminalUpdate{
				StatusCode:    http.StatusInternalServerError,
				DurationMs:    int(time.Since(startedAt).Milliseconds()),
				ErrorMessage:  &errorMessage,
				ProviderChain: finalizeProviderChain(providerChain, http.StatusInternalServerError, &errorMessage),
			})
			c.Error(decisionErr)
			return
		}
		terminalUpdate := repository.MessageRequestTerminalUpdate{
			StatusCode: streamDecision.StatusCode,
			DurationMs: int(time.Since(startedAt).Milliseconds()),
		}
		if detectedStreamError && streamDecision.ErrorMessage != "" && streamDecision.StatusCode >= http.StatusBadRequest {
			terminalUpdate.ErrorMessage = &streamDecision.ErrorMessage
		}
		terminalUpdate.ProviderChain = finalizeProviderChain(providerChain, terminalUpdate.StatusCode, terminalUpdate.ErrorMessage)
		h.finalizeMessageRequest(c, messageRequestID, terminalUpdate)
		if shouldCountFailureForCircuit(terminalUpdate.StatusCode) {
			circuitbreakersvc.RecordFailure(provider, false)
		} else if terminalUpdate.StatusCode >= http.StatusOK && terminalUpdate.StatusCode < http.StatusMultipleChoices {
			circuitbreakersvc.RecordSuccess(provider)
		}
		return
	}

	responseBody, err := io.ReadAll(upstreamResp.Body)
	if err != nil {
		errorMessage := "读取上游响应失败"
		h.finalizeMessageRequest(c, messageRequestID, repository.MessageRequestTerminalUpdate{
			StatusCode:    http.StatusBadGateway,
			DurationMs:    int(time.Since(startedAt).Milliseconds()),
			ErrorMessage:  &errorMessage,
			ProviderChain: finalizeProviderChain(providerChain, http.StatusBadGateway, &errorMessage),
		})
		writeAppError(c, appErrors.NewProviderError(errorMessage, appErrors.CodeProviderError).WithError(err))
		return
	}
	if endpointKind == proxyEndpointResponses && sessionID != "" && h.sessions != nil {
		if promptCacheKey := extractCodexPromptCacheKey(responseBody); promptCacheKey != "" {
			h.sessions.UpdateCodexSessionWithPromptCacheKey(c.Request.Context(), sessionID, promptCacheKey, provider.ID)
		}
	}
	decision, err := h.evaluateUpstreamErrorDecision(c.Request.Context(), upstreamResp.StatusCode, responseBody)
	if err != nil {
		errorMessage := "应用错误规则失败"
		h.finalizeMessageRequest(c, messageRequestID, repository.MessageRequestTerminalUpdate{
			StatusCode:    http.StatusInternalServerError,
			DurationMs:    int(time.Since(startedAt).Milliseconds()),
			ErrorMessage:  &errorMessage,
			ProviderChain: finalizeProviderChain(providerChain, http.StatusInternalServerError, &errorMessage),
		})
		writeAppError(c, err)
		return
	}
	responseBody = decision.ResponseBody
	if fixedBody, changed := h.maybeFixResponseBody(upstreamResp.Header, responseBody); changed {
		responseBody = fixedBody
	}
	if decision.StatusCode >= http.StatusBadRequest && !decision.OverrideApplied {
		responseBody = buildNormalizedProxyErrorResponseBody(decision.StatusCode, decision.ErrorMessage, decision.RequestID)
		c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	terminalUpdate := buildTerminalUpdate(endpointKind, decision.StatusCode, time.Since(startedAt), responseBody)
	if decision.ErrorMessage != "" && terminalUpdate.StatusCode >= http.StatusBadRequest {
		terminalUpdate.ErrorMessage = &decision.ErrorMessage
	}
	terminalUpdate.ProviderChain = finalizeProviderChain(providerChain, terminalUpdate.StatusCode, terminalUpdate.ErrorMessage)
	if shouldCountFailureForCircuit(terminalUpdate.StatusCode) {
		circuitbreakersvc.RecordFailure(provider, false)
	} else if terminalUpdate.StatusCode >= http.StatusOK && terminalUpdate.StatusCode < http.StatusMultipleChoices {
		circuitbreakersvc.RecordSuccess(provider)
	}
	finalStatusCode := terminalUpdate.StatusCode
	c.Status(finalStatusCode)
	if finalStatusCode != upstreamResp.StatusCode {
		c.Writer.Header().Del("Content-Length")
	}
	if _, err := c.Writer.Write(responseBody); err != nil {
		c.Error(err)
	}
	h.finalizeMessageRequest(c, messageRequestID, terminalUpdate)
}

func writeAppError(c *gin.Context, err error) {
	var appErr *appErrors.AppError
	if stderrors.As(err, &appErr) {
		if writeProxyNormalizedAppError(c, appErr) {
			return
		}
		c.AbortWithStatusJSON(appErr.HTTPStatus, appErr.ToResponse())
		return
	}

	fallback := appErrors.NewInternalError("代理鉴权失败")
	c.AbortWithStatusJSON(fallback.HTTPStatus, fallback.ToResponse())
}

func appErrorAs(err error) *appErrors.AppError {
	var appErr *appErrors.AppError
	if stderrors.As(err, &appErr) {
		return appErr
	}
	return nil
}

func writeProxyNormalizedAppError(c *gin.Context, appErr *appErrors.AppError) bool {
	if c == nil || appErr == nil {
		return false
	}
	switch appErr.Type {
	case appErrors.ErrorTypeAuthentication:
		writeNormalizedProxyErrorResponse(c, appErr.HTTPStatus, appErr.Message, proxyAuthenticationErrorType(appErr.Code), "")
		return true
	case appErrors.ErrorTypePermissionDenied:
		writeNormalizedProxyErrorResponse(c, appErr.HTTPStatus, appErr.Message, "permission_error", "")
		return true
	case appErrors.ErrorTypeInvalidRequest:
		writeNormalizedProxyErrorResponse(c, appErr.HTTPStatus, appErr.Message, "invalid_request_error", "")
		return true
	case appErrors.ErrorTypeRateLimitError:
		writeNormalizedProxyRateLimitErrorResponse(c, appErr)
		return true
	default:
		return false
	}
}

func proxyAuthenticationErrorType(code appErrors.ErrorCode) string {
	switch code {
	case appErrors.CodeInvalidAPIKey, appErrors.CodeDisabledAPIKey, appErrors.CodeExpiredAPIKey:
		return "invalid_api_key"
	case appErrors.CodeDisabledUser:
		return "user_disabled"
	case appErrors.CodeUserExpired:
		return "user_expired"
	default:
		return "authentication_error"
	}
}

func writeNormalizedProxyRateLimitErrorResponse(c *gin.Context, appErr *appErrors.AppError) {
	if c == nil || appErr == nil {
		return
	}
	limitType := proxyRateLimitType(appErr.Code)
	current := proxyRateLimitDetailValue(appErr.Details, "current")
	limit := proxyRateLimitDetailValue(appErr.Details, "limit")
	resetTime := proxyRateLimitResetTime(appErr.Details)

	errorBody := map[string]any{
		"type":       "rate_limit_error",
		"message":    strings.TrimSpace(appErr.Message),
		"code":       "rate_limit_exceeded",
		"limit_type": limitType,
		"current":    current,
		"limit":      limit,
		"reset_time": resetTime,
	}
	if errorBody["message"] == "" {
		errorBody["message"] = http.StatusText(appErr.HTTPStatus)
	}
	body, err := json.Marshal(map[string]any{"error": errorBody})
	if err != nil {
		body = []byte(`{"error":{"type":"rate_limit_error","message":"` + http.StatusText(appErr.HTTPStatus) + `","code":"rate_limit_exceeded"}}`)
	}

	if limit != nil {
		c.Header("X-RateLimit-Limit", stringifyHeaderValue(limit))
	}
	if limitType != "" {
		c.Header("X-RateLimit-Type", limitType)
	}
	if remaining, ok := proxyRateLimitRemaining(limit, current); ok {
		c.Header("X-RateLimit-Remaining", remaining)
	}
	if resetTime != nil {
		if resetAt, ok := resetTime.(string); ok && strings.TrimSpace(resetAt) != "" {
			c.Header("X-RateLimit-Reset", strings.TrimSpace(resetAt))
		}
	}

	c.Abort()
	c.Data(appErr.HTTPStatus, "application/json; charset=utf-8", body)
}

func proxyRateLimitType(code appErrors.ErrorCode) string {
	switch code {
	case appErrors.CodeConcurrentSessionsExceeded:
		return "concurrent_sessions"
	case appErrors.CodeRPMLimitExceeded:
		return "rpm"
	case appErrors.Code5HLimitExceeded:
		return "usd_5h"
	case appErrors.CodeDailyLimitExceeded:
		return "daily_quota"
	case appErrors.CodeWeeklyLimitExceeded:
		return "usd_weekly"
	case appErrors.CodeMonthlyLimitExceeded:
		return "usd_monthly"
	case appErrors.CodeTotalLimitExceeded:
		return "usd_total"
	default:
		return "rate_limit"
	}
}

func proxyRateLimitDetailValue(details map[string]interface{}, key string) any {
	if details == nil {
		return nil
	}
	value, ok := details[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil
		}
		if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return parsed
		}
		return trimmed
	case int:
		return typed
	case int64:
		return typed
	case float64:
		return typed
	case float32:
		return float64(typed)
	default:
		return typed
	}
}

func proxyRateLimitResetTime(details map[string]interface{}) any {
	if details == nil {
		return nil
	}
	for _, key := range []string{"reset_time", "resetTime"} {
		if value, ok := details[key]; ok {
			return value
		}
	}
	return nil
}

func stringifyHeaderValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 64)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func proxyRateLimitRemaining(limit any, current any) (string, bool) {
	limitFloat, okLimit := asFloat64(limit)
	currentFloat, okCurrent := asFloat64(current)
	if !okLimit || !okCurrent {
		return "", false
	}
	remaining := limitFloat - currentFloat
	if remaining < 0 {
		remaining = 0
	}
	return strconv.FormatFloat(remaining, 'f', -1, 64), true
}

func asFloat64(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func shouldTrackConcurrentRequests(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	if c.Request.Method != http.MethodPost {
		return false
	}

	switch c.FullPath() {
	case "/v1/messages", "/v1/chat/completions", "/v1/responses":
		return true
	default:
		return false
	}
}

func decodeRequestBody(c *gin.Context) (map[string]any, bool) {
	requestBody, _, ok := decodeRequestBodyBytes(c)
	return requestBody, ok
}

func decodeRequestBodyBytes(c *gin.Context) (map[string]any, []byte, bool) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return nil, nil, false
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, nil, false
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	if len(bytes.TrimSpace(bodyBytes)) == 0 {
		return map[string]any{}, bodyBytes, true
	}

	var requestBody map[string]any
	if err := json.Unmarshal(bodyBytes, &requestBody); err != nil {
		return nil, nil, false
	}

	return requestBody, bodyBytes, true
}

func extractRequestMessages(requestBody map[string]any) any {
	if requestBody == nil {
		return nil
	}
	if messages, ok := requestBody["messages"]; ok {
		return messages
	}
	if input, ok := requestBody["input"]; ok {
		return input
	}
	return nil
}

func requestStringValue(value any) string {
	text, _ := value.(string)
	return text
}

func (h *Handler) shouldInterceptWarmup(ctx context.Context, endpointKind proxyEndpointKind, requestBody map[string]any) bool {
	if endpointKind != proxyEndpointMessages || h == nil || h.settings == nil {
		return false
	}
	settings, err := h.settings.Get(ctx)
	if err != nil || settings == nil || !settings.InterceptAnthropicWarmupRequests {
		return false
	}
	return isWarmupMessagesRequest(requestBody)
}

func isWarmupMessagesRequest(requestBody map[string]any) bool {
	if requestBody == nil {
		return false
	}
	messages, ok := requestBody["messages"].([]any)
	if !ok || len(messages) != 1 {
		return false
	}
	firstMessage, ok := messages[0].(map[string]any)
	if !ok || requestStringValue(firstMessage["role"]) != "user" {
		return false
	}
	content, ok := firstMessage["content"].([]any)
	if !ok || len(content) != 1 {
		return false
	}
	firstBlock, ok := content[0].(map[string]any)
	if !ok || requestStringValue(firstBlock["type"]) != "text" {
		return false
	}
	if strings.ToLower(strings.TrimSpace(requestStringValue(firstBlock["text"]))) != "warmup" {
		return false
	}
	cacheControl, ok := firstBlock["cache_control"].(map[string]any)
	if !ok {
		return false
	}
	return requestStringValue(cacheControl["type"]) == "ephemeral"
}

func (h *Handler) logWarmupIntercept(c *gin.Context, authResult *authsvc.AuthResult, requestBody map[string]any) {
	if h == nil || h.requestLogs == nil || authResult == nil || authResult.User == nil || authResult.Key == nil {
		return
	}

	sessionID, _ := GetProxySessionID(c)
	requestSequence := 1
	if sessionID != "" && h.sessions != nil {
		requestSequence = h.sessions.GetNextRequestSequence(c.Request.Context(), sessionID)
	}

	endpoint := ""
	userAgent := ""
	clientIP := ""
	messagesCount := countRequestPayloadItems(extractRequestMessages(requestBody))
	if c != nil {
		if c.FullPath() != "" {
			endpoint = c.FullPath()
		}
		if c.Request != nil {
			userAgent = c.Request.UserAgent()
			clientIP = c.ClientIP()
		}
	}

	blockedBy := "warmup"
	blockedReason := `{"reason":"anthropic_warmup_intercepted","note":"已由 CCH 抢答，未转发上游，不计费/不限流/不计入统计"}`
	durationMs := 0
	statusCode := http.StatusOK

	messageRequest := &model.MessageRequest{
		ProviderID:      0,
		UserID:          authResult.User.ID,
		Key:             authResult.Key.Key,
		Model:           requestStringValue(requestBody["model"]),
		SessionID:       stringPointer(sessionID),
		RequestSequence: requestSequence,
		ApiType:         stringPointer(string(resolveAPIType(endpoint))),
		Endpoint:        stringPointer(endpoint),
		UserAgent:       stringPointer(userAgent),
		ClientIP:        stringPointer(clientIP),
		MessagesCount:   intPointer(messagesCount),
		StatusCode:      &statusCode,
		DurationMs:      &durationMs,
		BlockedBy:       &blockedBy,
		BlockedReason:   &blockedReason,
	}
	_, _ = h.requestLogs.Create(c.Request.Context(), messageRequest)
}

func buildWarmupResponseBody(model string) map[string]any {
	if strings.TrimSpace(model) == "" {
		model = "unknown"
	}
	return map[string]any{
		"model": model,
		"id":    "msg_cch_" + randomWarmupSuffix(),
		"type":  "message",
		"role":  "assistant",
		"content": []map[string]any{{
			"type": "text",
			"text": "I'm ready to help you.",
		}},
		"stop_reason":   "end_turn",
		"stop_sequence": nil,
		"usage": map[string]any{
			"input_tokens":                0,
			"output_tokens":               0,
			"cache_creation_input_tokens": 0,
			"cache_read_input_tokens":     0,
		},
	}
}

func randomWarmupSuffix() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "warmup"
	}
	return hex.EncodeToString(buf[:])
}

func (h *Handler) expandProviderEndpointCandidates(ctx context.Context, provider *model.Provider) []*model.Provider {
	if provider == nil {
		return nil
	}
	if h == nil || h.providerVendors == nil || h.providerEndpoints == nil {
		return []*model.Provider{provider}
	}
	domain := providerVendorDomain(provider)
	if domain == "" {
		return []*model.Provider{provider}
	}
	vendor, err := h.providerVendors.GetByWebsiteDomain(ctx, domain)
	if err != nil || vendor == nil || vendor.ID <= 0 {
		return []*model.Provider{provider}
	}
	endpoints, err := h.providerEndpoints.ListActiveByVendorAndType(ctx, vendor.ID, provider.ProviderType)
	if err != nil || len(endpoints) == 0 {
		return []*model.Provider{provider}
	}
	endpoints = orderedProviderEndpointsForRouting(endpoints)
	candidates := make([]*model.Provider, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if !endpoint.IsActive() || strings.TrimSpace(endpoint.URL) == "" {
			continue
		}
		providerCopy := *provider
		providerCopy.URL = strings.TrimSpace(endpoint.URL)
		candidates = append(candidates, &providerCopy)
	}
	if len(candidates) == 0 {
		return []*model.Provider{provider}
	}
	return candidates
}

func providerVendorDomain(provider *model.Provider) string {
	if provider == nil {
		return ""
	}
	if provider.WebsiteUrl != nil && strings.TrimSpace(*provider.WebsiteUrl) != "" {
		return normalizedHostname(*provider.WebsiteUrl)
	}
	return normalizedHostname(provider.URL)
}

func normalizedHostname(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err == nil {
		if host := strings.TrimSpace(parsed.Hostname()); host != "" {
			return strings.ToLower(host)
		}
	}
	// url.Parse("example.com/path") treats the value as a path. Add a scheme and retry.
	if !strings.Contains(rawURL, "://") {
		if parsed, err := url.Parse("https://" + rawURL); err == nil {
			if host := strings.TrimSpace(parsed.Hostname()); host != "" {
				return strings.ToLower(host)
			}
		}
	}
	return strings.ToLower(rawURL)
}

func orderedProviderEndpointsForRouting(endpoints []*model.ProviderEndpoint) []*model.ProviderEndpoint {
	items := make([]*model.ProviderEndpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if endpoint != nil {
			items = append(items, endpoint)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		leftRank := providerEndpointProbeRank(items[i])
		rightRank := providerEndpointProbeRank(items[j])
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if items[i].SortOrder != items[j].SortOrder {
			return items[i].SortOrder < items[j].SortOrder
		}
		return items[i].ID < items[j].ID
	})
	return items
}

func providerEndpointProbeRank(endpoint *model.ProviderEndpoint) int {
	if endpoint == nil || endpoint.LastProbeOk == nil {
		return 1
	}
	if *endpoint.LastProbeOk {
		return 0
	}
	return 2
}

func (h *Handler) selectProviderForEndpoint(ctx context.Context, endpointKind proxyEndpointKind, requestedModel string) (*model.Provider, error) {
	candidates, err := h.selectProvidersForEndpoint(ctx, endpointKind, requestedModel)
	if err != nil {
		return nil, err
	}
	return candidates[0], nil
}

func (h *Handler) selectProvidersForEndpoint(ctx context.Context, endpointKind proxyEndpointKind, requestedModel string) ([]*model.Provider, error) {
	providers, err := h.providers.GetActiveProviders(ctx)
	if err != nil {
		return nil, err
	}
	providerConcurrentCounts, _ := providertrackersvc.Count(ctx)

	candidates := make([]*model.Provider, 0, len(providers))
	for _, provider := range providers {
		if provider == nil || !provider.IsActive() {
			continue
		}
		if circuitbreakersvc.IsOpen(provider) {
			continue
		}
		if !supportsEndpointKind(provider.ProviderType, endpointKind) {
			continue
		}
		if requestedModel != "" && !provider.SupportsModel(requestedModel) {
			continue
		}
		if limit := normalizeConcurrentSessionLimit(provider.LimitConcurrentSessions); limit > 0 {
			if providerConcurrentCounts != nil && providerConcurrentCounts[provider.ID] >= limit {
				continue
			}
		}
		if h != nil && h.stats != nil {
			if provider.Limit5hUSD != nil && provider.Limit5hUSD.GreaterThan(udecimal.Zero) {
				endTime := time.Now()
				startTime := endTime.Add(-5 * time.Hour)
				if current, err := h.stats.SumProviderCostInTimeRange(ctx, provider.ID, startTime, endTime); err == nil {
					if current.GreaterThan(*provider.Limit5hUSD) || current.Equal(*provider.Limit5hUSD) {
						continue
					}
				}
			}
			if provider.LimitDailyUSD != nil && provider.LimitDailyUSD.GreaterThan(udecimal.Zero) {
				startTime, endTime := dailyWindowBoundsForMode(h.settings, provider.DailyResetMode, provider.DailyResetTime)
				if current, err := h.stats.SumProviderCostInTimeRange(ctx, provider.ID, startTime, endTime); err == nil {
					if current.GreaterThan(*provider.LimitDailyUSD) || current.Equal(*provider.LimitDailyUSD) {
						continue
					}
				}
			}
			if provider.LimitWeeklyUSD != nil && provider.LimitWeeklyUSD.GreaterThan(udecimal.Zero) {
				startTime, endTime := weeklyWindowBounds(h.settings)
				if current, err := h.stats.SumProviderCostInTimeRange(ctx, provider.ID, startTime, endTime); err == nil {
					if current.GreaterThan(*provider.LimitWeeklyUSD) || current.Equal(*provider.LimitWeeklyUSD) {
						continue
					}
				}
			}
			if provider.LimitMonthlyUSD != nil && provider.LimitMonthlyUSD.GreaterThan(udecimal.Zero) {
				startTime, endTime := monthlyWindowBounds(h.settings)
				if current, err := h.stats.SumProviderCostInTimeRange(ctx, provider.ID, startTime, endTime); err == nil {
					if current.GreaterThan(*provider.LimitMonthlyUSD) || current.Equal(*provider.LimitMonthlyUSD) {
						continue
					}
				}
			}
			if provider.LimitTotalUSD != nil && provider.LimitTotalUSD.GreaterThan(udecimal.Zero) {
				if current, err := h.stats.SumProviderTotalCost(ctx, provider.ID, provider.TotalCostResetAt); err == nil {
					if current.GreaterThan(*provider.LimitTotalUSD) || current.Equal(*provider.LimitTotalUSD) {
						continue
					}
				}
			}
		}
		candidates = append(candidates, h.expandProviderEndpointCandidates(ctx, provider)...)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		leftPriority := 0
		if candidates[i].Priority != nil {
			leftPriority = *candidates[i].Priority
		}
		rightPriority := 0
		if candidates[j].Priority != nil {
			rightPriority = *candidates[j].Priority
		}
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}

		leftCostMultiplier := 1.0
		if candidates[i].CostMultiplier != nil {
			leftCostMultiplier = candidates[i].CostMultiplier.InexactFloat64()
		}
		rightCostMultiplier := 1.0
		if candidates[j].CostMultiplier != nil {
			rightCostMultiplier = candidates[j].CostMultiplier.InexactFloat64()
		}
		if leftCostMultiplier != rightCostMultiplier {
			return leftCostMultiplier < rightCostMultiplier
		}

		leftWeight := 1
		if candidates[i].Weight != nil {
			leftWeight = *candidates[i].Weight
		}
		rightWeight := 1
		if candidates[j].Weight != nil {
			rightWeight = *candidates[j].Weight
		}
		return leftWeight > rightWeight
	})

	if len(candidates) == 0 {
		message := "没有可用的 Responses 供应商。"
		if endpointKind == proxyEndpointMessages {
			message = "没有可用的 Messages 供应商。"
		} else if endpointKind == proxyEndpointMessagesCount {
			message = "没有可用的 Count Tokens 供应商。"
		} else if endpointKind == proxyEndpointChatCompletions {
			message = "没有可用的 Chat Completions 供应商。"
		}
		return nil, (&appErrors.AppError{
			Type:       appErrors.ErrorTypeProviderError,
			Message:    message,
			Code:       appErrors.CodeNoProviderAvailable,
			HTTPStatus: http.StatusServiceUnavailable,
		})
	}

	return candidates, nil
}

func buildProxyURL(baseURL string, requestURL *url.URL) (string, error) {
	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if requestURL == nil {
		return parsedBaseURL.String(), nil
	}

	basePath := strings.TrimRight(parsedBaseURL.Path, "/")
	requestPath := requestURL.Path

	if requestPath == basePath || strings.HasPrefix(requestPath, basePath+"/") {
		parsedBaseURL.Path = requestPath
		parsedBaseURL.RawQuery = requestURL.RawQuery
		return parsedBaseURL.String(), nil
	}

	if strings.HasSuffix(basePath, "/responses") || strings.HasSuffix(basePath, "/v1/responses") ||
		strings.HasSuffix(basePath, "/messages") || strings.HasSuffix(basePath, "/v1/messages") ||
		strings.HasSuffix(basePath, "/chat/completions") || strings.HasSuffix(basePath, "/v1/chat/completions") {
		parsedBaseURL.Path = basePath
		parsedBaseURL.RawQuery = requestURL.RawQuery
		return parsedBaseURL.String(), nil
	}

	parsedBaseURL.Path = basePath + requestPath
	parsedBaseURL.RawQuery = requestURL.RawQuery
	return parsedBaseURL.String(), nil
}

func copyProxyRequestHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		normalized := strings.ToLower(key)
		if normalized == "authorization" || normalized == "x-api-key" || normalized == "x-goog-api-key" || normalized == "host" || normalized == "content-length" {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func supportsEndpointKind(providerType string, endpointKind proxyEndpointKind) bool {
	switch endpointKind {
	case proxyEndpointMessages:
		fallthrough
	case proxyEndpointMessagesCount:
		return providerType == string(model.ProviderTypeClaude) || providerType == string(model.ProviderTypeClaudeAuth)
	case proxyEndpointResponses:
		return providerType == string(model.ProviderTypeCodex)
	case proxyEndpointChatCompletions:
		return providerType == string(model.ProviderTypeOpenAICompatible)
	default:
		return false
	}
}

func applyProviderAuthHeaders(dst http.Header, provider *model.Provider, endpointKind proxyEndpointKind) {
	if provider == nil {
		return
	}

	dst.Set("Content-Type", "application/json")

	switch endpointKind {
	case proxyEndpointMessages:
		fallthrough
	case proxyEndpointMessagesCount:
		dst.Set("Authorization", "Bearer "+provider.Key)
		dst.Set("x-api-key", provider.Key)
		if provider.ProviderType == string(model.ProviderTypeClaudeAuth) {
			dst.Del("x-api-key")
		}
	default:
		if provider.ProviderType == string(model.ProviderTypeGemini) || provider.ProviderType == string(model.ProviderTypeGeminiCli) {
			dst.Del("Authorization")
			dst.Set("x-goog-api-key", provider.Key)
			return
		}
		dst.Set("Authorization", "Bearer "+provider.Key)
	}
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func copyProxyResponseHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		normalized := strings.ToLower(key)
		if normalized == "content-length" || normalized == "transfer-encoding" || normalized == "connection" {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func isStreamingResponse(headers http.Header) bool {
	if headers == nil {
		return false
	}
	return strings.Contains(strings.ToLower(headers.Get("Content-Type")), "text/event-stream")
}

func (h *Handler) createMessageRequest(
	c *gin.Context,
	authResult *authsvc.AuthResult,
	provider *model.Provider,
	providerChain []model.ProviderChainItem,
	requestBody map[string]any,
	originalModel string,
	effectiveModel string,
	sessionID string,
	requestSequence int,
) int {
	if h == nil || h.requestLogs == nil || authResult == nil || authResult.User == nil || authResult.Key == nil || provider == nil {
		return 0
	}

	endpoint := ""
	userAgent := ""
	clientIP := ""
	messagesCount := countRequestPayloadItems(extractRequestMessages(requestBody))
	if c != nil {
		if c.FullPath() != "" {
			endpoint = c.FullPath()
		}
		if c.Request != nil {
			userAgent = c.Request.UserAgent()
			clientIP = c.ClientIP()
		}
	}

	messageRequest := &model.MessageRequest{
		ProviderID:      provider.ID,
		UserID:          authResult.User.ID,
		Key:             authResult.Key.Key,
		Model:           effectiveModel,
		CostMultiplier:  provider.CostMultiplier,
		SessionID:       stringPointer(sessionID),
		RequestSequence: requestSequence,
		ApiType:         stringPointer(string(resolveAPIType(endpoint))),
		Endpoint:        stringPointer(endpoint),
		OriginalModel:   stringPointer(originalModel),
		ProviderChain:   providerChain,
		UserAgent:       stringPointer(userAgent),
		ClientIP:        stringPointer(clientIP),
		MessagesCount:   intPointer(messagesCount),
	}

	if _, err := h.requestLogs.Create(c.Request.Context(), messageRequest); err != nil {
		// persistence is best-effort for the minimal slice
		return 0
	}
	return messageRequest.ID
}

func providerCostMultiplier(provider *model.Provider) *float64 {
	if provider == nil || provider.CostMultiplier == nil {
		return nil
	}
	value := provider.CostMultiplier.InexactFloat64()
	return &value
}

func buildInitialProviderChain(provider *model.Provider) []model.ProviderChainItem {
	if provider == nil {
		return nil
	}
	return []model.ProviderChainItem{{
		ID:              provider.ID,
		Name:            provider.Name,
		ProviderType:    stringPointer(provider.ProviderType),
		EndpointURL:     stringPointer(provider.URL),
		Reason:          stringPointer("initial_selection"),
		SelectionMethod: stringPointer("weighted_random"),
		Priority:        provider.Priority,
		Weight:          provider.Weight,
		CostMultiplier:  providerCostMultiplier(provider),
		Timestamp:       int64Pointer(time.Now().UnixMilli()),
	}}
}

func finalizeProviderChain(base []model.ProviderChainItem, statusCode int, errorMessage *string) []model.ProviderChainItem {
	if len(base) == 0 {
		return nil
	}
	finalized := make([]model.ProviderChainItem, len(base))
	copy(finalized, base)
	finalized[len(finalized)-1].StatusCode = intPointer(statusCode)
	finalized[len(finalized)-1].ErrorMessage = errorMessage
	return finalized
}

func buildTerminalUpdate(endpointKind proxyEndpointKind, statusCode int, duration time.Duration, responseBody []byte) repository.MessageRequestTerminalUpdate {
	update := repository.MessageRequestTerminalUpdate{
		StatusCode: statusCode,
		DurationMs: int(duration.Milliseconds()),
	}

	if len(bytes.TrimSpace(responseBody)) == 0 {
		return update
	}

	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		if fakeStatusCode, fakeErrorMessage, ok := detectFake200HTMLResponse(statusCode, responseBody); ok {
			update.StatusCode = fakeStatusCode
			update.ErrorMessage = &fakeErrorMessage
		} else if fakeStatusCode, fakeErrorMessage, ok := detectFake200PlainTextResponse(statusCode, responseBody); ok {
			update.StatusCode = fakeStatusCode
			update.ErrorMessage = &fakeErrorMessage
		}
		return update
	}
	if fakeStatusCode, fakeErrorMessage, ok := detectFake200JSONResponse(statusCode, payload, responseBody); ok {
		update.StatusCode = fakeStatusCode
		update.ErrorMessage = &fakeErrorMessage
		return update
	}

	if statusCode >= http.StatusBadRequest {
		if errorMessage := extractErrorMessage(payload); errorMessage != "" {
			update.ErrorMessage = &errorMessage
		}
		return update
	}

	switch endpointKind {
	case proxyEndpointMessages:
		populateAnthropicUsageUpdate(&update, payload)
	case proxyEndpointMessagesCount:
		if inputTokens, ok := lookupInt(payload, "input_tokens"); ok {
			update.InputTokens = &inputTokens
		}
	case proxyEndpointResponses:
		populateResponsesUsageUpdate(&update, payload)
	case proxyEndpointChatCompletions:
		populateChatCompletionsUsageUpdate(&update, payload)
	}

	return update
}

func shouldCountFailureForCircuit(statusCode int) bool {
	if statusCode == http.StatusNotFound {
		return false
	}
	return statusCode >= http.StatusBadRequest
}

func extractErrorMessageOrBody(responseBody []byte) string {
	if len(bytes.TrimSpace(responseBody)) == 0 {
		return "not found"
	}
	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err == nil {
		if message := extractErrorMessage(payload); message != "" {
			return message
		}
	}
	return strings.TrimSpace(string(responseBody))
}

func detectFake200HTMLResponse(statusCode int, responseBody []byte) (int, string, bool) {
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return 0, "", false
	}
	trimmed := strings.TrimSpace(string(responseBody))
	if trimmed == "" {
		return 0, "", false
	}
	lowerTrimmed := strings.ToLower(trimmed)
	if strings.HasPrefix(lowerTrimmed, "<!doctype html") || strings.HasPrefix(lowerTrimmed, "<html") {
		return http.StatusBadGateway, "上游返回了 HTML 错误页", true
	}
	return 0, "", false
}

func detectFake200PlainTextResponse(statusCode int, responseBody []byte) (int, string, bool) {
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return 0, "", false
	}
	trimmed := strings.TrimSpace(string(responseBody))
	if trimmed == "" {
		return 0, "", false
	}
	if !isLikelyUpstreamErrorMessage(trimmed) {
		return 0, "", false
	}
	return inferUpstreamErrorStatusCode(trimmed, responseBody), trimmed, true
}

func detectFake200JSONResponse(statusCode int, payload map[string]any, responseBody []byte) (int, string, bool) {
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return 0, "", false
	}
	if payload == nil {
		return 0, "", false
	}
	errorMessage := extractErrorMessage(payload)
	if errorMessage != "" {
		return inferUpstreamErrorStatusCode(errorMessage, responseBody), errorMessage, true
	}
	if message, ok := payload["message"].(string); ok && isLikelyUpstreamErrorMessage(message) {
		message = strings.TrimSpace(message)
		return inferUpstreamErrorStatusCode(message, responseBody), message, true
	}
	return 0, "", false
}

func isLikelyUpstreamErrorMessage(message string) bool {
	lowerMessage := strings.ToLower(strings.TrimSpace(message))
	if lowerMessage == "" {
		return false
	}
	keywords := []string{
		"error",
		"rate limit",
		"too many requests",
		"forbidden",
		"unauthorized",
		"not found",
		"invalid",
		"timeout",
		"timed out",
		"service unavailable",
		"overloaded",
		"限流",
		"未授权",
		"无权限",
		"超时",
		"不可用",
	}
	for _, keyword := range keywords {
		if strings.Contains(lowerMessage, keyword) {
			return true
		}
	}
	return false
}

func inferUpstreamErrorStatusCode(message string, responseBody []byte) int {
	lowerText := strings.ToLower(strings.TrimSpace(message + "\n" + string(responseBody)))
	matchers := []struct {
		statusCode int
		keywords   []string
	}{
		{statusCode: http.StatusTooManyRequests, keywords: []string{"too many requests", "rate limit", "rate limited", "thrott", "resource_exhausted", "限流", "请求过于频繁"}},
		{statusCode: http.StatusUnauthorized, keywords: []string{"unauthorized", "unauthenticated", "invalid api key", "invalid token", "expired token", "未授权", "鉴权失败", "密钥无效"}},
		{statusCode: http.StatusForbidden, keywords: []string{"forbidden", "permission denied", "access denied", "无权限", "权限不足", "禁止访问"}},
		{statusCode: http.StatusNotFound, keywords: []string{"not found", "unknown model", "does not exist", "未找到", "不存在", "模型不存在"}},
		{statusCode: http.StatusBadRequest, keywords: []string{"bad request", "invalid json", "json parse", "invalid argument", "无效请求", "格式错误"}},
		{statusCode: http.StatusServiceUnavailable, keywords: []string{"service unavailable", "server is busy", "temporarily unavailable", "maintenance", "overloaded", "服务不可用", "系统繁忙", "维护中"}},
		{statusCode: http.StatusGatewayTimeout, keywords: []string{"gateway timeout", "timed out", "deadline exceeded", "网关超时", "上游超时"}},
	}
	for _, matcher := range matchers {
		for _, keyword := range matcher.keywords {
			if strings.Contains(lowerText, keyword) {
				return matcher.statusCode
			}
		}
	}
	return http.StatusBadGateway
}

func extractErrorMessage(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if errorValue, ok := payload["error"]; ok {
		switch value := errorValue.(type) {
		case string:
			return strings.TrimSpace(value)
		case map[string]any:
			if message, ok := value["message"].(string); ok {
				return strings.TrimSpace(message)
			}
			if code, ok := value["code"].(string); ok && strings.TrimSpace(code) != "" {
				return strings.TrimSpace(code)
			}
		}
	}
	if message, ok := payload["message"].(string); ok {
		return strings.TrimSpace(message)
	}
	return ""
}

func buildNormalizedProxyErrorResponseBody(statusCode int, message string, requestID string) []byte {
	return buildNormalizedProxyErrorResponseBodyWithType(statusCode, message, "", requestID)
}

func buildNormalizedProxyErrorResponseBodyWithType(statusCode int, message string, errorType string, requestID string) []byte {
	if strings.TrimSpace(errorType) == "" {
		errorType = proxyErrorTypeForStatus(statusCode)
	}
	payload := map[string]any{
		"error": map[string]any{
			"message": strings.TrimSpace(message),
			"type":    errorType,
			"code":    proxyErrorCodeForStatus(statusCode, errorType),
		},
	}
	if payload["error"].(map[string]any)["message"] == "" {
		payload["error"].(map[string]any)["message"] = http.StatusText(statusCode)
	}
	if strings.TrimSpace(requestID) != "" {
		payload["request_id"] = strings.TrimSpace(requestID)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return []byte(`{"error":{"message":"` + http.StatusText(statusCode) + `","type":"api_error","code":"api_error"}}`)
	}
	return body
}

func writeNormalizedProxyErrorResponse(c *gin.Context, statusCode int, message string, errorType string, requestID string) {
	if c == nil {
		return
	}
	body := buildNormalizedProxyErrorResponseBodyWithType(statusCode, message, errorType, requestID)
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Abort()
	c.Data(statusCode, "application/json; charset=utf-8", body)
}

func proxyErrorTypeForStatus(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "invalid_request_error"
	case http.StatusUnauthorized:
		return "authentication_error"
	case http.StatusPaymentRequired:
		return "payment_required_error"
	case http.StatusForbidden:
		return "permission_error"
	case http.StatusNotFound:
		return "not_found_error"
	case http.StatusTooManyRequests:
		return "rate_limit_error"
	case http.StatusInternalServerError:
		return "internal_server_error"
	case http.StatusBadGateway:
		return "bad_gateway_error"
	case http.StatusServiceUnavailable:
		return "service_unavailable_error"
	case http.StatusGatewayTimeout:
		return "gateway_timeout_error"
	default:
		return "api_error"
	}
}

func proxyErrorCodeForStatus(statusCode int, errorType string) string {
	if strings.TrimSpace(errorType) != "" && errorType != "api_error" {
		return errorType
	}
	switch statusCode {
	case http.StatusBadRequest:
		return "invalid_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusPaymentRequired:
		return "payment_required"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusTooManyRequests:
		return "rate_limit_exceeded"
	case http.StatusInternalServerError:
		return "internal_error"
	case http.StatusBadGateway:
		return "bad_gateway"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	case http.StatusGatewayTimeout:
		return "gateway_timeout"
	default:
		return "http_" + strconv.Itoa(statusCode)
	}
}

func extractCodexPromptCacheKey(responseBody []byte) string {
	if len(bytes.TrimSpace(responseBody)) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return ""
	}
	if response, ok := payload["response"].(map[string]any); ok {
		if promptCacheKey := requestStringValue(response["prompt_cache_key"]); promptCacheKey != "" {
			return promptCacheKey
		}
	}
	return requestStringValue(payload["prompt_cache_key"])
}

func extractCodexPromptCacheKeyFromSSE(streamBody []byte) string {
	lines := strings.Split(string(streamBody), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		if promptCacheKey := extractCodexPromptCacheKey([]byte(payload)); promptCacheKey != "" {
			return promptCacheKey
		}
	}
	return ""
}

func populateAnthropicUsageUpdate(update *repository.MessageRequestTerminalUpdate, payload map[string]any) {
	usage, ok := payload["usage"].(map[string]any)
	if !ok {
		return
	}
	if inputTokens, ok := lookupInt(usage, "input_tokens"); ok {
		update.InputTokens = &inputTokens
	}
	if outputTokens, ok := lookupInt(usage, "output_tokens"); ok {
		update.OutputTokens = &outputTokens
	}
	if cacheCreationInputTokens, ok := lookupInt(usage, "cache_creation_input_tokens"); ok {
		update.CacheCreationInputTokens = &cacheCreationInputTokens
	}
	if cacheReadInputTokens, ok := lookupInt(usage, "cache_read_input_tokens"); ok {
		update.CacheReadInputTokens = &cacheReadInputTokens
	}
	if cacheCreationDetails, ok := usage["cache_creation"].(map[string]any); ok {
		if cacheCreation5mInputTokens, ok := lookupInt(cacheCreationDetails, "ephemeral_5m_input_tokens"); ok {
			update.CacheCreation5mInputTokens = &cacheCreation5mInputTokens
		}
		if cacheCreation1hInputTokens, ok := lookupInt(cacheCreationDetails, "ephemeral_1h_input_tokens"); ok {
			update.CacheCreation1hInputTokens = &cacheCreation1hInputTokens
		}
	}
}

func populateResponsesUsageUpdate(update *repository.MessageRequestTerminalUpdate, payload map[string]any) {
	usage, ok := payload["usage"].(map[string]any)
	if !ok {
		return
	}
	if inputTokens, ok := lookupInt(usage, "input_tokens"); ok {
		update.InputTokens = &inputTokens
	}
	if outputTokens, ok := lookupInt(usage, "output_tokens"); ok {
		update.OutputTokens = &outputTokens
	}
	if inputTokensDetails, ok := usage["input_tokens_details"].(map[string]any); ok {
		if cacheReadInputTokens, ok := lookupInt(inputTokensDetails, "cached_tokens"); ok {
			update.CacheReadInputTokens = &cacheReadInputTokens
		}
	}
}

func populateChatCompletionsUsageUpdate(update *repository.MessageRequestTerminalUpdate, payload map[string]any) {
	usage, ok := payload["usage"].(map[string]any)
	if !ok {
		return
	}
	if inputTokens, ok := lookupInt(usage, "prompt_tokens"); ok {
		update.InputTokens = &inputTokens
	}
	if outputTokens, ok := lookupInt(usage, "completion_tokens"); ok {
		update.OutputTokens = &outputTokens
	}
	if promptTokensDetails, ok := usage["prompt_tokens_details"].(map[string]any); ok {
		if cacheReadInputTokens, ok := lookupInt(promptTokensDetails, "cached_tokens"); ok {
			update.CacheReadInputTokens = &cacheReadInputTokens
		}
	}
}

func lookupInt(payload map[string]any, key string) (int, bool) {
	if payload == nil {
		return 0, false
	}
	value, ok := payload[key]
	if !ok {
		return 0, false
	}
	switch number := value.(type) {
	case float64:
		return int(number), true
	case int:
		return number, true
	case int32:
		return int(number), true
	case int64:
		return int(number), true
	default:
		return 0, false
	}
}

func (h *Handler) finalizeMessageRequest(c *gin.Context, id int, update repository.MessageRequestTerminalUpdate) {
	if h == nil || h.requestLogs == nil || id <= 0 {
		return
	}
	reqCtx := context.Background()
	if c != nil && c.Request != nil {
		reqCtx = c.Request.Context()
	}
	_ = h.requestLogs.UpdateTerminal(reqCtx, id, update)
}

type proxyAPIType string

const (
	proxyAPITypeClaude  proxyAPIType = "claude"
	proxyAPITypeOpenAI  proxyAPIType = "openai"
	proxyAPITypeCodex   proxyAPIType = "codex"
	proxyAPITypeUnknown proxyAPIType = "unknown"
)

func resolveAPIType(endpoint string) proxyAPIType {
	switch endpoint {
	case "/v1/messages", "/v1/messages/count_tokens":
		return proxyAPITypeClaude
	case "/v1/chat/completions":
		return proxyAPITypeOpenAI
	case "/v1/responses":
		return proxyAPITypeCodex
	default:
		return proxyAPITypeUnknown
	}
}

func countRequestPayloadItems(messages any) int {
	items, ok := messages.([]any)
	if !ok {
		return 0
	}
	return len(items)
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func intPointer(value int) *int {
	return &value
}

func int64Pointer(value int64) *int64 {
	return &value
}
