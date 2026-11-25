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
}

// Quote is the checkout summary before paying.
type Quote struct {
	SubtotalKobo int64             `json:"subtotalKobo"`
	Options      []delivery.Option `json:"options"`
	ZoneName     string            `json:"zoneName"`
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
	return Quote{SubtotalKobo: bag.SubtotalKobo, Options: opts, ZoneName: dest.Zone.Name}, nil
}

// Started is a checkout waiting for payment.
type Started struct {
	OrderNumber string `json:"orderNumber"`
	PaymentURL  string `json:"paymentUrl"`
}

// Checkout creates the order and starts the Paystack payment.
func (s *Service) Checkout(ctx context.Context, cartID pgtype.UUID, in CheckoutInput) (Started, error) {
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
		var subtotal int64
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
			subtotal += v.PriceKobo * int64(l.Qty)
		}
		if len(problems) > 0 {
			return &apperr.Conflict{Code: "stock_changed", Message: "Your bag changed: " + strings.Join(problems, "; ") + ". Update your bag and try again."}
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
		total = subtotal + opt.FeeKobo
		from, _ := time.Parse(time.DateOnly, opt.From)
		to, _ := time.Parse(time.DateOnly, opt.To)

		order, err = q.InsertOrder(ctx, sqlc.InsertOrderParams{
			CartID: cartID, PaymentReference: ref,
			CustomerName: in.Name, CustomerPhone: in.Phone, CustomerEmail: in.Email,
			StateName: dest.StateName, LgaName: dest.LgaName, Street: in.Street, Landmark: in.Landmark,
			ZoneName: dest.Zone.Name, Speed: sqlc.DeliverySpeed(in.Speed),
			SubtotalKobo: subtotal, DeliveryKobo: opt.FeeKobo, TotalKobo: total,
			ExpectedFrom: pgtype.Date{Time: from, Valid: true}, ExpectedTo: pgtype.Date{Time: to, Valid: true},
		})
		if err != nil {
			return err
		}
		for _, l := range bag.Lines {
			v := live[l.VariantID]
			if err := q.InsertOrderItem(ctx, sqlc.InsertOrderItemParams{
				OrderID: order.ID, VariantID: l.VariantID, ProductName: l.Name, ProductSlug: l.Slug,
				VariantLabel: l.Option, Sku: v.Sku, ImageKey: l.Image, UnitKobo: v.PriceKobo, Qty: int32(l.Qty),
			}); err != nil {
				return err
			}
			// Reserve the stock now, so two people can't pay for the last one.
			if err := q.ChangeStock(ctx, sqlc.ChangeStockParams{ID: l.VariantID, Delta: int32(-l.Qty)}); err != nil {
				return err
			}
		}
		return q.InsertOrderEvent(ctx, sqlc.InsertOrderEventParams{OrderID: order.ID, Status: sqlc.OrderStatusPendingPayment, Note: "Order placed, waiting for payment."})
	})
	if err != nil {
		return Started{}, err
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
