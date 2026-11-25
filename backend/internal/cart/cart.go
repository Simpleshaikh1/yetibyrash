// Package cart is the shopper's bag. A bag is identified by a random id kept in a cookie,
// so shoppers don't need an account. Prices and stock are always read live.
package cart

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/db/sqlc"
)

// MaxQty is the most of one option a bag can hold.
const MaxQty = 20

type Line struct {
	VariantID     int64  `json:"variantId"`
	ProductID     int64  `json:"productId"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	Option        string `json:"option"` // "Espresso / M", "" for a one-option product
	SKU           string `json:"sku"`
	Image         string `json:"image"`
	UnitKobo      int64  `json:"unitKobo"`
	CompareAtKobo *int64 `json:"compareAtKobo"`
	Qty           int    `json:"qty"`
	Stock         int    `json:"stock"`
	LineKobo      int64  `json:"lineKobo"`
	// Problem stops checkout until fixed: "No longer available", "Sold out", "Only 2 left".
	Problem string `json:"problem"`
}

type Bag struct {
	Lines        []Line `json:"lines"`
	Count        int    `json:"count"` // total items, for the bag icon
	SubtotalKobo int64  `json:"subtotalKobo"`
	CanCheckout  bool   `json:"canCheckout"`
}

type Service struct{ q *sqlc.Queries }

func NewService(db sqlc.DBTX) *Service { return &Service{q: sqlc.New(db)} }

// ParseID turns the cookie value into a bag id. Anything invalid means "no bag yet".
func ParseID(s string) (pgtype.UUID, bool) {
	var id pgtype.UUID
	if s == "" || id.Scan(s) != nil {
		return pgtype.UUID{}, false
	}
	return id, true
}

// Get returns the bag; a missing or unknown id gives an empty bag.
func (s *Service) Get(ctx context.Context, id pgtype.UUID) (Bag, error) {
	bag := Bag{Lines: []Line{}}
	if !id.Valid {
		return bag, nil
	}
	rows, err := s.q.GetCartLines(ctx, id)
	if err != nil {
		return bag, fmt.Errorf("cart lines: %w", err)
	}
	bag.CanCheckout = len(rows) > 0
	for _, r := range rows {
		l := Line{
			VariantID: r.VariantID, ProductID: r.ProductID, Slug: r.Slug, Name: r.Name, SKU: r.Sku,
			Image: r.ImageKey, UnitKobo: r.PriceKobo, Qty: int(r.Qty), Stock: int(r.Stock),
			LineKobo: r.PriceKobo * int64(r.Qty),
		}
		l.Option = strings.Join(nonEmpty(r.Colour, r.Size), " / ")
		if r.CompareAtKobo.Valid {
			l.CompareAtKobo = &r.CompareAtKobo.Int64
		}
		switch {
		case !r.Available:
			l.Problem = "No longer available"
		case r.Stock == 0:
			l.Problem = "Sold out"
		case int(r.Qty) > int(r.Stock):
			l.Problem = fmt.Sprintf("Only %d left", r.Stock)
		}
		if l.Problem != "" {
			bag.CanCheckout = false
		} else {
			bag.SubtotalKobo += l.LineKobo
			bag.Count += l.Qty
		}
		bag.Lines = append(bag.Lines, l)
	}
	return bag, nil
}

// Add puts qty of an option in the bag, creating the bag if needed. It returns the bag's id.
func (s *Service) Add(ctx context.Context, id pgtype.UUID, variantID int64, qty int) (pgtype.UUID, error) {
	if qty < 1 || qty > MaxQty {
		return id, apperr.NewInvalid("qty", fmt.Sprintf("Choose a quantity from 1 to %d.", MaxQty))
	}
	v, err := s.q.GetVariantForCart(ctx, variantID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !v.Available) {
		return id, &apperr.Conflict{Code: "not_available", Message: "This option isn't available any more."}
	}
	if err != nil {
		return id, err
	}

	if id.Valid {
		exists, err := s.q.CartExists(ctx, id)
		if err != nil {
			return id, err
		}
		if !exists {
			id = pgtype.UUID{}
		}
	}
	inBag := 0
	if id.Valid {
		bag, err := s.Get(ctx, id)
		if err != nil {
			return id, err
		}
		for _, l := range bag.Lines {
			if l.VariantID == variantID {
				inBag = l.Qty
			}
		}
	}
	if v.Stock == 0 {
		return id, &apperr.Conflict{Code: "sold_out", Message: "Sorry, this just sold out."}
	}
	if inBag+qty > int(v.Stock) {
		return id, &apperr.Conflict{Code: "not_enough_stock", Message: fmt.Sprintf("Only %d left, and you already have %d in your bag.", v.Stock, inBag)}
	}

	if !id.Valid {
		if id, err = s.q.CreateCart(ctx); err != nil {
			return id, err
		}
	}
	if _, err := s.q.AddCartItem(ctx, sqlc.AddCartItemParams{CartID: id, VariantID: variantID, Qty: int32(qty)}); err != nil {
		return id, err
	}
	return id, s.q.TouchCart(ctx, id)
}

// SetQty changes a line's quantity; 0 removes it.
func (s *Service) SetQty(ctx context.Context, id pgtype.UUID, variantID int64, qty int) error {
	if !id.Valid {
		return apperr.ErrNotFound
	}
	if qty == 0 {
		return s.q.DeleteCartItem(ctx, sqlc.DeleteCartItemParams{CartID: id, VariantID: variantID})
	}
	if qty < 0 || qty > MaxQty {
		return apperr.NewInvalid("qty", fmt.Sprintf("Choose a quantity from 0 to %d.", MaxQty))
	}
	v, err := s.q.GetVariantForCart(ctx, variantID)
	if err != nil {
		return err
	}
	if qty > int(v.Stock) {
		return &apperr.Conflict{Code: "not_enough_stock", Message: fmt.Sprintf("Only %d left.", v.Stock)}
	}
	n, err := s.q.SetCartItemQty(ctx, sqlc.SetCartItemQtyParams{CartID: id, VariantID: variantID, Qty: int32(qty)})
	if err != nil {
		return err
	}
	if n == 0 {
		return apperr.ErrNotFound
	}
	return s.q.TouchCart(ctx, id)
}

func nonEmpty(parts ...string) []string {
	out := []string{}
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
