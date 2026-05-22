package config

import "time"

// Config holds all application configuration
type Config struct {
	Env string `mapstructure:"env"`

	Server         ServerConfig         `mapstructure:"server"`
	Database       DatabaseConfig       `mapstructure:"database"`
	Redis          RedisConfig          `mapstructure:"redis"`
	Log            LogConfig            `mapstructure:"log"`
	Auth           AuthConfig           `mapstructure:"auth"`
	MessageRequest MessageRequestConfig `mapstructure:"message_request"`
	Features       FeaturesConfig       `mapstructure:"features"`
	Proxy          ProxyConfig          `mapstructure:"proxy"`
	Session        SessionConfig        `mapstructure:"session"`
	SmartProbing   SmartProbingConfig   `mapstructure:"smart_probing"`
	APITest        APITestConfig        `mapstructure:"api_test"`
	App            AppConfig            `mapstructure:"app"`

	Timezone    string `mapstructure:"timezone"`
	DebugMode   bool   `mapstructure:"debug_mode"`
	AutoMigrate bool   `mapstructure:"auto_migrate"`
}

type ServerConfig struct {
	Port            int           `mapstructure:"port"`
	Host            string        `mapstructure:"host"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type DatabaseConfig struct {
	DSN      string `mapstructure:"dsn"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`

	PoolMax         int           `mapstructure:"pool_max"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	ConnectTimeout  time.Duration `mapstructure:"connect_timeout"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	URL      string `mapstructure:"url"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`

	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`

	TLSRejectUnauthorized bool `mapstructure:"tls_reject_unauthorized"`

	MaxRetries      int           `mapstructure:"max_retries"`
	MinRetryBackoff time.Duration `mapstructure:"min_retry_backoff"`
	MaxRetryBackoff time.Duration `mapstructure:"max_retry_backoff"`

	Enabled bool `mapstructure:"enabled"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type AuthConfig struct {
	AdminToken string `mapstructure:"admin_token"`
}

type MessageRequestConfig struct {
	WriteMode            string `mapstructure:"write_mode"`
	AsyncFlushIntervalMs int    `mapstructure:"async_flush_interval_ms"`
	AsyncBatchSize       int    `mapstructure:"async_batch_size"`
	AsyncMaxPending      int    `mapstructure:"async_max_pending"`
}

type FeaturesConfig struct {
	EnableRateLimit                     bool `mapstructure:"enable_rate_limit"`
	EnableSecureCookies                 bool `mapstructure:"enable_secure_cookies"`
	EnableMultiProviderTypes            bool `mapstructure:"enable_multi_provider_types"`
	EnableCircuitBreakerOnNetworkErrors bool `mapstructure:"enable_circuit_breaker_on_network_errors"`
	EnableProviderCache                 bool `mapstructure:"enable_provider_cache"`
	EnableSmartProbing                  bool `mapstructure:"enable_smart_probing"`
}

type ProxyConfig struct {
	MaxRetryAttemptsDefault int           `mapstructure:"max_retry_attempts_default"`
	FetchBodyTimeout        time.Duration `mapstructure:"fetch_body_timeout"`
	FetchHeadersTimeout     time.Duration `mapstructure:"fetch_headers_timeout"`
	FetchConnectTimeout     time.Duration `mapstructure:"fetch_connect_timeout"`
}

type SessionConfig struct {
	TTL                          int  `mapstructure:"ttl"`
	ShortContextThreshold        int  `mapstructure:"short_context_threshold"`
	EnableShortContextDetection  bool `mapstructure:"enable_short_context_detection"`
	StoreSessionMessages         bool `mapstructure:"store_session_messages"`
}

type SmartProbingConfig struct {
	IntervalMs int `mapstructure:"interval_ms"`
	TimeoutMs  int `mapstructure:"timeout_ms"`
}

type APITestConfig struct {
	TimeoutMs int `mapstructure:"timeout_ms"`
}

type AppConfig struct {
	URL string `mapstructure:"url"`
}

func (c *Config) IsDevelopment() bool {
	return c.Env == "development"
}

func (c *Config) IsProduction() bool {
	return c.Env == "production"
}

func (c *Config) IsTest() bool {
	return c.Env == "test"
}
