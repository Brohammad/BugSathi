package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
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
	platformredis "github.com/Brohammad/BugSathi/internal/platform/redis"
	reportcache "github.com/Brohammad/BugSathi/internal/reports/adapter/cache"
	sharingdomain "github.com/Brohammad/BugSathi/internal/sharing/domain"
	uploadminio "github.com/Brohammad/BugSathi/internal/uploads/adapter/minio"
	uploadoutbox "github.com/Brohammad/BugSathi/internal/uploads/adapter/outbox"
	uploadpg "github.com/Brohammad/BugSathi/internal/uploads/adapter/postgres"
	uploadmemaccess "github.com/Brohammad/BugSathi/internal/uploads/adapter/memory"
	uploadsvc "github.com/Brohammad/BugSathi/internal/uploads/service"
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

	mediaService := mediasvc.New(mediapg.NewStore(pool), objectStore, mediaffmpeg.New(), mediasvc.ClaimConfig{
		Owner:         cfg.Media.WorkerID,
		Lease:         cfg.Media.ClaimLease,
		RenewInterval: cfg.Media.ClaimRenew,
	})
	log.Info("media claim owner", "worker_id", mediaService.Owner(), "lease", cfg.Media.ClaimLease)
	analyzer := observability.Analyzer{
		Inner:   newAnalyzer(cfg),
		Metrics: metrics,
		Name:    cfg.AI.Provider,
	}
	aiClaimLease := cfg.AI.ClaimLease
	if aiClaimLease <= 0 {
		aiClaimLease = cfg.AI.Timeout + 30*time.Second
		if aiClaimLease < 2*time.Minute {
			aiClaimLease = 2 * time.Minute
		}
	}
	aiService := aisvc.New(aipg.NewStore(pool), analyzer, cfg.AI.MaxFrames, aiClaimLease, cfg.AI.ClaimRenew)
	if cfg.Redis.Enabled() {
		redisClient, rerr := platformredis.New(cfg.Redis.URL)
		if rerr != nil {
			log.Error("redis connect failed", "error", rerr)
			os.Exit(1)
		}
		defer redisClient.Close()
		aiService = aiService.WithCacheInvalidator(reportcache.NewRedisReportCache(redisClient.Raw(), cfg.Cache.ReportTTL))
		log.Info("ai report cache invalidation enabled via redis")
	}

	for _, topic := range []string{
		mediadomain.TopicRecordingUploaded,
		mediadomain.TopicFramesExtracted,
		aidomain.TopicAnalysisStarted,
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

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		relay.Run(ctx)
	}()
	go observability.NewOutboxLagPoller(pool, metrics).Run(ctx)

	if gc := cfg.Hardening.UploadGC; gc.TTL > 0 {
		uploadGC := uploadsvc.New(
			uploadpg.NewRecordingRepo(pool),
			objectStore,
			uploadmemaccess.AccessOK{},
			15*time.Minute,
		)
		interval := gc.Interval
		if interval <= 0 {
			interval = 15 * time.Minute
		}
		batch := gc.Batch
		if batch <= 0 {
			batch = 50
		}
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			run := func() {
				n, err := uploadGC.SweepAbandonedUploads(ctx, gc.TTL, batch)
				if err != nil && ctx.Err() == nil {
					log.Error("upload gc failed", "error", err)
					return
				}
				if n > 0 {
					log.Info("upload gc swept abandoned uploading", "count", n)
				}
			}
			run()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					run()
				}
			}
		}()
		log.Info("upload abandoned GC enabled", "ttl", gc.TTL, "interval", interval, "batch", batch)
	}

	attemptStore := platformkafka.NewPostgresAttemptStore(pool, log)
	mediaConsumer := mediakafka.NewConsumer(cfg.Kafka, cfg.Hardening.KafkaRetry, mediaService, log, metrics, kafkaPub, attemptStore)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := mediaConsumer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("media consumer stopped", "error", err)
			stop()
		}
	}()

	aiConsumer := aikafka.NewConsumer(cfg.Kafka, cfg.Hardening.KafkaRetry, aiService, log, metrics, kafkaPub, attemptStore)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := aiConsumer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("ai consumer stopped", "error", err)
			stop()
		}
	}()

	startedConsumer := aikafka.NewStartedConsumer(cfg.Kafka, cfg.Hardening.KafkaRetry, log, metrics)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := startedConsumer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("analysis started consumer stopped", "error", err)
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
		stop()
	}

	drainDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(drainDone)
	}()
	select {
	case <-drainDone:
		log.Info("pipeline drained")
	case <-time.After(config.ShutdownTimeout()):
		log.Warn("pipeline drain timed out")
	}

	_ = mediaConsumer.Close()
	_ = aiConsumer.Close()
	_ = startedConsumer.Close()

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
