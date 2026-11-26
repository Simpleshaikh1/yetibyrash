// Package config reads the app's settings from environment variables.
// Every setting has a development default, so the app starts with no setup;
// in production, Check refuses to start with settings that are missing or unsafe.
package config

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Env         string // "development" or "production"
	HTTPAddr    string // address the API listens on, e.g. ":8090" (Render sets PORT)
	DatabaseURL string
	// WebDir holds the built shop and admin (WebDir/shop, WebDir/admin). Empty in development,
	// where Vite serves them.
	WebDir string
	// AdminOrigins are the addresses the admin app is served from; admin changes from anywhere else are refused.
	AdminOrigins []string
	// ShopURL is the shop's public address; Paystack sends shoppers back to <ShopURL>/order/<number>.
	ShopURL string
	// Paystack keys. With no secret key in development, a built-in test payment page is used instead.
	PaystackSecretKey string

	// Photos: "local" keeps them in MediaDir; "r2" keeps them in a Cloudflare R2 bucket
	// (needed on hosts like Render, whose disk is wiped on every deploy).
	MediaStorage      string
	MediaDir          string
	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string
	// MediaPublicURL is where the bucket's files are public, e.g. https://media.yetibyrash.com
	MediaPublicURL string

	// Email. Development default is Mailpit (inbox at http://localhost:8025).
	SMTPAddr     string
	SMTPUsername string
	SMTPPassword string
	MailFrom     string

	// SMS through Termii. Without a key in development, texts arrive in Mailpit instead.
	TermiiAPIKey   string
	TermiiSenderID string
	TermiiChannel  string

	// ProxyHops is how many proxies sit in front of the app (Render: 1). Visitor addresses,
	// used for rate limits, are read from X-Forwarded-For only that far back.
	ProxyHops int

	// SentryDSN sends errors to Sentry (sentry.io) when set.
	SentryDSN string
}

// Load reads backend/.env (if present) and then the environment.
// Real environment variables always win over the .env file.
func Load() Config {
	loadDotEnv(".env")
	addr := get("HTTP_ADDR", ":8090")
	if port := get("PORT", ""); port != "" {
		addr = ":" + port
	}
	hops, _ := strconv.Atoi(get("TRUSTED_PROXY_HOPS", "0"))
	return Config{
		Env:          get("APP_ENV", "development"),
		HTTPAddr:     addr,
		DatabaseURL:  get("DATABASE_URL", "postgres://yetibyrash:yetibyrash@localhost:5432/yetibyrash?sslmode=disable"),
		WebDir:       get("WEB_DIR", ""),
		AdminOrigins: list(get("ADMIN_ORIGINS", "http://localhost:5174,http://127.0.0.1:5174")),
		ShopURL:      strings.TrimRight(get("SHOP_URL", "http://localhost:5173"), "/"),

		PaystackSecretKey: get("PAYSTACK_SECRET_KEY", ""),

		MediaStorage:      get("MEDIA_STORAGE", "local"),
		MediaDir:          get("MEDIA_DIR", "var/media"),
		R2AccountID:       get("R2_ACCOUNT_ID", ""),
		R2AccessKeyID:     get("R2_ACCESS_KEY_ID", ""),
		R2SecretAccessKey: get("R2_SECRET_ACCESS_KEY", ""),
		R2Bucket:          get("R2_BUCKET", ""),
		MediaPublicURL:    strings.TrimRight(get("MEDIA_PUBLIC_URL", ""), "/"),

		SMTPAddr:     get("SMTP_ADDR", "localhost:1025"),
		SMTPUsername: get("SMTP_USERNAME", ""),
		SMTPPassword: get("SMTP_PASSWORD", ""),
		MailFrom:     get("MAIL_FROM", "Yetibyrash <orders@yetibyrash.test>"),

		TermiiAPIKey:   get("TERMII_API_KEY", ""),
		TermiiSenderID: get("TERMII_SENDER_ID", "Yetibyrash"),
		TermiiChannel:  get("TERMII_CHANNEL", "generic"),

		ProxyHops: hops,
		SentryDSN: get("SENTRY_DSN", ""),
	}
}

func (c Config) IsProduction() bool { return c.Env == "production" }

// Check lists what's wrong for production: missing keys, development addresses, plain http.
// Warnings are worth fixing but don't stop the app; problems do.
func (c Config) Check() (problems, warnings []string) {
	if !c.IsProduction() {
		return nil, nil
	}
	need := func(ok bool, msg string) {
		if !ok {
			problems = append(problems, msg)
		}
	}
	need(!strings.Contains(c.DatabaseURL, "@localhost") && !strings.Contains(c.DatabaseURL, "yetibyrash:yetibyrash@"),
		"DATABASE_URL still points at the development database")
	need(strings.HasPrefix(c.ShopURL, "https://"), "SHOP_URL must be the shop's https:// address")
	need(len(c.AdminOrigins) > 0, "ADMIN_ORIGINS must list the admin's https:// address")
	for _, o := range c.AdminOrigins {
		need(strings.HasPrefix(o, "https://"), "ADMIN_ORIGINS must be https:// addresses, not "+o)
	}
	need(c.PaystackSecretKey != "", "PAYSTACK_SECRET_KEY must be set")
	if strings.HasPrefix(c.PaystackSecretKey, "sk_test_") {
		warnings = append(warnings, "PAYSTACK_SECRET_KEY is a TEST key: no real money moves. Switch to sk_live_ for launch")
	}
	need(c.SMTPAddr != "localhost:1025" && !strings.HasPrefix(c.SMTPAddr, "127.0.0.1"), "SMTP_ADDR must be a real email service, not Mailpit")
	need(c.SMTPUsername != "" && c.SMTPPassword != "", "SMTP_USERNAME and SMTP_PASSWORD must be set")
	need(!strings.Contains(c.MailFrom, ".test"), "MAIL_FROM must use your own domain, e.g. Yetibyrash <orders@yetibyrash.com>")
	switch c.MediaStorage {
	case "r2":
		need(c.R2AccountID != "" && c.R2AccessKeyID != "" && c.R2SecretAccessKey != "" && c.R2Bucket != "",
			"R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY and R2_BUCKET must be set for MEDIA_STORAGE=r2")
		if u, err := url.Parse(c.MediaPublicURL); err != nil || u.Scheme != "https" || u.Host == "" {
			problems = append(problems, "MEDIA_PUBLIC_URL must be the bucket's public https:// address")
		}
	case "local":
		warnings = append(warnings, fmt.Sprintf("photos are stored on this server's disk (%s); on Render that disk is wiped on every deploy unless it's a persistent disk. Use MEDIA_STORAGE=r2", c.MediaDir))
	default:
		problems = append(problems, "MEDIA_STORAGE must be r2 or local")
	}
	need(c.WebDir != "", "WEB_DIR must point at the built shop and admin (the Docker image sets it)")
	if c.TermiiAPIKey == "" {
		warnings = append(warnings, "TERMII_API_KEY is not set: no text messages; sign-in codes go by email only")
	}
	if c.SentryDSN == "" {
		warnings = append(warnings, "SENTRY_DSN is not set: errors are only in the logs")
	}
	if c.ProxyHops == 0 {
		warnings = append(warnings, "TRUSTED_PROXY_HOPS is 0: behind Render's proxy, rate limits see one address for everyone. Set it to 1")
	}
	return problems, warnings
}

func get(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func list(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimRight(strings.TrimSpace(p), "/"); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// loadDotEnv sets KEY=VALUE lines from path as environment variables,
// skipping blanks, comments and keys that are already set.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // no .env file is fine
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"`)
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
}
