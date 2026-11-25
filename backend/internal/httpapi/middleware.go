package httpapi

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// statusRecorder remembers the status code a handler wrote, for logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// logRequests logs one line per request: method, path, status and duration.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"took", time.Since(start).Round(time.Microsecond),
		)
	})
}

// recoverPanics turns a panic in a handler into a 500 response instead of crashing the server.
func recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				slog.Error("panic in handler", "panic", v, "path", r.URL.Path, "stack", string(debug.Stack()))
				writeError(w, http.StatusInternalServerError, "internal", "Something went wrong on our side. Please try again.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
