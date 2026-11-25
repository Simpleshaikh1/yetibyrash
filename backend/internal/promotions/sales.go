// Package promotions works out sale prices, discount codes and free gifts.
// Every price the shop shows or charges goes through here, so the product card,
// the bag and checkout always agree.
package promotions

import (
	"context"
	"fmt"
	"time"

	"yetibyrash/backend/internal/db/sqlc"
)

// Sale is a live promotion.
type Sale struct {
	ID            int64
	Name          string
	Kind          sqlc.DiscountKind // percent | fixed
	Value         int64             // percent, or kobo off each item
	Scope         sqlc.PromotionScope
	Tier          string
	CategoryID    int64
	Badge         string
	Announcement  string
	ExcludeOnSale bool
	EndsAt        time.Time
}

type target struct {
	tier       string
	categoryID int64
	parentID   int64
	onSale     bool
}

// Sales prices a set of products during whatever sales are live right now.
type Sales struct {
	live    []Sale
	targets map[int64]target
}

// LiveSales loads the running sales (for the announcement bar and the admin).
func LiveSales(ctx context.Context, q *sqlc.Queries) ([]Sale, error) {
	rows, err := q.LiveSales(ctx)
	if err != nil {
		return nil, fmt.Errorf("live sales: %w", err)
	}
	out := make([]Sale, 0, len(rows))
	for _, r := range rows {
		s := Sale{
			ID: r.ID, Name: r.Name, Kind: r.Kind, Value: r.Value, Scope: r.AppliesTo,
			Badge: r.BadgeText, Announcement: r.Announcement, ExcludeOnSale: r.ExcludeOnSale, EndsAt: r.EndsAt.Time,
		}
		if r.Tier.Valid {
			s.Tier = string(r.Tier.ProductTier)
		}
		if r.CategoryID.Valid {
			s.CategoryID = r.CategoryID.Int64
		}
		out = append(out, s)
	}
	return out, nil
}

// LoadSales prepares sale prices for the given products.
func LoadSales(ctx context.Context, q *sqlc.Queries, productIDs []int64) (*Sales, error) {
	s := &Sales{targets: map[int64]target{}}
	var err error
	if s.live, err = LiveSales(ctx, q); err != nil {
		return nil, err
	}
	if len(s.live) == 0 || len(productIDs) == 0 {
		return s, nil
	}
	rows, err := q.SaleTargets(ctx, productIDs)
	if err != nil {
		return nil, fmt.Errorf("sale targets: %w", err)
	}
	for _, r := range rows {
		s.targets[r.ID] = target{tier: string(r.Tier), categoryID: r.CategoryID, parentID: r.ParentID, onSale: r.OnSale}
	}
	return s, nil
}

// Price is one price after any sale.
type Price struct {
	Kobo          int64  // what the shopper pays
	CompareAtKobo int64  // the crossed-out price, 0 = none
	SaleID        int64  // 0 = no sale applied
	Badge         string // e.g. "20% OFF · BLACK FRIDAY"
}

// Apply prices one option of a product. compareAt is the option's own old price (0 = none).
// When two sales cover the product, the one that saves the shopper more wins.
func (s *Sales) Apply(productID, kobo, compareAt int64) Price {
	out := Price{Kobo: kobo}
	if compareAt > kobo {
		out.CompareAtKobo = compareAt
	}
	if s == nil {
		return out
	}
	t, ok := s.targets[productID]
	if !ok {
		return out
	}
	for _, sale := range s.live {
		if !sale.covers(t) {
			continue
		}
		p := SalePrice(sale.Kind, sale.Value, kobo)
		if p >= out.Kobo {
			continue
		}
		out.Kobo = p
		out.SaleID = sale.ID
		out.Badge = sale.Badge
		out.CompareAtKobo = max(compareAt, kobo)
	}
	return out
}

func (sale Sale) covers(t target) bool {
	if sale.ExcludeOnSale && t.onSale {
		return false
	}
	switch sale.Scope {
	case sqlc.PromotionScopeTier:
		return t.tier == sale.Tier
	case sqlc.PromotionScopeCategory:
		return t.categoryID == sale.CategoryID || t.parentID == sale.CategoryID
	}
	return true
}

// SalePrice takes a percent or a fixed amount off, rounding down to whole naira.
// A fixed discount that would leave less than ₦100 doesn't apply.
func SalePrice(kind sqlc.DiscountKind, value, kobo int64) int64 {
	switch kind {
	case sqlc.DiscountKindPercent:
		if value <= 0 || value > 90 {
			return kobo
		}
		return kobo * (100 - value) / 100 / 100 * 100
	case sqlc.DiscountKindFixed:
		if kobo-value < 10000 {
			return kobo
		}
		return kobo - value
	}
	return kobo
}
