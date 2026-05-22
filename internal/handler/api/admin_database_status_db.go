package api

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/ding113/claude-code-hub/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/uptrace/bun"
)

type DatabaseStatusSource struct {
	db     *bun.DB
	host   string
	port   int
	dbName string
}

func NewDatabaseStatusSource(db *bun.DB, cfg config.DatabaseConfig) *DatabaseStatusSource {
	host := cfg.Host
	port := cfg.Port
	dbName := cfg.DBName
	if cfg.DSN != "" {
		if parsed, err := url.Parse(cfg.DSN); err == nil {
			if parsed.Hostname() != "" {
				host = parsed.Hostname()
			}
			if parsed.Port() != "" {
				if parsedPort, parseErr := strconv.Atoi(parsed.Port()); parseErr == nil {
					port = parsedPort
				}
			}
			if trimmed := strings.Trim(parsed.Path, "/"); trimmed != "" {
				dbName = trimmed
			}
		}
	}
	return &DatabaseStatusSource{
		db:     db,
		host:   host,
		port:   port,
		dbName: dbName,
	}
}

func (s *DatabaseStatusSource) GetStatus(ctx context.Context) (gin.H, error) {
	containerName := s.host
	if containerName == "" {
		containerName = "database"
	}
	if s.port > 0 {
		containerName += ":" + strconv.Itoa(s.port)
	}
	dbName := s.dbName
	if dbName == "" {
		dbName = "postgres"
	}
	if s.db == nil {
		return gin.H{
			"isAvailable":     false,
			"containerName":   containerName,
			"databaseName":    dbName,
			"databaseSize":    "N/A",
			"tableCount":      0,
			"postgresVersion": "N/A",
			"error":           "数据库连接不可用，请检查数据库服务状态",
		}, nil
	}
	if err := s.db.PingContext(ctx); err != nil {
		return gin.H{
			"isAvailable":     false,
			"containerName":   containerName,
			"databaseName":    dbName,
			"databaseSize":    "N/A",
			"tableCount":      0,
			"postgresVersion": "N/A",
			"error":           "数据库连接不可用，请检查数据库服务状态",
		}, nil
	}

	var info struct {
		DatabaseSize    string `bun:"database_size"`
		TableCount      int    `bun:"table_count"`
		PostgresVersion string `bun:"postgres_version"`
	}
	query := s.db.NewRaw(`
		SELECT
			pg_size_pretty(pg_database_size(current_database())) AS database_size,
			(SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public') AS table_count,
			version() AS postgres_version
	`)
	if err := query.Scan(ctx, &info); err != nil {
		return gin.H{
			"isAvailable":     true,
			"containerName":   containerName,
			"databaseName":    dbName,
			"databaseSize":    "Unknown",
			"tableCount":      0,
			"postgresVersion": "Unknown",
			"error":           err.Error(),
		}, nil
	}

	return gin.H{
		"isAvailable":     true,
		"containerName":   containerName,
		"databaseName":    dbName,
		"databaseSize":    info.DatabaseSize,
		"tableCount":      info.TableCount,
		"postgresVersion": info.PostgresVersion,
	}, nil
}
