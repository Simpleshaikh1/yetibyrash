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
	"yetibyrash/backend/internal/reviews"
)

// Orders turns order steps into customer emails and text messages, using the
// templates staff edit on the Customer messages screen.
type Orders struct {
	q       *sqlc.Queries
	mailer  Mailer
	sms     SMS // nil = no text messages
	shopURL string
}

func NewOrders(db sqlc.DBTX, mailer Mailer, sms SMS, shopURL string) *Orders {
	return &Orders{q: sqlc.New(db), mailer: mailer, sms: sms, shopURL: strings.TrimRight(shopURL, "/")}
}

// template returns a message template, or ok=false when it's missing or switched off.
func (o *Orders) template(ctx context.Context, event string) (sqlc.GetTemplateRow, bool, error) {
	tpl, err := o.q.GetTemplate(ctx, event)
	if errors.Is(err, pgx.ErrNoRows) {
		return tpl, false, nil
	}
	if err != nil {
		return tpl, false, err
	}
	return tpl, tpl.Active, nil
}

// HandleJob is the jobs.Handler for jobs.KindOrderEmail.
func (o *Orders) HandleJob(ctx context.Context, payload []byte) error {
	var j jobs.OrderEmail
	if err := json.Unmarshal(payload, &j); err != nil {
		return fmt.Errorf("bad order email job: %w", err)
	}
	tpl, ok, err := o.template(ctx, j.Event)
	if err != nil || !ok {
		return err
	}
	fields, ord, err := o.orderFields(ctx, j.OrderID, j.Note)
	if err != nil {
		return err
	}
	if err := o.mailer.Send(ctx, ord.CustomerEmail, Fill(tpl.Subject, fields), FillBody(tpl.Body, fields)); err != nil {
		return fmt.Errorf("send %s email for order %d: %w", j.Event, j.OrderID, err)
	}
	slog.Info("order email sent", "order_id", j.OrderID, "event", j.Event)
	return nil
}

// HandleSMSJob is the jobs.Handler for jobs.KindOrderSMS. Steps whose template has no
// SMS text send nothing.
func (o *Orders) HandleSMSJob(ctx context.Context, payload []byte) error {
	if o.sms == nil {
		return nil
	}
	var j jobs.OrderEmail
	if err := json.Unmarshal(payload, &j); err != nil {
		return fmt.Errorf("bad order sms job: %w", err)
	}
	tpl, ok, err := o.template(ctx, j.Event)
	if err != nil || !ok || strings.TrimSpace(tpl.Sms) == "" {
		return err
	}
	fields, ord, err := o.orderFields(ctx, j.OrderID, j.Note)
	if err != nil {
		return err
	}
	if err := o.sms.SendSMS(ctx, ord.CustomerPhone, SMSText(tpl.Sms, fields)); err != nil {
		return fmt.Errorf("send %s sms for order %d: %w", j.Event, j.OrderID, err)
	}
	slog.Info("order sms sent", "order_id", j.OrderID, "event", j.Event)
	return nil
}

// HandleReviewRequest is the jobs.Handler for jobs.KindReviewRequest: a few days after
// delivery it sends the customer a private link to review what they bought.
func (o *Orders) HandleReviewRequest(ctx context.Context, payload []byte) error {
	var j jobs.ReviewRequest
	if err := json.Unmarshal(payload, &j); err != nil {
		return fmt.Errorf("bad review request job: %w", err)
	}
	tpl, ok, err := o.template(ctx, "review_request")
	if err != nil || !ok {
		return err
	}
	fields, ord, err := o.orderFields(ctx, j.OrderID, "")
	if err != nil {
		return err
	}
	if ord.Status != sqlc.OrderStatusDelivered {
		return nil // cancelled or returned since
	}
	token, hash := reviews.NewToken()
	if err := o.q.InsertReviewRequest(ctx, sqlc.InsertReviewRequestParams{OrderID: j.OrderID, TokenHash: hash}); err != nil {
		return err
	}
	fields["review_link"] = o.shopURL + "/review/" + token
	if err := o.mailer.Send(ctx, ord.CustomerEmail, Fill(tpl.Subject, fields), FillBody(tpl.Body, fields)); err != nil {
		return fmt.Errorf("send review request for order %d: %w", j.OrderID, err)
	}
	// The email went; a failed text is logged rather than retried, so the email isn't sent twice.
	if o.sms != nil && strings.TrimSpace(tpl.Sms) != "" {
		if err := o.sms.SendSMS(ctx, ord.CustomerPhone, SMSText(tpl.Sms, fields)); err != nil {
			slog.Warn("review request sms failed", "order_id", j.OrderID, "error", err)
		}
	}
	slog.Info("review request sent", "order_id", j.OrderID)
	return nil
}

// Render fills a template for an order (used by tests and previews).
func (o *Orders) Render(ctx context.Context, orderID int64, subject, body, note string) (renderedSubject, renderedBody, to string, err error) {
	fields, ord, err := o.orderFields(ctx, orderID, note)
	if err != nil {
		return "", "", "", err
	}
	return Fill(subject, fields), FillBody(body, fields), ord.CustomerEmail, nil
}

// orderFields gives every placeholder's value for an order.
func (o *Orders) orderFields(ctx context.Context, orderID int64, note string) (map[string]string, sqlc.AdminGetOrderRow, error) {
	ord, err := o.q.AdminGetOrder(ctx, orderID)
	if err != nil {
		return nil, ord, fmt.Errorf("load order %d: %w", orderID, err)
	}
	items, err := o.q.ListOrderItems(ctx, orderID)
	if err != nil {
		return nil, ord, err
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
	return fields, ord, nil
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
