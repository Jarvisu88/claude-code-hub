package api

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	"github.com/gin-gonic/gin"
)

type errorRuleStore interface {
	List(ctx context.Context, opts *repository.ListOptions) ([]*model.ErrorRule, error)
	GetByID(ctx context.Context, id int) (*model.ErrorRule, error)
	Create(ctx context.Context, rule *model.ErrorRule) (*model.ErrorRule, error)
	Update(ctx context.Context, rule *model.ErrorRule) (*model.ErrorRule, error)
	Delete(ctx context.Context, id int) error
	RefreshCache(ctx context.Context) (*repository.CacheStats, error)
	GetCacheStats(ctx context.Context) (*repository.CacheStats, error)
}

type ErrorRuleActionHandler struct {
	auth  adminAuthenticator
	store errorRuleStore
}

func NewErrorRuleActionHandler(auth adminAuthenticator, store errorRuleStore) *ErrorRuleActionHandler {
	return &ErrorRuleActionHandler{auth: auth, store: store}
}

func (h *ErrorRuleActionHandler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("/error-rules")
	protected.Use((&Handler{auth: h.auth}).AdminAuthMiddleware())
	protected.GET("", h.list)
	protected.GET("/:id", h.get)
	protected.POST("", h.create)
	protected.PUT("/:id", h.update)
	protected.DELETE("/:id", h.delete)

	protected.POST("/listErrorRules", h.list)
	protected.POST("/createErrorRuleAction", h.create)
	protected.POST("/updateErrorRuleAction", h.updateFromBody)
	protected.POST("/deleteErrorRuleAction", h.deleteFromBody)
	protected.POST("/refreshCacheAction", h.refreshCache)
	protected.POST("/getCacheStats", h.getCacheStats)
}

func (h *ErrorRuleActionHandler) list(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("错误规则仓储未初始化"))
		return
	}
	items, err := h.store.List(c.Request.Context(), repository.NewListOptions())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": items})
}

func (h *ErrorRuleActionHandler) get(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("错误规则仓储未初始化"))
		return
	}
	id, ok := parseIntParam(c, "id")
	if !ok {
		return
	}
	item, err := h.store.GetByID(c.Request.Context(), id)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": item})
}

func (h *ErrorRuleActionHandler) create(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("错误规则仓储未初始化"))
		return
	}
	var input struct {
		Pattern            string         `json:"pattern"`
		MatchType          string         `json:"matchType"`
		Category           string         `json:"category"`
		Description        *string        `json:"description"`
		OverrideResponse   map[string]any `json:"overrideResponse"`
		OverrideStatusCode *int           `json:"overrideStatusCode"`
		IsEnabled          *bool          `json:"isEnabled"`
		IsDefault          *bool          `json:"isDefault"`
		Priority           int            `json:"priority"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}

	pattern := strings.TrimSpace(input.Pattern)
	if pattern == "" {
		writeAdminError(c, appErrors.NewInvalidRequest("pattern 不能为空"))
		return
	}
	category := strings.TrimSpace(input.Category)
	if category == "" {
		writeAdminError(c, appErrors.NewInvalidRequest("category 不能为空"))
		return
	}
	matchType := strings.TrimSpace(input.MatchType)
	if matchType == "" {
		matchType = "regex"
	}
	if !isAllowedString(matchType, "regex", "contains", "exact") {
		writeAdminError(c, appErrors.NewInvalidRequest("matchType 无效"))
		return
	}
	if matchType == "regex" {
		if _, err := regexp.Compile(pattern); err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("pattern 不是合法正则表达式"))
			return
		}
	}
	if input.OverrideStatusCode != nil && (*input.OverrideStatusCode < 400 || *input.OverrideStatusCode > 599) {
		writeAdminError(c, appErrors.NewInvalidRequest("overrideStatusCode 必须在 400-599 之间"))
		return
	}

	isEnabled := true
	if input.IsEnabled != nil {
		isEnabled = *input.IsEnabled
	}
	isDefault := false
	if input.IsDefault != nil {
		isDefault = *input.IsDefault
	}
	created, err := h.store.Create(c.Request.Context(), &model.ErrorRule{
		Pattern:            pattern,
		MatchType:          matchType,
		Category:           category,
		Description:        trimNullableString(input.Description),
		OverrideResponse:   input.OverrideResponse,
		OverrideStatusCode: input.OverrideStatusCode,
		IsEnabled:          isEnabled,
		IsDefault:          isDefault,
		Priority:           input.Priority,
	})
	if err != nil {
		writeAdminError(c, err)
		return
	}
	status := http.StatusCreated
	if strings.Contains(c.FullPath(), "Action") {
		status = http.StatusOK
	}
	c.JSON(status, gin.H{"ok": true, "data": created})
}

func (h *ErrorRuleActionHandler) update(c *gin.Context) {
	id, ok := parseIntParam(c, "id")
	if !ok {
		return
	}
	raw, ok := bindRawJSONMap(c)
	if !ok {
		return
	}
	h.updateWithRaw(c, id, raw)
}

func (h *ErrorRuleActionHandler) updateFromBody(c *gin.Context) {
	raw, ok := bindRawJSONMap(c)
	if !ok {
		return
	}
	id, ok := parseBodyID(c, raw)
	if !ok {
		return
	}
	h.updateWithRaw(c, id, raw)
}

func (h *ErrorRuleActionHandler) updateWithRaw(c *gin.Context, id int, raw map[string]json.RawMessage) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("错误规则仓储未初始化"))
		return
	}
	rule, err := h.store.GetByID(c.Request.Context(), id)
	if err != nil {
		writeAdminError(c, err)
		return
	}

	if value, present, err := decodeOptionalString(raw, "pattern"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("pattern 类型无效"))
			return
		}
		value = strings.TrimSpace(value)
		if value == "" {
			writeAdminError(c, appErrors.NewInvalidRequest("pattern 不能为空"))
			return
		}
		rule.Pattern = value
	}
	if value, present, err := decodeOptionalString(raw, "matchType"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("matchType 类型无效"))
			return
		}
		value = strings.TrimSpace(value)
		if !isAllowedString(value, "regex", "contains", "exact") {
			writeAdminError(c, appErrors.NewInvalidRequest("matchType 无效"))
			return
		}
		rule.MatchType = value
	}
	if value, present, err := decodeOptionalString(raw, "category"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("category 类型无效"))
			return
		}
		value = strings.TrimSpace(value)
		if value == "" {
			writeAdminError(c, appErrors.NewInvalidRequest("category 不能为空"))
			return
		}
		rule.Category = value
	}
	if value, present, err := decodeOptionalNullableString(raw, "description"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("description 类型无效"))
			return
		}
		rule.Description = trimNullableString(value)
	}
	if value, present, err := decodeOptionalMap(raw, "overrideResponse"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("overrideResponse 类型无效"))
			return
		}
		rule.OverrideResponse = value
	}
	if value, present, err := decodeOptionalIntPointer(raw, "overrideStatusCode"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("overrideStatusCode 类型无效"))
			return
		}
		if value != nil && (*value < 400 || *value > 599) {
			writeAdminError(c, appErrors.NewInvalidRequest("overrideStatusCode 必须在 400-599 之间"))
			return
		}
		rule.OverrideStatusCode = value
	}
	if value, present, err := decodeOptionalBool(raw, "isEnabled"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("isEnabled 类型无效"))
			return
		}
		rule.IsEnabled = value
	}
	if value, present, err := decodeOptionalBool(raw, "isDefault"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("isDefault 类型无效"))
			return
		}
		rule.IsDefault = value
	}
	if value, present, err := decodeOptionalInt(raw, "priority"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("priority 类型无效"))
			return
		}
		rule.Priority = value
	}
	if rule.MatchType == "regex" {
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("pattern 不是合法正则表达式"))
			return
		}
	}

	updated, err := h.store.Update(c.Request.Context(), rule)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": updated})
}

func (h *ErrorRuleActionHandler) delete(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("错误规则仓储未初始化"))
		return
	}
	id, ok := parseIntParam(c, "id")
	if !ok {
		return
	}
	if err := h.store.Delete(c.Request.Context(), id); err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"id": id, "deleted": true}})
}

func (h *ErrorRuleActionHandler) deleteFromBody(c *gin.Context) {
	raw, ok := bindRawJSONMap(c)
	if !ok {
		return
	}
	id, ok := parseBodyID(c, raw)
	if !ok {
		return
	}
	if err := h.store.Delete(c.Request.Context(), id); err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"id": id, "deleted": true}})
}

func (h *ErrorRuleActionHandler) refreshCache(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("错误规则仓储未初始化"))
		return
	}
	stats, err := h.store.RefreshCache(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{
		"stats": stats,
		"syncResult": gin.H{
			"inserted": 0,
			"updated":  0,
			"skipped":  0,
			"deleted":  0,
		},
	}})
}

func (h *ErrorRuleActionHandler) getCacheStats(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("错误规则仓储未初始化"))
		return
	}
	stats, err := h.store.GetCacheStats(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": stats})
}
