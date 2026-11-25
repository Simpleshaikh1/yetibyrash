package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type healthResponse struct {
	Status   string `json:"status"`   // "ok" or "degraded"
	Database string `json:"database"` // "ok" or "unreachable"
	Version  string `json:"version"`
}

// healthHandler reports whether the API and its database are working.
// Hosting platforms call it to decide if the app is up.
func healthHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		res := healthResponse{Status: "ok", Database: "ok", Version: d.Version}
		status := http.StatusOK
		if err := d.DB.Ping(ctx); err != nil {
			slog.Warn("health check: database unreachable", "error", err)
			res.Status = "degraded"
			res.Database = "unreachable"
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, res)
	}
}
