package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// SMS sends one text message to a Nigerian mobile number ("0803 000 0000").
type SMS interface {
	SendSMS(ctx context.Context, phone, text string) error
}

// TermiiSMS sends through Termii (termii.com), a Nigerian SMS provider.
// Sender IDs must be registered with Termii before they work on live traffic.
type TermiiSMS struct {
	APIKey   string
	SenderID string // e.g. "Yetibyrash"
	Channel  string // "generic", or "dnd" once Termii enables it (reaches numbers on Do-Not-Disturb)
	BaseURL  string // default https://api.ng.termii.com
	Client   *http.Client
}

func (t TermiiSMS) SendSMS(ctx context.Context, phone, text string) error {
	to, ok := international(phone)
	if !ok {
		return fmt.Errorf("sms: %q is not a Nigerian mobile number", phone)
	}
	base := t.BaseURL
	if base == "" {
		base = "https://api.ng.termii.com"
	}
	channel := t.Channel
	if channel == "" {
		channel = "generic"
	}
	body, _ := json.Marshal(map[string]string{
		"api_key": t.APIKey, "to": to, "from": t.SenderID, "sms": text, "type": "plain", "channel": channel,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/sms/send", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := t.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sms: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("sms: termii answered %d: %s", res.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}

// MailSMS is for development without an SMS account: each text arrives as an email
// (in Mailpit) addressed to sms-08030000000@sms.yetibyrash.test, so it can be read and tested.
type MailSMS struct{ Mailer Mailer }

func (m MailSMS) SendSMS(ctx context.Context, phone, text string) error {
	digits := nonDigits.ReplaceAllString(phone, "")
	return m.Mailer.Send(ctx, "sms-"+digits+"@sms.yetibyrash.test", "SMS to "+phone, text+"\n")
}

var nonDigits = regexp.MustCompile(`\D`)

// international turns "0803 000 0000" into "2348030000000" for SMS providers.
func international(phone string) (string, bool) {
	d := nonDigits.ReplaceAllString(phone, "")
	switch {
	case len(d) == 11 && d[0] == '0':
		return "234" + d[1:], true
	case len(d) == 13 && strings.HasPrefix(d, "234"):
		return d, true
	}
	return "", false
}

// SMSText fills an SMS template on one line, with spaces tidied.
func SMSText(tpl string, fields map[string]string) string {
	return strings.Join(strings.Fields(Fill(tpl, fields)), " ")
}
