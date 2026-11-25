package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yetibyrash/backend/internal/catalog"
)

// fakeCatalog records the filter it was asked for and returns canned answers.
type fakeCatalog struct {
	gotFilter catalog.ProductFilter
	gotTier   catalog.Tier
}

func (f *fakeCatalog) Categories(context.Context) ([]catalog.Category, error) {
	return []catalog.Category{{ID: 1, Name: "Bags", Slug: "bags", Children: []catalog.Category{}}}, nil
}

func (f *fakeCatalog) ListProducts(_ context.Context, pf catalog.ProductFilter) (catalog.ProductList, error) {
	f.gotFilter = pf
	return catalog.ProductList{Items: []catalog.ProductCard{}, Page: pf.Page, PageSize: catalog.PageSize}, nil
}

func (f *fakeCatalog) Product(_ context.Context, slug string) (catalog.ProductDetail, error) {
	if slug == "ribbed-lounge-joggers" {
		return catalog.ProductDetail{Slug: slug, Name: "Ribbed lounge joggers"}, nil
	}
	return catalog.ProductDetail{}, catalog.ErrNotFound
}

func (f *fakeCatalog) Home(_ context.Context, tier catalog.Tier) (catalog.Home, error) {
	f.gotTier = tier
	return catalog.Home{Tagline: "Elegance meets affordability"}, nil
}

func get(t *testing.T, h http.Handler, url string) (*httptest.ResponseRecorder, errorBody) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	var eb errorBody
	if rec.Code >= 400 {
		if err := json.NewDecoder(rec.Body).Decode(&eb); err != nil {
			t.Fatalf("decode error body: %v", err)
		}
	}
	return rec, eb
}

func TestProductsPassesFiltersThrough(t *testing.T) {
	fc := &fakeCatalog{}
	router := NewRouter(Deps{DB: fakeDB{}, Catalog: fc})

	rec, _ := get(t, router, "/api/v1/products?category=clothing&tier=premium&colour=Espresso&size=M&band=2&sort=price_asc&page=3")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	want := catalog.ProductFilter{CategorySlug: "clothing", Tier: catalog.TierPremium, Colour: "Espresso", Size: "M", BandID: 2, Sort: "price_asc", Page: 3}
	if fc.gotFilter != want {
		t.Fatalf("filter = %+v, want %+v", fc.gotFilter, want)
	}
}

func TestProductsRejectsBadInput(t *testing.T) {
	router := NewRouter(Deps{DB: fakeDB{}, Catalog: &fakeCatalog{}})
	tests := []struct {
		url, code string
	}{
		{"/api/v1/products?tier=luxury", "invalid_tier"},
		{"/api/v1/products?sort=cheapest", "invalid_sort"},
		{"/api/v1/products?page=0", "invalid_page"},
		{"/api/v1/products?page=abc", "invalid_page"},
		{"/api/v1/products?band=-1", "invalid_band"},
		{"/api/v1/home?tier=gold", "invalid_tier"},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			rec, body := get(t, router, tt.url)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if body.Error.Code != tt.code {
				t.Fatalf("code = %q, want %q", body.Error.Code, tt.code)
			}
		})
	}
}

func TestProductNotFound(t *testing.T) {
	router := NewRouter(Deps{DB: fakeDB{}, Catalog: &fakeCatalog{}})

	rec, body := get(t, router, "/api/v1/products/does-not-exist")
	if rec.Code != http.StatusNotFound || body.Error.Code != "not_found" {
		t.Fatalf("got %d %q, want 404 not_found", rec.Code, body.Error.Code)
	}

	rec, _ = get(t, router, "/api/v1/products/ribbed-lounge-joggers")
	if rec.Code != http.StatusOK {
		t.Fatalf("existing product: status = %d, want 200", rec.Code)
	}
}

func TestHomeDefaultsToBothTiers(t *testing.T) {
	fc := &fakeCatalog{}
	router := NewRouter(Deps{DB: fakeDB{}, Catalog: fc})
	fc.gotTier = "unset"

	rec, _ := get(t, router, "/api/v1/home")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if fc.gotTier != "" {
		t.Fatalf("tier = %q, want empty (both tiers)", fc.gotTier)
	}
}
