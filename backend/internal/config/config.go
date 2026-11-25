// Package config reads the app's settings from environment variables.
// Every setting has a development default, so the app starts with no setup.
package config

import (
	"bufio"
	"os"
	"strings"
)

type Config struct {
	Env         string // "development" or "production"
	HTTPAddr    string // address the API listens on, e.g. ":8090"
	DatabaseURL string
	MediaDir    string // folder for uploaded product photos (local storage)
	// AdminOrigins are the addresses the admin app is served from; admin changes from anywhere else are refused.
	AdminOrigins []string
	// ShopURL is the shop's public address; Paystack sends shoppers back to <ShopURL>/order/<number>.
	ShopURL string
	// Paystack keys. With no secret key in development, a built-in test payment page is used instead.
	PaystackSecretKey string

	// Email. Development default is Mailpit (inbox at http://localhost:8025).
	SMTPAddr     string
	SMTPUsername string
	SMTPPassword string
	MailFrom     string
}

// Load reads backend/.env (if present) and then the environment.
// Real environment variables always win over the .env file.
func Load() Config {
	loadDotEnv(".env")
	return Config{
		Env:          get("APP_ENV", "development"),
		HTTPAddr:     get("HTTP_ADDR", ":8090"),
		DatabaseURL:  get("DATABASE_URL", "postgres://yetibyrash:yetibyrash@localhost:5432/yetibyrash?sslmode=disable"),
		MediaDir:     get("MEDIA_DIR", "var/media"),
		AdminOrigins: list(get("ADMIN_ORIGINS", "http://localhost:5174,http://127.0.0.1:5174")),
		ShopURL:      get("SHOP_URL", "http://localhost:5173"),

		PaystackSecretKey: get("PAYSTACK_SECRET_KEY", ""),

		SMTPAddr:     get("SMTP_ADDR", "localhost:1025"),
		SMTPUsername: get("SMTP_USERNAME", ""),
		SMTPPassword: get("SMTP_PASSWORD", ""),
		MailFrom:     get("MAIL_FROM", "Yetibyrash <orders@yetibyrash.test>"),
	}
}

func (c Config) IsProduction() bool { return c.Env == "production" }

func get(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func list(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
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
