package main

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
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
	"github.com/ding113/claude-code-hub/web"
	"github.com/gin-gonic/gin"
	"github.com/uptrace/bun"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger.Init(logger.Config{
		Level:  cfg.Log.Level,
		Format: cfg.Log.Format,
	})

	logger.Info().Msg("Starting Claude Code Hub...")

	validator.Init()

	db, err := database.NewPostgres(cfg.Database)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
	}
	defer database.ClosePostgres(db)

	if cfg.AutoMigrate {
		if err := database.AutoMigrate(context.Background(), db); err != nil {
			logger.Fatal().Err(err).Msg("Failed to auto-migrate PostgreSQL schema")
		}
		if err := database.SeedLocalDevData(context.Background(), db, database.ResolveBootstrapAppURL()); err != nil {
			logger.Fatal().Err(err).Msg("Failed to seed local dev data")
		}
	}

	rdb, err := database.NewRedis(cfg.Redis)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	defer database.CloseRedis(rdb)

	if cfg.Log.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := setupRouter(cfg, db, rdb)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		logger.Info().
			Str("host", cfg.Server.Host).
			Int("port", cfg.Server.Port).
			Msg("Server listening")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server forced to shutdown")
	}

	logger.Info().Msg("Server exited")
}

func setupRouter(cfg *config.Config, db *bun.DB, rdb *database.RedisClient) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(requestLogger())

	repoFactory := repository.NewFactory(db)
	proxyAuthService := authsvc.NewServiceFromFactory(repoFactory, cfg.Auth.AdminToken, authsvc.NewRedisSessionReader(rdb))

	registerLocalMockRoutes(router)
	apihandler.NewSmokeDebugHandler().RegisterRoutes(router)

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
	apihandler.NewAuthHandler(proxyAuthService, apihandler.NewRedisAuthSessionStore(rdb), repoFactory.AuditLog()).RegisterRoutes(router)
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
	apihandler.NewAvailabilityProbeAllHandler(proxyAuthService, repoFactory.Provider(), nil, repoFactory.ProviderEndpoint(), repoFactory.ProviderEndpointProbeLog()).RegisterRoutes(router)
	apihandler.NewAvailabilityEndpointsHandler(proxyAuthService, repoFactory.Provider(), repoFactory.ProviderEndpoint(), repoFactory.ProviderEndpointProbeLog()).RegisterRoutes(router)
	apihandler.NewIPGeoHandler(proxyAuthService, repoFactory.SystemSettings(), nil).RegisterRoutes(router)
	apihandler.NewLeaderboardHandler(proxyAuthService, repoFactory.SystemSettings(), repoFactory.MessageRequest()).RegisterRoutes(router)

	proxySessionManager := sessionsvc.NewManager(cfg.Session, rdb)
	endpointprobesvc.Configure(rdb, 24*time.Hour)
	livechainsvc.Configure(rdb, time.Duration(cfg.Session.TTL)*time.Second)
	providertrackersvc.Configure(rdb)
	sessiontrackersvc.Configure(rdb, time.Duration(cfg.Session.TTL)*time.Second)
	apihandler.ConfigureUsageLogsExportStore(rdb)
	proxyHTTPClient := &http.Client{Timeout: cfg.Proxy.FetchBodyTimeout}
	v1handler.NewHandler(proxyAuthService, proxySessionManager, repoFactory.Provider(), repoFactory.MessageRequest(), proxyHTTPClient, repoFactory.SystemSettings(), repoFactory.ProviderVendor(), repoFactory.ProviderEndpoint(), repoFactory.RequestFilter(), repoFactory.SensitiveWord(), repoFactory.ErrorRule()).RegisterRoutes(router.Group("/v1"))
	apihandler.NewProxyStatusHandler(proxyAuthService, repoFactory.User(), repoFactory.MessageRequest()).RegisterDirectRoutes(router)

	apihandler.NewSystemSettingsHandler(proxyAuthService, repoFactory.SystemSettings()).RegisterRoutes(router.Group("/api/system-settings"))
	apihandler.NewModelPricesActionHandler(proxyAuthService, repoFactory.ModelPrice()).RegisterDirectRoutes(router.Group("/api/prices"))

	api := router.Group("/api/actions")
	{
		apihandler.NewHandler(proxyAuthService, repoFactory.User(), repoFactory.Key(), repoFactory.Provider(), repoFactory.AuditLog()).RegisterRoutes(api)
		apihandler.NewSystemSettingsActionHandler(proxyAuthService, repoFactory.SystemSettings()).RegisterRoutes(api)
		apihandler.NewUsageLogsActionHandler(proxyAuthService, repoFactory.MessageRequest()).RegisterRoutes(api)
		apihandler.NewSessionOriginChainActionHandler(proxyAuthService, repoFactory.MessageRequest()).RegisterRoutes(api)
		apihandler.NewModelPricesActionHandler(proxyAuthService, repoFactory.ModelPrice()).RegisterActionRoutes(api)
		apihandler.NewStatisticsActionHandler(proxyAuthService, repoFactory.Statistics()).RegisterRoutes(api)
		apihandler.NewOverviewActionHandler(proxyAuthService, repoFactory.User(), repoFactory.Key(), repoFactory.Provider(), repoFactory.MessageRequest()).RegisterRoutes(api)
		apihandler.NewProxyStatusHandler(proxyAuthService, repoFactory.User(), repoFactory.MessageRequest()).RegisterActionRoutes(api)
		apihandler.NewProviderSlotsActionHandler(proxyAuthService, repoFactory.Provider(), repoFactory.MessageRequest()).RegisterRoutes(api)
		apihandler.NewDashboardRealtimeActionHandler(proxyAuthService, repoFactory.MessageRequest(), repoFactory.Statistics(), repoFactory.Provider()).RegisterRoutes(api)
		apihandler.NewRequestFilterActionHandler(proxyAuthService, repoFactory.RequestFilter()).RegisterRoutes(api)
		apihandler.NewSensitiveWordActionHandler(proxyAuthService, repoFactory.SensitiveWord()).RegisterRoutes(api)
		apihandler.NewErrorRuleActionHandler(proxyAuthService, repoFactory.ErrorRule()).RegisterRoutes(api)
		webhookTester := apihandler.NewWebhookDeliveryTester(nil)
		apihandler.NewNotificationsActionHandler(proxyAuthService, repoFactory.NotificationSettings(), repoFactory.WebhookTarget(), webhookTester).RegisterRoutes(api)
		apihandler.NewWebhookTargetsActionHandler(proxyAuthService, repoFactory.WebhookTarget(), webhookTester).RegisterRoutes(api)
		apihandler.NewNotificationBindingsActionHandler(proxyAuthService, repoFactory.NotificationTargetBinding()).RegisterRoutes(api)
	}

	// Serve embedded frontend (SPA fallback)
	registerFrontend(router)

	return router
}

func registerLocalMockRoutes(router *gin.Engine) {
	router.POST("/__mock__/v1/messages", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"id": "msg_local_mock", "type": "message", "role": "assistant",
			"content":     []gin.H{{"type": "text", "text": "local mock response"}},
			"stop_reason": "end_turn",
			"usage":       gin.H{"input_tokens": 10, "output_tokens": 5},
		})
	})
	router.POST("/__mock__/v1/messages/count_tokens", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"input_tokens": 10})
	})
	router.POST("/__mock__/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"id": "chatcmpl_local_mock", "object": "chat.completion",
			"choices": []gin.H{{"index": 0, "message": gin.H{"role": "assistant", "content": "local mock response"}}},
			"usage":   gin.H{"prompt_tokens": 10, "completion_tokens": 5},
		})
	})
	router.POST("/__mock__/v1/responses", func(c *gin.Context) {
		if strings.Contains(strings.ToLower(c.GetHeader("Accept")), "text/event-stream") {
			c.Header("Content-Type", "text/event-stream")
			c.Status(http.StatusOK)
			_, _ = c.Writer.Write([]byte("event: response.created\ndata: {\"response\":{\"id\":\"resp_local_mock\",\"prompt_cache_key\":\"019b82ff-08ff-75a3-a203-7e10274fdbd8\"}}\n\ndata: [DONE]\n\n"))
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"id": "resp_local_mock", "status": "completed",
			"prompt_cache_key": "019b82ff-08ff-75a3-a203-7e10274fdbd8",
			"usage":            gin.H{"input_tokens": 10, "output_tokens": 5},
		})
	})
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()
		logger.Info().
			Str("method", c.Request.Method).
			Str("path", path).
			Int("status", c.Writer.Status()).
			Dur("latency", time.Since(start)).
			Msg("Request")
	}
}

func healthCheck(db *bun.DB, rdb *database.RedisClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if err := db.PingContext(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "database": "disconnected", "error": err.Error()})
			return
		}
		redisStatus := "disabled"
		if rdb != nil {
			if err := rdb.Ping(ctx).Err(); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "redis": "disconnected", "error": err.Error()})
				return
			}
			redisStatus = "connected"
		}
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "database": "connected", "redis": redisStatus})
	}
}

// registerFrontend serves the embedded frontend SPA from web/dist.
// Static assets are served directly; all other non-API routes get index.html
// for client-side routing.
func registerFrontend(router *gin.Engine) {
	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		logger.Warn().Err(err).Msg("Embedded frontend dist not available, skipping SPA serving")
		return
	}

	// Check if dist actually contains files (it will be empty if not built)
	entries, err := fs.ReadDir(distFS, ".")
	if err != nil || len(entries) == 0 {
		logger.Info().Msg("No embedded frontend files found, skipping SPA serving")
		return
	}

	logger.Info().Msg("Serving embedded frontend from web/dist")

	httpFS := http.FS(distFS)

	// Read index.html once for SPA fallback
	indexHTML, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		logger.Warn().Err(err).Msg("No index.html in embedded dist, skipping SPA fallback")
		return
	}

	router.NoRoute(func(c *gin.Context) {
		reqPath := c.Request.URL.Path

		// Skip API and proxy routes
		if strings.HasPrefix(reqPath, "/api/") ||
			strings.HasPrefix(reqPath, "/v1/") ||
			strings.HasPrefix(reqPath, "/health") ||
			strings.HasPrefix(reqPath, "/__mock__/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// Try to serve the static file directly
		cleanPath := strings.TrimPrefix(reqPath, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		// Check if the file exists in the embedded FS
		if f, err := distFS.Open(cleanPath); err == nil {
			f.Close()
			// Check if it has a file extension (static asset)
			if ext := path.Ext(cleanPath); ext != "" {
				c.FileFromFS(reqPath, httpFS)
				return
			}
		}

		// SPA fallback: serve index.html for all non-file routes
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	})
}
