package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds process configuration loaded from the environment (12-factor).
type Config struct {
	AppEnv   string
	HTTPAddr string
	LogLevel string

	Postgres      PostgresConfig
	MinIO         MinIOConfig
	Kafka         KafkaConfig
	Auth          AuthConfig
	AI            AIConfig
	Observability ObservabilityConfig
	Cache         CacheConfig
	Hardening     HardeningConfig
}

type ObservabilityConfig struct {
	OTLPEndpoint string // e.g. http://localhost:4318/v1/traces ; empty disables export
	EnablePprof  bool
}

type AIConfig struct {
	Provider  string // mock | openai
	BaseURL   string
	APIKey    string
	Model     string
	Timeout   time.Duration
	MaxFrames int
}

type AuthConfig struct {
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type PostgresConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DB              string
	SSLMode         string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
}

func (p PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		p.User, p.Password, p.Host, p.Port, p.DB, p.SSLMode,
	)
}

type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

type KafkaConfig struct {
	Brokers  []string
	ClientID string
}

type CacheConfig struct {
	ReportTTL time.Duration // 0 disables
}

type HardeningConfig struct {
	MaxBodyBytes int64
	RateLimit    RateLimitConfig
	KafkaRetry   KafkaRetryConfig
}

type RateLimitConfig struct {
	RPS       float64
	Burst     int
	AuthRPS   float64
	AuthBurst int
	Window    time.Duration
}

func (c RateLimitConfig) Enabled() bool {
	return c.RPS > 0
}

type KafkaRetryConfig struct {
	Base        time.Duration
	Max         time.Duration
	MaxAttempts int // after this many handler failures, message goes to DLQ (0 = default 5)
}

// Load reads configuration from environment variables with safe local defaults.
func Load() (Config, error) {
	cfg := Config{
		AppEnv:   getenv("APP_ENV", "development"),
		HTTPAddr: getenv("HTTP_ADDR", ":8080"),
		LogLevel: getenv("LOG_LEVEL", "info"),
		Postgres: PostgresConfig{
			Host:            getenv("POSTGRES_HOST", "localhost"),
			Port:            getenvInt("POSTGRES_PORT", 5432),
			User:            getenv("POSTGRES_USER", "bugsathi"),
			Password:        getenv("POSTGRES_PASSWORD", "bugsathi"),
			DB:              getenv("POSTGRES_DB", "bugsathi"),
			SSLMode:         getenv("POSTGRES_SSLMODE", "disable"),
			MaxConns:        int32(getenvInt("POSTGRES_MAX_CONNS", 10)),
			MinConns:        int32(getenvInt("POSTGRES_MIN_CONNS", 1)),
			MaxConnLifetime: getenvDuration("POSTGRES_MAX_CONN_LIFETIME", time.Hour),
		},
		MinIO: MinIOConfig{
			Endpoint:  getenv("MINIO_ENDPOINT", "localhost:9000"),
			AccessKey: getenv("MINIO_ACCESS_KEY", "bugsathi"),
			SecretKey: getenv("MINIO_SECRET_KEY", "bugsathi_secret"),
			Bucket:    getenv("MINIO_BUCKET", "bugsathi"),
			UseSSL:    getenvBool("MINIO_USE_SSL", false),
		},
		Kafka: KafkaConfig{
			Brokers:  strings.Split(getenv("KAFKA_BROKERS", "localhost:19092"), ","),
			ClientID: getenv("KAFKA_CLIENT_ID", "bugsathi"),
		},
		Auth: AuthConfig{
			JWTSecret:       getenv("JWT_SECRET", "dev-only-change-me-32chars-minimum!!"),
			AccessTokenTTL:  getenvDuration("AUTH_ACCESS_TTL", 15*time.Minute),
			RefreshTokenTTL: getenvDuration("AUTH_REFRESH_TTL", 7*24*time.Hour),
		},
		AI: AIConfig{
			Provider:  getenv("AI_PROVIDER", "mock"),
			BaseURL:   getenv("AI_BASE_URL", "https://api.openai.com/v1"),
			APIKey:    getenv("AI_API_KEY", ""),
			Model:     getenv("AI_MODEL", "gpt-4o-mini"),
			Timeout:   getenvDuration("AI_TIMEOUT", 60*time.Second),
			MaxFrames: getenvInt("AI_MAX_FRAMES", 5),
		},
		Observability: ObservabilityConfig{
			OTLPEndpoint: getenv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
			EnablePprof:  getenvBool("ENABLE_PPROF", false),
		},
		Cache: CacheConfig{
			ReportTTL: getenvDuration("REPORT_CACHE_TTL", 30*time.Second),
		},
		Hardening: HardeningConfig{
			MaxBodyBytes: int64(getenvInt("MAX_BODY_BYTES", 1<<20)),
			RateLimit: RateLimitConfig{
				RPS:       getenvFloat("RATE_LIMIT_RPS", 20),
				Burst:     getenvInt("RATE_LIMIT_BURST", 40),
				AuthRPS:   getenvFloat("AUTH_RATE_LIMIT_RPS", 5),
				AuthBurst: getenvInt("AUTH_RATE_LIMIT_BURST", 10),
				Window:    getenvDuration("RATE_LIMIT_WINDOW", time.Minute),
			},
			KafkaRetry: KafkaRetryConfig{
				Base:        getenvDuration("KAFKA_RETRY_BASE", time.Second),
				Max:         getenvDuration("KAFKA_RETRY_MAX", 30*time.Second),
				MaxAttempts: getenvInt("KAFKA_RETRY_MAX_ATTEMPTS", 5),
			},
		},
	}

	if cfg.HTTPAddr == "" {
		return Config{}, fmt.Errorf("HTTP_ADDR must not be empty")
	}
	if len(cfg.Auth.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getenvFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

func getenvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

// ShutdownTimeout is the graceful shutdown window for HTTP servers.
func ShutdownTimeout() time.Duration {
	return 10 * time.Second
}
