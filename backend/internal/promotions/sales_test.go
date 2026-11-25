package promotions

import (
	"testing"

	"yetibyrash/backend/internal/db/sqlc"
)

func TestSalePrice(t *testing.T) {
	cases := []struct {
		kind        sqlc.DiscountKind
		value, kobo int64
		want        int64
	}{
		{sqlc.DiscountKindPercent, 20, 1450000, 1160000}, // ₦14,500 → ₦11,600
		{sqlc.DiscountKindPercent, 15, 999900, 849900},   // ₦8,499.15 rounds down to ₦8,499
		{sqlc.DiscountKindPercent, 95, 1000000, 1000000}, // over 90% never applies
		{sqlc.DiscountKindFixed, 200000, 1000000, 800000},
		{sqlc.DiscountKindFixed, 995000, 1000000, 1000000}, // would leave ₦50: under ₦100
	}
	for _, c := range cases {
		if got := SalePrice(c.kind, c.value, c.kobo); got != c.want {
			t.Errorf("SalePrice(%s %d, %d) = %d, want %d", c.kind, c.value, c.kobo, got, c.want)
		}
	}
}

func TestApplyPicksBiggestSavingAndRespectsScope(t *testing.T) {
	s := &Sales{
		live: []Sale{
			{ID: 1, Kind: sqlc.DiscountKindPercent, Value: 10, Scope: sqlc.PromotionScopeAll, Badge: "10% OFF", ExcludeOnSale: true},
			{ID: 2, Kind: sqlc.DiscountKindPercent, Value: 25, Scope: sqlc.PromotionScopeCategory, CategoryID: 7, Badge: "BAGS 25%"},
			{ID: 3, Kind: sqlc.DiscountKindFixed, Value: 300000, Scope: sqlc.PromotionScopeTier, Tier: "premium", Badge: "₦3,000 OFF"},
		},
		targets: map[int64]target{
			100: {tier: "everyday", categoryID: 70, parentID: 7}, // a bag (sub-category of 7)
			200: {tier: "everyday", categoryID: 9},               // earrings
			300: {tier: "everyday", categoryID: 9, onSale: true}, // already marked down
			400: {tier: "premium", categoryID: 9},                // premium watch
		},
	}
	check := func(id, kobo, compareAt, wantKobo, wantCompare, wantSale int64) {
		t.Helper()
		p := s.Apply(id, kobo, compareAt)
		if p.Kobo != wantKobo || p.CompareAtKobo != wantCompare || p.SaleID != wantSale {
			t.Errorf("Apply(%d) = %+v, want kobo %d compare %d sale %d", id, p, wantKobo, wantCompare, wantSale)
		}
	}
	check(100, 2000000, 0, 1500000, 2000000, 2)                   // the parent category's 25% beats 10% everything
	check(200, 2000000, 0, 1800000, 2000000, 1)                   // only the store-wide 10%
	check(300, 750000, 950000, 750000, 950000, 0)                 // skipped: already on sale
	check(400, 2000000, 0, 1700000, 2000000, 3)                   // ₦3,000 off beats 10%
	check(999, 2000000, 0, 2000000, 0, 0)                         // unknown product: unchanged
	if p := (*Sales)(nil).Apply(1, 500000, 0); p.Kobo != 500000 { // no sales loaded
		t.Errorf("nil Sales changed the price: %+v", p)
	}
}

func TestDays(t *testing.T) {
	start, err := DayStart("startsOn", "2026-12-01")
	if err != nil {
		t.Fatal(err)
	}
	end, err := DayEnd("endsOn", "2026-12-31")
	if err != nil {
		t.Fatal(err)
	}
	if got := start.Time.UTC().Format("2006-01-02 15:04"); got != "2026-11-30 23:00" {
		t.Errorf("start = %s, want midnight Lagos (23:00 UTC the day before)", got)
	}
	if StartDay(start) != "2026-12-01" || EndDay(end) != "2026-12-31" {
		t.Errorf("round trip = %s – %s", StartDay(start), EndDay(end))
	}
	if err := CheckDays(end, start); err == nil {
		t.Error("an end before the start was accepted")
	}
	if _, err := DayStart("startsOn", "1/12/2026"); err == nil {
		t.Error("a badly formatted day was accepted")
	}
}

func TestNormalizeCode(t *testing.T) {
	if got := NormalizeCode(" welcome 10 "); got != "WELCOME10" {
		t.Errorf("NormalizeCode = %q", got)
	}
}
