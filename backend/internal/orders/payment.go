package orders

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/db/sqlc"
	"yetibyrash/backend/internal/jobs"
	"yetibyrash/backend/internal/paystack"
)

// Payment outcomes recorded in the payments table.
const (
	OutcomeSuccess        = "success"
	OutcomeNoOrder        = "no_order"        // paid, but we have no order with that reference
	OutcomeAmountMismatch = "amount_mismatch" // paid a different amount from the order total
)

// ConfirmPayment records a successful Paystack payment and marks its order paid.
// Paystack may report the same payment more than once (webhook, retries, our own check);
// only the first report changes anything.
func (s *Service) ConfirmPayment(ctx context.Context, p paystack.Payment) error {
	if !p.Paid {
		return nil
	}
	payload := p.Raw
	if len(payload) == 0 || payload[0] != '{' {
		payload = []byte("{}")
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		o, err := q.GetOrderByReferenceForUpdate(ctx, p.Reference)
		if errors.Is(err, pgx.ErrNoRows) {
			// Money arrived for an order we don't have. Keep it so staff can sort it out.
			slog.Warn("payment with no matching order", "reference", p.Reference, "amount_kobo", p.AmountKobo)
			_, err := q.InsertPayment(ctx, sqlc.InsertPaymentParams{Reference: p.Reference, AmountKobo: p.AmountKobo, Channel: p.Channel, Outcome: OutcomeNoOrder, Payload: payload})
			return ignoreNoRows(err)
		}
		if err != nil {
			return err
		}

		if p.AmountKobo != o.TotalKobo {
			slog.Warn("payment amount doesn't match order", "order", o.Number, "paid", p.AmountKobo, "total", o.TotalKobo)
			_, err := q.InsertPayment(ctx, sqlc.InsertPaymentParams{Reference: p.Reference, OrderID: pgtype.Int8{Int64: o.ID, Valid: true}, AmountKobo: p.AmountKobo, Channel: p.Channel, Outcome: OutcomeAmountMismatch, Payload: payload})
			if err != nil {
				return ignoreNoRows(err) // no rows = already recorded
			}
			return q.InsertOrderEvent(ctx, sqlc.InsertOrderEventParams{OrderID: o.ID, Status: o.Status,
				Note: fmt.Sprintf("Paystack received %s but the order total is %s. Not marked paid; check Paystack.", naira(p.AmountKobo), naira(o.TotalKobo))})
		}

		if _, err := q.InsertPayment(ctx, sqlc.InsertPaymentParams{Reference: p.Reference, OrderID: pgtype.Int8{Int64: o.ID, Valid: true}, AmountKobo: p.AmountKobo, Channel: p.Channel, Outcome: OutcomeSuccess, Payload: payload}); ignoreNoRows(err) != nil {
			return err
		}

		switch o.Status {
		case sqlc.OrderStatusPendingPayment:
			// Stock was reserved at checkout.
		case sqlc.OrderStatusAbandoned:
			// The hold ran out and the stock went back; take it again (never below zero).
			items, err := q.ListOrderItems(ctx, o.ID)
			if err != nil {
				return err
			}
			for _, it := range items {
				if err := q.ChangeStock(ctx, sqlc.ChangeStockParams{ID: it.VariantID, Delta: -it.Qty}); err != nil {
					return err
				}
			}
			if err := q.InsertOrderEvent(ctx, sqlc.InsertOrderEventParams{OrderID: o.ID, Status: sqlc.OrderStatusPaid,
				Note: "Payment arrived after the 30-minute hold ended. Check stock before packing."}); err != nil {
				return err
			}
		case sqlc.OrderStatusCancelled, sqlc.OrderStatusRefunded:
			return q.InsertOrderEvent(ctx, sqlc.InsertOrderEventParams{OrderID: o.ID, Status: o.Status,
				Note: "A payment arrived for this cancelled order. Refund it in Paystack."})
		default:
			return nil // already paid: nothing to do
		}

		if err := q.MarkOrderPaid(ctx, o.ID); err != nil {
			return err
		}
		if o.Status == sqlc.OrderStatusPendingPayment {
			if err := q.InsertOrderEvent(ctx, sqlc.InsertOrderEventParams{OrderID: o.ID, Status: sqlc.OrderStatusPaid,
				Note: "Paid with Paystack" + channelText(p.Channel) + ".", Public: true}); err != nil {
				return err
			}
		}
		// The "we've got your order" email goes out from the job queue, only if this commits.
		if err := jobs.Enqueue(ctx, q, jobs.KindOrderEmail, jobs.OrderEmail{OrderID: o.ID, Event: string(sqlc.OrderStatusPaid)}); err != nil {
			return err
		}
		if o.CartID.Valid {
			if err := q.DeleteCart(ctx, o.CartID); err != nil {
				return err
			}
		}
		slog.Info("order paid", "order", o.Number, "amount_kobo", p.AmountKobo, "channel", p.Channel)
		return nil
	})
}

// abandon cancels an unpaid order and returns its stock.
func (s *Service) abandon(ctx context.Context, reference, note string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		o, err := q.GetOrderByReferenceForUpdate(ctx, reference)
		if err != nil {
			return err
		}
		if o.Status != sqlc.OrderStatusPendingPayment {
			return nil // paid in the meantime
		}
		items, err := q.ListOrderItems(ctx, o.ID)
		if err != nil {
			return err
		}
		for _, it := range items {
			if err := q.ChangeStock(ctx, sqlc.ChangeStockParams{ID: it.VariantID, Delta: it.Qty}); err != nil {
				return err
			}
		}
		if err := q.SetOrderStatus(ctx, sqlc.SetOrderStatusParams{ID: o.ID, Status: sqlc.OrderStatusAbandoned}); err != nil {
			return err
		}
		return q.InsertOrderEvent(ctx, sqlc.InsertOrderEventParams{OrderID: o.ID, Status: sqlc.OrderStatusAbandoned, Note: note})
	})
}

// ReleaseStale abandons orders unpaid after HoldMinutes. Before giving up on one it asks
// Paystack, in case the payment went through but the webhook never arrived.
func (s *Service) ReleaseStale(ctx context.Context) error {
	stale, err := s.q.ListStalePendingOrders(ctx, HoldMinutes)
	if err != nil {
		return err
	}
	for _, o := range stale {
		if p, err := s.pay.Verify(ctx, o.PaymentReference); err == nil && p.Paid {
			if err := s.ConfirmPayment(ctx, p); err != nil {
				slog.Error("confirm late payment", "order", o.Number, "error", err)
			}
			continue
		} else if err != nil {
			slog.Warn("couldn't check payment before releasing", "order", o.Number, "error", err)
			continue // try again next round rather than cancel something that may be paid
		}
		if err := s.abandon(ctx, o.PaymentReference, fmt.Sprintf("Not paid within %d minutes. Stock released.", HoldMinutes)); err != nil {
			slog.Error("release unpaid order", "order", o.Number, "error", err)
			continue
		}
		slog.Info("unpaid order released", "order", o.Number)
	}
	return nil
}

// RunReleaser calls ReleaseStale every minute until ctx ends.
func (s *Service) RunReleaser(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.ReleaseStale(ctx); err != nil && ctx.Err() == nil {
				slog.Error("release unpaid orders", "error", err)
			}
		}
	}
}

// ---------- The order page ----------

type ItemView struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Option   string `json:"option"`
	Image    string `json:"image"`
	UnitKobo int64  `json:"unitKobo"`
	Qty      int    `json:"qty"`
}

type EventView struct {
	Status string    `json:"status"`
	Note   string    `json:"note"`
	At     time.Time `json:"at"`
}

// View is what the shopper sees on the order confirmation page.
type View struct {
	Number       string      `json:"number"`
	Status       string      `json:"status"`
	Name         string      `json:"name"`
	Phone        string      `json:"phone"`
	Email        string      `json:"email"`
	Address      string      `json:"address"`
	Speed        string      `json:"speed"`
	SubtotalKobo int64       `json:"subtotalKobo"`
	DeliveryKobo int64       `json:"deliveryKobo"`
	TotalKobo    int64       `json:"totalKobo"`
	ExpectedFrom string      `json:"expectedFrom"`
	ExpectedTo   string      `json:"expectedTo"`
	PaidAt       *time.Time  `json:"paidAt"`
	Items        []ItemView  `json:"items"`
	Events       []EventView `json:"events"`
}

// Order returns an order for the shopper. The payment reference acts as the key: without it,
// the order number alone shows nothing. A pending order is checked with Paystack first.
func (s *Service) Order(ctx context.Context, number, reference string) (View, error) {
	o, err := s.q.GetOrderByNumber(ctx, number)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && reference != o.PaymentReference) {
		return View{}, apperr.ErrNotFound
	}
	if err != nil {
		return View{}, err
	}
	if o.Status == sqlc.OrderStatusPendingPayment {
		if p, err := s.pay.Verify(ctx, reference); err == nil && p.Paid {
			if err := s.ConfirmPayment(ctx, p); err != nil {
				return View{}, err
			}
			if o, err = s.q.GetOrderByNumber(ctx, number); err != nil {
				return View{}, err
			}
		} else if err != nil {
			slog.Warn("verify payment for order page", "order", number, "error", err)
		}
	}

	v := View{
		Number: o.Number, Status: string(o.Status), Name: o.CustomerName, Phone: o.CustomerPhone, Email: o.CustomerEmail,
		Address:      strings.Join(nonEmpty(o.Street, o.Landmark, o.LgaName, o.StateName), ", "),
		Speed:        string(o.Speed),
		SubtotalKobo: o.SubtotalKobo, DeliveryKobo: o.DeliveryKobo, TotalKobo: o.TotalKobo,
		ExpectedFrom: o.ExpectedFrom.Time.Format(time.DateOnly), ExpectedTo: o.ExpectedTo.Time.Format(time.DateOnly),
		Items: []ItemView{}, Events: []EventView{},
	}
	if o.PaidAt.Valid {
		t := o.PaidAt.Time
		v.PaidAt = &t
	}
	items, err := s.q.ListOrderItems(ctx, o.ID)
	if err != nil {
		return View{}, err
	}
	for _, it := range items {
		v.Items = append(v.Items, ItemView{Name: it.ProductName, Slug: it.ProductSlug, Option: it.VariantLabel, Image: it.ImageKey, UnitKobo: it.UnitKobo, Qty: int(it.Qty)})
	}
	events, err := s.q.ListOrderEvents(ctx, o.ID)
	if err != nil {
		return View{}, err
	}
	for _, e := range events {
		note := e.Note
		if !e.Public {
			note = "" // staff-only notes never reach the shopper
		}
		v.Events = append(v.Events, EventView{Status: string(e.Status), Note: note, At: e.CreatedAt.Time})
	}
	return v, nil
}

func ignoreNoRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

func channelText(c string) string {
	switch c {
	case "card":
		return " (card)"
	case "bank_transfer", "bank":
		return " (bank transfer)"
	case "ussd":
		return " (USSD)"
	case "":
		return ""
	}
	return " (" + c + ")"
}

func naira(kobo int64) string {
	n := kobo / 100
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return "₦" + s
}

func nonEmpty(parts ...string) []string {
	out := []string{}
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}
