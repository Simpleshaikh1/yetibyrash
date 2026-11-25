package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"yetibyrash/backend/internal/db/sqlc"
	"yetibyrash/backend/internal/jobs"
)

// Orders turns order steps into customer emails using the message templates staff edit.
type Orders struct {
	q       *sqlc.Queries
	mailer  Mailer
	shopURL string
}

func NewOrders(db sqlc.DBTX, mailer Mailer, shopURL string) *Orders {
	return &Orders{q: sqlc.New(db), mailer: mailer, shopURL: strings.TrimRight(shopURL, "/")}
}

// HandleJob is the jobs.Handler for jobs.KindOrderEmail.
func (o *Orders) HandleJob(ctx context.Context, payload []byte) error {
	var j jobs.OrderEmail
	if err := json.Unmarshal(payload, &j); err != nil {
		return fmt.Errorf("bad order email job: %w", err)
	}
	tpl, err := o.q.GetTemplate(ctx, sqlc.OrderStatus(j.Event))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // no message for this step
	}
	if err != nil {
		return err
	}
	if !tpl.Active {
		return nil // staff switched this message off
	}
	subject, body, to, err := o.Render(ctx, j.OrderID, tpl.Subject, tpl.Body, j.Note)
	if err != nil {
		return err
	}
	if err := o.mailer.Send(ctx, to, subject, body); err != nil {
		return fmt.Errorf("send %s email for order %d: %w", j.Event, j.OrderID, err)
	}
	slog.Info("order email sent", "order_id", j.OrderID, "event", j.Event)
	return nil
}

// Render fills a template for an order. The Messages screen uses it for previews too.
func (o *Orders) Render(ctx context.Context, orderID int64, subject, body, note string) (renderedSubject, renderedBody, to string, err error) {
	ord, err := o.q.AdminGetOrder(ctx, orderID)
	if err != nil {
		return "", "", "", fmt.Errorf("load order %d: %w", orderID, err)
	}
	items, err := o.q.ListOrderItems(ctx, orderID)
	if err != nil {
		return "", "", "", err
	}
	var lines []string
	for _, it := range items {
		name := it.ProductName
		if it.VariantLabel != "" {
			name += " (" + it.VariantLabel + ")"
		}
		if it.IsGift {
			lines = append(lines, fmt.Sprintf("%d × %s — free gift", it.Qty, name))
			continue
		}
		lines = append(lines, fmt.Sprintf("%d × %s — %s", it.Qty, name, Naira(it.UnitKobo*int64(it.Qty))))
	}
	if ord.DiscountKobo > 0 {
		label := "Discount"
		if ord.DiscountCode != "" {
			label += " (" + ord.DiscountCode + ")"
		}
		lines = append(lines, label+" — −"+Naira(ord.DiscountKobo))
	}
	if ord.DeliveryKobo > 0 {
		lines = append(lines, "Delivery — "+Naira(ord.DeliveryKobo))
	} else {
		lines = append(lines, "Delivery — free")
	}

	link := ""
	if ord.CourierTrackingUrl != "" && ord.TrackingNumber != "" {
		link = strings.ReplaceAll(ord.CourierTrackingUrl, "{tracking_number}", url.QueryEscape(ord.TrackingNumber))
	}
	fields := map[string]string{
		"first_name":      strings.Fields(ord.CustomerName + " ")[0],
		"order_number":    ord.Number,
		"total":           Naira(ord.TotalKobo),
		"items":           strings.Join(lines, "\n"),
		"delivery_dates":  DateRange(ord.ExpectedFrom.Time, ord.ExpectedTo.Time),
		"courier":         orDefault(ord.CourierName, "our courier"),
		"tracking_number": ord.TrackingNumber,
		"tracking_link":   link,
		"track_url":       o.shopURL + "/track?number=" + url.QueryEscape(ord.Number),
		"note":            note,
	}
	return Fill(subject, fields), FillBody(body, fields), ord.CustomerEmail, nil
}

// Fill replaces {placeholders} with values. Unknown ones are left as written.
func Fill(s string, fields map[string]string) string {
	for k, v := range fields {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}

// FillBody fills a message body. A line whose placeholders all came out empty is
// dropped, so "Tracking number: {tracking_number}" disappears when there is none.
func FillBody(body string, fields map[string]string) string {
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		used, filled := 0, 0
		for k, v := range fields {
			if strings.Contains(l, "{"+k+"}") {
				used++
				if strings.TrimSpace(v) != "" {
					filled++
				}
			}
		}
		if used > 0 && filled == 0 {
			continue
		}
		out = append(out, Fill(l, fields))
	}
	return tidy(strings.Join(out, "\n"))
}

// tidy removes the blank lines and trailing spaces left by empty placeholders.
func tidy(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, l := range lines {
		l = strings.TrimRight(l, " ")
		if l == "" {
			if blank {
				continue
			}
			blank = true
		} else {
			blank = false
		}
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n")) + "\n"
}

// Naira formats kobo as "₦14,500".
func Naira(kobo int64) string {
	s := fmt.Sprint(kobo / 100)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return "₦" + s
}

// DateRange formats "Thu 26 Nov" or "Fri 27 Nov – Sat 28 Nov".
func DateRange(from, to time.Time) string {
	if from.Equal(to) {
		return from.Format("Mon 2 Jan")
	}
	return from.Format("Mon 2 Jan") + " – " + to.Format("Mon 2 Jan")
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
