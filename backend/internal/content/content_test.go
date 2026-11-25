package content_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/catalog"
	"yetibyrash/backend/internal/content"
	"yetibyrash/backend/internal/media"
	"yetibyrash/backend/internal/staff"
	"yetibyrash/backend/internal/testdb"
)

// The admin sends a new page with an empty updatedAt; it must still decode.
func TestNewPageJSONDecodes(t *testing.T) {
	var p content.Page
	if err := json.Unmarshal([]byte(`{"id":0,"slug":"faq","title":"FAQ","body":"","published":false,"updatedAt":""}`), &p); err != nil {
		t.Fatalf("decode new page: %v", err)
	}
}

func TestHomepageAndPages(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	store, err := media.NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c := content.NewService(pool, store)
	cat := catalog.NewService(pool)
	owner := staff.Member{Name: "Owner", Role: staff.RoleOwner}

	// Banners can only link inside the shop.
	var invalid *apperr.Invalid
	_, err = c.SaveBanner(ctx, owner, content.Banner{Headline: "Detty December", ButtonText: "Shop", ButtonLink: "https://elsewhere.example", Active: true})
	if !errors.As(err, &invalid) || invalid.Field != "buttonLink" {
		t.Fatalf("outside link: err = %v", err)
	}
	// A premium-only banner, and one for both tiers that ended yesterday.
	if _, err := c.SaveBanner(ctx, owner, content.Banner{Headline: "Premium watches", ButtonText: "See watches", ButtonLink: "/shop/watches", Tier: "premium", Active: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SaveBanner(ctx, owner, content.Banner{Headline: "Old sale", ButtonText: "Shop", ButtonLink: "/shop", StartsOn: "2026-01-01", EndsOn: "2026-01-31", Active: true}); err != nil {
		t.Fatal(err)
	}

	admin, err := c.Admin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Hide best sellers, put New in first, rename it.
	secs := admin.Sections
	var order []content.Section
	for _, s := range secs {
		if s.Key == "new_in" {
			s.Title = "Just landed"
			s.ItemCount = 3
			order = append([]content.Section{s}, order...)
			continue
		}
		if s.Key == "best_sellers" {
			s.Visible = false
		}
		order = append(order, s)
	}
	if err := c.SaveSections(ctx, owner, order); err != nil {
		t.Fatal(err)
	}

	premium, err := cat.Home(ctx, catalog.TierPremium)
	if err != nil {
		t.Fatal(err)
	}
	if premium.Sections[0].Key != "new_in" || premium.Sections[0].Title != "Just landed" || len(premium.NewIn) > 3 {
		t.Fatalf("sections = %+v, new in = %d", premium.Sections, len(premium.NewIn))
	}
	for _, s := range premium.Sections {
		if s.Key == "best_sellers" {
			t.Fatal("hidden section still on the homepage")
		}
	}
	if premium.Hero == nil || premium.Hero.Headline != "Premium watches" {
		t.Fatalf("premium hero = %+v", premium.Hero)
	}
	if everyday, _ := cat.Home(ctx, catalog.TierEveryday); everyday.Hero != nil {
		t.Fatalf("everyday hero = %+v, want none (premium-only and expired banners)", everyday.Hero)
	}

	// Pages: drafts are hidden; publishing needs words.
	var about content.Page
	for _, p := range admin.Pages {
		if p.Slug == "about" {
			about = p
		}
	}
	about.Published = true
	if _, err := c.SavePage(ctx, owner, about); !errors.As(err, &invalid) || invalid.Field != "body" {
		t.Fatalf("publishing an empty page: err = %v", err)
	}
	if _, err := c.Page(ctx, "about"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("draft page: err = %v, want not found", err)
	}
	about.Body = "## Our story\n\nYetibyrash started in Lagos."
	if _, err := c.SavePage(ctx, owner, about); err != nil {
		t.Fatal(err)
	}
	if p, err := c.Page(ctx, "about"); err != nil || p.Title != "About us" {
		t.Fatalf("published page = %+v, %v", p, err)
	}

	// The announcement bar: a live sale's message first, then the staff's messages.
	if _, err := pool.Exec(ctx, `INSERT INTO promotions (name, kind, value, starts_at, ends_at, announcement)
		VALUES ('Flash', 'percent', 10, now() - interval '1 hour', now() + interval '1 day', 'Flash sale: 10% off everything')`); err != nil {
		t.Fatal(err)
	}
	site, err := c.Site(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(site.Announcements) != 3 || site.Announcements[0].Text != "Flash sale: 10% off everything" {
		t.Fatalf("announcements = %+v", site.Announcements)
	}
	if len(site.Pages) != 1 || site.Pages[0].Slug != "about" {
		t.Fatalf("footer pages = %+v", site.Pages)
	}
}
