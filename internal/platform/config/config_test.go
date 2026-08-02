package config

import "testing"

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
