package orders_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/orders"
)

func (e *env) variant(t *testing.T, sku string) int64 {
	t.Helper()
	var id int64
	if err := e.pool.QueryRow(context.Background(), `SELECT id FROM variants WHERE sku = $1`, sku).Scan(&id); err != nil {
		t.Fatalf("variant %s: %v", sku, err)
	}
	return id
}

func (e *env) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func (e *env) int(t *testing.T, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// A sale, a discount code and a free gift together, then the unpaid order giving them back.
func TestCheckoutWithPromotions(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	// 25% off bags (a top category: covers its sub-categories), 10% off everything except
	// items already marked down, ₦5,000 off over ₦20,000 for one use, a free hoop above ₦20,000.
	e.exec(t, `INSERT INTO promotions (name, kind, value, applies_to, category_id, starts_at, ends_at, badge_text, announcement)
		SELECT 'Bag week', 'percent', 25, 'category', id, now() - interval '1 hour', now() + interval '2 days', '25% OFF BAGS', 'Bag week: 25% off'
		FROM categories WHERE slug = 'bags'`)
	e.exec(t, `INSERT INTO promotions (name, kind, value, starts_at, ends_at, badge_text)
		VALUES ('Everything', 'percent', 10, now() - interval '1 hour', now() + interval '2 days', '10% OFF')`)
	e.exec(t, `INSERT INTO discount_codes (code, kind, value, min_order_kobo, max_uses) VALUES ('SAVE5', 'fixed', 500000, 2000000, 1)`)
	hoops := e.variant(t, "ER-HUG-GOL")
	e.exec(t, `INSERT INTO gift_offers (threshold_kobo, variant_id, quantity) VALUES (2000000, $1, 5)`, hoops)
	hoopStock := e.int(t, `SELECT stock FROM variants WHERE id = $1`, hoops)

	tote := e.variant(t, "BG-TOT-TAN")
	cartID, err := e.cart.Add(ctx, pgtype.UUID{}, tote, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.cart.ApplyCode(ctx, cartID, "nosuch"); err == nil {
		t.Fatal("an unknown code was accepted")
	}
	if err := e.cart.ApplyCode(ctx, cartID, " save5 "); err != nil {
		t.Fatal(err)
	}

	bag, err := e.cart.Get(ctx, cartID)
	if err != nil {
		t.Fatal(err)
	}
	l := bag.Lines[0]
	if l.UnitKobo != 2400000 || l.CompareAtKobo == nil || *l.CompareAtKobo != 3200000 || l.SaleBadge != "25% OFF BAGS" {
		t.Fatalf("tote line = %+v, want ₦24,000 (25%% off ₦32,000)", l)
	}
	if bag.Code == nil || bag.Code.Problem != "" || bag.DiscountKobo != 500000 {
		t.Fatalf("bag code = %+v, discount %d", bag.Code, bag.DiscountKobo)
	}
	if !bag.Gift.Qualifies() || bag.Gift.Name != "Gold huggie hoops" {
		t.Fatalf("bag gift = %+v", bag.Gift)
	}

	started, err := e.orders.Checkout(ctx, cartID, 0, orders.CheckoutInput{
		Name: "Amaka Obi", Phone: "08030000000", Email: "amaka@example.test", StateID: e.lagos, LgaID: e.etiOsa,
		Street: "12 Admiralty Way", Speed: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}
	var subtotal, discount, delivery, total int64
	var code string
	var promo, offer pgtype.Int8
	if err := e.pool.QueryRow(ctx, `SELECT subtotal_kobo, discount_kobo, delivery_kobo, total_kobo, discount_code, promotion_id, gift_offer_id
		FROM orders WHERE number = $1`, started.OrderNumber).Scan(&subtotal, &discount, &delivery, &total, &code, &promo, &offer); err != nil {
		t.Fatal(err)
	}
	if subtotal != 2400000 || discount != 500000 || code != "SAVE5" || total != subtotal-discount+delivery || !promo.Valid || !offer.Valid {
		t.Fatalf("order: subtotal %d discount %d delivery %d total %d code %q promo %v gift %v", subtotal, discount, delivery, total, code, promo, offer)
	}
	if n := e.int(t, `SELECT COUNT(*) FROM order_items i JOIN orders o ON o.id = i.order_id
		WHERE o.number = $1 AND i.is_gift AND i.unit_kobo = 0 AND i.variant_id = $2`, started.OrderNumber, hoops); n != 1 {
		t.Fatalf("gift lines = %d, want 1", n)
	}
	if e.int(t, `SELECT used FROM discount_codes WHERE code = 'SAVE5'`) != 1 || e.int(t, `SELECT given FROM gift_offers`) != 1 {
		t.Fatal("code use or gift wasn't counted")
	}
	if got := e.int(t, `SELECT stock FROM variants WHERE id = $1`, hoops); got != hoopStock-1 {
		t.Fatalf("hoop stock = %d, want %d", got, hoopStock-1)
	}

	// The code allows one use: a second shopper gets a clear refusal.
	cart2, err := e.cart.Add(ctx, pgtype.UUID{}, e.variant(t, "BG-TOT-BLA"), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.cart.ApplyCode(ctx, cart2, "SAVE5"); err != nil {
		t.Fatal(err)
	}
	if bag2, _ := e.cart.Get(ctx, cart2); bag2.Code == nil || bag2.Code.Problem != "This code has been fully used." || bag2.DiscountKobo != 0 {
		t.Fatalf("second bag code = %+v", bag2.Code)
	}
	var conflict *apperr.Conflict
	_, err = e.orders.Checkout(ctx, cart2, 0, orders.CheckoutInput{
		Name: "Bisi Ade", Phone: "08031111111", Email: "bisi@example.test", StateID: e.lagos, LgaID: e.etiOsa,
		Street: "3 Bourdillon Road", Speed: "standard",
	})
	if !errors.As(err, &conflict) || conflict.Code != "code_not_valid" {
		t.Fatalf("checkout with a used-up code: err = %v", err)
	}

	// A product already marked down skips the store-wide 10%.
	cart3, err := e.cart.Add(ctx, pgtype.UUID{}, e.variant(t, "BD-STR-GOL-M"), 1)
	if err != nil {
		t.Fatal(err)
	}
	if bag3, _ := e.cart.Get(ctx, cart3); bag3.Lines[0].UnitKobo != 750000 || bag3.Lines[0].SaleBadge != "" {
		t.Fatalf("bracelet line = %+v, want its own ₦7,500 markdown only", bag3.Lines[0])
	}

	// Left unpaid: the hold runs out, and the code use, gift and stock all come back.
	e.exec(t, `UPDATE orders SET created_at = now() - interval '2 hours' WHERE number = $1`, started.OrderNumber)
	if err := e.orders.ReleaseStale(ctx); err != nil {
		t.Fatal(err)
	}
	if e.int(t, `SELECT used FROM discount_codes WHERE code = 'SAVE5'`) != 0 || e.int(t, `SELECT given FROM gift_offers`) != 0 {
		t.Fatal("abandoning didn't give back the code use or the gift")
	}
	if got := e.int(t, `SELECT stock FROM variants WHERE id = $1`, hoops); got != hoopStock {
		t.Fatalf("hoop stock = %d, want %d back", got, hoopStock)
	}
}
