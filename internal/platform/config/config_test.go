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
	if cfg.AI.MaxFrames != 5 || cfg.AI.FrameMaxBytes != 5<<20 {
		t.Fatalf("AI frame limits = count %d bytes %d", cfg.AI.MaxFrames, cfg.AI.FrameMaxBytes)
	}
	dsn := cfg.Postgres.DSN()
	if dsn == "" {
		t.Fatal("expected DSN")
	}
}

func TestLoadRejectsUnsafeAIFrameLimits(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("AI_MAX_FRAMES", "11")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "AI_MAX_FRAMES") {
		t.Fatalf("err=%v", err)
	}

	t.Setenv("AI_MAX_FRAMES", "5")
	t.Setenv("AI_FRAME_MAX_BYTES", "20971521")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "AI_FRAME_MAX_BYTES") {
		t.Fatalf("err=%v", err)
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

func TestMinIOPresignEndpointDefaultsToInternal(t *testing.T) {
	cfg := MinIOConfig{Endpoint: "minio:9000", UseSSL: false}
	ep, ssl := cfg.PresignEndpoint()
	if ep != "minio:9000" || ssl {
		t.Fatalf("PresignEndpoint() = %q ssl=%v", ep, ssl)
	}
}

func TestMinIOPresignEndpointUsesPublicHost(t *testing.T) {
	cfg := MinIOConfig{
		Endpoint:       "minio:9000",
		PublicEndpoint: "s3.example.com",
		PublicUseSSL:   true,
	}
	ep, ssl := cfg.PresignEndpoint()
	if ep != "s3.example.com" || !ssl {
		t.Fatalf("PresignEndpoint() = %q ssl=%v", ep, ssl)
	}
}

func TestLoadMinIOPublicEndpoint(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("MINIO_PUBLIC_ENDPOINT", "s3.example.com")
	t.Setenv("MINIO_PUBLIC_USE_SSL", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MinIO.PublicEndpoint != "s3.example.com" || !cfg.MinIO.PublicUseSSL {
		t.Fatalf("MinIO public = %+v", cfg.MinIO)
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
