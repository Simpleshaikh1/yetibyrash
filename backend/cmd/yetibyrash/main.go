// Command yetibyrash runs the whole shop: the API, the background worker (emails, texts,
// review requests) and, in production, the built shop and admin apps.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"yetibyrash/backend/internal/cart"
	"yetibyrash/backend/internal/catalog"
	"yetibyrash/backend/internal/config"
	"yetibyrash/backend/internal/content"
	"yetibyrash/backend/internal/customers"
	"yetibyrash/backend/internal/db"
	"yetibyrash/backend/internal/delivery"
	"yetibyrash/backend/internal/httpapi"
	"yetibyrash/backend/internal/jobs"
	"yetibyrash/backend/internal/media"
	"yetibyrash/backend/internal/monitor"
	"yetibyrash/backend/internal/notify"
	"yetibyrash/backend/internal/orders"
	"yetibyrash/backend/internal/paystack"
	"yetibyrash/backend/internal/promotions"
	"yetibyrash/backend/internal/reviews"
	"yetibyrash/backend/internal/staff"
	"yetibyrash/backend/internal/web"
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

	// Production must have real keys and safe addresses; refuse to start otherwise.
	problems, warnings := cfg.Check()
	for _, w := range warnings {
		slog.Warn("setting", "note", w)
	}
	if len(problems) > 0 {
		for _, p := range problems {
			slog.Error("setting", "problem", p)
		}
		return fmt.Errorf("%d production setting(s) need fixing (see above)", len(problems))
	}
	monitor.Init(cfg.SentryDSN, cfg.Env, version)
	defer monitor.Flush()

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

	// Photos: an R2 bucket in production (Render's disk is wiped on deploy), a folder otherwise.
	var photos media.Storage
	mediaOrigin := ""
	if cfg.MediaStorage == "r2" {
		r2, err := media.NewR2Storage(media.R2Config{
			AccountID: cfg.R2AccountID, AccessKeyID: cfg.R2AccessKeyID, SecretAccessKey: cfg.R2SecretAccessKey,
			Bucket: cfg.R2Bucket, PublicURL: cfg.MediaPublicURL,
		})
		if err != nil {
			return err
		}
		if err := r2.Check(ctx); err != nil {
			return err
		}
		photos, mediaOrigin = r2, r2.PublicURL()
	} else {
		local, err := media.NewLocalStorage(cfg.MediaDir)
		if err != nil {
			return err
		}
		photos = local
	}

	var site http.Handler
	if cfg.WebDir != "" {
		site = web.Handler(filepath.Join(cfg.WebDir, "shop"), filepath.Join(cfg.WebDir, "admin"))
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
	var sms notify.SMS
	switch {
	case cfg.TermiiAPIKey != "":
		sms = notify.TermiiSMS{APIKey: cfg.TermiiAPIKey, SenderID: cfg.TermiiSenderID, Channel: cfg.TermiiChannel}
	case !cfg.IsProduction():
		sms = notify.MailSMS{Mailer: mailer}
		slog.Warn("no TERMII_API_KEY: text messages go to Mailpit instead (development only)")
	default:
		slog.Warn("no TERMII_API_KEY: text messages are off; sign-in codes go by email only")
	}
	notifySvc := notify.NewOrders(pool, mailer, sms, cfg.ShopURL)
	runner := jobs.NewRunner(pool)
	runner.Handle(jobs.KindOrderEmail, notifySvc.HandleJob)
	runner.Handle(jobs.KindOrderSMS, notifySvc.HandleSMSJob)
	runner.Handle(jobs.KindReviewRequest, notifySvc.HandleReviewRequest)
	go runner.Run(ctx)

	catalogSvc := catalog.NewService(pool)
	srv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewRouter(httpapi.Deps{
			DB:             pool,
			Catalog:        catalogSvc,
			Staff:          staff.NewService(pool),
			Admin:          catalog.NewAdminService(pool, photos),
			Media:          photos.Handler(),
			Cart:           cart.NewService(pool),
			Delivery:       deliverySvc,
			Orders:         orderSvc,
			PaystackSecret: cfg.PaystackSecretKey,
			DevPay:         devPay,
			Notify:         notifySvc,
			Content:        content.NewService(pool, photos),
			Customers:      customers.NewService(pool, mailer, sms),
			Reviews:        reviews.NewService(pool, photos),
			Cards:          catalogSvc,
			Promotions:     promotions.NewService(pool),
			Version:        version,
			SecureCookies:  cfg.IsProduction(),
			AllowedOrigins: cfg.AdminOrigins,
			Web:            site,
			Security:       httpapi.SecurityOptions{HTTPSOnly: cfg.IsProduction(), MediaOrigin: mediaOrigin},
			ProxyHops:      cfg.ProxyHops,
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
