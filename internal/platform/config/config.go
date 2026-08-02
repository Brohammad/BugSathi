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

	Postgres PostgresConfig
	MinIO    MinIOConfig
	Kafka    KafkaConfig
}

type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DB       string
	SSLMode  string
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

// Load reads configuration from environment variables with safe local defaults.
func Load() (Config, error) {
	cfg := Config{
		AppEnv:   getenv("APP_ENV", "development"),
		HTTPAddr: getenv("HTTP_ADDR", ":8080"),
		LogLevel: getenv("LOG_LEVEL", "info"),
		Postgres: PostgresConfig{
			Host:     getenv("POSTGRES_HOST", "localhost"),
			Port:     getenvInt("POSTGRES_PORT", 5432),
			User:     getenv("POSTGRES_USER", "bugsathi"),
			Password: getenv("POSTGRES_PASSWORD", "bugsathi"),
			DB:       getenv("POSTGRES_DB", "bugsathi"),
			SSLMode:  getenv("POSTGRES_SSLMODE", "disable"),
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
	}

	if cfg.HTTPAddr == "" {
		return Config{}, fmt.Errorf("HTTP_ADDR must not be empty")
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

// ShutdownTimeout is the graceful shutdown window for HTTP servers.
func ShutdownTimeout() time.Duration {
	return 10 * time.Second
}
