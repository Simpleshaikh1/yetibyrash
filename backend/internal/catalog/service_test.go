package catalog_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"yetibyrash/backend/internal/catalog"
	"yetibyrash/backend/internal/db"
	"yetibyrash/backend/internal/seed"
)

// newTestDB creates an empty database, migrates it, loads the example products,
// and drops it when the test ends. Skips if Postgres isn't running.
func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		adminURL = "postgres://yetibyrash:yetibyrash@localhost:5432/yetibyrash?sslmode=disable"
	}
	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := admin.Ping(pingCtx); err != nil {
		admin.Close()
		t.Skipf("Postgres not reachable (start it with `docker compose up -d`): %v", err)
	}

	name := fmt.Sprintf("yetibyrash_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create test database: %v", err)
	}
	t.Cleanup(func() {
		admin.Exec(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)")
		admin.Close()
	})

	u, _ := url.Parse(adminURL)
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := seed.Demo(ctx, pool); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return pool
}

func slugs(items []catalog.ProductCard) []string {
	out := make([]string, len(items))
	for i, p := range items {
		out[i] = p.Slug
	}
	return out
}

func TestCatalogAgainstDemoData(t *testing.T) {
	pool := newTestDB(t)
	svc := catalog.NewService(pool)
	ctx := context.Background()

	t.Run("categories", func(t *testing.T) {
		cats, err := svc.Categories(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(cats) != 5 {
			t.Fatalf("got %d top categories, want 5", len(cats))
		}
		if cats[0].Slug != "jewelry" || len(cats[0].Children) != 6 {
			t.Fatalf("first category = %s with %d children, want jewelry with 6", cats[0].Slug, len(cats[0].Children))
		}
	})

	all, err := svc.ListProducts(ctx, catalog.ProductFilter{})
	if err != nil {
		t.Fatal(err)
	}
	bandID := func(label string) int64 {
		for _, b := range all.Filters.PriceBands {
			if b.Label == label {
				return b.ID
			}
		}
		t.Fatalf("no price band %q", label)
		return 0
	}

	tests := []struct {
		name   string
		filter catalog.ProductFilter
		want   int
	}{
		{"everything", catalog.ProductFilter{}, 14},
		{"top category includes sub-categories", catalog.ProductFilter{CategorySlug: "clothing"}, 4},
		{"sub-category", catalog.ProductFilter{CategorySlug: "earrings"}, 2},
		{"premium tier", catalog.ProductFilter{Tier: catalog.TierPremium}, 4},
		{"colour", catalog.ProductFilter{Colour: "Espresso"}, 3},
		{"size", catalog.ProductFilter{Size: "XXL"}, 1},
		{"price band under ₦10,000", catalog.ProductFilter{BandID: bandID("Under ₦10,000")}, 5},
		{"category and tier together", catalog.ProductFilter{CategorySlug: "bags", Tier: catalog.TierPremium}, 1},
		{"page 2 is empty", catalog.ProductFilter{Page: 2}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.ListProducts(ctx, tt.filter)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Items) != tt.want {
				t.Fatalf("got %d products %v, want %d", len(got.Items), slugs(got.Items), tt.want)
			}
		})
	}

	t.Run("cheapest first", func(t *testing.T) {
		got, err := svc.ListProducts(ctx, catalog.ProductFilter{Sort: catalog.SortPriceAsc})
		if err != nil {
			t.Fatal(err)
		}
		if got.Items[0].Slug != "opaque-black-tights" || got.Items[0].PriceKobo != 350000 {
			t.Fatalf("first = %s at %d kobo, want opaque-black-tights at 350000", got.Items[0].Slug, got.Items[0].PriceKobo)
		}
	})

	t.Run("card details", func(t *testing.T) {
		cards := map[string]catalog.ProductCard{}
		for _, p := range all.Items {
			cards[p.Slug] = p
		}
		if c := cards["beaded-stretch-bracelet"]; c.CompareAtKobo == nil || *c.CompareAtKobo != 950000 {
			t.Fatalf("sale bracelet compare-at = %v, want 950000", c.CompareAtKobo)
		}
		if c := cards["cropped-baby-tee"]; c.InStock || !c.Preorder {
			t.Fatalf("baby tee inStock=%v preorder=%v, want sold out with pre-order", c.InStock, c.Preorder)
		}
		if c := cards["opaque-black-tights"]; c.PriceKobo != 350000 || c.MaxPriceKobo != 420000 {
			t.Fatalf("tights price range = %d–%d, want 350000–420000", c.PriceKobo, c.MaxPriceKobo)
		}
	})

	t.Run("product page", func(t *testing.T) {
		p, err := svc.Product(ctx, "ribbed-lounge-joggers")
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Variants) != 20 || len(p.Colours) != 4 || p.SizeSet == nil || len(p.SizeSet.Sizes) != 5 {
			t.Fatalf("variants=%d colours=%d sizeSet=%v, want 20, 4 and 5 sizes", len(p.Variants), len(p.Colours), p.SizeSet)
		}
		if len(p.Details) != 4 || len(p.Pairings) != 3 {
			t.Fatalf("details=%d pairings=%d, want 4 and 3", len(p.Details), len(p.Pairings))
		}
		if p.Parent == nil || p.Parent.Slug != "clothing" || p.ReturnRule != "7 days, unworn with tags" {
			t.Fatalf("parent=%v returnRule=%q", p.Parent, p.ReturnRule)
		}
		stock := map[string]int{}
		for _, v := range p.Variants {
			stock[v.Colour+"/"+v.Size] = v.Stock
		}
		if stock["Blush/L"] != 0 || stock["Espresso/M"] != 2 {
			t.Fatalf("Blush/L=%d Espresso/M=%d, want 0 and 2", stock["Blush/L"], stock["Espresso/M"])
		}
		if p.LowStockThreshold != 6 {
			t.Fatalf("low stock threshold = %d, want 6", p.LowStockThreshold)
		}
	})

	t.Run("one-size product has no size set", func(t *testing.T) {
		p, err := svc.Product(ctx, "mini-structured-tote")
		if err != nil {
			t.Fatal(err)
		}
		if p.SizeSet != nil || len(p.Variants) != 4 || len(p.Details) != 6 {
			t.Fatalf("sizeSet=%v variants=%d details=%d, want nil, 4 and 6", p.SizeSet, len(p.Variants), len(p.Details))
		}
	})

	t.Run("unknown product", func(t *testing.T) {
		if _, err := svc.Product(ctx, "nope"); !errors.Is(err, catalog.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("home", func(t *testing.T) {
		h, err := svc.Home(ctx, catalog.TierEveryday)
		if err != nil {
			t.Fatal(err)
		}
		if len(h.Categories) != 5 || len(h.NewIn) != 6 || h.Tagline != "Elegance meets affordability" {
			t.Fatalf("categories=%d newIn=%d tagline=%q", len(h.Categories), len(h.NewIn), h.Tagline)
		}
		for _, p := range h.NewIn {
			if p.Tier != catalog.TierEveryday {
				t.Fatalf("%s is %s, want only everyday products", p.Slug, p.Tier)
			}
		}
	})
}
