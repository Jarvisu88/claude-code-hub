package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

func Load() (*Config, error) {
	v := viper.New()

	setDefaults(v)

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.AddConfigPath("/etc/claude-code-hub")

	v.SetEnvPrefix("")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnvVariables(v)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("env", "development")
	v.SetDefault("timezone", "Asia/Shanghai")
	v.SetDefault("debug_mode", false)
	v.SetDefault("auto_migrate", true)

	v.SetDefault("server.port", 23000)
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.read_timeout", 30*time.Second)
	v.SetDefault("server.write_timeout", 120*time.Second)
	v.SetDefault("server.shutdown_timeout", 30*time.Second)

	v.SetDefault("database.dsn", "")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "")
	v.SetDefault("database.dbname", "claude_code_hub")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.pool_max", 20)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.idle_timeout", 20*time.Second)
	v.SetDefault("database.connect_timeout", 10*time.Second)
	v.SetDefault("database.conn_max_lifetime", 30*time.Minute)

	v.SetDefault("redis.url", "")
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 10)
	v.SetDefault("redis.min_idle_conns", 2)
	v.SetDefault("redis.dial_timeout", 5*time.Second)
	v.SetDefault("redis.read_timeout", 3*time.Second)
	v.SetDefault("redis.write_timeout", 3*time.Second)
	v.SetDefault("redis.tls_reject_unauthorized", true)
	v.SetDefault("redis.max_retries", 5)
	v.SetDefault("redis.min_retry_backoff", 200*time.Millisecond)
	v.SetDefault("redis.max_retry_backoff", 2*time.Second)
	v.SetDefault("redis.enabled", true)

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

	v.SetDefault("auth.admin_token", "")

	v.SetDefault("message_request.write_mode", "async")
	v.SetDefault("message_request.async_flush_interval_ms", 250)
	v.SetDefault("message_request.async_batch_size", 200)
	v.SetDefault("message_request.async_max_pending", 10000)

	v.SetDefault("features.enable_rate_limit", true)
	v.SetDefault("features.enable_secure_cookies", true)
	v.SetDefault("features.enable_multi_provider_types", false)
	v.SetDefault("features.enable_circuit_breaker_on_network_errors", false)
	v.SetDefault("features.enable_provider_cache", true)
	v.SetDefault("features.enable_smart_probing", false)

	v.SetDefault("proxy.max_retry_attempts_default", 2)
	v.SetDefault("proxy.fetch_body_timeout", 600*time.Second)
	v.SetDefault("proxy.fetch_headers_timeout", 600*time.Second)
	v.SetDefault("proxy.fetch_connect_timeout", 30*time.Second)

	v.SetDefault("session.ttl", 300)
	v.SetDefault("session.short_context_threshold", 2)
	v.SetDefault("session.enable_short_context_detection", true)
	v.SetDefault("session.store_session_messages", false)

	v.SetDefault("smart_probing.interval_ms", 30000)
	v.SetDefault("smart_probing.timeout_ms", 5000)

	v.SetDefault("api_test.timeout_ms", 15000)

	v.SetDefault("app.url", "")
}

func bindEnvVariables(v *viper.Viper) {
	_ = v.BindEnv("env", "NODE_ENV")
	_ = v.BindEnv("timezone", "TZ")
	_ = v.BindEnv("debug_mode", "DEBUG_MODE")
	_ = v.BindEnv("auto_migrate", "AUTO_MIGRATE")

	_ = v.BindEnv("server.port", "PORT")
	_ = v.BindEnv("server.host", "SERVER_HOST")

	_ = v.BindEnv("database.dsn", "DSN")
	_ = v.BindEnv("database.host", "DATABASE_HOST")
	_ = v.BindEnv("database.port", "DATABASE_PORT")
	_ = v.BindEnv("database.user", "DATABASE_USER")
	_ = v.BindEnv("database.password", "DATABASE_PASSWORD")
	_ = v.BindEnv("database.dbname", "DATABASE_NAME")
	_ = v.BindEnv("database.sslmode", "DATABASE_SSLMODE")
	_ = v.BindEnv("database.pool_max", "DB_POOL_MAX")
	_ = v.BindEnv("database.idle_timeout", "DB_POOL_IDLE_TIMEOUT")
	_ = v.BindEnv("database.connect_timeout", "DB_POOL_CONNECT_TIMEOUT")

	_ = v.BindEnv("redis.url", "REDIS_URL")
	_ = v.BindEnv("redis.host", "REDIS_HOST")
	_ = v.BindEnv("redis.port", "REDIS_PORT")
	_ = v.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = v.BindEnv("redis.db", "REDIS_DB")
	_ = v.BindEnv("redis.pool_size", "REDIS_POOL_SIZE")
	_ = v.BindEnv("redis.tls_reject_unauthorized", "REDIS_TLS_REJECT_UNAUTHORIZED")
	_ = v.BindEnv("redis.max_retries", "REDIS_MAX_RETRIES")
	_ = v.BindEnv("redis.enabled", "REDIS_ENABLED")

	_ = v.BindEnv("log.level", "LOG_LEVEL")
	_ = v.BindEnv("log.format", "LOG_FORMAT")

	_ = v.BindEnv("auth.admin_token", "ADMIN_TOKEN")

	_ = v.BindEnv("message_request.write_mode", "MESSAGE_REQUEST_WRITE_MODE")
	_ = v.BindEnv("message_request.async_flush_interval_ms", "MESSAGE_REQUEST_ASYNC_FLUSH_INTERVAL_MS")
	_ = v.BindEnv("message_request.async_batch_size", "MESSAGE_REQUEST_ASYNC_BATCH_SIZE")
	_ = v.BindEnv("message_request.async_max_pending", "MESSAGE_REQUEST_ASYNC_MAX_PENDING")

	_ = v.BindEnv("features.enable_rate_limit", "ENABLE_RATE_LIMIT")
	_ = v.BindEnv("features.enable_secure_cookies", "ENABLE_SECURE_COOKIES")
	_ = v.BindEnv("features.enable_multi_provider_types", "ENABLE_MULTI_PROVIDER_TYPES")
	_ = v.BindEnv("features.enable_circuit_breaker_on_network_errors", "ENABLE_CIRCUIT_BREAKER_ON_NETWORK_ERRORS")
	_ = v.BindEnv("features.enable_provider_cache", "ENABLE_PROVIDER_CACHE")
	_ = v.BindEnv("features.enable_smart_probing", "ENABLE_SMART_PROBING")

	_ = v.BindEnv("proxy.max_retry_attempts_default", "MAX_RETRY_ATTEMPTS_DEFAULT")

	_ = v.BindEnv("session.ttl", "SESSION_TTL")
	_ = v.BindEnv("session.short_context_threshold", "SHORT_CONTEXT_THRESHOLD")
	_ = v.BindEnv("session.enable_short_context_detection", "ENABLE_SHORT_CONTEXT_DETECTION")
	_ = v.BindEnv("session.store_session_messages", "STORE_SESSION_MESSAGES")

	_ = v.BindEnv("smart_probing.interval_ms", "PROBE_INTERVAL_MS")
	_ = v.BindEnv("smart_probing.timeout_ms", "PROBE_TIMEOUT_MS")

	_ = v.BindEnv("api_test.timeout_ms", "API_TEST_TIMEOUT_MS")

	_ = v.BindEnv("app.url", "APP_URL")
}

func validate(cfg *Config) error {
	if cfg.Env != "development" && cfg.Env != "production" && cfg.Env != "test" {
		return fmt.Errorf("invalid env: %s, must be one of: development, production, test", cfg.Env)
	}

	validLogLevels := map[string]bool{
		"fatal": true, "error": true, "warn": true,
		"info": true, "debug": true, "trace": true,
	}
	if !validLogLevels[cfg.Log.Level] {
		return fmt.Errorf("invalid log level: %s", cfg.Log.Level)
	}

	if cfg.Database.PoolMax < 1 || cfg.Database.PoolMax > 200 {
		return fmt.Errorf("invalid database.pool_max: %d, must be between 1 and 200", cfg.Database.PoolMax)
	}

	if cfg.Proxy.MaxRetryAttemptsDefault < 1 || cfg.Proxy.MaxRetryAttemptsDefault > 10 {
		return fmt.Errorf("invalid proxy.max_retry_attempts_default: %d, must be between 1 and 10", cfg.Proxy.MaxRetryAttemptsDefault)
	}

	if cfg.MessageRequest.WriteMode != "sync" && cfg.MessageRequest.WriteMode != "async" {
		return fmt.Errorf("invalid message_request.write_mode: %s", cfg.MessageRequest.WriteMode)
	}

	return nil
}
