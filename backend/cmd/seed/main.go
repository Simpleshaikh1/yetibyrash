// Command seed fills the development database with example products.
//
//	go run ./cmd/seed          add the examples if there are no products yet
//	go run ./cmd/seed -reset   delete every product first, then add the examples
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"yetibyrash/backend/internal/config"
	"yetibyrash/backend/internal/db"
	"yetibyrash/backend/internal/seed"
)

func main() {
	reset := flag.Bool("reset", false, "delete all products before seeding")
	flag.Parse()

	cfg := config.Load()
	if cfg.IsProduction() {
		slog.Error("refusing to seed example products in production")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		slog.Error("migrate", "error", err)
		os.Exit(1)
	}

	if *reset {
		if err := seed.Reset(ctx, pool); err != nil {
			slog.Error("reset products", "error", err)
			os.Exit(1)
		}
		slog.Info("deleted all products")
	}

	inserted, err := seed.Demo(ctx, pool)
	if err != nil {
		slog.Error("seed demo products", "error", err)
		os.Exit(1)
	}
	if inserted {
		slog.Info("example products added")
	} else {
		slog.Info("products already exist, nothing added (use -reset to replace them)")
	}
}
