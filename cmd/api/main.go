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
	"github.com/Brohammad/BugSathi/internal/auth/service"
	"github.com/Brohammad/BugSathi/internal/platform/config"
	"github.com/Brohammad/BugSathi/internal/platform/db"
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

	log := logging.New(cfg.LogLevel)
	log.Info("starting api",
		"env", cfg.AppEnv,
		"addr", cfg.HTTPAddr,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

	authSvc := service.New(
		authpg.NewUserRepo(pool),
		authpg.NewRefreshRepo(pool),
		password.NewArgon2id(),
		tokenMgr,
		cfg.Auth.RefreshTokenTTL,
	)
	authHandler := authhttp.NewHandler(authSvc)

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
	authHandler.RegisterRoutes(mux)

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: httpx.RequestIDs(mux),
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
