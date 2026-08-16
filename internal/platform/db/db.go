package db

import (
	"context"
	"fmt"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return ConnectWithPool(ctx, dsn, config.PostgresConfig{
		MaxConns: 10, MinConns: 1, MaxConnLifetime: time.Hour,
	})
}

func ConnectWithPool(ctx context.Context, dsn string, poolCfg config.PostgresConfig) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	if poolCfg.MaxConns > 0 {
		cfg.MaxConns = poolCfg.MaxConns
	} else {
		cfg.MaxConns = 10
	}
	if poolCfg.MinConns > 0 {
		cfg.MinConns = poolCfg.MinConns
	} else {
		cfg.MinConns = 1
	}
	if poolCfg.MaxConnLifetime > 0 {
		cfg.MaxConnLifetime = poolCfg.MaxConnLifetime
	} else {
		cfg.MaxConnLifetime = time.Hour
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}
