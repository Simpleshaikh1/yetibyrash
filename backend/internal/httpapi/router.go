// Package httpapi holds the HTTP routes, handlers and middleware.
package httpapi

import (
	"context"
	"net/http"
)

// Pinger is anything that can check the database is reachable.
// *pgxpool.Pool satisfies it; tests pass a fake.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps is everything the handlers need. It grows as milestones add features.
type Deps struct {
	DB      Pinger
	Catalog CatalogService
	Version string
}

// NewRouter registers every route and wraps them in the shared middleware.
func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", healthHandler(d))

	// Shop catalogue
	mux.HandleFunc("GET /api/v1/home", homeHandler(d.Catalog))
	mux.HandleFunc("GET /api/v1/categories", categoriesHandler(d.Catalog))
	mux.HandleFunc("GET /api/v1/products", productsHandler(d.Catalog))
	mux.HandleFunc("GET /api/v1/products/{slug}", productHandler(d.Catalog))

	// Unknown /api paths get a JSON 404 instead of Go's plain-text one.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "There is no API endpoint at this address.")
	})

	return recoverPanics(logRequests(mux))
}
