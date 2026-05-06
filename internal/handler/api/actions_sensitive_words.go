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

type sensitiveWordStore interface {
	List(ctx context.Context, opts *repository.ListOptions) ([]*model.SensitiveWord, error)
	GetByID(ctx context.Context, id int) (*model.SensitiveWord, error)
	Create(ctx context.Context, word *model.SensitiveWord) (*model.SensitiveWord, error)
	Update(ctx context.Context, word *model.SensitiveWord) (*model.SensitiveWord, error)
	Delete(ctx context.Context, id int) error
	RefreshCache(ctx context.Context) (*repository.CacheStats, error)
	GetCacheStats(ctx context.Context) (*repository.CacheStats, error)
}

type SensitiveWordActionHandler struct {
	auth  adminAuthenticator
	store sensitiveWordStore
}

func NewSensitiveWordActionHandler(auth adminAuthenticator, store sensitiveWordStore) *SensitiveWordActionHandler {
	return &SensitiveWordActionHandler{auth: auth, store: store}
}

func (h *SensitiveWordActionHandler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("/sensitive-words")
	protected.Use((&Handler{auth: h.auth}).AdminAuthMiddleware())
	protected.GET("", h.list)
	protected.GET("/:id", h.get)
	protected.POST("", h.create)
	protected.PUT("/:id", h.update)
	protected.DELETE("/:id", h.delete)

	protected.POST("/listSensitiveWords", h.list)
	protected.POST("/createSensitiveWordAction", h.create)
	protected.POST("/updateSensitiveWordAction", h.updateFromBody)
	protected.POST("/deleteSensitiveWordAction", h.deleteFromBody)
	protected.POST("/refreshCacheAction", h.refreshCache)
	protected.POST("/getCacheStats", h.getCacheStats)
}

func (h *SensitiveWordActionHandler) list(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("敏感词仓储未初始化"))
		return
	}
	items, err := h.store.List(c.Request.Context(), repository.NewListOptions())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": items})
}

func (h *SensitiveWordActionHandler) get(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("敏感词仓储未初始化"))
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

func (h *SensitiveWordActionHandler) create(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("敏感词仓储未初始化"))
		return
	}
	var input struct {
		Word        string  `json:"word"`
		MatchType   string  `json:"matchType"`
		Description *string `json:"description"`
		IsEnabled   *bool   `json:"isEnabled"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}

	word := strings.TrimSpace(input.Word)
	if word == "" {
		writeAdminError(c, appErrors.NewInvalidRequest("word 不能为空"))
		return
	}
	matchType := strings.TrimSpace(input.MatchType)
	if matchType == "" {
		matchType = "contains"
	}
	if !isAllowedString(matchType, "contains", "exact", "regex") {
		writeAdminError(c, appErrors.NewInvalidRequest("matchType 无效"))
		return
	}
	if matchType == "regex" {
		if _, err := regexp.Compile(word); err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("word 不是合法正则表达式"))
			return
		}
	}

	isEnabled := true
	if input.IsEnabled != nil {
		isEnabled = *input.IsEnabled
	}
	created, err := h.store.Create(c.Request.Context(), &model.SensitiveWord{
		Word:        word,
		MatchType:   matchType,
		Description: trimNullableString(input.Description),
		IsEnabled:   isEnabled,
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

func (h *SensitiveWordActionHandler) update(c *gin.Context) {
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

func (h *SensitiveWordActionHandler) updateFromBody(c *gin.Context) {
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

func (h *SensitiveWordActionHandler) updateWithRaw(c *gin.Context, id int, raw map[string]json.RawMessage) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("敏感词仓储未初始化"))
		return
	}
	word, err := h.store.GetByID(c.Request.Context(), id)
	if err != nil {
		writeAdminError(c, err)
		return
	}

	if value, present, err := decodeOptionalString(raw, "word"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("word 类型无效"))
			return
		}
		value = strings.TrimSpace(value)
		if value == "" {
			writeAdminError(c, appErrors.NewInvalidRequest("word 不能为空"))
			return
		}
		word.Word = value
	}
	if value, present, err := decodeOptionalString(raw, "matchType"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("matchType 类型无效"))
			return
		}
		value = strings.TrimSpace(value)
		if !isAllowedString(value, "contains", "exact", "regex") {
			writeAdminError(c, appErrors.NewInvalidRequest("matchType 无效"))
			return
		}
		word.MatchType = value
	}
	if value, present, err := decodeOptionalNullableString(raw, "description"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("description 类型无效"))
			return
		}
		word.Description = trimNullableString(value)
	}
	if value, present, err := decodeOptionalBool(raw, "isEnabled"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("isEnabled 类型无效"))
			return
		}
		word.IsEnabled = value
	}
	if word.MatchType == "regex" {
		if _, err := regexp.Compile(strings.TrimSpace(word.Word)); err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("word 不是合法正则表达式"))
			return
		}
	}

	updated, err := h.store.Update(c.Request.Context(), word)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": updated})
}

func (h *SensitiveWordActionHandler) delete(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("敏感词仓储未初始化"))
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

func (h *SensitiveWordActionHandler) deleteFromBody(c *gin.Context) {
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

func (h *SensitiveWordActionHandler) refreshCache(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("敏感词仓储未初始化"))
		return
	}
	stats, err := h.store.RefreshCache(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"stats": stats}})
}

func (h *SensitiveWordActionHandler) getCacheStats(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("敏感词仓储未初始化"))
		return
	}
	stats, err := h.store.GetCacheStats(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": stats})
}
