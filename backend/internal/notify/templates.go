package notify

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/db/sqlc"
)

// Template is one customer message on the Messages screen.
type Template struct {
	Event   string `json:"event"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	SMS     string `json:"sms"` // short text message; "" = no SMS for this step
	Active  bool   `json:"active"`
}

// Placeholders lists what staff can put in a message, with an example of each.
var Placeholders = []struct {
	Key     string `json:"key"`
	Example string `json:"example"`
}{
	{"first_name", "Amaka"},
	{"order_number", "YB-20418"},
	{"total", "₦29,000"},
	{"items", "1 × Ribbed lounge joggers (Espresso / M) — ₦14,500\n1 × Pearl drop earrings (Gold) — ₦12,000\nDelivery — ₦2,500"},
	{"delivery_dates", "Thu 26 Nov – Fri 27 Nov"},
	{"courier", "Own rider (Lagos)"},
	{"tracking_number", "LAG-88213"},
	{"tracking_link", ""},
	{"track_url", "https://yetibyrash.example/track?number=YB-20418"},
	{"note", "Your order left our Lekki studio today."},
	{"review_link", "https://yetibyrash.example/review/Ab3xY…"},
}

func (o *Orders) Templates(ctx context.Context) ([]Template, error) {
	rows, err := o.q.ListTemplates(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Template, 0, len(rows))
	for _, r := range rows {
		out = append(out, Template{Event: r.Event, Subject: r.Subject, Body: r.Body, SMS: r.Sms, Active: r.Active})
	}
	return out, nil
}

func (o *Orders) SaveTemplate(ctx context.Context, t Template) error {
	t.Subject = strings.TrimSpace(t.Subject)
	t.Body = strings.TrimSpace(t.Body)
	if t.Subject == "" || utf8.RuneCountInString(t.Subject) > 150 {
		return apperr.NewInvalid("subject", "Write a subject line under 150 characters.")
	}
	if t.Body == "" || utf8.RuneCountInString(t.Body) > 3000 {
		return apperr.NewInvalid("body", "Write a message under 3,000 characters.")
	}
	t.SMS = strings.Join(strings.Fields(t.SMS), " ")
	if utf8.RuneCountInString(t.SMS) > 300 {
		return apperr.NewInvalid("sms", "Keep the text message under 300 characters (about two SMS).")
	}
	if _, err := o.q.GetTemplate(ctx, t.Event); errors.Is(err, pgx.ErrNoRows) {
		return apperr.ErrNotFound
	} else if err != nil {
		return err
	}
	return o.q.UpdateTemplate(ctx, sqlc.UpdateTemplateParams{Event: t.Event, Subject: t.Subject, Body: t.Body, Sms: t.SMS, Active: t.Active})
}

// Preview fills a message with example values, for the Messages screen.
func Preview(subject, body string) (string, string) {
	fields := exampleFields()
	return Fill(subject, fields), FillBody(body, fields)
}

// PreviewSMS fills a text message with example values.
func PreviewSMS(text string) string { return SMSText(text, exampleFields()) }

func exampleFields() map[string]string {
	fields := map[string]string{}
	for _, p := range Placeholders {
		fields[p.Key] = p.Example
	}
	return fields
}
