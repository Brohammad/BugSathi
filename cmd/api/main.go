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
	"github.com/Brohammad/BugSathi/internal/auth/adapter/password"
	authpg "github.com/Brohammad/BugSathi/internal/auth/adapter/postgres"
	authsvc "github.com/Brohammad/BugSathi/internal/auth/service"
	collabhttp "github.com/Brohammad/BugSathi/internal/collab/adapter/httpapi"
	collabhub "github.com/Brohammad/BugSathi/internal/collab/adapter/hub"
	collabpg "github.com/Brohammad/BugSathi/internal/collab/adapter/postgres"
	collabdomain "github.com/Brohammad/BugSathi/internal/collab/domain"
	collabport "github.com/Brohammad/BugSathi/internal/collab/port"
	collabsvc "github.com/Brohammad/BugSathi/internal/collab/service"
	"github.com/Brohammad/BugSathi/internal/platform/config"
	"github.com/Brohammad/BugSathi/internal/platform/db"
	"github.com/Brohammad/BugSathi/internal/platform/health"
	"github.com/Brohammad/BugSathi/internal/platform/httpx"
	platformkafka "github.com/Brohammad/BugSathi/internal/platform/kafka"
	"github.com/Brohammad/BugSathi/internal/platform/logging"
	"github.com/Brohammad/BugSathi/internal/platform/observability"
	"github.com/Brohammad/BugSathi/internal/platform/pprofx"
	platformredis "github.com/Brohammad/BugSathi/internal/platform/redis"
	projecthttp "github.com/Brohammad/BugSathi/internal/projects/adapter/httpapi"
	projectpg "github.com/Brohammad/BugSathi/internal/projects/adapter/postgres"
	projectsvc "github.com/Brohammad/BugSathi/internal/projects/service"
	reportcache "github.com/Brohammad/BugSathi/internal/reports/adapter/cache"
	reporthttp "github.com/Brohammad/BugSathi/internal/reports/adapter/httpapi"
	reportpg "github.com/Brohammad/BugSathi/internal/reports/adapter/postgres"
	reportsvc "github.com/Brohammad/BugSathi/internal/reports/service"
	sharehttp "github.com/Brohammad/BugSathi/internal/sharing/adapter/httpapi"
	sharepg "github.com/Brohammad/BugSathi/internal/sharing/adapter/postgres"
	sharingdomain "github.com/Brohammad/BugSathi/internal/sharing/domain"
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

	pool, err := db.ConnectWithPool(ctx, cfg.Postgres.DSN(), cfg.Postgres)
	if err != nil {
		log.Error("postgres connect failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	var redisClient *platformredis.Client
	if cfg.Redis.Enabled() {
		redisClient, err = platformredis.New(cfg.Redis.URL)
		if err != nil {
			log.Error("redis connect failed", "error", err)
			os.Exit(1)
		}
		defer redisClient.Close()
		log.Info("redis enabled for multi-replica SSE, rate limits, and report cache")
	}

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

	objectStore, err := uploadminio.New(cfg.MinIO)
	if err != nil {
		log.Error("minio client failed", "error", err)
		os.Exit(1)
	}
	if err := objectStore.EnsureBucket(ctx); err != nil {
		log.Warn("minio ensure bucket", "error", err)
	}

	projectService := projectsvc.New(projectpg.NewRepo(pool), objectStore, log, cfg.List)
	projectHandler := projecthttp.NewHandler(projectService)

	uploadService := uploadsvc.New(
		uploadpg.NewRecordingRepo(pool),
		objectStore,
		uploadaccess.New(projectService),
		15*time.Minute,
	).WithUploadMaxBytes(cfg.Hardening.UploadMaxBytes)
	uploadHandler := uploadhttp.NewHandler(uploadService)

	var reportDetailCache reportsvc.DetailCache = reportcache.NewReportCache(cfg.Cache.ReportTTL)
	if redisClient != nil && cfg.Cache.ReportTTL > 0 {
		reportDetailCache = reportcache.NewRedisReportCache(redisClient.Raw(), cfg.Cache.ReportTTL)
	}

	reportService := reportsvc.New(
		reportpg.NewRepo(pool),
		uploadaccess.New(projectService),
		objectStore,
		reportDetailCache,
		cfg.List,
	)
	reportHandler := reporthttp.NewHandler(reportService)

	shareService := sharesvc.New(
		sharepg.NewRepo(pool),
		uploadaccess.New(projectService),
		sharepg.NewReportReader(pool),
		objectStore,
		cfg.Sharing,
		cfg.List,
	)
	shareHandler := sharehttp.NewHandler(shareService)

	var collabHub collabport.Hub = collabhub.New()
	var redisHub *collabhub.RedisHub
	if redisClient != nil {
		redisHub = collabhub.NewRedis(redisClient.Raw())
		collabHub = redisHub
		defer redisHub.Close()
	}

	collabService := collabsvc.New(
		collabpg.NewRepo(pool),
		uploadaccess.New(projectService),
		collabpg.NewReportGuard(pool),
		collabpg.NewAuthorLookup(pool),
		collabHub,
		cfg.List,
	)
	collabHandler := collabhttp.NewHandler(collabService)

	trusted, err := httpx.ParseTrustedNetworks(cfg.Hardening.RateLimit.TrustedProxies)
	if err != nil {
		log.Error("TRUSTED_PROXIES invalid", "error", err)
		os.Exit(1)
	}

	kafkaPub := platformkafka.NewPublisher(cfg.Kafka)
	defer kafkaPub.Close()
	for _, topic := range []string{
		domain.TopicRecordingUploaded,
		sharingdomain.TopicShareCreated,
		collabdomain.TopicCommentCreated,
	} {
		if err := platformkafka.EnsureTopic(cfg.Kafka.Brokers, topic, 3); err != nil {
			log.Warn("ensure kafka topic", "topic", topic, "error", err)
		}
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
		status := map[string]string{"postgres": "up"}
		if err := pool.Ping(pingCtx); err != nil {
			status["postgres"] = "down"
			health.WriteReady(w, false, status)
			return
		}
		if redisClient != nil {
			if err := redisClient.Ping(pingCtx); err != nil {
				status["redis"] = "down"
				health.WriteReady(w, false, status)
				return
			}
			status["redis"] = "up"
		}
		health.WriteReady(w, true, status)
	})
	mux.Handle("GET /metrics", observability.Handler(reg))
	if cfg.Observability.EnablePprof {
		pprofx.Mount(mux)
		log.Warn("pprof enabled at /debug/pprof/")
	}
	authHandler.RegisterRoutes(mux)
	projectHandler.RegisterRoutes(mux, protect)
	uploadHandler.RegisterRoutes(mux, protect)
	reportHandler.RegisterRoutes(mux, protect)
	shareHandler.RegisterRoutes(mux, protect)
	collabHandler.RegisterRoutes(mux, protect)

	core := observability.Middleware("api", metrics, mux)
	core = httpx.MaxBodyBytes(cfg.Hardening.MaxBodyBytes, core)
	if redisClient != nil {
		core = httpx.RateLimitRedis(cfg.Hardening.RateLimit, trusted, metrics, redisClient.Raw(), core)
	} else {
		core = httpx.RateLimit(cfg.Hardening.RateLimit, trusted, metrics, core)
	}

	server := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpx.RequestIDs(
			httpx.CORS(cfg.Hardening.CORSOrigins,
				httpx.SecurityHeaders(httpx.IsProduction(cfg.AppEnv), core),
			),
		),
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
