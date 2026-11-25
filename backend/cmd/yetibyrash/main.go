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

	"yetibyrash/backend/internal/cart"
	"yetibyrash/backend/internal/catalog"
	"yetibyrash/backend/internal/config"
	"yetibyrash/backend/internal/db"
	"yetibyrash/backend/internal/delivery"
	"yetibyrash/backend/internal/httpapi"
	"yetibyrash/backend/internal/jobs"
	"yetibyrash/backend/internal/media"
	"yetibyrash/backend/internal/notify"
	"yetibyrash/backend/internal/orders"
	"yetibyrash/backend/internal/paystack"
	"yetibyrash/backend/internal/staff"
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

	photos, err := media.NewLocalStorage(cfg.MediaDir)
	if err != nil {
		return err
	}

	// Payments: real Paystack when a secret key is set; in development without keys,
	// a built-in test payment page so checkout can be tried end to end.
	var pay paystack.Provider
	var devPay *paystack.DevProvider
	switch {
	case cfg.PaystackSecretKey != "":
		pay = paystack.NewClient(cfg.PaystackSecretKey)
	case cfg.IsProduction():
		return errors.New("PAYSTACK_SECRET_KEY must be set in production")
	default:
		devPay = paystack.NewDevProvider()
		pay = devPay
		slog.Warn("no PAYSTACK_SECRET_KEY: using the built-in test payment page (development only)")
	}

	deliverySvc := delivery.NewService(pool)
	orderSvc := orders.NewService(pool, deliverySvc, pay, cfg.ShopURL)
	go orderSvc.RunReleaser(ctx) // releases stock of orders unpaid after 30 minutes

	// Customer emails go through the job queue, so a slow or down mail server never blocks an order.
	mailer := notify.SMTPMailer{Addr: cfg.SMTPAddr, From: cfg.MailFrom, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword}
	notifySvc := notify.NewOrders(pool, mailer, cfg.ShopURL)
	runner := jobs.NewRunner(pool)
	runner.Handle(jobs.KindOrderEmail, notifySvc.HandleJob)
	go runner.Run(ctx)

	srv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewRouter(httpapi.Deps{
			DB:             pool,
			Catalog:        catalog.NewService(pool),
			Staff:          staff.NewService(pool),
			Admin:          catalog.NewAdminService(pool, photos),
			Media:          photos.Handler(),
			Cart:           cart.NewService(pool),
			Delivery:       deliverySvc,
			Orders:         orderSvc,
			PaystackSecret: cfg.PaystackSecretKey,
			DevPay:         devPay,
			Notify:         notifySvc,
			Version:        version,
			SecureCookies:  cfg.IsProduction(),
			AllowedOrigins: cfg.AdminOrigins,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second, // photo uploads on slow mobile data need time
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
