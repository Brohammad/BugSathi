package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	aikafka "github.com/Brohammad/BugSathi/internal/ai/adapter/kafka"
	aimock "github.com/Brohammad/BugSathi/internal/ai/adapter/mock"
	aiopenai "github.com/Brohammad/BugSathi/internal/ai/adapter/openai"
	aipg "github.com/Brohammad/BugSathi/internal/ai/adapter/postgres"
	aidomain "github.com/Brohammad/BugSathi/internal/ai/domain"
	"github.com/Brohammad/BugSathi/internal/ai/port"
	aisvc "github.com/Brohammad/BugSathi/internal/ai/service"
	collabdomain "github.com/Brohammad/BugSathi/internal/collab/domain"
	mediaffmpeg "github.com/Brohammad/BugSathi/internal/media/adapter/ffmpeg"
	mediakafka "github.com/Brohammad/BugSathi/internal/media/adapter/kafka"
	mediapg "github.com/Brohammad/BugSathi/internal/media/adapter/postgres"
	mediadomain "github.com/Brohammad/BugSathi/internal/media/domain"
	mediasvc "github.com/Brohammad/BugSathi/internal/media/service"
	"github.com/Brohammad/BugSathi/internal/platform/config"
	"github.com/Brohammad/BugSathi/internal/platform/db"
	"github.com/Brohammad/BugSathi/internal/platform/health"
	"github.com/Brohammad/BugSathi/internal/platform/httpx"
	platformkafka "github.com/Brohammad/BugSathi/internal/platform/kafka"
	"github.com/Brohammad/BugSathi/internal/platform/logging"
	"github.com/Brohammad/BugSathi/internal/platform/observability"
	"github.com/Brohammad/BugSathi/internal/platform/pprofx"
	sharingdomain "github.com/Brohammad/BugSathi/internal/sharing/domain"
	uploadminio "github.com/Brohammad/BugSathi/internal/uploads/adapter/minio"
	uploadoutbox "github.com/Brohammad/BugSathi/internal/uploads/adapter/outbox"
	uploadpg "github.com/Brohammad/BugSathi/internal/uploads/adapter/postgres"
	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	addr := getenv("WORKER_HTTP_ADDR", ":8081")
	log := logging.New(cfg.LogLevel)
	log.Info("starting worker", "env", cfg.AppEnv, "addr", addr, "ai_provider", cfg.AI.Provider)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTrace, err := observability.SetupTracing(ctx, "bugsathi-worker", cfg.Observability.OTLPEndpoint)
	if err != nil {
		log.Error("tracing setup failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = shutdownTrace(context.Background()) }()

	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)

	pool, err := db.ConnectWithPool(ctx, cfg.Postgres.DSN(), cfg.Postgres)
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

	mediaService := mediasvc.New(mediapg.NewStore(pool), objectStore, mediaffmpeg.New())
	analyzer := observability.Analyzer{
		Inner:   newAnalyzer(cfg),
		Metrics: metrics,
		Name:    cfg.AI.Provider,
	}
	aiService := aisvc.New(aipg.NewStore(pool), analyzer, cfg.AI.MaxFrames)

	for _, topic := range []string{
		mediadomain.TopicRecordingUploaded,
		mediadomain.TopicFramesExtracted,
		aidomain.TopicAnalysisCompleted,
		aidomain.TopicReportGenerated,
		sharingdomain.TopicShareCreated,
		collabdomain.TopicCommentCreated,
		platformkafka.DLQTopic(mediadomain.TopicRecordingUploaded),
		platformkafka.DLQTopic(mediadomain.TopicFramesExtracted),
	} {
		if err := platformkafka.EnsureTopic(cfg.Kafka.Brokers, topic, 3); err != nil {
			log.Warn("ensure topic", "topic", topic, "error", err)
		}
	}

	kafkaPub := platformkafka.NewPublisher(cfg.Kafka)
	defer kafkaPub.Close()
	relay := uploadoutbox.NewRelay(uploadpg.NewOutboxRepo(pool), kafkaPub, log)
	go relay.Run(ctx)
	go observability.NewOutboxLagPoller(pool, metrics).Run(ctx)

	mediaConsumer := mediakafka.NewConsumer(cfg.Kafka, cfg.Hardening.KafkaRetry, mediaService, log, metrics, kafkaPub)
	defer mediaConsumer.Close()
	go func() {
		if err := mediaConsumer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("media consumer stopped", "error", err)
			stop()
		}
	}()

	aiConsumer := aikafka.NewConsumer(cfg.Kafka, cfg.Hardening.KafkaRetry, aiService, log, metrics, kafkaPub)
	defer aiConsumer.Close()
	go func() {
		if err := aiConsumer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("ai consumer stopped", "error", err)
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
		health.WriteReady(w, true, map[string]string{"postgres": "up", "ai_provider": cfg.AI.Provider})
	})
	mux.Handle("GET /metrics", observability.Handler(reg))
	if cfg.Observability.EnablePprof {
		pprofx.Mount(mux)
		log.Warn("pprof enabled at /debug/pprof/")
	}

	server := &http.Server{Addr: addr, Handler: httpx.RequestIDs(observability.Middleware("worker", metrics, mux))}
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

func newAnalyzer(cfg config.Config) port.Analyzer {
	switch strings.ToLower(cfg.AI.Provider) {
	case "openai":
		return aiopenai.New(aiopenai.Config{
			BaseURL: cfg.AI.BaseURL,
			APIKey:  cfg.AI.APIKey,
			Model:   cfg.AI.Model,
			Timeout: cfg.AI.Timeout,
		})
	default:
		return aimock.New()
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
