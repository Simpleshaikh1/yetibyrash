// Package orders turns a bag into an order, takes payment through Paystack, and keeps stock right.
//
// Money flow:
//  1. Checkout locks the options being bought, recalculates every price on the server,
//     takes the stock (reserving it), and saves the order as pending_payment.
//  2. The shopper pays on Paystack.
//  3. Paystack's webhook, or our own check when the shopper returns, calls ConfirmPayment,
//     which marks the order paid. It is safe to call twice.
//  4. Orders not paid within HoldMinutes are abandoned and their stock goes back.
package orders

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/cart"
	"yetibyrash/backend/internal/db/sqlc"
	"yetibyrash/backend/internal/delivery"
	"yetibyrash/backend/internal/paystack"
	"yetibyrash/backend/internal/promotions"
)

// HoldMinutes is how long stock stays reserved for an unpaid order.
const HoldMinutes = 30

type Service struct {
	pool     *pgxpool.Pool
	q        *sqlc.Queries
	cart     *cart.Service
	delivery *delivery.Service
	pay      paystack.Provider
	shopURL  string // e.g. http://localhost:5173, for Paystack's return links
}

func NewService(pool *pgxpool.Pool, d *delivery.Service, pay paystack.Provider, shopURL string) *Service {
	return &Service{
		pool: pool, q: sqlc.New(pool), cart: cart.NewService(pool), delivery: d, pay: pay,
		shopURL: strings.TrimRight(shopURL, "/"),
	}
}

// CheckoutInput is the checkout form.
type CheckoutInput struct {
	Name     string         `json:"name"`
	Phone    string         `json:"phone"`
	Email    string         `json:"email"`
	StateID  int64          `json:"stateId"`
	LgaID    int64          `json:"lgaId"`   // when the state has listed LGAs
	LgaText  string         `json:"lgaText"` // otherwise
	Street   string         `json:"street"`
	Landmark string         `json:"landmark"`
	Speed    delivery.Speed `json:"speed"`
	// SaveAddress keeps this address on the signed-in customer's account.
	SaveAddress bool `json:"saveAddress"`
}

// Quote is the checkout summary before paying.
type Quote struct {
	SubtotalKobo int64             `json:"subtotalKobo"`
	Options      []delivery.Option `json:"options"`
	ZoneName     string            `json:"zoneName"`
	// The bag's discount code for this address: DiscountKobo comes off the items, and
	// FreeDelivery makes the chosen delivery free. CodeProblem says why it doesn't apply.
	Code         string `json:"code"`
	DiscountKobo int64  `json:"discountKobo"`
	FreeDelivery bool   `json:"freeDelivery"`
	CodeProblem  string `json:"codeProblem"`
	// Gift is the free gift going in with this order, "" for none.
	Gift string `json:"gift"`
}

// Quote prices the bag for a destination without creating anything.
func (s *Service) Quote(ctx context.Context, cartID pgtype.UUID, stateID, lgaID int64, lgaText string) (Quote, error) {
	bag, err := s.cart.Get(ctx, cartID)
	if err != nil {
		return Quote{}, err
	}
	if len(bag.Lines) == 0 {
		return Quote{}, &apperr.Conflict{Code: "empty_bag", Message: "Your bag is empty."}
	}
	dest, err := s.delivery.Resolve(ctx, stateID, lgaID, lgaText)
	if err != nil {
		return Quote{}, err
	}
	opts, err := s.delivery.Options(ctx, dest, bag.SubtotalKobo)
	if err != nil {
		return Quote{}, err
	}
	quote := Quote{SubtotalKobo: bag.SubtotalKobo, Options: opts, ZoneName: dest.Zone.Name}
	if bag.Gift.Qualifies() {
		quote.Gift = bag.Gift.Name
	}
	if bag.Code != nil {
		res, _, err := promotions.EvaluateCode(ctx, s.q, bag.Code.Code, promotions.CodeCheck{SubtotalKobo: bag.SubtotalKobo, ZoneID: dest.Zone.ID}, false)
		switch {
		case errors.Is(err, promotions.ErrNoSuchCode):
			quote.Code, quote.CodeProblem = bag.Code.Code, "This code has ended. Remove it from your bag to continue."
		case err != nil:
			return Quote{}, err
		default:
			quote.Code, quote.DiscountKobo, quote.FreeDelivery, quote.CodeProblem = res.Code, res.DiscountKobo, res.FreeDelivery, res.Problem
		}
	}
	return quote, nil
}

// Started is a checkout waiting for payment.
type Started struct {
	OrderNumber string `json:"orderNumber"`
	PaymentURL  string `json:"paymentUrl"`
}

// Checkout creates the order and starts the Paystack payment. customerID is the signed-in
// shopper (0 for a guest).
func (s *Service) Checkout(ctx context.Context, cartID pgtype.UUID, customerID int64, in CheckoutInput) (Started, error) {
	if err := cleanCheckout(&in); err != nil {
		return Started{}, err
	}
	bag, err := s.cart.Get(ctx, cartID)
	if err != nil {
		return Started{}, err
	}
	if len(bag.Lines) == 0 {
		return Started{}, &apperr.Conflict{Code: "empty_bag", Message: "Your bag is empty."}
	}
	dest, err := s.delivery.Resolve(ctx, in.StateID, in.LgaID, in.LgaText)
	if err != nil {
		return Started{}, err
	}

	ref := "YBP-" + randomHex(8)
	var order sqlc.InsertOrderRow
	var total int64

	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		ids := make([]int64, 0, len(bag.Lines))
		for _, l := range bag.Lines {
			ids = append(ids, l.VariantID)
		}
		locked, err := q.LockVariantsForCheckout(ctx, ids)
		if err != nil {
			return err
		}
		live := map[int64]sqlc.LockVariantsForCheckoutRow{}
		for _, v := range locked {
			live[v.ID] = v
		}

		// Prices and stock come from the locked rows, never from the browser.
		productIDs := make([]int64, 0, len(bag.Lines))
		for _, l := range bag.Lines {
			productIDs = append(productIDs, l.ProductID)
		}
		sales, err := promotions.LoadSales(ctx, q, productIDs)
		if err != nil {
			return err
		}
		var subtotal int64
		var saleID int64
		unit := map[int64]int64{}
		var problems []string
		for _, l := range bag.Lines {
			v, ok := live[l.VariantID]
			switch {
			case !ok || !v.Available:
				problems = append(problems, fmt.Sprintf("%s is no longer available", label(l)))
			case int(v.Stock) < l.Qty && v.Stock == 0:
				problems = append(problems, fmt.Sprintf("%s just sold out", label(l)))
			case int(v.Stock) < l.Qty:
				problems = append(problems, fmt.Sprintf("only %d of %s left", v.Stock, label(l)))
			}
			price := sales.Apply(l.ProductID, v.PriceKobo, 0)
			unit[l.VariantID] = price.Kobo
			if saleID == 0 {
				saleID = price.SaleID
			}
			subtotal += price.Kobo * int64(l.Qty)
		}
		if len(problems) > 0 {
			return &apperr.Conflict{Code: "stock_changed", Message: "Your bag changed: " + strings.Join(problems, "; ") + ". Update your bag and try again."}
		}

		// Discount code: checked again here with the address and customer, and its use
		// reserved under a lock so a limited code can't be overused.
		var discount int64
		freeDelivery := false
		code := ""
		if bag.Code != nil {
			res, codeID, err := promotions.EvaluateCode(ctx, q, bag.Code.Code, promotions.CodeCheck{
				SubtotalKobo: subtotal, ZoneID: dest.Zone.ID, Phone: in.Phone, Email: in.Email,
			}, true)
			if errors.Is(err, promotions.ErrNoSuchCode) {
				return &apperr.Conflict{Code: "code_ended", Message: "Your discount code has ended. Remove it from your bag to continue."}
			}
			if err != nil {
				return err
			}
			if res.Problem != "" {
				return &apperr.Conflict{Code: "code_not_valid", Message: res.Problem + " Remove the code from your bag to continue without it."}
			}
			if err := q.UseCode(ctx, codeID); err != nil {
				return err
			}
			code, discount, freeDelivery = res.Code, res.DiscountKobo, res.FreeDelivery
		}

		// Free gift: measured on the items after sale prices, before the code.
		gift, err := promotions.LiveGift(ctx, q, subtotal)
		if err != nil {
			return err
		}
		if gift.Qualifies() {
			g, err := q.LockGift(ctx, gift.OfferID)
			if err != nil {
				return err
			}
			if g.Given >= g.Quantity {
				gift = nil // the last one went a moment ago
			} else if err := q.GiveGift(ctx, gift.OfferID); err != nil {
				return err
			}
		} else {
			gift = nil
		}

		opts, err := s.delivery.Options(ctx, dest, subtotal)
		if err != nil {
			return err
		}
		var opt *delivery.Option
		for i := range opts {
			if opts[i].Speed == in.Speed {
				opt = &opts[i]
			}
		}
		if opt == nil {
			return apperr.NewInvalid("speed", "That delivery option isn't available for your address. Choose another.")
		}
		if freeDelivery {
			discount += opt.FeeKobo
		}
		total = subtotal - discount + opt.FeeKobo
		from, _ := time.Parse(time.DateOnly, opt.From)
		to, _ := time.Parse(time.DateOnly, opt.To)

		params := sqlc.InsertOrderParams{
			CartID: cartID, PaymentReference: ref,
			CustomerName: in.Name, CustomerPhone: in.Phone, CustomerEmail: in.Email,
			StateName: dest.StateName, LgaName: dest.LgaName, Street: in.Street, Landmark: in.Landmark,
			ZoneName: dest.Zone.Name, Speed: sqlc.DeliverySpeed(in.Speed),
			SubtotalKobo: subtotal, DeliveryKobo: opt.FeeKobo, TotalKobo: total,
			ExpectedFrom: pgtype.Date{Time: from, Valid: true}, ExpectedTo: pgtype.Date{Time: to, Valid: true},
			DiscountKobo: discount, DiscountCode: code,
			PromotionID: pgtype.Int8{Int64: saleID, Valid: saleID != 0},
			CustomerID:  pgtype.Int8{Int64: customerID, Valid: customerID != 0},
		}
		if gift != nil {
			params.GiftOfferID = pgtype.Int8{Int64: gift.OfferID, Valid: true}
		}
		order, err = q.InsertOrder(ctx, params)
		if err != nil {
			return err
		}
		for _, l := range bag.Lines {
			v := live[l.VariantID]
			if err := q.InsertOrderItem(ctx, sqlc.InsertOrderItemParams{
				OrderID: order.ID, VariantID: l.VariantID, ProductName: l.Name, ProductSlug: l.Slug,
				VariantLabel: l.Option, Sku: v.Sku, ImageKey: l.Image, UnitKobo: unit[l.VariantID], Qty: int32(l.Qty),
			}); err != nil {
				return err
			}
			// Reserve the stock now, so two people can't pay for the last one.
			if err := q.ChangeStock(ctx, sqlc.ChangeStockParams{ID: l.VariantID, Delta: int32(-l.Qty)}); err != nil {
				return err
			}
		}
		if customerID != 0 && in.SaveAddress {
			if err := saveAddress(ctx, q, customerID, in); err != nil {
				return err
			}
		}
		if gift != nil {
			if err := q.InsertOrderItem(ctx, sqlc.InsertOrderItemParams{
				OrderID: order.ID, VariantID: gift.VariantID, ProductName: gift.Name, ProductSlug: gift.Slug,
				VariantLabel: gift.Option, Sku: gift.SKU, ImageKey: gift.Image, UnitKobo: 0, Qty: 1, IsGift: true,
			}); err != nil {
				return err
			}
			if err := q.ChangeStock(ctx, sqlc.ChangeStockParams{ID: gift.VariantID, Delta: -1}); err != nil {
				return err
			}
		}
		return q.InsertOrderEvent(ctx, sqlc.InsertOrderEventParams{OrderID: order.ID, Status: sqlc.OrderStatusPendingPayment, Note: "Order placed, waiting for payment."})
	})
	if err != nil {
		return Started{}, err
	}
	if customerID != 0 {
		// Fill in the phone/email/name the account doesn't have yet. Another account may
		// already use them, in which case nothing changes.
		if err := s.q.SetCustomerContact(ctx, sqlc.SetCustomerContactParams{
			ID: customerID, Name: in.Name,
			Phone: pgtype.Text{String: in.Phone, Valid: true}, Email: pgtype.Text{String: strings.ToLower(in.Email), Valid: true},
		}); err != nil {
			slog.Info("customer contact not updated", "customer", customerID, "error", err)
		}
	}

	payURL, err := s.pay.Initialize(ctx, paystack.Init{
		Email: in.Email, AmountKobo: total, Reference: ref, OrderNumber: order.Number,
		CallbackURL: s.shopURL + "/order/" + order.Number,
		CancelURL:   s.shopURL + "/bag",
	})
	if err != nil {
		slog.Error("paystack initialize failed", "order", order.Number, "error", err)
		if aerr := s.abandon(ctx, ref, "Payment couldn't start, so the order was cancelled."); aerr != nil {
			slog.Error("release stock after failed payment start", "order", order.Number, "error", aerr)
		}
		return Started{}, &apperr.Conflict{Code: "payment_unavailable", Message: "We couldn't open the payment page. Nothing was charged. Please try again in a minute."}
	}
	slog.Info("checkout started", "order", order.Number, "total_kobo", total)
	return Started{OrderNumber: order.Number, PaymentURL: payURL}, nil
}

var (
	digits   = regexp.MustCompile(`\D`)
	ngMobile = regexp.MustCompile(`^0[789][01]\d{8}$`)
)

// NormalizePhone accepts Nigerian mobile numbers in the usual forms
// (0803…, +234 803…, 234803…) and returns "0803 000 0000".
func NormalizePhone(s string) (string, bool) {
	d := digits.ReplaceAllString(s, "")
	if strings.HasPrefix(d, "234") && len(d) == 13 {
		d = "0" + d[3:]
	}
	if !ngMobile.MatchString(d) {
		return "", false
	}
	return d[:4] + " " + d[4:7] + " " + d[7:], true
}

func cleanCheckout(in *CheckoutInput) error {
	in.Name = strings.Join(strings.Fields(in.Name), " ")
	in.Email = strings.TrimSpace(in.Email)
	in.Street = strings.TrimSpace(in.Street)
	in.Landmark = strings.TrimSpace(in.Landmark)
	if utf8.RuneCountInString(in.Name) < 2 || utf8.RuneCountInString(in.Name) > 80 {
		return apperr.NewInvalid("name", "Enter your full name.")
	}
	phone, ok := NormalizePhone(in.Phone)
	if !ok {
		return apperr.NewInvalid("phone", "Enter an 11-digit Nigerian mobile number, e.g. 0803 000 0000.")
	}
	in.Phone = phone
	if a, err := mail.ParseAddress(in.Email); err != nil || a.Address != in.Email || !strings.Contains(in.Email, ".") {
		return apperr.NewInvalid("email", "Enter a valid email address for your receipt.")
	}
	if utf8.RuneCountInString(in.Street) < 5 || utf8.RuneCountInString(in.Street) > 200 {
		return apperr.NewInvalid("street", "Enter your house number and street.")
	}
	if utf8.RuneCountInString(in.Landmark) > 120 {
		return apperr.NewInvalid("landmark", "Keep the landmark under 120 characters.")
	}
	if in.Speed != delivery.Standard && in.Speed != delivery.Express {
		return apperr.NewInvalid("speed", "Choose a delivery option.")
	}
	return nil
}

func label(l cart.Line) string {
	if l.Option == "" {
		return l.Name
	}
	return l.Name + " (" + l.Option + ")"
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return strings.ToUpper(hex.EncodeToString(b))
}

// saveAddress keeps a checkout address on the customer's account, skipping exact repeats
// and stopping at 10 addresses.
func saveAddress(ctx context.Context, q *sqlc.Queries, customerID int64, in CheckoutInput) error {
	lga := pgtype.Int8{Int64: in.LgaID, Valid: in.LgaID != 0}
	lgaText := ""
	if in.LgaID == 0 {
		lgaText = strings.TrimSpace(in.LgaText)
	}
	if _, err := q.FindAddress(ctx, sqlc.FindAddressParams{CustomerID: customerID, StateID: in.StateID, LgaID: lga, LgaText: lgaText, Lower: in.Street}); err == nil {
		return nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if n, err := q.CountAddresses(ctx, customerID); err != nil || n >= 10 {
		return err
	}
	_, err := q.InsertAddress(ctx, sqlc.InsertAddressParams{
		CustomerID: customerID, StateID: in.StateID, LgaID: lga, LgaText: lgaText, Street: in.Street, Landmark: in.Landmark,
	})
	return err
}
