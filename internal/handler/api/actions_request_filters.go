package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	"github.com/gin-gonic/gin"
)

type requestFilterStore interface {
	List(ctx context.Context, opts *repository.ListOptions) ([]*model.RequestFilter, error)
	GetByID(ctx context.Context, id int) (*model.RequestFilter, error)
	Create(ctx context.Context, filter *model.RequestFilter) (*model.RequestFilter, error)
	Update(ctx context.Context, filter *model.RequestFilter) (*model.RequestFilter, error)
	Delete(ctx context.Context, id int) error
	RefreshCache(ctx context.Context) (*repository.CacheStats, error)
	GetCacheStats(ctx context.Context) (*repository.CacheStats, error)
}

type RequestFilterActionHandler struct {
	auth  adminAuthenticator
	store requestFilterStore
}

type requestFilterCreateInput struct {
	Name           string           `json:"name"`
	Description    *string          `json:"description"`
	Scope          string           `json:"scope"`
	Action         string           `json:"action"`
	MatchType      *string          `json:"matchType"`
	Target         string           `json:"target"`
	Replacement    any              `json:"replacement"`
	Priority       int              `json:"priority"`
	IsEnabled      *bool            `json:"isEnabled"`
	BindingType    string           `json:"bindingType"`
	ProviderIds    []int            `json:"providerIds"`
	GroupTags      []string         `json:"groupTags"`
	RuleMode       string           `json:"ruleMode"`
	ExecutionPhase string           `json:"executionPhase"`
	Operations     []map[string]any `json:"operations"`
}

func NewRequestFilterActionHandler(auth adminAuthenticator, store requestFilterStore) *RequestFilterActionHandler {
	return &RequestFilterActionHandler{auth: auth, store: store}
}

func (h *RequestFilterActionHandler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("/request-filters")
	protected.Use((&Handler{auth: h.auth}).AdminAuthMiddleware())
	protected.GET("", h.list)
	protected.GET("/:id", h.get)
	protected.POST("", h.create)
	protected.PUT("/:id", h.update)
	protected.DELETE("/:id", h.delete)

	protected.POST("/listRequestFilters", h.list)
	protected.POST("/createRequestFilterAction", h.create)
	protected.POST("/updateRequestFilterAction", h.updateFromBody)
	protected.POST("/deleteRequestFilterAction", h.deleteFromBody)
	protected.POST("/refreshRequestFiltersCache", h.refreshCache)
	protected.POST("/refreshCache", h.refreshCache)
	protected.POST("/getCacheStats", h.getCacheStats)
}

func (h *RequestFilterActionHandler) list(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("请求过滤器仓储未初始化"))
		return
	}
	items, err := h.store.List(c.Request.Context(), repository.NewListOptions())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": items})
}

func (h *RequestFilterActionHandler) get(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("请求过滤器仓储未初始化"))
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

func (h *RequestFilterActionHandler) create(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("请求过滤器仓储未初始化"))
		return
	}
	var input requestFilterCreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	filter, err := buildRequestFilterForCreate(input)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	created, err := h.store.Create(c.Request.Context(), filter)
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

func buildRequestFilterForCreate(input requestFilterCreateInput) (*model.RequestFilter, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, appErrors.NewInvalidRequest("name 不能为空")
	}

	scope := strings.TrimSpace(input.Scope)
	if scope == "" {
		scope = "body"
	}
	if !isAllowedString(scope, "header", "body") {
		return nil, appErrors.NewInvalidRequest("scope 仅支持 header/body")
	}

	action := strings.TrimSpace(input.Action)
	if action == "" {
		return nil, appErrors.NewInvalidRequest("action 不能为空")
	}
	if !isAllowedString(action, "remove", "set", "json_path") {
		return nil, appErrors.NewInvalidRequest("action 仅支持 remove/set/json_path")
	}

	ruleMode := strings.TrimSpace(input.RuleMode)
	if ruleMode == "" {
		ruleMode = "simple"
	}
	executionPhase := strings.TrimSpace(input.ExecutionPhase)
	if executionPhase == "" {
		executionPhase = "guard"
	}
	if ruleMode != "simple" || executionPhase != "guard" {
		return nil, appErrors.NewInvalidRequest("当前仅支持 simple + guard 请求过滤器")
	}

	target := strings.TrimSpace(input.Target)
	if ruleMode != "advanced" && target == "" {
		return nil, appErrors.NewInvalidRequest("target 不能为空")
	}

	bindingType := strings.TrimSpace(input.BindingType)
	if bindingType == "" {
		bindingType = "global"
	}
	if !isAllowedString(bindingType, "global", "providers", "groups") {
		return nil, appErrors.NewInvalidRequest("bindingType 无效")
	}

	providerIDs := trimPositiveInts(input.ProviderIds)
	groupTags := trimStrings(input.GroupTags)
	if bindingType == "providers" && len(providerIDs) == 0 {
		return nil, appErrors.NewInvalidRequest("bindingType=providers 时 providerIds 不能为空")
	}
	if bindingType == "groups" && len(groupTags) == 0 {
		return nil, appErrors.NewInvalidRequest("bindingType=groups 时 groupTags 不能为空")
	}
	if bindingType == "global" {
		providerIDs = nil
		groupTags = nil
	}
	if len(input.Operations) > 0 {
		return nil, appErrors.NewInvalidRequest("当前未启用 advanced operations")
	}

	isEnabled := true
	if input.IsEnabled != nil {
		isEnabled = *input.IsEnabled
	}

	return &model.RequestFilter{
		Name:           name,
		Description:    trimNullableString(input.Description),
		Scope:          scope,
		Action:         action,
		MatchType:      trimStringPtr(input.MatchType),
		Target:         target,
		Replacement:    input.Replacement,
		Priority:       input.Priority,
		IsEnabled:      isEnabled,
		BindingType:    bindingType,
		ProviderIds:    providerIDs,
		GroupTags:      groupTags,
		RuleMode:       ruleMode,
		ExecutionPhase: executionPhase,
		Operations:     nil,
	}, nil
}

func (h *RequestFilterActionHandler) update(c *gin.Context) {
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

func (h *RequestFilterActionHandler) updateFromBody(c *gin.Context) {
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

func (h *RequestFilterActionHandler) updateWithRaw(c *gin.Context, id int, raw map[string]json.RawMessage) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("请求过滤器仓储未初始化"))
		return
	}
	filter, err := h.store.GetByID(c.Request.Context(), id)
	if err != nil {
		writeAdminError(c, err)
		return
	}

	if value, present, err := decodeOptionalString(raw, "name"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("name 类型无效"))
			return
		}
		value = strings.TrimSpace(value)
		if value == "" {
			writeAdminError(c, appErrors.NewInvalidRequest("name 不能为空"))
			return
		}
		filter.Name = value
	}
	if value, present, err := decodeOptionalNullableString(raw, "description"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("description 类型无效"))
			return
		}
		filter.Description = trimNullableString(value)
	}
	if value, present, err := decodeOptionalString(raw, "scope"); present {
		if err != nil || !isAllowedString(strings.TrimSpace(value), "header", "body") {
			writeAdminError(c, appErrors.NewInvalidRequest("scope 仅支持 header/body"))
			return
		}
		filter.Scope = strings.TrimSpace(value)
	}
	if value, present, err := decodeOptionalString(raw, "action"); present {
		if err != nil || !isAllowedString(strings.TrimSpace(value), "remove", "set", "json_path") {
			writeAdminError(c, appErrors.NewInvalidRequest("action 仅支持 remove/set/json_path"))
			return
		}
		filter.Action = strings.TrimSpace(value)
	}
	if value, present, err := decodeOptionalNullableString(raw, "matchType"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("matchType 类型无效"))
			return
		}
		if value != nil {
			trimmed := strings.TrimSpace(*value)
			if trimmed != "" && !isAllowedString(trimmed, "regex", "contains", "exact") {
				writeAdminError(c, appErrors.NewInvalidRequest("matchType 无效"))
				return
			}
			value = &trimmed
		}
		filter.MatchType = value
	}
	if value, present, err := decodeOptionalString(raw, "target"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("target 类型无效"))
			return
		}
		filter.Target = strings.TrimSpace(value)
	}
	if value, present, err := decodeOptionalAny(raw, "replacement"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("replacement 类型无效"))
			return
		}
		filter.Replacement = value
	}
	if value, present, err := decodeOptionalInt(raw, "priority"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("priority 类型无效"))
			return
		}
		filter.Priority = value
	}
	if value, present, err := decodeOptionalBool(raw, "isEnabled"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("isEnabled 类型无效"))
			return
		}
		filter.IsEnabled = value
	}
	if value, present, err := decodeOptionalString(raw, "bindingType"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("bindingType 类型无效"))
			return
		}
		value = strings.TrimSpace(value)
		if !isAllowedString(value, "global", "providers", "groups") {
			writeAdminError(c, appErrors.NewInvalidRequest("bindingType 无效"))
			return
		}
		filter.BindingType = value
	}
	if value, present, err := decodeOptionalIntSlice(raw, "providerIds"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("providerIds 类型无效"))
			return
		}
		filter.ProviderIds = trimPositiveInts(value)
	}
	if value, present, err := decodeOptionalStringSlice(raw, "groupTags"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("groupTags 类型无效"))
			return
		}
		filter.GroupTags = trimStrings(value)
	}
	if value, present, err := decodeOptionalString(raw, "ruleMode"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("ruleMode 类型无效"))
			return
		}
		value = strings.TrimSpace(value)
		filter.RuleMode = value
	}
	if value, present, err := decodeOptionalString(raw, "executionPhase"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("executionPhase 类型无效"))
			return
		}
		value = strings.TrimSpace(value)
		filter.ExecutionPhase = value
	}
	if value, present, err := decodeOptionalOperations(raw, "operations"); present {
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest("operations 类型无效"))
			return
		}
		filter.Operations = value
	}

	if filter.RuleMode != "" && filter.RuleMode != "simple" {
		writeAdminError(c, appErrors.NewInvalidRequest("当前仅支持 simple 请求过滤器"))
		return
	}
	filter.RuleMode = "simple"
	if filter.RuleMode != "advanced" && strings.TrimSpace(filter.Target) == "" {
		writeAdminError(c, appErrors.NewInvalidRequest("target 不能为空"))
		return
	}
	if filter.ExecutionPhase != "" && filter.ExecutionPhase != "guard" {
		writeAdminError(c, appErrors.NewInvalidRequest("当前仅支持 guard 执行阶段"))
		return
	}
	filter.ExecutionPhase = "guard"
	if filter.BindingType == "providers" && len(trimPositiveInts(filter.ProviderIds)) == 0 {
		writeAdminError(c, appErrors.NewInvalidRequest("bindingType=providers 时 providerIds 不能为空"))
		return
	}
	if filter.BindingType == "groups" && len(trimStrings(filter.GroupTags)) == 0 {
		writeAdminError(c, appErrors.NewInvalidRequest("bindingType=groups 时 groupTags 不能为空"))
		return
	}
	if filter.BindingType == "global" {
		filter.ProviderIds = nil
		filter.GroupTags = nil
	}
	if len(filter.Operations) > 0 {
		writeAdminError(c, appErrors.NewInvalidRequest("当前未启用 advanced operations"))
		return
	}
	filter.Operations = nil

	updated, err := h.store.Update(c.Request.Context(), filter)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": updated})
}

func (h *RequestFilterActionHandler) delete(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("请求过滤器仓储未初始化"))
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

func (h *RequestFilterActionHandler) deleteFromBody(c *gin.Context) {
	raw, ok := bindRawJSONMap(c)
	if !ok {
		return
	}
	id, ok := parseBodyID(c, raw)
	if !ok {
		return
	}
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(id)}}
	h.delete(c)
}

func (h *RequestFilterActionHandler) refreshCache(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("请求过滤器仓储未初始化"))
		return
	}
	stats, err := h.store.RefreshCache(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"count": stats.ActiveCount, "stats": stats}})
}

func (h *RequestFilterActionHandler) getCacheStats(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("请求过滤器仓储未初始化"))
		return
	}
	stats, err := h.store.GetCacheStats(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": stats})
}

func isAllowedString(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func trimStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func trimNullableString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func trimStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	items := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func trimPositiveInts(values []int) []int {
	if len(values) == 0 {
		return nil
	}
	items := make([]int, 0, len(values))
	for _, value := range values {
		if value > 0 {
			items = append(items, value)
		}
	}
	if len(items) == 0 {
		return nil
	}
	return items
}
