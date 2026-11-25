package orders

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/db/sqlc"
	"yetibyrash/backend/internal/jobs"
	"yetibyrash/backend/internal/staff"
)

// Flow is the order of steps after payment. Staff move an order one step at a time.
var Flow = []sqlc.OrderStatus{
	sqlc.OrderStatusPaid, sqlc.OrderStatusPacked, sqlc.OrderStatusShipped,
	sqlc.OrderStatusOutForDelivery, sqlc.OrderStatusDelivered,
}

// NextStatus returns the step after s, or "" if s is the last step or not in the flow.
func NextStatus(s sqlc.OrderStatus) sqlc.OrderStatus {
	for i, f := range Flow {
		if f == s && i+1 < len(Flow) {
			return Flow[i+1]
		}
	}
	return ""
}

// BoardOrder is one card on the orders board.
type BoardOrder struct {
	ID        int64     `json:"id"`
	Number    string    `json:"number"`
	Status    string    `json:"status"`
	StatusAt  time.Time `json:"statusAt"`
	PaidAt    time.Time `json:"paidAt"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Area      string    `json:"area"` // "Eti-Osa, Lagos"
	Speed     string    `json:"speed"`
	TotalKobo int64     `json:"totalKobo"`
	Items     int       `json:"items"`
	Late      bool      `json:"late"` // paid over 24h ago and still not packed
}

// Board lists paid orders and later for the orders board. search matches number, name or phone.
func (s *Service) Board(ctx context.Context, status, search string) ([]BoardOrder, error) {
	p := sqlc.AdminListOrdersParams{RecentDays: 7, Search: pgtype.Text{String: strings.TrimSpace(search), Valid: strings.TrimSpace(search) != ""}}
	if status != "" {
		p.Status = sqlc.NullOrderStatus{OrderStatus: sqlc.OrderStatus(status), Valid: true}
		p.RecentDays = 90
	}
	rows, err := s.q.AdminListOrders(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]BoardOrder, 0, len(rows))
	for _, r := range rows {
		b := BoardOrder{
			ID: r.ID, Number: r.Number, Status: string(r.Status), StatusAt: r.StatusAt.Time, PaidAt: r.PaidAt.Time,
			Name: r.CustomerName, Phone: r.CustomerPhone, Area: r.LgaName + ", " + r.StateName, Speed: string(r.Speed),
			TotalKobo: r.TotalKobo, Items: int(r.ItemCount),
		}
		b.Late = r.Status == sqlc.OrderStatusPaid && r.PaidAt.Valid && time.Since(r.PaidAt.Time) > 24*time.Hour
		out = append(out, b)
	}
	return out, nil
}

type AdminEvent struct {
	Status string    `json:"status"`
	Note   string    `json:"note"`
	Public bool      `json:"public"`
	By     string    `json:"by"`
	At     time.Time `json:"at"`
}

type AdminPayment struct {
	Reference  string    `json:"reference"`
	AmountKobo int64     `json:"amountKobo"`
	Channel    string    `json:"channel"`
	Outcome    string    `json:"outcome"`
	At         time.Time `json:"at"`
}

// AdminOrder is the order detail screen.
type AdminOrder struct {
	View
	ID             int64          `json:"id"`
	Reference      string         `json:"reference"`
	ZoneName       string         `json:"zoneName"`
	StatusAt       time.Time      `json:"statusAt"`
	CreatedAt      time.Time      `json:"createdAt"`
	CourierID      *int64         `json:"courierId"`
	CourierName    string         `json:"courierName"`
	TrackingNumber string         `json:"trackingNumber"`
	Next           string         `json:"next"` // the step "Move to…" goes to, "" if none
	CanCancel      bool           `json:"canCancel"`
	AdminEvents    []AdminEvent   `json:"adminEvents"`
	Payments       []AdminPayment `json:"payments"`
}

func (s *Service) AdminOrder(ctx context.Context, id int64) (AdminOrder, error) {
	o, err := s.q.AdminGetOrder(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminOrder{}, apperr.ErrNotFound
	}
	if err != nil {
		return AdminOrder{}, err
	}
	v, err := s.Order(ctx, o.Number, o.PaymentReference)
	if err != nil {
		return AdminOrder{}, err
	}
	out := AdminOrder{
		View: v, ID: o.ID, Reference: o.PaymentReference, ZoneName: o.ZoneName, StatusAt: o.StatusAt.Time,
		CreatedAt: o.CreatedAt.Time, CourierName: o.CourierName, TrackingNumber: o.TrackingNumber,
		Next:        string(NextStatus(o.Status)),
		CanCancel:   o.Status == sqlc.OrderStatusPaid || o.Status == sqlc.OrderStatusPacked,
		AdminEvents: []AdminEvent{}, Payments: []AdminPayment{},
	}
	if o.CourierID.Valid {
		out.CourierID = &o.CourierID.Int64
	}
	events, err := s.q.AdminListOrderEvents(ctx, id)
	if err != nil {
		return AdminOrder{}, err
	}
	for _, e := range events {
		out.AdminEvents = append(out.AdminEvents, AdminEvent{Status: string(e.Status), Note: e.Note, Public: e.Public, By: e.StaffName, At: e.CreatedAt.Time})
	}
	pays, err := s.q.ListOrderPayments(ctx, pgtype.Int8{Int64: id, Valid: true})
	if err != nil {
		return AdminOrder{}, err
	}
	for _, p := range pays {
		out.Payments = append(out.Payments, AdminPayment{Reference: p.Reference, AmountKobo: p.AmountKobo, Channel: p.Channel, Outcome: p.Outcome, At: p.ReceivedAt.Time})
	}
	return out, nil
}

// AdvanceInput moves an order to its next step.
type AdvanceInput struct {
	To             string `json:"to"`             // must be the next step; guards against two staff clicking at once
	CourierID      int64  `json:"courierId"`      // required when moving to shipped
	TrackingNumber string `json:"trackingNumber"` // optional
	Note           string `json:"note"`           // optional note shown to the customer
}

// Advance moves an order one step on, records who did it, and queues the customer's message.
func (s *Service) Advance(ctx context.Context, by staff.Member, id int64, in AdvanceInput) error {
	in.Note = strings.TrimSpace(in.Note)
	in.TrackingNumber = strings.TrimSpace(in.TrackingNumber)
	if len(in.Note) > 500 {
		return apperr.NewInvalid("note", "Keep the note to the customer under 500 characters.")
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		o, err := q.LockOrder(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return err
		}
		next := NextStatus(o.Status)
		if next == "" || string(next) != in.To {
			return &apperr.Conflict{Code: "status_changed", Message: fmt.Sprintf("Order %s is now %s, so it can't move to %s. Reload to see the latest.", o.Number, statusLabel(o.Status), statusLabel(sqlc.OrderStatus(in.To)))}
		}
		if next == sqlc.OrderStatusShipped {
			if in.CourierID == 0 {
				return apperr.NewInvalid("courierId", "Choose the courier carrying this parcel.")
			}
			if _, err := q.GetCourier(ctx, in.CourierID); errors.Is(err, pgx.ErrNoRows) {
				return apperr.NewInvalid("courierId", "Choose the courier carrying this parcel.")
			} else if err != nil {
				return err
			}
			if len(in.TrackingNumber) > 80 {
				return apperr.NewInvalid("trackingNumber", "Keep the tracking number under 80 characters.")
			}
			if err := q.SetOrderShipping(ctx, sqlc.SetOrderShippingParams{ID: id, CourierID: pgtype.Int8{Int64: in.CourierID, Valid: true}, TrackingNumber: in.TrackingNumber}); err != nil {
				return err
			}
		}
		if err := q.SetOrderStatus(ctx, sqlc.SetOrderStatusParams{ID: id, Status: next}); err != nil {
			return err
		}
		if err := q.InsertOrderEvent(ctx, sqlc.InsertOrderEventParams{OrderID: id, Status: next, Note: in.Note, Public: true,
			StaffID: pgtype.Int8{Int64: by.ID, Valid: by.ID != 0}}); err != nil {
			return err
		}
		if err := staff.Audit(ctx, q, by, "order."+string(next), "order", id, map[string]any{"number": o.Number, "note": in.Note}); err != nil {
			return err
		}
		return jobs.Enqueue(ctx, q, jobs.KindOrderEmail, jobs.OrderEmail{OrderID: id, Event: string(next), Note: in.Note})
	})
}

// Cancel cancels a paid order that hasn't shipped, returns its stock, and tells the customer.
// The refund itself is done in Paystack.
func (s *Service) Cancel(ctx context.Context, by staff.Member, id int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return apperr.NewInvalid("reason", "Say why the order is cancelled; the customer sees this.")
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		o, err := q.LockOrder(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return err
		}
		if o.Status != sqlc.OrderStatusPaid && o.Status != sqlc.OrderStatusPacked {
			return &apperr.Conflict{Code: "cannot_cancel", Message: fmt.Sprintf("Order %s is %s; only paid or packed orders can be cancelled here.", o.Number, statusLabel(o.Status))}
		}
		items, err := q.ListOrderItems(ctx, id)
		if err != nil {
			return err
		}
		for _, it := range items {
			if err := q.ChangeStock(ctx, sqlc.ChangeStockParams{ID: it.VariantID, Delta: it.Qty}); err != nil {
				return err
			}
		}
		if err := q.SetOrderStatus(ctx, sqlc.SetOrderStatusParams{ID: id, Status: sqlc.OrderStatusCancelled}); err != nil {
			return err
		}
		if err := q.InsertOrderEvent(ctx, sqlc.InsertOrderEventParams{OrderID: id, Status: sqlc.OrderStatusCancelled, Note: reason, Public: true,
			StaffID: pgtype.Int8{Int64: by.ID, Valid: by.ID != 0}}); err != nil {
			return err
		}
		if err := staff.Audit(ctx, q, by, "order.cancelled", "order", id, map[string]any{"number": o.Number, "reason": reason}); err != nil {
			return err
		}
		return jobs.Enqueue(ctx, q, jobs.KindOrderEmail, jobs.OrderEmail{OrderID: id, Event: string(sqlc.OrderStatusCancelled), Note: reason})
	})
}

var statusLabels = map[sqlc.OrderStatus]string{
	sqlc.OrderStatusPendingPayment: "waiting for payment", sqlc.OrderStatusPaid: "paid", sqlc.OrderStatusPacked: "packed",
	sqlc.OrderStatusShipped: "shipped", sqlc.OrderStatusOutForDelivery: "out for delivery", sqlc.OrderStatusDelivered: "delivered",
	sqlc.OrderStatusAbandoned: "unpaid", sqlc.OrderStatusCancelled: "cancelled", sqlc.OrderStatusRefunded: "refunded",
}

func statusLabel(s sqlc.OrderStatus) string {
	if l, ok := statusLabels[s]; ok {
		return l
	}
	return string(s)
}
