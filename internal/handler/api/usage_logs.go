package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	"github.com/gin-gonic/gin"
)

type usageLogsStore interface {
	ListRecent(ctx context.Context, limit int) ([]*model.MessageRequest, error)
	ListFiltered(ctx context.Context, limit int, modelName, endpoint, sessionID string, statusCode *int) ([]*model.MessageRequest, error)
	GetByID(ctx context.Context, id int) (*model.MessageRequest, error)
	GetSummary(ctx context.Context, modelName, endpoint string, statusCode *int) (repository.MessageRequestSummary, error)
	GetFilterOptions(ctx context.Context) (repository.MessageRequestFilterOptions, error)
	FindSessionIDSuggestions(ctx context.Context, term string, limit int) ([]string, error)
}

type UsageLogsActionHandler struct {
	auth  adminAuthenticator
	store usageLogsStore
}

func NewUsageLogsActionHandler(auth adminAuthenticator, store usageLogsStore) *UsageLogsActionHandler {
	return &UsageLogsActionHandler{auth: auth, store: store}
}

func (h *UsageLogsActionHandler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("/usage-logs")
	protected.Use((&Handler{auth: h.auth}).AdminAuthMiddleware())
	protected.GET("", h.list)
	protected.POST("/getUsageLogs", h.list)
	protected.GET("/models", h.models)
	protected.POST("/getModelList", h.models)
	protected.GET("/status-codes", h.statusCodes)
	protected.POST("/getStatusCodeList", h.statusCodes)
	protected.GET("/endpoints", h.endpoints)
	protected.GET("/summary", h.summary)
	protected.POST("/getUsageLogsStats", h.summary)
	protected.GET("/filter-options", h.filterOptions)
	protected.POST("/getFilterOptions", h.filterOptions)
	protected.GET("/session-id-suggestions", h.sessionIDSuggestions)
	protected.POST("/getUsageLogSessionIdSuggestions", h.sessionIDSuggestionsAction)
	protected.GET("/:id", h.detail)
}

func (h *UsageLogsActionHandler) list(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("日志仓储未初始化"))
		return
	}
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("limit 必须是整数"))
			return
		}
		limit = value
	}
	modelName := c.Query("model")
	endpoint := c.Query("endpoint")
	sessionID := c.Query("sessionId")
	var statusCode *int
	if raw := c.Query("statusCode"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("statusCode 必须是整数"))
			return
		}
		statusCode = &value
	}

	logs, err := h.store.ListFiltered(c.Request.Context(), limit, modelName, endpoint, sessionID, statusCode)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{
		"logs":  logs,
		"count": len(logs),
		"filters": gin.H{
			"model":      modelName,
			"endpoint":   endpoint,
			"sessionId":  sessionID,
			"statusCode": statusCode,
		},
	}})
}

func (h *UsageLogsActionHandler) detail(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("日志仓储未初始化"))
		return
	}
	id, ok := parseIntParam(c, "id")
	if !ok {
		return
	}
	log, err := h.store.GetByID(c.Request.Context(), id)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": log})
}

func (h *UsageLogsActionHandler) models(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("日志仓储未初始化"))
		return
	}
	options, err := h.store.GetFilterOptions(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": options.Models})
}

func (h *UsageLogsActionHandler) statusCodes(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("日志仓储未初始化"))
		return
	}
	options, err := h.store.GetFilterOptions(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": options.StatusCodes})
}

func (h *UsageLogsActionHandler) endpoints(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("日志仓储未初始化"))
		return
	}
	options, err := h.store.GetFilterOptions(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": options.Endpoints})
}

func (h *UsageLogsActionHandler) summary(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("日志仓储未初始化"))
		return
	}
	modelName := c.Query("model")
	endpoint := c.Query("endpoint")
	var statusCode *int
	if raw := c.Query("statusCode"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("statusCode 必须是整数"))
			return
		}
		statusCode = &value
	}
	summary, err := h.store.GetSummary(c.Request.Context(), modelName, endpoint, statusCode)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": summary})
}

func (h *UsageLogsActionHandler) filterOptions(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("日志仓储未初始化"))
		return
	}
	options, err := h.store.GetFilterOptions(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": options})
}

func (h *UsageLogsActionHandler) sessionIDSuggestions(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("日志仓储未初始化"))
		return
	}
	term := strings.TrimSpace(c.Query("term"))
	if len(term) < 2 {
		c.JSON(http.StatusOK, gin.H{"ok": true, "data": []string{}})
		return
	}
	limit := 20
	if raw := c.Query("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("limit 必须是整数"))
			return
		}
		limit = value
	}
	results, err := h.store.FindSessionIDSuggestions(c.Request.Context(), term, limit)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": results})
}

func (h *UsageLogsActionHandler) sessionIDSuggestionsAction(c *gin.Context) {
	var input struct {
		Term  string `json:"term"`
		Limit *int   `json:"limit"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	term := strings.TrimSpace(input.Term)
	if len(term) < 2 {
		c.JSON(http.StatusOK, gin.H{"ok": true, "data": []string{}})
		return
	}
	limit := 20
	if input.Limit != nil {
		limit = *input.Limit
	}
	results, err := h.store.FindSessionIDSuggestions(c.Request.Context(), term, limit)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": results})
}
