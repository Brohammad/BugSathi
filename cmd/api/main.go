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

	authhttp "github.com/Brohammad/BugSathi/internal/auth/adapter/httpapi"
	"github.com/Brohammad/BugSathi/internal/auth/adapter/jwtmgr"
	authpg "github.com/Brohammad/BugSathi/internal/auth/adapter/postgres"
	"github.com/Brohammad/BugSathi/internal/auth/adapter/password"
	authsvc "github.com/Brohammad/BugSathi/internal/auth/service"
	collabhttp "github.com/Brohammad/BugSathi/internal/collab/adapter/httpapi"
	collabhub "github.com/Brohammad/BugSathi/internal/collab/adapter/hub"
	collabpg "github.com/Brohammad/BugSathi/internal/collab/adapter/postgres"
	collabsvc "github.com/Brohammad/BugSathi/internal/collab/service"
	"github.com/Brohammad/BugSathi/internal/platform/config"
	"github.com/Brohammad/BugSathi/internal/platform/db"
	"github.com/Brohammad/BugSathi/internal/platform/health"
	"github.com/Brohammad/BugSathi/internal/platform/httpx"
	platformkafka "github.com/Brohammad/BugSathi/internal/platform/kafka"
	"github.com/Brohammad/BugSathi/internal/platform/logging"
	"github.com/Brohammad/BugSathi/internal/platform/observability"
	projecthttp "github.com/Brohammad/BugSathi/internal/projects/adapter/httpapi"
	projectpg "github.com/Brohammad/BugSathi/internal/projects/adapter/postgres"
	projectsvc "github.com/Brohammad/BugSathi/internal/projects/service"
	reporthttp "github.com/Brohammad/BugSathi/internal/reports/adapter/httpapi"
	reportpg "github.com/Brohammad/BugSathi/internal/reports/adapter/postgres"
	reportsvc "github.com/Brohammad/BugSathi/internal/reports/service"
	sharehttp "github.com/Brohammad/BugSathi/internal/sharing/adapter/httpapi"
	sharepg "github.com/Brohammad/BugSathi/internal/sharing/adapter/postgres"
	sharesvc "github.com/Brohammad/BugSathi/internal/sharing/service"
	uploadaccess "github.com/Brohammad/BugSathi/internal/uploads/adapter/access"
	uploadhttp "github.com/Brohammad/BugSathi/internal/uploads/adapter/httpapi"
	uploadminio "github.com/Brohammad/BugSathi/internal/uploads/adapter/minio"
	uploadoutbox "github.com/Brohammad/BugSathi/internal/uploads/adapter/outbox"
	uploadpg "github.com/Brohammad/BugSathi/internal/uploads/adapter/postgres"
	"github.com/Brohammad/BugSathi/internal/uploads/domain"
	uploadsvc "github.com/Brohammad/BugSathi/internal/uploads/service"
	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	log := logging.New(cfg.LogLevel)
	log.Info("starting api",
		"env", cfg.AppEnv,
		"addr", cfg.HTTPAddr,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTrace, err := observability.SetupTracing(ctx, "bugsathi-api", cfg.Observability.OTLPEndpoint)
	if err != nil {
		log.Error("tracing setup failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = shutdownTrace(context.Background()) }()

	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)

	pool, err := db.Connect(ctx, cfg.Postgres.DSN())
	if err != nil {
		log.Error("postgres connect failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	tokenMgr, err := jwtmgr.New(cfg.Auth.JWTSecret, cfg.Auth.AccessTokenTTL)
	if err != nil {
		log.Error("jwt manager failed", "error", err)
		os.Exit(1)
	}

	authService := authsvc.New(
		authpg.NewUserRepo(pool),
		authpg.NewRefreshRepo(pool),
		password.NewArgon2id(),
		tokenMgr,
		cfg.Auth.RefreshTokenTTL,
	)
	authHandler := authhttp.NewHandler(authService)

	projectService := projectsvc.New(projectpg.NewRepo(pool))
	projectHandler := projecthttp.NewHandler(projectService)

	objectStore, err := uploadminio.New(cfg.MinIO)
	if err != nil {
		log.Error("minio client failed", "error", err)
		os.Exit(1)
	}
	if err := objectStore.EnsureBucket(ctx); err != nil {
		log.Warn("minio ensure bucket", "error", err)
	}

	uploadService := uploadsvc.New(
		uploadpg.NewRecordingRepo(pool),
		objectStore,
		uploadaccess.New(projectService),
		15*time.Minute,
	)
	uploadHandler := uploadhttp.NewHandler(uploadService)

	reportService := reportsvc.New(
		reportpg.NewRepo(pool),
		uploadaccess.New(projectService),
		objectStore,
	)
	reportHandler := reporthttp.NewHandler(reportService)

	shareService := sharesvc.New(
		sharepg.NewRepo(pool),
		uploadaccess.New(projectService),
		sharepg.NewReportReader(pool),
		objectStore,
	)
	shareHandler := sharehttp.NewHandler(shareService)

	collabService := collabsvc.New(
		collabpg.NewRepo(pool),
		uploadaccess.New(projectService),
		collabpg.NewReportGuard(pool),
		collabpg.NewAuthorLookup(pool),
		collabhub.New(),
	)
	collabHandler := collabhttp.NewHandler(collabService)

	kafkaPub := platformkafka.NewPublisher(cfg.Kafka)
	defer kafkaPub.Close()
	if err := platformkafka.EnsureTopic(cfg.Kafka.Brokers, domain.TopicRecordingUploaded, 3); err != nil {
		log.Warn("ensure kafka topic", "error", err)
	}
	relay := uploadoutbox.NewRelay(uploadpg.NewOutboxRepo(pool), kafkaPub, log)
	go relay.Run(ctx)
	go observability.NewOutboxLagPoller(pool, metrics).Run(ctx)

	protect := func(next http.Handler) http.Handler {
		return authhttp.RequireAccess(authService, next)
	}

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
		health.WriteReady(w, true, map[string]string{"postgres": "up"})
	})
	mux.Handle("GET /metrics", observability.Handler(reg))
	authHandler.RegisterRoutes(mux)
	projectHandler.RegisterRoutes(mux, protect)
	uploadHandler.RegisterRoutes(mux, protect)
	reportHandler.RegisterRoutes(mux, protect)
	shareHandler.RegisterRoutes(mux, protect)
	collabHandler.RegisterRoutes(mux, protect)

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: httpx.RequestIDs(observability.Middleware("api", metrics, mux)),
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("api listening", "addr", cfg.HTTPAddr)
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
	log.Info("api stopped")
}
