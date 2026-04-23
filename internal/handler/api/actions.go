package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

const adminAuthResultContextKey = "admin_auth_result"

type adminAuthenticator interface {
	AuthenticateAdminToken(token string) (*authsvc.AuthResult, error)
}

type userLister interface {
	List(ctx context.Context, opts *repository.ListOptions) ([]*model.User, error)
	GetByID(ctx context.Context, id int) (*model.User, error)
	Create(ctx context.Context, user *model.User) (*model.User, error)
	Update(ctx context.Context, user *model.User) (*model.User, error)
	Delete(ctx context.Context, id int) error
}

type keyLister interface {
	List(ctx context.Context, opts *repository.ListOptions) ([]*model.Key, error)
	GetByID(ctx context.Context, id int) (*model.Key, error)
	Create(ctx context.Context, key *model.Key) (*model.Key, error)
	Update(ctx context.Context, key *model.Key) (*model.Key, error)
	Delete(ctx context.Context, id int) error
}

type providerLister interface {
	List(ctx context.Context, opts *repository.ListOptions) ([]*model.Provider, error)
	GetByID(ctx context.Context, id int) (*model.Provider, error)
	Create(ctx context.Context, provider *model.Provider) (*model.Provider, error)
	Update(ctx context.Context, provider *model.Provider) (*model.Provider, error)
	Delete(ctx context.Context, id int) error
}

type Handler struct {
	auth      adminAuthenticator
	users     userLister
	keys      keyLister
	providers providerLister
}

func NewHandler(auth adminAuthenticator, users userLister, keys keyLister, providers providerLister) *Handler {
	return &Handler{
		auth:      auth,
		users:     users,
		keys:      keys,
		providers: providers,
	}
}

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/openapi.json", h.openapiJSON)
	group.GET("/docs", h.docs)
	group.GET("/scalar", h.scalar)

	protected := group.Group("")
	protected.Use(h.AdminAuthMiddleware())
	// REST-like minimal compatibility
	protected.GET("/users", h.listUsers)
	protected.GET("/users/:id", h.getUser)
	protected.POST("/users", h.createUser)
	protected.PUT("/users/:id", h.updateUser)
	protected.DELETE("/users/:id", h.deleteUser)
	protected.GET("/keys", h.listKeys)
	protected.GET("/keys/:id", h.getKey)
	protected.POST("/keys", h.createKey)
	protected.PUT("/keys/:id", h.updateKey)
	protected.DELETE("/keys/:id", h.deleteKey)
	protected.GET("/providers", h.listProviders)
	protected.GET("/providers/:id", h.getProvider)
	protected.POST("/providers", h.createProvider)
	protected.PUT("/providers/:id", h.updateProvider)
	protected.DELETE("/providers/:id", h.deleteProvider)

	// Node action-style minimal compatibility aliases
	protected.POST("/users/getUsers", h.listUsers)
	protected.POST("/users/getUser", h.getUserFromBody)
	protected.POST("/users/addUser", h.createUser)
	protected.POST("/users/editUser", h.updateUserFromBody)
	protected.POST("/users/removeUser", h.deleteUserFromBody)

	protected.POST("/keys/getKeys", h.listKeys)
	protected.POST("/keys/getKey", h.getKeyFromBody)
	protected.POST("/keys/addKey", h.createKey)
	protected.POST("/keys/editKey", h.updateKeyFromBody)
	protected.POST("/keys/removeKey", h.deleteKeyFromBody)

	protected.POST("/providers/getProviders", h.listProviders)
	protected.POST("/providers/getProvider", h.getProviderFromBody)
	protected.POST("/providers/addProvider", h.createProvider)
	protected.POST("/providers/editProvider", h.updateProviderFromBody)
	protected.POST("/providers/removeProvider", h.deleteProviderFromBody)
}

func (h *Handler) AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.auth == nil {
			writeAdminError(c, appErrors.NewInternalError("管理鉴权服务未初始化"))
			return
		}

		authResult, err := h.auth.AuthenticateAdminToken(resolveAdminToken(c))
		if err != nil {
			writeAdminError(c, err)
			return
		}

		c.Set(adminAuthResultContextKey, authResult)
		c.Next()
	}
}

func (h *Handler) listUsers(c *gin.Context) {
	if h == nil || h.users == nil {
		writeAdminError(c, appErrors.NewInternalError("用户仓储未初始化"))
		return
	}
	users, err := h.users.List(c.Request.Context(), repository.NewListOptions())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": users})
}

func (h *Handler) getUser(c *gin.Context) {
	if h == nil || h.users == nil {
		writeAdminError(c, appErrors.NewInternalError("用户仓储未初始化"))
		return
	}
	id, ok := parseIntParam(c, "id")
	if !ok {
		return
	}
	user, err := h.users.GetByID(c.Request.Context(), id)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": user})
}

func (h *Handler) getUserFromBody(c *gin.Context) {
	var input struct {
		ID int `json:"id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(input.ID)}}
	h.getUser(c)
}

func (h *Handler) createUser(c *gin.Context) {
	if h == nil || h.users == nil {
		writeAdminError(c, appErrors.NewInternalError("用户仓储未初始化"))
		return
	}
	var input struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
		Role        string  `json:"role"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		writeAdminError(c, appErrors.NewInvalidRequest("name 不能为空"))
		return
	}
	user, err := h.users.Create(c.Request.Context(), &model.User{
		Name:        input.Name,
		Description: input.Description,
		Role:        strings.TrimSpace(input.Role),
	})
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true, "data": user})
}

func (h *Handler) updateUser(c *gin.Context) {
	if h == nil || h.users == nil {
		writeAdminError(c, appErrors.NewInternalError("用户仓储未初始化"))
		return
	}
	id, ok := parseIntParam(c, "id")
	if !ok {
		return
	}
	var input struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Role        *string `json:"role"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	user := &model.User{ID: id}
	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if trimmed == "" {
			writeAdminError(c, appErrors.NewInvalidRequest("name 不能为空"))
			return
		}
		user.Name = trimmed
	}
	if input.Description != nil {
		user.Description = input.Description
	}
	if input.Role != nil {
		user.Role = strings.TrimSpace(*input.Role)
	}
	updated, err := h.users.Update(c.Request.Context(), user)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": updated})
}

func (h *Handler) updateUserFromBody(c *gin.Context) {
	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	idValue, ok := raw["id"].(float64)
	if !ok {
		writeAdminError(c, appErrors.NewInvalidRequest("id 为必填字段"))
		return
	}
	rawBytes, _ := json.Marshal(raw)
	c.Request.Body = io.NopCloser(bytes.NewReader(rawBytes))
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(idValue))}}
	h.updateUser(c)
}

func (h *Handler) deleteUser(c *gin.Context) {
	if h == nil || h.users == nil {
		writeAdminError(c, appErrors.NewInternalError("用户仓储未初始化"))
		return
	}
	id, ok := parseIntParam(c, "id")
	if !ok {
		return
	}
	if err := h.users.Delete(c.Request.Context(), id); err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"id": id, "deleted": true}})
}

func (h *Handler) deleteUserFromBody(c *gin.Context) {
	var input struct {
		ID int `json:"id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(input.ID)}}
	h.deleteUser(c)
}

func (h *Handler) listKeys(c *gin.Context) {
	if h == nil || h.keys == nil {
		writeAdminError(c, appErrors.NewInternalError("密钥仓储未初始化"))
		return
	}
	keys, err := h.keys.List(c.Request.Context(), repository.NewListOptions())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": keys})
}

func (h *Handler) getKey(c *gin.Context) {
	if h == nil || h.keys == nil {
		writeAdminError(c, appErrors.NewInternalError("密钥仓储未初始化"))
		return
	}
	id, ok := parseIntParam(c, "id")
	if !ok {
		return
	}
	key, err := h.keys.GetByID(c.Request.Context(), id)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": key})
}

func (h *Handler) getKeyFromBody(c *gin.Context) {
	var input struct {
		ID int `json:"id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(input.ID)}}
	h.getKey(c)
}

func (h *Handler) createKey(c *gin.Context) {
	if h == nil || h.keys == nil {
		writeAdminError(c, appErrors.NewInternalError("密钥仓储未初始化"))
		return
	}
	var input struct {
		UserID int    `json:"userId"`
		Key    string `json:"key"`
		Name   string `json:"name"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	input.Key = strings.TrimSpace(input.Key)
	input.Name = strings.TrimSpace(input.Name)
	if input.UserID <= 0 || input.Key == "" || input.Name == "" {
		writeAdminError(c, appErrors.NewInvalidRequest("userId、key、name 为必填字段"))
		return
	}
	key, err := h.keys.Create(c.Request.Context(), &model.Key{
		UserID: input.UserID,
		Key:    input.Key,
		Name:   input.Name,
	})
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true, "data": key})
}

func (h *Handler) updateKey(c *gin.Context) {
	if h == nil || h.keys == nil {
		writeAdminError(c, appErrors.NewInternalError("密钥仓储未初始化"))
		return
	}
	id, ok := parseIntParam(c, "id")
	if !ok {
		return
	}
	var input struct {
		Name *string `json:"name"`
		Key  *string `json:"key"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	key := &model.Key{ID: id}
	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if trimmed == "" {
			writeAdminError(c, appErrors.NewInvalidRequest("name 不能为空"))
			return
		}
		key.Name = trimmed
	}
	if input.Key != nil {
		trimmed := strings.TrimSpace(*input.Key)
		if trimmed == "" {
			writeAdminError(c, appErrors.NewInvalidRequest("key 不能为空"))
			return
		}
		key.Key = trimmed
	}
	updated, err := h.keys.Update(c.Request.Context(), key)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": updated})
}

func (h *Handler) updateKeyFromBody(c *gin.Context) {
	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	idValue, ok := raw["id"].(float64)
	if !ok {
		writeAdminError(c, appErrors.NewInvalidRequest("id 为必填字段"))
		return
	}
	rawBytes, _ := json.Marshal(raw)
	c.Request.Body = io.NopCloser(bytes.NewReader(rawBytes))
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(idValue))}}
	h.updateKey(c)
}

func (h *Handler) deleteKey(c *gin.Context) {
	if h == nil || h.keys == nil {
		writeAdminError(c, appErrors.NewInternalError("密钥仓储未初始化"))
		return
	}
	id, ok := parseIntParam(c, "id")
	if !ok {
		return
	}
	if err := h.keys.Delete(c.Request.Context(), id); err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"id": id, "deleted": true}})
}

func (h *Handler) deleteKeyFromBody(c *gin.Context) {
	var input struct {
		ID int `json:"id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(input.ID)}}
	h.deleteKey(c)
}

func (h *Handler) listProviders(c *gin.Context) {
	if h == nil || h.providers == nil {
		writeAdminError(c, appErrors.NewInternalError("供应商仓储未初始化"))
		return
	}
	providers, err := h.providers.List(c.Request.Context(), repository.NewListOptions())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": providers})
}

func (h *Handler) getProvider(c *gin.Context) {
	if h == nil || h.providers == nil {
		writeAdminError(c, appErrors.NewInternalError("供应商仓储未初始化"))
		return
	}
	id, ok := parseIntParam(c, "id")
	if !ok {
		return
	}
	provider, err := h.providers.GetByID(c.Request.Context(), id)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": provider})
}

func (h *Handler) getProviderFromBody(c *gin.Context) {
	var input struct {
		ID int `json:"id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(input.ID)}}
	h.getProvider(c)
}

func (h *Handler) createProvider(c *gin.Context) {
	if h == nil || h.providers == nil {
		writeAdminError(c, appErrors.NewInternalError("供应商仓储未初始化"))
		return
	}
	var input struct {
		Name         string `json:"name"`
		URL          string `json:"url"`
		Key          string `json:"key"`
		ProviderType string `json:"providerType"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.URL = strings.TrimSpace(input.URL)
	input.Key = strings.TrimSpace(input.Key)
	input.ProviderType = strings.TrimSpace(input.ProviderType)
	if input.Name == "" || input.URL == "" || input.Key == "" {
		writeAdminError(c, appErrors.NewInvalidRequest("name、url、key 为必填字段"))
		return
	}
	provider, err := h.providers.Create(c.Request.Context(), &model.Provider{
		Name:         input.Name,
		URL:          input.URL,
		Key:          input.Key,
		ProviderType: input.ProviderType,
	})
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true, "data": provider})
}

func (h *Handler) updateProvider(c *gin.Context) {
	if h == nil || h.providers == nil {
		writeAdminError(c, appErrors.NewInternalError("供应商仓储未初始化"))
		return
	}
	id, ok := parseIntParam(c, "id")
	if !ok {
		return
	}
	var input struct {
		Name         *string `json:"name"`
		URL          *string `json:"url"`
		Key          *string `json:"key"`
		ProviderType *string `json:"providerType"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	provider := &model.Provider{ID: id}
	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if trimmed == "" {
			writeAdminError(c, appErrors.NewInvalidRequest("name 不能为空"))
			return
		}
		provider.Name = trimmed
	}
	if input.URL != nil {
		trimmed := strings.TrimSpace(*input.URL)
		if trimmed == "" {
			writeAdminError(c, appErrors.NewInvalidRequest("url 不能为空"))
			return
		}
		provider.URL = trimmed
	}
	if input.Key != nil {
		trimmed := strings.TrimSpace(*input.Key)
		if trimmed == "" {
			writeAdminError(c, appErrors.NewInvalidRequest("key 不能为空"))
			return
		}
		provider.Key = trimmed
	}
	if input.ProviderType != nil {
		provider.ProviderType = strings.TrimSpace(*input.ProviderType)
	}
	updated, err := h.providers.Update(c.Request.Context(), provider)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": updated})
}

func (h *Handler) updateProviderFromBody(c *gin.Context) {
	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	idValue, ok := raw["id"].(float64)
	if !ok {
		writeAdminError(c, appErrors.NewInvalidRequest("id 为必填字段"))
		return
	}
	rawBytes, _ := json.Marshal(raw)
	c.Request.Body = io.NopCloser(bytes.NewReader(rawBytes))
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(idValue))}}
	h.updateProvider(c)
}

func (h *Handler) deleteProvider(c *gin.Context) {
	if h == nil || h.providers == nil {
		writeAdminError(c, appErrors.NewInternalError("供应商仓储未初始化"))
		return
	}
	id, ok := parseIntParam(c, "id")
	if !ok {
		return
	}
	if err := h.providers.Delete(c.Request.Context(), id); err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"id": id, "deleted": true}})
}

func (h *Handler) deleteProviderFromBody(c *gin.Context) {
	var input struct {
		ID int `json:"id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(input.ID)}}
	h.deleteProvider(c)
}

func (h *Handler) openapiJSON(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"openapi": "3.1.0",
		"info": gin.H{
			"title":   "Claude Code Hub Go Rewrite Actions API",
			"version": "0.1.0",
		},
		"paths": gin.H{
			"/api/actions/users":                                         gin.H{"get": gin.H{"summary": "List users"}, "post": gin.H{"summary": "Create user"}},
			"/api/actions/users/{id}":                                    gin.H{"get": gin.H{"summary": "Get user"}, "put": gin.H{"summary": "Update user"}, "delete": gin.H{"summary": "Delete user"}},
			"/api/actions/keys":                                          gin.H{"get": gin.H{"summary": "List keys"}, "post": gin.H{"summary": "Create key"}},
			"/api/actions/keys/{id}":                                     gin.H{"get": gin.H{"summary": "Get key"}, "put": gin.H{"summary": "Update key"}, "delete": gin.H{"summary": "Delete key"}},
			"/api/actions/providers":                                     gin.H{"get": gin.H{"summary": "List providers"}, "post": gin.H{"summary": "Create provider"}},
			"/api/actions/providers/{id}":                                gin.H{"get": gin.H{"summary": "Get provider"}, "put": gin.H{"summary": "Update provider"}, "delete": gin.H{"summary": "Delete provider"}},
			"/api/actions/system-settings":                               gin.H{"get": gin.H{"summary": "Get system settings"}, "put": gin.H{"summary": "Update system settings"}},
			"/api/actions/system-settings/fetchSystemSettings":           gin.H{"post": gin.H{"summary": "Fetch system settings"}},
			"/api/actions/system-settings/saveSystemSettings":            gin.H{"post": gin.H{"summary": "Save system settings"}},
			"/api/actions/usage-logs":                                    gin.H{"get": gin.H{"summary": "List usage logs"}},
			"/api/actions/usage-logs/getUsageLogs":                       gin.H{"post": gin.H{"summary": "Get usage logs"}},
			"/api/actions/usage-logs/getModelList":                       gin.H{"post": gin.H{"summary": "Get usage log model list"}},
			"/api/actions/usage-logs/getStatusCodeList":                  gin.H{"post": gin.H{"summary": "Get usage log status code list"}},
			"/api/actions/usage-logs/getEndpointList":                    gin.H{"post": gin.H{"summary": "Get usage log endpoint list"}},
			"/api/actions/usage-logs/getUsageLogsStats":                  gin.H{"post": gin.H{"summary": "Get usage log summary"}},
			"/api/actions/usage-logs/getFilterOptions":                   gin.H{"post": gin.H{"summary": "Get usage log filter options"}},
			"/api/actions/usage-logs/getUsageLogSessionIdSuggestions":    gin.H{"post": gin.H{"summary": "Get usage log session id suggestions"}},
			"/api/actions/usage-logs/{id}":                               gin.H{"get": gin.H{"summary": "Get usage log detail"}},
			"/api/actions/usage-logs/summary":                            gin.H{"get": gin.H{"summary": "Get usage logs summary"}},
			"/api/actions/usage-logs/filter-options":                     gin.H{"get": gin.H{"summary": "Get usage logs filter options"}},
			"/api/actions/usage-logs/session-id-suggestions":             gin.H{"get": gin.H{"summary": "Get usage logs session id suggestions"}},
			"/api/actions/usage-logs/models":                             gin.H{"get": gin.H{"summary": "Get usage log model options"}},
			"/api/actions/usage-logs/status-codes":                       gin.H{"get": gin.H{"summary": "Get usage log status code options"}},
			"/api/actions/usage-logs/endpoints":                          gin.H{"get": gin.H{"summary": "Get usage log endpoint options"}},
			"/api/actions/session-origin-chain":                          gin.H{"get": gin.H{"summary": "Get session origin chain"}},
			"/api/actions/session-origin-chain/getSessionOriginChain":    gin.H{"post": gin.H{"summary": "Get session origin chain"}},
			"/api/actions/model-prices":                                  gin.H{"post": gin.H{"summary": "Model prices actions"}},
			"/api/actions/model-prices/getModelPrices":                   gin.H{"post": gin.H{"summary": "Get model prices"}},
			"/api/actions/model-prices/getModelPricesPaginated":          gin.H{"post": gin.H{"summary": "Get paginated model prices"}},
			"/api/actions/model-prices/hasPriceTable":                    gin.H{"post": gin.H{"summary": "Check price table existence"}},
			"/api/actions/model-prices/getAvailableModelsByProviderType": gin.H{"post": gin.H{"summary": "Get available models by provider type"}},
			"/api/actions/statistics/getUserStatistics":                  gin.H{"post": gin.H{"summary": "Get user statistics"}},
			"/api/actions/overview/getOverviewData":                      gin.H{"post": gin.H{"summary": "Get overview data"}},
			"/api/actions/proxy-status/getProxyStatus":                   gin.H{"post": gin.H{"summary": "Get proxy status"}},
			"/api/actions/provider-slots/getProviderSlots":               gin.H{"post": gin.H{"summary": "Get provider slots"}},
			"/api/actions/dashboard-realtime/getDashboardRealtimeData":   gin.H{"post": gin.H{"summary": "Get dashboard realtime data"}},
		},
	})
}

func (h *Handler) docs(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, `<html><body><h1>Swagger Docs Placeholder</h1><p>OpenAPI JSON: <a href="/api/actions/openapi.json">/api/actions/openapi.json</a></p></body></html>`)
}

func (h *Handler) scalar(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, `<html><body><h1>Scalar Docs Placeholder</h1><p>OpenAPI JSON: <a href="/api/actions/openapi.json">/api/actions/openapi.json</a></p></body></html>`)
}

func resolveAdminToken(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if token := strings.TrimSpace(c.GetHeader("x-api-key")); token != "" {
		return token
	}
	authorization := strings.TrimSpace(c.GetHeader("Authorization"))
	parts := strings.Fields(authorization)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return authorization
}

func writeAdminError(c *gin.Context, err error) {
	var appErr *appErrors.AppError
	if ok := errorsAs(err, &appErr); ok {
		c.AbortWithStatusJSON(appErr.HTTPStatus, gin.H{"ok": false, "error": appErr.ToResponse().Error})
		return
	}

	fallback := appErrors.NewInternalError("管理接口请求失败")
	c.AbortWithStatusJSON(fallback.HTTPStatus, gin.H{"ok": false, "error": fallback.ToResponse().Error})
}

func errorsAs(err error, target **appErrors.AppError) bool {
	if err == nil {
		return false
	}
	if appErr, ok := err.(*appErrors.AppError); ok {
		*target = appErr
		return true
	}
	return false
}

func parseIntParam(c *gin.Context, name string) (int, bool) {
	if c == nil {
		return 0, false
	}
	value := strings.TrimSpace(c.Param(name))
	id, err := strconv.Atoi(value)
	if err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("无效的 ID 参数"))
		return 0, false
	}
	return id, true
}
