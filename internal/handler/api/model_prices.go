package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/ding113/claude-code-hub/internal/repository"
	"github.com/gin-gonic/gin"
)

type modelPriceStore interface {
	ListAllLatestPrices(ctx context.Context) ([]*model.ModelPrice, error)
	ListAllLatestPricesPaginated(ctx context.Context, page, pageSize int, search string) (*repository.PaginatedPrices, error)
	HasAnyRecords(ctx context.Context) (bool, error)
	GetAllModelNames(ctx context.Context) ([]string, error)
	GetChatModelNames(ctx context.Context) ([]string, error)
}

type ModelPricesActionHandler struct {
	auth  adminAuthenticator
	store modelPriceStore
}

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
	result, err := h.store.ListAllLatestPricesPaginated(c.Request.Context(), page, pageSize, search)
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
		Page     int    `json:"page"`
		PageSize int    `json:"pageSize"`
		Search   string `json:"search"`
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
	result, err := h.store.ListAllLatestPricesPaginated(c.Request.Context(), input.Page, input.PageSize, input.Search)
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
	if h == nil || h.store == nil {
		writeAdminError(c, appErrors.NewInternalError("价格仓储未初始化"))
		return
	}
	models, err := h.store.GetAllModelNames(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "data": gin.H{"count": len(models)}})
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
