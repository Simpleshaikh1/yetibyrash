package promotions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"yetibyrash/backend/internal/db/sqlc"
)

// Gift is the live free-gift offer as the bag shows it.
type Gift struct {
	OfferID       int64  `json:"-"`
	VariantID     int64  `json:"-"`
	SKU           string `json:"-"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	Option        string `json:"option"` // "Gold", "" for one option
	Image         string `json:"image"`
	ThresholdKobo int64  `json:"thresholdKobo"`
	// RemainingKobo is how much more the shopper needs to spend; 0 = the gift is in the bag.
	RemainingKobo int64 `json:"remainingKobo"`
}

// Qualifies reports whether the bag gets the gift.
func (g *Gift) Qualifies() bool { return g != nil && g.RemainingKobo == 0 }

// LiveGift returns the running offer measured against a bag's item total (after sale prices,
// before any code), or nil when there is no offer or the gifts have run out.
func LiveGift(ctx context.Context, q *sqlc.Queries, subtotalKobo int64) (*Gift, error) {
	r, err := q.LiveGift(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("live gift: %w", err)
	}
	g := &Gift{
		OfferID: r.ID, VariantID: r.VariantID, SKU: r.Sku, Slug: r.ProductSlug, Name: r.ProductName,
		Option: joinNonEmpty(" / ", r.Colour, r.Size), Image: r.ImageKey, ThresholdKobo: r.ThresholdKobo,
	}
	g.RemainingKobo = max(0, r.ThresholdKobo-subtotalKobo)
	return g, nil
}

func joinNonEmpty(sep string, parts ...string) string {
	out := []string{}
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}
