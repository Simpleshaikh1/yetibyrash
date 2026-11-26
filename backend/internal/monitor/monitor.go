// Package monitor reports errors to Sentry (sentry.io) so the shop owner hears about
// problems without reading logs. With no SENTRY_DSN it does nothing.
package monitor

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
)

var enabled bool

// Init starts error reporting. Call Flush before the program exits.
func Init(dsn, env, release string) {
	if dsn == "" {
		return
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:         dsn,
		Environment: env,
		Release:     release,
		// Errors only, no performance tracing: keeps it within the free plan.
		EnableTracing: false,
		// Never send cookies, form bodies or IP addresses (customers' personal data).
		SendDefaultPII: false,
	})
	if err != nil {
		slog.Error("sentry init", "error", err)
		return
	}
	enabled = true
	slog.Info("error reporting on", "to", "sentry")
}

// Error reports an error with a short description of what was happening and a few details.
func Error(what string, err error, details map[string]string) {
	if !enabled || err == nil {
		return
	}
	sentry.WithScope(func(s *sentry.Scope) {
		s.SetTag("where", what)
		for k, v := range details {
			s.SetTag(k, v)
		}
		sentry.CaptureException(fmt.Errorf("%s: %w", what, err))
	})
}

// Panic reports a recovered panic.
func Panic(v any, path string) {
	if !enabled {
		return
	}
	sentry.WithScope(func(s *sentry.Scope) {
		s.SetTag("path", path)
		sentry.CurrentHub().Recover(v)
	})
}

// Flush waits briefly for queued reports to go out.
func Flush() {
	if enabled {
		sentry.Flush(3 * time.Second)
	}
}
