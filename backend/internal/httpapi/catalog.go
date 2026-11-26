package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"yetibyrash/backend/internal/catalog"
	"yetibyrash/backend/internal/monitor"
)

// CatalogService is what the shop's catalogue endpoints need. *catalog.Service satisfies it.
type CatalogService interface {
	Categories(ctx context.Context) ([]catalog.Category, error)
	ListProducts(ctx context.Context, f catalog.ProductFilter) (catalog.ProductList, error)
	Product(ctx context.Context, slug string) (catalog.ProductDetail, error)
	Home(ctx context.Context, tier catalog.Tier) (catalog.Home, error)
}

// GET /api/v1/categories
func categoriesHandler(c CatalogService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cats, err := c.Categories(r.Context())
		if err != nil {
			internalError(w, "list categories", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"categories": cats})
	}
}

// GET /api/v1/home?tier=everyday
func homeHandler(c CatalogService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tier, ok := catalog.ParseTier(r.URL.Query().Get("tier"))
		if !ok {
			badTier(w)
			return
		}
		home, err := c.Home(r.Context(), tier)
		if err != nil {
			internalError(w, "home", err)
			return
		}
		writeJSON(w, http.StatusOK, home)
	}
}

// GET /api/v1/products?category=&tier=&colour=&size=&band=&sort=&page=
func productsHandler(c CatalogService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		tier, ok := catalog.ParseTier(q.Get("tier"))
		if !ok {
			badTier(w)
			return
		}
		sort := q.Get("sort")
		if sort != "" && !catalog.ValidSort(sort) {
			writeError(w, http.StatusBadRequest, "invalid_sort", "Sort must be newest, price_asc or price_desc.")
			return
		}
		page, ok := positiveInt(q.Get("page"), 1, 1000)
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid_page", "Page must be a whole number from 1 to 1000.")
			return
		}
		band, ok := positiveInt(q.Get("band"), 0, 1<<31)
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid_band", "Price band must be a price band id.")
			return
		}

		list, err := c.ListProducts(r.Context(), catalog.ProductFilter{
			CategorySlug: q.Get("category"),
			Tier:         tier,
			Colour:       q.Get("colour"),
			Size:         q.Get("size"),
			BandID:       int64(band),
			Sort:         sort,
			Page:         page,
		})
		if errors.Is(err, catalog.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "invalid_band", "That price band doesn't exist.")
			return
		}
		if err != nil {
			internalError(w, "list products", err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// GET /api/v1/products/{slug}
func productHandler(c CatalogService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := c.Product(r.Context(), r.PathValue("slug"))
		if errors.Is(err, catalog.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "We couldn't find that product. It may have sold out or been removed.")
			return
		}
		if err != nil {
			internalError(w, "get product", err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

func badTier(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "invalid_tier", "Tier must be everyday or premium.")
}

func internalError(w http.ResponseWriter, what string, err error) {
	slog.Error(what, "error", err)
	monitor.Error(what, err, nil)
	writeError(w, http.StatusInternalServerError, "internal", "Something went wrong on our side. Please try again.")
}

// positiveInt parses s as a whole number between 1 and max. Empty s returns def.
func positiveInt(s string, def, max int) (int, bool) {
	if s == "" {
		return def, true
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > max {
		return 0, false
	}
	return n, true
}
