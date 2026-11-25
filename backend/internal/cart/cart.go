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
	"yetibyrash/backend/internal/promotions"
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
	UnitKobo      int64  `json:"unitKobo"`      // after any sale
	CompareAtKobo *int64 `json:"compareAtKobo"` // crossed-out price, null = none
	SaleBadge     string `json:"saleBadge"`
	SaleID        int64  `json:"-"`
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
	// Code is the discount code on the bag (null = none). DiscountKobo is what it takes off
	// the items; free delivery is applied at checkout once the address is known.
	Code         *promotions.CodeResult `json:"code"`
	DiscountKobo int64                  `json:"discountKobo"`
	// Gift is the free-gift offer and how close the bag is to it (null = no offer running).
	Gift *promotions.Gift `json:"gift"`
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
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ProductID)
	}
	sales, err := promotions.LoadSales(ctx, s.q, ids)
	if err != nil {
		return bag, err
	}
	for _, r := range rows {
		price := sales.Apply(r.ProductID, r.PriceKobo, r.CompareAtKobo.Int64)
		l := Line{
			VariantID: r.VariantID, ProductID: r.ProductID, Slug: r.Slug, Name: r.Name, SKU: r.Sku,
			Image: r.ImageKey, UnitKobo: price.Kobo, Qty: int(r.Qty), Stock: int(r.Stock),
			LineKobo: price.Kobo * int64(r.Qty), SaleBadge: price.Badge, SaleID: price.SaleID,
		}
		l.Option = strings.Join(nonEmpty(r.Colour, r.Size), " / ")
		if price.CompareAtKobo > 0 {
			l.CompareAtKobo = &price.CompareAtKobo
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
	if err := s.applyOffers(ctx, id, &bag); err != nil {
		return bag, err
	}
	return bag, nil
}

// applyOffers adds the bag's discount code and the free-gift progress.
func (s *Service) applyOffers(ctx context.Context, id pgtype.UUID, bag *Bag) error {
	if bag.SubtotalKobo == 0 {
		return nil
	}
	code, err := s.q.GetCartCode(ctx, id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("cart code: %w", err)
	}
	if code != "" {
		res, _, err := promotions.EvaluateCode(ctx, s.q, code, promotions.CodeCheck{SubtotalKobo: bag.SubtotalKobo}, false)
		switch {
		case errors.Is(err, promotions.ErrNoSuchCode):
			res = promotions.CodeResult{Code: code, Problem: "This code has ended. Remove it to continue."}
		case err != nil:
			return err
		}
		bag.Code = &res
		bag.DiscountKobo = res.DiscountKobo
	}
	bag.Gift, err = promotions.LiveGift(ctx, s.q, bag.SubtotalKobo)
	return err
}

// ApplyCode puts a discount code on the bag. An unknown code is refused; a code the bag
// doesn't qualify for yet is kept, and the bag says what's missing.
func (s *Service) ApplyCode(ctx context.Context, id pgtype.UUID, code string) error {
	code = promotions.NormalizeCode(code)
	if code == "" {
		return apperr.NewInvalid("code", "Enter a discount code.")
	}
	if !id.Valid {
		return apperr.NewInvalid("code", "Add something to your bag first.")
	}
	if exists, err := s.q.CartExists(ctx, id); err != nil {
		return err
	} else if !exists {
		return apperr.NewInvalid("code", "Add something to your bag first.")
	}
	if _, _, err := promotions.EvaluateCode(ctx, s.q, code, promotions.CodeCheck{}, false); errors.Is(err, promotions.ErrNoSuchCode) {
		return apperr.NewInvalid("code", "We don't recognise that code. Check the spelling.")
	} else if err != nil {
		return err
	}
	return s.q.SetCartCode(ctx, sqlc.SetCartCodeParams{ID: id, DiscountCode: code})
}

// RemoveCode takes the discount code off the bag.
func (s *Service) RemoveCode(ctx context.Context, id pgtype.UUID) error {
	if !id.Valid {
		return nil
	}
	return s.q.SetCartCode(ctx, sqlc.SetCartCodeParams{ID: id, DiscountCode: ""})
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
