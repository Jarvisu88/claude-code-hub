package database

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/config"
	"github.com/ding113/claude-code-hub/internal/pkg/logger"
	"github.com/redis/go-redis/v9"
)

const (
	defaultMaxRetries      = 5
	defaultMinRetryBackoff = 200 * time.Millisecond
	defaultMaxRetryBackoff = 2 * time.Second
	defaultPoolSize        = 10
	defaultMinIdleConns    = 2
	defaultDialTimeout     = 5 * time.Second
	defaultReadTimeout     = 3 * time.Second
	defaultWriteTimeout    = 3 * time.Second
)

type RedisClient struct {
	*redis.Client
	useTLS  bool
	safeURL string
}

func NewRedis(cfg config.RedisConfig) (*RedisClient, error) {
	if !cfg.Enabled {
		logger.Warn().Msg("[Redis] Redis disabled (enabled=false)")
		return nil, nil
	}

	if cfg.URL == "" && cfg.Host == "" {
		logger.Warn().Msg("[Redis] Redis URL or Host not configured")
		return nil, nil
	}

	safeURL := ""
	if cfg.URL != "" {
		safeURL = maskRedisURL(cfg.URL)
	} else {
		safeURL = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	}

	opts, useTLS, err := buildRedisOptions(cfg)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), opts.DialTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		logger.Error().Err(err).Str("redisUrl", safeURL).Msg("[Redis] Connection error")
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	logger.Info().
		Bool("tlsEnabled", useTLS).
		Str("redisUrl", safeURL).
		Int("poolSize", opts.PoolSize).
		Msg("[Redis] Connected successfully")

	return &RedisClient{
		Client:  client,
		useTLS:  useTLS,
		safeURL: safeURL,
	}, nil
}

func CloseRedis(client *RedisClient) error {
	if client == nil || client.Client == nil {
		return nil
	}
	logger.Info().Msg("[Redis] Closing connection")
	return client.Client.Close()
}

func (c *RedisClient) HealthCheck(ctx context.Context) error {
	if c == nil || c.Client == nil {
		return fmt.Errorf("redis client is nil")
	}
	return c.Client.Ping(ctx).Err()
}

func (c *RedisClient) GetRedisClient() *redis.Client {
	if c == nil {
		return nil
	}
	return c.Client
}

func buildRedisOptions(cfg config.RedisConfig) (*redis.Options, bool, error) {
	var opts *redis.Options
	var useTLS bool

	if cfg.URL != "" {
		var err error
		opts, err = redis.ParseURL(cfg.URL)
		if err != nil {
			return nil, false, fmt.Errorf("failed to parse redis URL: %w", err)
		}
		useTLS = strings.HasPrefix(cfg.URL, "rediss://")

		if useTLS {
			parsed, _ := url.Parse(cfg.URL)
			opts.TLSConfig = &tls.Config{
				InsecureSkipVerify: !cfg.TLSRejectUnauthorized,
				ServerName:         parsed.Hostname(),
			}
		}
	} else {
		port := cfg.Port
		if port == 0 {
			port = 6379
		}
		opts = &redis.Options{
			Addr:     fmt.Sprintf("%s:%d", cfg.Host, port),
			Password: cfg.Password,
			DB:       cfg.DB,
		}
	}

	applyDefault := func(current, def int) int {
		if current > 0 {
			return current
		}
		return def
	}
	applyDefaultDur := func(current, def time.Duration) time.Duration {
		if current > 0 {
			return current
		}
		return def
	}

	opts.PoolSize = applyDefault(cfg.PoolSize, defaultPoolSize)
	opts.MinIdleConns = applyDefault(cfg.MinIdleConns, defaultMinIdleConns)
	opts.DialTimeout = applyDefaultDur(cfg.DialTimeout, defaultDialTimeout)
	opts.ReadTimeout = applyDefaultDur(cfg.ReadTimeout, defaultReadTimeout)
	opts.WriteTimeout = applyDefaultDur(cfg.WriteTimeout, defaultWriteTimeout)
	opts.MaxRetries = applyDefault(cfg.MaxRetries, defaultMaxRetries)
	opts.MinRetryBackoff = applyDefaultDur(cfg.MinRetryBackoff, defaultMinRetryBackoff)
	opts.MaxRetryBackoff = applyDefaultDur(cfg.MaxRetryBackoff, defaultMaxRetryBackoff)
	opts.PoolTimeout = opts.ReadTimeout

	return opts, useTLS, nil
}

func maskRedisURL(redisURL string) string {
	if redisURL == "" {
		return ""
	}
	parsed, err := url.Parse(redisURL)
	if err != nil {
		return redisURL
	}
	if _, hasPassword := parsed.User.Password(); hasPassword {
		parsed.User = url.UserPassword(parsed.User.Username(), "****")
	}
	return parsed.String()
}
