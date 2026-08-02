package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/config"
	"github.com/Brohammad/BugSathi/internal/platform/health"
	"github.com/Brohammad/BugSathi/internal/platform/httpx"
	"github.com/Brohammad/BugSathi/internal/platform/logging"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	// Worker health binds on a separate port so it can coexist with the API locally.
	addr := getenv("WORKER_HTTP_ADDR", ":8081")

	log := logging.New(cfg.LogLevel)
	log.Info("starting worker",
		"env", cfg.AppEnv,
		"addr", addr,
		"note", "kafka consumers land in later milestones; health only for now",
	)

	healthHandler := health.NewHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler.Healthz)
	mux.HandleFunc("GET /readyz", healthHandler.Readyz)

	server := &http.Server{
		Addr:    addr,
		Handler: httpx.RequestIDs(mux),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Placeholder for future Kafka consumer loops.
	go runIdleWorker(ctx, log)

	errCh := make(chan error, 1)
	go func() {
		log.Info("worker health listening", "addr", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout())
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	log.Info("worker stopped")
}

func runIdleWorker(ctx context.Context, log *slog.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Debug("worker idle; waiting for milestone 5+ kafka consumers")
		}
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
