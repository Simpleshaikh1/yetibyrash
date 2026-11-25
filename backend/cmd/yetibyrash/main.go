// Command yetibyrash runs the Yetibyrash API.
// Later milestones add the background worker and the built shop and admin apps to this same binary.
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

	"yetibyrash/backend/internal/config"
	"yetibyrash/backend/internal/db"
	"yetibyrash/backend/internal/httpapi"
)

// version is set at build time: go build -ldflags "-X main.version=1.2.3"
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()

	var handler slog.Handler = slog.NewTextHandler(os.Stdout, nil)
	if cfg.IsProduction() {
		handler = slog.NewJSONHandler(os.Stdout, nil)
	}
	slog.SetDefault(slog.New(handler))

	// Stop cleanly on Ctrl+C or when the host sends SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewRouter(httpapi.Deps{DB: pool, Version: version}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("api listening", "addr", cfg.HTTPAddr, "env", cfg.Env, "version", version)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
	}
	return nil
}
