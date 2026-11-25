// Package httpapi holds the HTTP routes, handlers and middleware.
package httpapi

import (
	"context"
	"net/http"
	"time"

	"yetibyrash/backend/internal/staff"
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
	Staff   StaffService
	Admin   AdminCatalog
	Media   http.Handler // serves /media/<key> (product photos)
	Version string

	SecureCookies  bool     // send the admin cookie over HTTPS only (production)
	AllowedOrigins []string // admin app addresses allowed to make changes
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

	if d.Media != nil {
		mux.Handle("GET /media/", d.Media)
	}

	if d.Staff != nil {
		registerAdmin(mux, d)
	}

	// Unknown /api paths get a JSON 404 instead of Go's plain-text one.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "There is no API endpoint at this address.")
	})

	return recoverPanics(logRequests(mux))
}

func registerAdmin(mux *http.ServeMux, d Deps) {
	a := &adminAuth{
		staff:          d.Staff,
		secureCookies:  d.SecureCookies,
		allowedOrigins: d.AllowedOrigins,
		loginLimit:     newRateLimiter(5, 15*time.Minute),
	}
	h := adminCatalogHandlers{c: d.Admin}

	anyStaff := a.requireStaff()
	products := a.requireStaff(staff.PermProducts)
	catalogue := a.requireStaff(staff.PermCatalogue)
	productsOrCatalogue := a.requireStaff(staff.PermProducts, staff.PermCatalogue)
	owner := a.requireStaff(staff.PermStaff)

	const p = "/api/admin/v1"
	mux.Handle("POST "+p+"/auth/login", a.checkOrigin(http.HandlerFunc(a.login)))
	mux.Handle("POST "+p+"/auth/logout", a.checkOrigin(http.HandlerFunc(a.logout)))
	mux.Handle("GET "+p+"/auth/me", anyStaff(a.me))

	mux.Handle("GET "+p+"/setup", productsOrCatalogue(h.setup))

	mux.Handle("GET "+p+"/products", products(h.listProducts))
	mux.Handle("POST "+p+"/products", products(h.createProduct))
	mux.Handle("GET "+p+"/products/{id}", products(h.getProduct))
	mux.Handle("PUT "+p+"/products/{id}", products(h.updateProduct))
	mux.Handle("POST "+p+"/products/{id}/images", products(h.uploadImage))
	mux.Handle("DELETE "+p+"/products/{id}/images/{imageId}", products(h.deleteImage))
	mux.Handle("POST "+p+"/products/{id}/images/{imageId}/main", products(h.makeImageMain))

	mux.Handle("POST "+p+"/size-sets", catalogue(h.saveSizeSet))
	mux.Handle("PUT "+p+"/size-sets/{id}", catalogue(h.saveSizeSet))
	mux.Handle("POST "+p+"/colours", catalogue(h.saveColour))
	mux.Handle("PUT "+p+"/colours/{id}", catalogue(h.saveColour))
	mux.Handle("DELETE "+p+"/colours/{id}", catalogue(h.deleteColour))
	mux.Handle("POST "+p+"/categories", catalogue(h.saveCategory))
	mux.Handle("PUT "+p+"/categories/{id}", catalogue(h.saveCategory))
	mux.Handle("PUT "+p+"/categories/{id}/fields", catalogue(h.saveCategoryFields))

	mux.Handle("GET "+p+"/staff", owner(a.listStaff))
	mux.Handle("POST "+p+"/staff", owner(a.createStaff))
	mux.Handle("PUT "+p+"/staff/{id}", owner(a.updateStaff))
	mux.Handle("PUT "+p+"/staff/{id}/password", owner(a.setStaffPassword))
}
