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

	mediaffmpeg "github.com/Brohammad/BugSathi/internal/media/adapter/ffmpeg"
	mediakafka "github.com/Brohammad/BugSathi/internal/media/adapter/kafka"
	mediapg "github.com/Brohammad/BugSathi/internal/media/adapter/postgres"
	"github.com/Brohammad/BugSathi/internal/media/domain"
	mediasvc "github.com/Brohammad/BugSathi/internal/media/service"
	"github.com/Brohammad/BugSathi/internal/platform/config"
	"github.com/Brohammad/BugSathi/internal/platform/db"
	"github.com/Brohammad/BugSathi/internal/platform/health"
	"github.com/Brohammad/BugSathi/internal/platform/httpx"
	platformkafka "github.com/Brohammad/BugSathi/internal/platform/kafka"
	"github.com/Brohammad/BugSathi/internal/platform/logging"
	uploadminio "github.com/Brohammad/BugSathi/internal/uploads/adapter/minio"
	uploadoutbox "github.com/Brohammad/BugSathi/internal/uploads/adapter/outbox"
	uploadpg "github.com/Brohammad/BugSathi/internal/uploads/adapter/postgres"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	addr := getenv("WORKER_HTTP_ADDR", ":8081")
	log := logging.New(cfg.LogLevel)
	log.Info("starting worker", "env", cfg.AppEnv, "addr", addr)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.Postgres.DSN())
	if err != nil {
		log.Error("postgres connect failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	objectStore, err := uploadminio.New(cfg.MinIO)
	if err != nil {
		log.Error("minio client failed", "error", err)
		os.Exit(1)
	}

	mediaService := mediasvc.New(
		mediapg.NewStore(pool),
		objectStore,
		mediaffmpeg.New(),
	)

	if err := platformkafka.EnsureTopic(cfg.Kafka.Brokers, domain.TopicRecordingUploaded, 3); err != nil {
		log.Warn("ensure uploaded topic", "error", err)
	}
	if err := platformkafka.EnsureTopic(cfg.Kafka.Brokers, domain.TopicFramesExtracted, 3); err != nil {
		log.Warn("ensure frames topic", "error", err)
	}

	kafkaPub := platformkafka.NewPublisher(cfg.Kafka)
	defer kafkaPub.Close()
	relay := uploadoutbox.NewRelay(uploadpg.NewOutboxRepo(pool), kafkaPub, log)
	go relay.Run(ctx)

	consumer := mediakafka.NewConsumer(cfg.Kafka, mediaService, log)
	defer consumer.Close()
	go func() {
		if err := consumer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("media consumer stopped", "error", err)
			stop()
		}
	}()

	healthHandler := health.NewHandler()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler.Healthz)
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(pingCtx); err != nil {
			health.WriteReady(w, false, map[string]string{"postgres": "down"})
			return
		}
		health.WriteReady(w, true, map[string]string{"postgres": "up", "consumer": "running"})
	})

	server := &http.Server{Addr: addr, Handler: httpx.RequestIDs(mux)}
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
	_ = server.Shutdown(shutdownCtx)
	log.Info("worker stopped")
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
