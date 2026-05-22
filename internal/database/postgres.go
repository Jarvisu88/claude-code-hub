package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/config"
	"github.com/ding113/claude-code-hub/internal/pkg/logger"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

func NewPostgres(cfg config.DatabaseConfig) (*bun.DB, error) {
	dsn := cfg.DSN
	if dsn == "" {
		dsn = buildDSN(cfg)
	}

	if dsn == "" {
		return nil, errors.New("DSN environment variable is not set")
	}

	if strings.Contains(dsn, "user:password@host:port") {
		return nil, errors.New("DSN contains placeholder template, please set a valid DSN")
	}

	connector := pgdriver.NewConnector(
		pgdriver.WithDSN(dsn),
		pgdriver.WithDialTimeout(cfg.ConnectTimeout),
		pgdriver.WithReadTimeout(cfg.IdleTimeout),
	)

	sqlDB := sql.OpenDB(connector)
	sqlDB.SetMaxOpenConns(cfg.PoolMax)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.IdleTimeout)

	db := bun.NewDB(sqlDB, pgdialect.New())

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logDSN := sanitizeDSN(dsn)
	logger.Info().
		Str("dsn", logDSN).
		Int("pool_max", cfg.PoolMax).
		Dur("connect_timeout", cfg.ConnectTimeout).
		Msg("PostgreSQL connected")

	return db, nil
}

func ClosePostgres(db *bun.DB) error {
	if db != nil {
		logger.Info().Msg("Closing PostgreSQL connection")
		return db.Close()
	}
	return nil
}

type HealthStatus struct {
	Healthy   bool          `json:"healthy"`
	Latency   time.Duration `json:"latency"`
	Error     string        `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Stats     *PoolStats    `json:"stats,omitempty"`
}

type PoolStats struct {
	MaxOpenConnections int           `json:"max_open_connections"`
	OpenConnections    int           `json:"open_connections"`
	InUse              int           `json:"in_use"`
	Idle               int           `json:"idle"`
	WaitCount          int64         `json:"wait_count"`
	WaitDuration       time.Duration `json:"wait_duration"`
}

func HealthCheck(ctx context.Context, db *bun.DB) (*HealthStatus, error) {
	if db == nil {
		return nil, errors.New("database connection is nil")
	}

	status := &HealthStatus{
		Healthy:   false,
		Timestamp: time.Now(),
	}

	start := time.Now()
	err := db.PingContext(ctx)
	status.Latency = time.Since(start)

	if err != nil {
		status.Error = err.Error()
		return status, err
	}

	status.Healthy = true

	stats := db.DB.Stats()
	status.Stats = &PoolStats{
		MaxOpenConnections: stats.MaxOpenConnections,
		OpenConnections:    stats.OpenConnections,
		InUse:              stats.InUse,
		Idle:               stats.Idle,
		WaitCount:          stats.WaitCount,
		WaitDuration:       stats.WaitDuration,
	}

	return status, nil
}

func buildDSN(cfg config.DatabaseConfig) string {
	if cfg.Host == "" {
		return ""
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, cfg.SSLMode,
	)
}

func sanitizeDSN(dsn string) string {
	if !strings.Contains(dsn, "://") {
		return dsn
	}
	parts := strings.SplitN(dsn, "://", 2)
	if len(parts) != 2 {
		return dsn
	}
	rest := parts[1]
	atIndex := strings.Index(rest, "@")
	if atIndex == -1 {
		return dsn
	}
	userPass := rest[:atIndex]
	hostAndRest := rest[atIndex:]
	colonIndex := strings.Index(userPass, ":")
	if colonIndex == -1 {
		return dsn
	}
	return fmt.Sprintf("%s://%s:***%s", parts[0], userPass[:colonIndex], hostAndRest)
}
