package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ding113/claude-code-hub/internal/config"
	"github.com/ding113/claude-code-hub/internal/database"
	apihandler "github.com/ding113/claude-code-hub/internal/handler/api"
	v1handler "github.com/ding113/claude-code-hub/internal/handler/v1"
	"github.com/ding113/claude-code-hub/internal/pkg/logger"
	"github.com/ding113/claude-code-hub/internal/pkg/validator"
	"github.com/ding113/claude-code-hub/internal/repository"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	endpointprobesvc "github.com/ding113/claude-code-hub/internal/service/endpointprobe"
	livechainsvc "github.com/ding113/claude-code-hub/internal/service/livechain"
	providertrackersvc "github.com/ding113/claude-code-hub/internal/service/providertracker"
	sessionsvc "github.com/ding113/claude-code-hub/internal/service/session"
	sessiontrackersvc "github.com/ding113/claude-code-hub/internal/service/sessiontracker"
	"github.com/gin-gonic/gin"
	"github.com/uptrace/bun"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	logger.Init(logger.Config{
		Level:  cfg.Log.Level,
		Format: cfg.Log.Format,
	})

	logger.Info().Msg("Starting Claude Code Hub...")

	// 初始化验证器
	validator.Init()

	// 连接数据库
	db, err := database.NewPostgres(cfg.Database)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
	}
	defer database.ClosePostgres(db)

	// 连接 Redis
	rdb, err := database.NewRedis(cfg.Redis)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	defer database.CloseRedis(rdb)

	// 创建 Gin 引擎
	if cfg.Log.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := setupRouter(cfg, db, rdb)

	// 创建 HTTP 服务器
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// 启动服务器
	go func() {
		logger.Info().
			Str("host", cfg.Server.Host).
			Int("port", cfg.Server.Port).
			Msg("Server listening")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Server failed")
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down server...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server forced to shutdown")
	}

	logger.Info().Msg("Server exited")
}

// setupRouter 设置路由
func setupRouter(cfg *config.Config, db *bun.DB, rdb *database.RedisClient) *gin.Engine {
	router := gin.New()

	// 添加中间件
	router.Use(gin.Recovery())
	router.Use(requestLogger())

	repoFactory := repository.NewFactory(db)
	proxyAuthService := authsvc.NewServiceFromFactory(repoFactory, cfg.Auth.AdminToken)

	// 健康检查
	router.GET("/health", healthCheck(db, rdb))
	apihandler.NewPlatformHandler(
		func(ctx context.Context) error { return db.PingContext(ctx) },
		func(ctx context.Context) error {
			if rdb == nil {
				return nil
			}
			return rdb.Ping(ctx).Err()
		},
		"0.1.0",
	).RegisterRoutes(router)
	apihandler.NewAuthHandler(proxyAuthService).RegisterRoutes(router)
	apihandler.NewAdminSystemConfigHandler(proxyAuthService, repoFactory.SystemSettings()).RegisterRoutes(router)
	apihandler.NewAdminLogLevelHandler(proxyAuthService).RegisterRoutes(router)
	apihandler.NewAdminDatabaseStatusHandler(proxyAuthService, apihandler.NewDatabaseStatusSource(db, cfg.Database)).RegisterRoutes(router)
	backupRunner := apihandler.NewDBBackupExecRunner(cfg.Database)
	apihandler.NewAdminDatabaseExportHandler(proxyAuthService, backupRunner).RegisterRoutes(router)
	apihandler.NewAdminDatabaseImportHandler(proxyAuthService, backupRunner).RegisterRoutes(router)
	apihandler.NewAdminLogCleanupHandler(proxyAuthService, apihandler.NewDBLogCleanupRunner(db)).RegisterRoutes(router)
	apihandler.NewInternalDataGenHandler(proxyAuthService, repoFactory.MessageRequest()).RegisterRoutes(router)
	apihandler.NewCurrentAvailabilityHandler(proxyAuthService, repoFactory.Provider(), repoFactory.MessageRequest()).RegisterRoutes(router)
	apihandler.NewAvailabilityHandler(proxyAuthService, repoFactory.Provider(), repoFactory.MessageRequest()).RegisterRoutes(router)
	apihandler.NewAvailabilityProbeAllHandler(proxyAuthService, repoFactory.Provider(), nil).RegisterRoutes(router)
	apihandler.NewAvailabilityEndpointsHandler(proxyAuthService, repoFactory.Provider()).RegisterRoutes(router)
	apihandler.NewIPGeoHandler(proxyAuthService, repoFactory.SystemSettings(), nil).RegisterRoutes(router)
	apihandler.NewLeaderboardHandler(proxyAuthService, repoFactory.MessageRequest()).RegisterRoutes(router)

	// API v1 路由组 (代理 API)
	proxySessionManager := sessionsvc.NewManager(cfg.Session, rdb)
	endpointprobesvc.Configure(rdb, 24*time.Hour)
	livechainsvc.Configure(rdb, time.Duration(cfg.Session.TTL)*time.Second)
	providertrackersvc.Configure(rdb)
	sessiontrackersvc.Configure(rdb, time.Duration(cfg.Session.TTL)*time.Second)
	apihandler.ConfigureUsageLogsExportStore(rdb)
	proxyHTTPClient := &http.Client{Timeout: cfg.Proxy.FetchBodyTimeout}
	v1handler.NewHandler(proxyAuthService, proxySessionManager, repoFactory.Provider(), repoFactory.MessageRequest(), proxyHTTPClient).RegisterRoutes(router.Group("/v1"))
	apihandler.NewProxyStatusHandler(proxyAuthService, repoFactory.User(), repoFactory.MessageRequest()).RegisterDirectRoutes(router)

	// 管理 API 路由组
	apihandler.NewSystemSettingsHandler(proxyAuthService, repoFactory.SystemSettings()).RegisterRoutes(router.Group("/api/system-settings"))
	apihandler.NewModelPricesActionHandler(proxyAuthService, repoFactory.ModelPrice()).RegisterDirectRoutes(router.Group("/api/prices"))

	api := router.Group("/api/actions")
	{
		apihandler.NewHandler(proxyAuthService, repoFactory.User(), repoFactory.Key(), repoFactory.Provider()).RegisterRoutes(api)
		apihandler.NewSystemSettingsActionHandler(proxyAuthService, repoFactory.SystemSettings()).RegisterRoutes(api)
		apihandler.NewUsageLogsActionHandler(proxyAuthService, repoFactory.MessageRequest()).RegisterRoutes(api)
		apihandler.NewSessionOriginChainActionHandler(proxyAuthService, repoFactory.MessageRequest()).RegisterRoutes(api)
		apihandler.NewModelPricesActionHandler(proxyAuthService, repoFactory.ModelPrice()).RegisterActionRoutes(api)
		apihandler.NewStatisticsActionHandler(proxyAuthService, repoFactory.Statistics()).RegisterRoutes(api)
		apihandler.NewOverviewActionHandler(proxyAuthService, repoFactory.User(), repoFactory.Key(), repoFactory.Provider(), repoFactory.MessageRequest()).RegisterRoutes(api)
		apihandler.NewProxyStatusHandler(proxyAuthService, repoFactory.User(), repoFactory.MessageRequest()).RegisterActionRoutes(api)
		apihandler.NewProviderSlotsActionHandler(proxyAuthService, repoFactory.Provider(), repoFactory.MessageRequest()).RegisterRoutes(api)
		apihandler.NewDashboardRealtimeActionHandler(proxyAuthService, repoFactory.MessageRequest(), repoFactory.Statistics(), repoFactory.Provider()).RegisterRoutes(api)
	}

	return router
}

// requestLogger 请求日志中间件
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		logger.Info().
			Str("method", c.Request.Method).
			Str("path", path).
			Int("status", status).
			Dur("latency", latency).
			Str("client_ip", c.ClientIP()).
			Msg("Request")
	}
}

// healthCheck 健康检查处理器
func healthCheck(db *bun.DB, rdb *database.RedisClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// 检查数据库连接
		if err := db.PingContext(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":   "unhealthy",
				"database": "disconnected",
				"error":    err.Error(),
			})
			return
		}

		// 检查 Redis 连接
		redisStatus := "disabled"
		if rdb != nil {
			if err := rdb.Ping(ctx).Err(); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"status":   "unhealthy",
					"redis":    "disconnected",
					"database": "connected",
					"error":    err.Error(),
				})
				return
			}
			redisStatus = "connected"
		}

		c.JSON(http.StatusOK, gin.H{
			"status":   "healthy",
			"database": "connected",
			"redis":    redisStatus,
		})
	}
}

// notImplemented 未实现的处理器
func notImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": gin.H{
			"type":    "not_implemented",
			"message": "This endpoint is not yet implemented",
		},
	})
}
