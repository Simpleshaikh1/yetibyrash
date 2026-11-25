package orders

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/db/sqlc"
)

// TrackStep is one bead on the shopper's tracker.
type TrackStep struct {
	Status  string     `json:"status"`
	Label   string     `json:"label"`
	At      *time.Time `json:"at"` // when it happened; nil = not yet
	Done    bool       `json:"done"`
	Current bool       `json:"current"`
}

type TrackNote struct {
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

// Tracking is the shopper's Track order page.
type Tracking struct {
	Number         string      `json:"number"`
	Status         string      `json:"status"`
	Headline       string      `json:"headline"` // "On its way"
	Area           string      `json:"area"`     // "Eti-Osa, Lagos"
	Items          int         `json:"items"`
	Speed          string      `json:"speed"`
	ExpectedFrom   string      `json:"expectedFrom"`
	ExpectedTo     string      `json:"expectedTo"`
	Courier        string      `json:"courier"`
	TrackingNumber string      `json:"trackingNumber"`
	TrackingLink   string      `json:"trackingLink"`
	Steps          []TrackStep `json:"steps"`
	Notes          []TrackNote `json:"notes"`
}

var stepLabels = map[sqlc.OrderStatus]string{
	sqlc.OrderStatusPaid: "Paid", sqlc.OrderStatusPacked: "Packed", sqlc.OrderStatusShipped: "Shipped",
	sqlc.OrderStatusOutForDelivery: "Out for delivery", sqlc.OrderStatusDelivered: "Delivered",
}

var headlines = map[sqlc.OrderStatus]string{
	sqlc.OrderStatusPendingPayment: "Waiting for payment", sqlc.OrderStatusPaid: "Payment received",
	sqlc.OrderStatusPacked: "Packed and ready", sqlc.OrderStatusShipped: "On its way",
	sqlc.OrderStatusOutForDelivery: "Out for delivery today", sqlc.OrderStatusDelivered: "Delivered",
	sqlc.OrderStatusAbandoned: "Not paid", sqlc.OrderStatusCancelled: "Cancelled", sqlc.OrderStatusRefunded: "Refunded",
}

// Track finds an order by number and the phone number it was placed with.
// A wrong phone gives the same "not found" as a wrong number, so numbers can't be guessed.
func (s *Service) Track(ctx context.Context, number, phone string) (Tracking, error) {
	norm, ok := NormalizePhone(phone)
	if !ok {
		return Tracking{}, apperr.NewInvalid("phone", "Enter the phone number you ordered with, e.g. 0803 000 0000.")
	}
	o, err := s.q.GetOrderForTracking(ctx, strings.TrimSpace(number))
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && o.CustomerPhone != norm) {
		return Tracking{}, apperr.ErrNotFound
	}
	if err != nil {
		return Tracking{}, err
	}

	t := Tracking{
		Number: o.Number, Status: string(o.Status), Headline: headlines[o.Status],
		Area: o.LgaName + ", " + o.StateName, Items: int(o.ItemCount), Speed: string(o.Speed),
		ExpectedFrom: o.ExpectedFrom.Time.Format(time.DateOnly), ExpectedTo: o.ExpectedTo.Time.Format(time.DateOnly),
		Courier: o.CourierName, TrackingNumber: o.TrackingNumber,
		Steps: []TrackStep{}, Notes: []TrackNote{},
	}
	if o.CourierTrackingUrl != "" && o.TrackingNumber != "" {
		t.TrackingLink = strings.ReplaceAll(o.CourierTrackingUrl, "{tracking_number}", url.QueryEscape(o.TrackingNumber))
	}

	events, err := s.q.ListOrderEvents(ctx, o.ID)
	if err != nil {
		return Tracking{}, err
	}
	reached := map[sqlc.OrderStatus]time.Time{}
	for _, e := range events {
		if _, seen := reached[e.Status]; !seen {
			reached[e.Status] = e.CreatedAt.Time
		}
		if e.Public && e.Note != "" {
			t.Notes = append(t.Notes, TrackNote{Text: e.Note, At: e.CreatedAt.Time})
		}
	}

	current := -1
	for i, st := range Flow {
		if st == o.Status {
			current = i
		}
	}
	for i, st := range Flow {
		step := TrackStep{Status: string(st), Label: stepLabels[st], Done: i <= current, Current: i == current}
		if at, ok := reached[st]; ok && step.Done {
			at := at
			step.At = &at
		}
		t.Steps = append(t.Steps, step)
	}
	return t, nil
}
