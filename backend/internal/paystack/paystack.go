// Package paystack talks to Paystack: starting a payment, checking it, and verifying webhooks.
// Docs: https://paystack.com/docs/api/transaction/
package paystack

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Payment is what Paystack reports about one transaction.
type Payment struct {
	Reference  string
	Paid       bool   // status "success"
	AmountKobo int64  // amount actually paid, in kobo
	Channel    string // card, bank_transfer, ussd…
	Raw        []byte // Paystack's JSON, kept for the payments record
}

// Provider starts and checks payments. Client is the real one; DevProvider fakes it for local work.
type Provider interface {
	// Initialize starts a payment and returns the page to send the shopper to.
	Initialize(ctx context.Context, in Init) (string, error)
	Verify(ctx context.Context, reference string) (Payment, error)
}

type Init struct {
	Email       string
	AmountKobo  int64
	Reference   string
	CallbackURL string // where Paystack sends the shopper after paying
	CancelURL   string // where the shopper lands if they cancel
	OrderNumber string
}

type Client struct {
	secret string
	base   string
	http   *http.Client
}

func NewClient(secretKey string) *Client {
	return &Client{secret: secretKey, base: "https://api.paystack.co", http: &http.Client{Timeout: 20 * time.Second}}
}

// WithBaseURL points the client at a fake server (tests).
func (c *Client) WithBaseURL(u string) *Client { c.base = u; return c }

type envelope struct {
	Status  bool            `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (c *Client) call(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.secret)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("paystack %s: %w", path, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("paystack %s: status %d, unreadable reply", path, res.StatusCode)
	}
	if res.StatusCode >= 300 || !env.Status {
		return nil, fmt.Errorf("paystack %s: status %d: %s", path, res.StatusCode, env.Message)
	}
	return env.Data, nil
}

func (c *Client) Initialize(ctx context.Context, in Init) (string, error) {
	body := map[string]any{
		"email":        in.Email,
		"amount":       in.AmountKobo,
		"reference":    in.Reference,
		"callback_url": in.CallbackURL,
		"channels":     []string{"card", "bank_transfer", "ussd", "bank"},
		"metadata": map[string]any{
			"order_number":  in.OrderNumber,
			"cancel_action": in.CancelURL,
		},
	}
	data, err := c.call(ctx, http.MethodPost, "/transaction/initialize", body)
	if err != nil {
		return "", err
	}
	var out struct {
		AuthorizationURL string `json:"authorization_url"`
	}
	if err := json.Unmarshal(data, &out); err != nil || out.AuthorizationURL == "" {
		return "", errors.New("paystack initialize: no authorization_url in reply")
	}
	return out.AuthorizationURL, nil
}

func (c *Client) Verify(ctx context.Context, reference string) (Payment, error) {
	data, err := c.call(ctx, http.MethodGet, "/transaction/verify/"+url.PathEscape(reference), nil)
	if err != nil {
		return Payment{}, err
	}
	return parseTransaction(data)
}

// parseTransaction reads a Paystack transaction object (from verify or a webhook).
func parseTransaction(data []byte) (Payment, error) {
	var t struct {
		Status    string `json:"status"`
		Reference string `json:"reference"`
		Amount    int64  `json:"amount"`
		Channel   string `json:"channel"`
	}
	if err := json.Unmarshal(data, &t); err != nil {
		return Payment{}, fmt.Errorf("paystack transaction: %w", err)
	}
	return Payment{Reference: t.Reference, Paid: t.Status == "success", AmountKobo: t.Amount, Channel: t.Channel, Raw: data}, nil
}

// ValidSignature checks the x-paystack-signature header: HMAC-SHA512 of the raw body with the secret key.
func ValidSignature(secretKey string, body []byte, signature string) bool {
	if secretKey == "" || signature == "" {
		return false
	}
	mac := hmac.New(sha512.New, []byte(secretKey))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(signature))
}

// Event is a webhook notification.
type Event struct {
	Event   string
	Payment Payment
}

// ParseEvent reads a webhook body. Only charge.success carries a payment.
func ParseEvent(body []byte) (Event, error) {
	var e struct {
		Event string          `json:"event"`
		Data  json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		return Event{}, err
	}
	out := Event{Event: e.Event}
	if e.Event == "charge.success" {
		p, err := parseTransaction(e.Data)
		if err != nil {
			return out, err
		}
		p.Raw = body
		out.Payment = p
	}
	return out, nil
}
