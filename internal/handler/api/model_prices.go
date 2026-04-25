package api

import (
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	"github.com/gin-gonic/gin"
	toml "github.com/pelletier/go-toml/v2"
)

type modelPriceStore interface {
	ListAllLatestPrices(ctx context.Context) ([]*model.ModelPrice, error)
	ListAllLatestPricesPaginated(ctx context.Context, page, pageSize int, search, source, litellmProvider string) (*repository.PaginatedPrices, error)
	HasAnyRecords(ctx context.Context) (bool, error)
	GetAllModelNames(ctx context.Context) ([]string, error)
	GetChatModelNames(ctx context.Context) ([]string, error)
}

type ModelPricesActionHandler struct {
	auth  adminAuthenticator
	store modelPriceStore
}

var modelPricesHTTPClient = &http.Client{Timeout: 10 * time.Second}

func NewModelPricesActionHandler(auth adminAuthenticator, store modelPriceStore) *ModelPricesActionHandler {
	return &ModelPricesActionHandler{auth: auth, store: store}
}

func (h *ModelPricesActionHandler) RegisterActionRoutes(group *gin.RouterGroup) {
	protected := group.Group("/model-prices")
	protected.Use((&Handler{auth: h.auth}).AdminAuthMiddleware())
	protected.GET("", h.list)
	protected.POST("/getModelPrices", h.list)
	protected.POST("/hasPriceTable", h.hasPriceTable)
	protected.POST("/getAvailableModelsByProviderType", h.availableModelsByProviderType)
	protected.POST("/getModelPricesPaginated", h.paginatedAction)
}

func (h *ModelPricesActionHandler) RegisterDirectRoutes(group *gin.RouterGroup) {
	protected := group.Group("")
	protected.Use((&Handler{auth: h.auth}).AdminAuthMiddleware())
	protected.GET("", h.directList)
	protected.GET("/cloud-model-count", h.cloudModelCount)
}

func (h *ModelPricesActionHandler) list(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("价格仓储未初始化"))
		return
	}
	prices, err := h.store.ListAllLatestPrices(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": prices})
}

func (h *ModelPricesActionHandler) directList(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("价格仓储未初始化"))
		return
	}

	page := 1
	pageSize := 50
	search := c.Query("search")
	source := c.Query("source")
	litellmProvider := c.Query("litellmProvider")
	if raw := c.Query("page"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			writeAdminError(c, appErrors.NewInvalidRequest("页码必须大于0"))
			return
		}
		page = value
	}
	if raw := c.Query("pageSize"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 200 {
			writeAdminError(c, appErrors.NewInvalidRequest("每页大小必须在1-200之间"))
			return
		}
		pageSize = value
	}
	if source != "" && source != "manual" && source != "litellm" {
		writeAdminError(c, appErrors.NewInvalidRequest("source 参数无效"))
		return
	}
	result, err := h.store.ListAllLatestPricesPaginated(c.Request.Context(), page, pageSize, search, source, litellmProvider)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": result})
}

func (h *ModelPricesActionHandler) paginatedAction(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("价格仓储未初始化"))
		return
	}

	var input struct {
		Page            int    `json:"page"`
		PageSize        int    `json:"pageSize"`
		Search          string `json:"search"`
		Source          string `json:"source"`
		LitellmProvider string `json:"litellmProvider"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}
	if input.Page < 1 {
		input.Page = 1
	}
	if input.PageSize < 1 {
		input.PageSize = 50
	}
	if input.Source != "" && input.Source != "manual" && input.Source != "litellm" {
		writeAdminError(c, appErrors.NewInvalidRequest("source 参数无效"))
		return
	}
	result, err := h.store.ListAllLatestPricesPaginated(c.Request.Context(), input.Page, input.PageSize, input.Search, input.Source, input.LitellmProvider)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": result})
}

func (h *ModelPricesActionHandler) hasPriceTable(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("价格仓储未初始化"))
		return
	}
	has, err := h.store.HasAnyRecords(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": has})
}

func (h *ModelPricesActionHandler) cloudModelCount(c *gin.Context) {
	url := strings.TrimSpace(os.Getenv("CLOUD_PRICE_TABLE_URL"))
	if url == "" {
		url = "https://claude-code-hub.app/config/prices-base.toml"
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "创建云端价格表请求失败"})
		return
	}
	req.Header.Set("Accept", "text/plain")
	resp, err := modelPricesHTTPClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": err.Error()})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": "云端价格表拉取失败"})
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": "读取云端价格表失败"})
		return
	}
	var parsed struct {
		Models map[string]map[string]any `toml:"models"`
	}
	if err := toml.Unmarshal(body, &parsed); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "云端价格表解析失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"count": len(parsed.Models)}})
}

func (h *ModelPricesActionHandler) availableModelsByProviderType(c *gin.Context) {
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("价格仓储未初始化"))
		return
	}
	models, err := h.store.GetChatModelNames(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": models})
}
