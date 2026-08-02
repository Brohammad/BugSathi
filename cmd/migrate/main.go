package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/config"
	"github.com/Brohammad/BugSathi/internal/platform/db"
	"github.com/Brohammad/BugSathi/internal/platform/migrate"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}
	dir := "migrations"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		fail(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.Postgres.DSN())
	if err != nil {
		fail(err)
	}
	defer pool.Close()

	if err := migrate.Up(ctx, pool, abs); err != nil {
		fail(err)
	}
	fmt.Println("migrations applied:", abs)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
	os.Exit(1)
}
