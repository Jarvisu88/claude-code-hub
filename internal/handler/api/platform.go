package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type PlatformHandler struct {
	dbPing    func(ctx context.Context) error
	redisPing func(ctx context.Context) error
	version   string
}

func NewPlatformHandler(dbPing func(ctx context.Context) error, redisPing func(ctx context.Context) error, version string) *PlatformHandler {
	if version == "" {
		version = "0.1.0"
	}
	return &PlatformHandler{
		dbPing:    dbPing,
		redisPing: redisPing,
		version:   version,
	}
}

func (h *PlatformHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/api/health", h.health)
	router.GET("/api/health/live", h.live)
	router.GET("/api/health/ready", h.ready)
	router.GET("/api/version", h.versionInfo)
}

func (h *PlatformHandler) health(c *gin.Context) {
	ctx := c.Request.Context()
	if h.dbPing != nil {
		if err := h.dbPing(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "database": "disconnected", "error": err.Error()})
			return
		}
	}

	redisStatus := "disabled"
	if h.redisPing != nil {
		if err := h.redisPing(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "database": "connected", "redis": "disconnected", "error": err.Error()})
			return
		}
		redisStatus = "connected"
	}

	c.JSON(http.StatusOK, gin.H{"status": "healthy", "database": "connected", "redis": redisStatus})
}

func (h *PlatformHandler) live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

func (h *PlatformHandler) ready(c *gin.Context) {
	ctx := c.Request.Context()
	if h.dbPing != nil {
		if err := h.dbPing(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "database": "disconnected", "error": err.Error()})
			return
		}
	}
	if h.redisPing != nil {
		if err := h.redisPing(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "redis": "disconnected", "error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (h *PlatformHandler) versionInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"name": "claude-code-hub-go-rewrite", "version": h.version})
}
