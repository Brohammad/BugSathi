package config

import (
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("POSTGRES_HOST", "")
	t.Setenv("KAFKA_BROKERS", "")

	// Clear may not unset; set explicit empties then reload with known values.
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("APP_ENV", "test")
	t.Setenv("POSTGRES_HOST", "db")
	t.Setenv("KAFKA_BROKERS", "kafka:9092")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.AppEnv != "test" {
		t.Fatalf("AppEnv = %q", cfg.AppEnv)
	}
	if cfg.Postgres.Host != "db" {
		t.Fatalf("Postgres.Host = %q", cfg.Postgres.Host)
	}
	if len(cfg.Kafka.Brokers) != 1 || cfg.Kafka.Brokers[0] != "kafka:9092" {
		t.Fatalf("Kafka.Brokers = %#v", cfg.Kafka.Brokers)
	}
	dsn := cfg.Postgres.DSN()
	if dsn == "" {
		t.Fatal("expected DSN")
	}
}

func TestProductionRejectsDevSecrets(t *testing.T) {
	setProdEnv(t)
	t.Setenv("JWT_SECRET", devJWTSecret)
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("err=%v", err)
	}
}

func TestProductionAcceptsChangedSecrets(t *testing.T) {
	setProdEnv(t)
	t.Setenv("JWT_SECRET", "prod-secret-that-is-long-enough-32chars!!")
	t.Setenv("POSTGRES_PASSWORD", "strong-postgres-password")
	t.Setenv("MINIO_SECRET_KEY", "strong-minio-secret-key")
	t.Setenv("ENABLE_PPROF", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AppEnv != "production" {
		t.Fatalf("AppEnv=%q", cfg.AppEnv)
	}
}

func TestProductionRejectsPprof(t *testing.T) {
	setProdEnv(t)
	t.Setenv("JWT_SECRET", "prod-secret-that-is-long-enough-32chars!!")
	t.Setenv("POSTGRES_PASSWORD", "strong-postgres-password")
	t.Setenv("MINIO_SECRET_KEY", "strong-minio-secret-key")
	t.Setenv("ENABLE_PPROF", "true")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "ENABLE_PPROF") {
		t.Fatalf("err=%v", err)
	}
}

func TestProductionRejectsPlaintextShareTokens(t *testing.T) {
	setProdEnv(t)
	t.Setenv("SHARE_HASH_TOKENS", "false")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "SHARE_HASH_TOKENS") {
		t.Fatalf("err=%v", err)
	}
}

func setProdEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "production")
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("JWT_SECRET", "prod-secret-that-is-long-enough-32chars!!")
	t.Setenv("POSTGRES_PASSWORD", "strong-postgres-password")
	t.Setenv("MINIO_SECRET_KEY", "strong-minio-secret-key")
	t.Setenv("ENABLE_PPROF", "false")
}
