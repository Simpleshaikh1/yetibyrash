// Package db opens the Postgres connection pool and runs migrations.
package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"yetibyrash/backend/migrations"
)

// Connect opens a pool and waits up to 30 seconds for Postgres to accept connections.
// The wait matters in development: GoLand may start the API before the database container is ready.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err = pool.Ping(pingCtx)
		cancel()
		if err == nil {
			return pool, nil
		}
		if time.Now().After(deadline) {
			pool.Close()
			return nil, fmt.Errorf("database not reachable after 30s (is `docker compose up` running?): %w", err)
		}
		slog.Info("waiting for database", "error", err)
		select {
		case <-ctx.Done():
			pool.Close()
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// Migrate applies every migration in backend/migrations that hasn't run yet.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	for _, r := range results {
		slog.Info("migration applied", "file", r.Source.Path, "took", r.Duration)
	}
	return nil
}
