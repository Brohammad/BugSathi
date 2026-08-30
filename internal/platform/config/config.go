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
	Media         MediaConfig
	Observability ObservabilityConfig
	Cache         CacheConfig
	Sharing       SharingConfig
	List          ListConfig
	Redis         RedisConfig
	Hardening     HardeningConfig
}

// RedisConfig enables shared SSE fanout, rate limits, and report cache across API replicas.
type RedisConfig struct {
	URL string
}

func (c RedisConfig) Enabled() bool {
	return strings.TrimSpace(c.URL) != ""
}

// SharingConfig controls share-link TTL defaults and token storage.
type SharingConfig struct {
	DefaultTTL time.Duration
	MaxTTL     time.Duration
	HashTokens bool
}

// ListConfig caps list endpoint page sizes.
type ListConfig struct {
	DefaultLimit int
	MaxLimit     int
}

// MediaConfig controls the processing claim that keeps two workers from
// extracting frames for the same recording at the same time.
type MediaConfig struct {
	WorkerID   string        // claim owner; defaults to hostname-pid
	ClaimLease time.Duration // lease validity without renewal
	ClaimRenew time.Duration // renewal interval; must be shorter than the lease
}

type ObservabilityConfig struct {
	OTLPEndpoint string // e.g. http://localhost:4318/v1/traces ; empty disables export
	EnablePprof  bool
}

type AIConfig struct {
	Provider      string // mock | openai
	BaseURL       string
	APIKey        string
	Model         string
	Timeout       time.Duration
	MaxFrames     int
	FrameMaxBytes int64         // maximum bytes loaded for one visual frame
	ClaimLease    time.Duration // soft lease via analyses.updated_at
	ClaimRenew    time.Duration
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
	MaxBodyBytes   int64
	UploadMaxBytes int64 // max accepted object size on upload complete; 0 disables
	CORSOrigins    []string
	RateLimit      RateLimitConfig
	KafkaRetry     KafkaRetryConfig
	UploadGC       UploadGCConfig
}

// UploadGCConfig sweeps abandoned UPLOADING sessions (0 TTL disables).
type UploadGCConfig struct {
	TTL      time.Duration
	Interval time.Duration
	Batch    int
}

type RateLimitConfig struct {
	RPS            float64
	Burst          int
	AuthRPS        float64
	AuthBurst      int
	Window         time.Duration
	TrustedProxies []string // IPs/CIDRs; X-Forwarded-For honored only from these
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
			Provider:      getenv("AI_PROVIDER", "mock"),
			BaseURL:       getenv("AI_BASE_URL", "https://api.openai.com/v1"),
			APIKey:        getenv("AI_API_KEY", ""),
			Model:         getenv("AI_MODEL", "gpt-4o-mini"),
			Timeout:       getenvDuration("AI_TIMEOUT", 60*time.Second),
			MaxFrames:     getenvInt("AI_MAX_FRAMES", 5),
			FrameMaxBytes: int64(getenvInt("AI_FRAME_MAX_BYTES", 5<<20)),
			ClaimLease:    getenvDuration("AI_CLAIM_LEASE", 0), // 0 → Timeout+30s (min 2m) in worker
			ClaimRenew:    getenvDuration("AI_CLAIM_RENEW", 30*time.Second),
		},
		Media: MediaConfig{
			WorkerID:   getenv("WORKER_ID", ""),
			ClaimLease: getenvDuration("MEDIA_CLAIM_LEASE", 2*time.Minute),
			ClaimRenew: getenvDuration("MEDIA_CLAIM_RENEW", 30*time.Second),
		},
		Observability: ObservabilityConfig{
			OTLPEndpoint: getenv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
			EnablePprof:  getenvBool("ENABLE_PPROF", false),
		},
		Cache: CacheConfig{
			ReportTTL: getenvDuration("REPORT_CACHE_TTL", 30*time.Second),
		},
		Sharing: SharingConfig{
			DefaultTTL: getenvDuration("SHARE_DEFAULT_TTL", 720*time.Hour),
			MaxTTL:     getenvDuration("SHARE_MAX_TTL", 2160*time.Hour),
			HashTokens: getenvBool("SHARE_HASH_TOKENS", true),
		},
		List: ListConfig{
			DefaultLimit: getenvInt("LIST_DEFAULT_LIMIT", 50),
			MaxLimit:     getenvInt("LIST_MAX_LIMIT", 100),
		},
		Redis: RedisConfig{
			URL: getenv("REDIS_URL", ""),
		},
		Hardening: HardeningConfig{
			MaxBodyBytes:   int64(getenvInt("MAX_BODY_BYTES", 1<<20)),
			UploadMaxBytes: int64(getenvInt("UPLOAD_MAX_BYTES", 500<<20)),
			CORSOrigins:    getenvCSV("CORS_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173"),
			RateLimit: RateLimitConfig{
				RPS:            getenvFloat("RATE_LIMIT_RPS", 20),
				Burst:          getenvInt("RATE_LIMIT_BURST", 40),
				AuthRPS:        getenvFloat("AUTH_RATE_LIMIT_RPS", 5),
				AuthBurst:      getenvInt("AUTH_RATE_LIMIT_BURST", 10),
				Window:         getenvDuration("RATE_LIMIT_WINDOW", time.Minute),
				TrustedProxies: getenvCSV("TRUSTED_PROXIES", ""),
			},
			KafkaRetry: KafkaRetryConfig{
				Base:        getenvDuration("KAFKA_RETRY_BASE", time.Second),
				Max:         getenvDuration("KAFKA_RETRY_MAX", 30*time.Second),
				MaxAttempts: getenvInt("KAFKA_RETRY_MAX_ATTEMPTS", 5),
			},
			UploadGC: UploadGCConfig{
				TTL:      getenvDuration("UPLOAD_ABANDONED_TTL", 24*time.Hour),
				Interval: getenvDuration("UPLOAD_GC_INTERVAL", 15*time.Minute),
				Batch:    getenvInt("UPLOAD_GC_BATCH", 50),
			},
		},
	}

	if cfg.HTTPAddr == "" {
		return Config{}, fmt.Errorf("HTTP_ADDR must not be empty")
	}
	if len(cfg.Auth.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if cfg.AI.MaxFrames < 1 || cfg.AI.MaxFrames > 10 {
		return Config{}, fmt.Errorf("AI_MAX_FRAMES must be between 1 and 10")
	}
	if cfg.AI.FrameMaxBytes < 1 || cfg.AI.FrameMaxBytes > 20<<20 {
		return Config{}, fmt.Errorf("AI_FRAME_MAX_BYTES must be between 1 and 20971520")
	}
	if err := cfg.validateProduction(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Known local-dev defaults that must not ship in production.
const (
	devJWTSecret        = "dev-only-change-me-32chars-minimum!!"
	devPostgresPassword = "bugsathi"
	devMinIOSecretKey   = "bugsathi_secret"
)

func (c Config) validateProduction() error {
	if !strings.EqualFold(c.AppEnv, "production") {
		return nil
	}
	switch {
	case c.Auth.JWTSecret == devJWTSecret:
		return fmt.Errorf("JWT_SECRET must be changed when APP_ENV=production")
	case c.Postgres.Password == devPostgresPassword:
		return fmt.Errorf("POSTGRES_PASSWORD must be changed when APP_ENV=production")
	case c.MinIO.SecretKey == devMinIOSecretKey:
		return fmt.Errorf("MINIO_SECRET_KEY must be changed when APP_ENV=production")
	case c.Observability.EnablePprof:
		return fmt.Errorf("ENABLE_PPROF must be false when APP_ENV=production")
	case !c.Sharing.HashTokens:
		return fmt.Errorf("SHARE_HASH_TOKENS must be true when APP_ENV=production")
	}
	return nil
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

func getenvCSV(key, fallback string) []string {
	raw := getenv(key, fallback)
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ShutdownTimeout is the graceful shutdown window for HTTP servers.
func ShutdownTimeout() time.Duration {
	return 10 * time.Second
}
