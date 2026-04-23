package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	"github.com/gin-gonic/gin"
)

type proxyStatusUserStore interface {
	List(ctx context.Context, opts *repository.ListOptions) ([]*model.User, error)
}

type proxyStatusLogStore interface {
	ListRecent(ctx context.Context, limit int) ([]*model.MessageRequest, error)
}

type ProxyStatusHandler struct {
	auth  adminAuthenticator
	users proxyStatusUserStore
	logs  proxyStatusLogStore
}

type proxyStatusRequest struct {
	RequestID    int    `json:"requestId"`
	KeyName      string `json:"keyName,omitempty"`
	Model        string `json:"model"`
	ProviderID   int    `json:"providerId,omitempty"`
	ProviderName string `json:"providerName,omitempty"`
	StartTime    int64  `json:"startTime,omitempty"`
	Duration     int64  `json:"duration,omitempty"`
	EndTime      int64  `json:"endTime,omitempty"`
	Elapsed      int64  `json:"elapsed,omitempty"`
}

type userProxyStatus struct {
	UserID         int                  `json:"userId"`
	UserName       string               `json:"userName"`
	ActiveCount    int                  `json:"activeCount"`
	ActiveRequests []proxyStatusRequest `json:"activeRequests"`
	LastRequest    *proxyStatusRequest  `json:"lastRequest"`
}

func NewProxyStatusHandler(auth adminAuthenticator, users proxyStatusUserStore, logs proxyStatusLogStore) *ProxyStatusHandler {
	return &ProxyStatusHandler{auth: auth, users: users, logs: logs}
}

func (h *ProxyStatusHandler) RegisterDirectRoutes(router gin.IRouter) {
	router.GET("/api/proxy-status", h.getDirect)
}

func (h *ProxyStatusHandler) RegisterActionRoutes(group *gin.RouterGroup) {
	protected := group.Group("/proxy-status")
	protected.Use((&Handler{auth: h.auth}).AdminAuthMiddleware())
	protected.GET("", h.getAction)
	protected.POST("/getProxyStatus", h.getAction)
}

func (h *ProxyStatusHandler) getDirect(c *gin.Context) {
	if !h.ensureAdmin(c) {
		return
	}
	status, err := h.buildStatus(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *ProxyStatusHandler) getAction(c *gin.Context) {
	if h == nil || h.users == nil || h.logs == nil {
		writeAdminError(c, appErrors.NewInternalError("代理状态服务未初始化"))
		return
	}
	status, err := h.buildStatus(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": status})
}

func (h *ProxyStatusHandler) ensureAdmin(c *gin.Context) bool {
	if h == nil || h.auth == nil {
		writeAdminError(c, appErrors.NewInternalError("代理状态鉴权服务未初始化"))
		return false
	}
	authResult, err := h.auth.AuthenticateAdminToken(resolveAdminToken(c))
	if err != nil {
		writeAdminError(c, err)
		return false
	}
	if authResult == nil || !authResult.IsAdmin {
		writeAdminError(c, appErrors.NewPermissionDenied("权限不足", appErrors.CodePermissionDenied))
		return false
	}
	return true
}

func (h *ProxyStatusHandler) buildStatus(ctx context.Context) (gin.H, error) {
	if h == nil || h.users == nil || h.logs == nil {
		return nil, appErrors.NewInternalError("代理状态服务未初始化")
	}
	users, err := h.users.List(ctx, repository.NewListOptions())
	if err != nil {
		return nil, err
	}
	logs, err := h.logs.ListRecent(ctx, 200)
	if err != nil {
		return nil, err
	}

	lastByUser := map[int]*proxyStatusRequest{}
	lastRequestAtByUser := map[int]time.Time{}
	activeByUser := map[int][]proxyStatusRequest{}
	now := time.Now()
	for _, log := range logs {
		if log == nil {
			continue
		}
		if isWarmupProxyStatusRequest(log) {
			continue
		}
		if !hasVisibleProxyStatusProvider(log) {
			continue
		}
		request := buildProxyStatusRequest(log, now)
		lastRequestAt := proxyStatusLastRequestAt(log)
		if lastAt, exists := lastRequestAtByUser[log.UserID]; !exists || lastRequestAt.After(lastAt) {
			lastRequestAtByUser[log.UserID] = lastRequestAt
			lastRequest := buildLastProxyStatusRequest(log, now)
			lastByUser[log.UserID] = &lastRequest
		}
		if !isCompletedProxyStatusRequest(log) {
			activeByUser[log.UserID] = append(activeByUser[log.UserID], request)
		}
	}

	responseUsers := make([]userProxyStatus, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		activeRequests := activeByUser[user.ID]
		if activeRequests == nil {
			activeRequests = []proxyStatusRequest{}
		}
		responseUsers = append(responseUsers, userProxyStatus{
			UserID:         user.ID,
			UserName:       user.Name,
			ActiveCount:    len(activeRequests),
			ActiveRequests: activeRequests,
			LastRequest:    lastByUser[user.ID],
		})
	}

	return gin.H{"users": responseUsers}, nil
}

func proxyStatusLastRequestAt(log *model.MessageRequest) time.Time {
	if log == nil {
		return time.Time{}
	}
	if !log.UpdatedAt.IsZero() {
		return log.UpdatedAt
	}
	if log.DurationMs != nil {
		return log.CreatedAt.Add(time.Duration(*log.DurationMs) * time.Millisecond)
	}
	return log.CreatedAt
}

func isCompletedProxyStatusRequest(log *model.MessageRequest) bool {
	if log == nil {
		return false
	}
	return log.DurationMs != nil
}

func isWarmupProxyStatusRequest(log *model.MessageRequest) bool {
	if log == nil || log.BlockedBy == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(*log.BlockedBy), "warmup")
}

func buildProxyStatusRequest(log *model.MessageRequest, now time.Time) proxyStatusRequest {
	if log == nil {
		return proxyStatusRequest{}
	}
	request := proxyStatusRequest{
		RequestID:    log.ID,
		KeyName:      resolveProxyStatusKeyName(log),
		Model:        log.Model,
		ProviderID:   log.ProviderID,
		ProviderName: resolveProxyStatusProviderName(log),
	}
	if request.Model == "" {
		request.Model = "unknown"
	}
	if !log.CreatedAt.IsZero() {
		request.StartTime = log.CreatedAt.UnixMilli()
	}
	if !isCompletedProxyStatusRequest(log) {
		request.Duration = now.Sub(log.CreatedAt).Milliseconds()
		return request
	}
	endTime := log.UpdatedAt
	if endTime.IsZero() {
		endTime = log.CreatedAt
		if log.DurationMs != nil {
			endTime = endTime.Add(time.Duration(*log.DurationMs) * time.Millisecond)
		}
	}
	request.EndTime = endTime.UnixMilli()
	request.Elapsed = now.Sub(endTime).Milliseconds()
	return request
}

func buildLastProxyStatusRequest(log *model.MessageRequest, now time.Time) proxyStatusRequest {
	request := buildProxyStatusRequest(log, now)
	request.StartTime = 0
	request.Duration = 0
	if request.EndTime == 0 {
		endTime := proxyStatusLastRequestAt(log)
		request.EndTime = endTime.UnixMilli()
		request.Elapsed = now.Sub(endTime).Milliseconds()
	}
	return request
}

func resolveProxyStatusProviderName(log *model.MessageRequest) string {
	if log == nil {
		return "unknown"
	}
	if log.ProviderName != nil && strings.TrimSpace(*log.ProviderName) != "" {
		return strings.TrimSpace(*log.ProviderName)
	}
	if log.Provider != nil && log.Provider.Name != "" {
		return log.Provider.Name
	}
	return "unknown"
}

func hasVisibleProxyStatusProvider(log *model.MessageRequest) bool {
	return resolveProxyStatusProviderName(log) != "unknown"
}

func resolveProxyStatusKeyName(log *model.MessageRequest) string {
	if log == nil {
		return "••••••"
	}
	if log.KeyName != nil && strings.TrimSpace(*log.KeyName) != "" {
		return strings.TrimSpace(*log.KeyName)
	}
	return maskProxyStatusKey(log.Key)
}

func maskProxyStatusKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "••••••"
	}
	if len(key) <= 8 {
		return "••••••"
	}
	return key[:4] + "••••••" + key[len(key)-4:]
}
