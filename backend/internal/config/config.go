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
}

// Load reads backend/.env (if present) and then the environment.
// Real environment variables always win over the .env file.
func Load() Config {
	loadDotEnv(".env")
	return Config{
		Env:         get("APP_ENV", "development"),
		HTTPAddr:    get("HTTP_ADDR", ":8090"),
		DatabaseURL: get("DATABASE_URL", "postgres://yetibyrash:yetibyrash@localhost:5432/yetibyrash?sslmode=disable"),
	}
}

func (c Config) IsProduction() bool { return c.Env == "production" }

func get(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
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
