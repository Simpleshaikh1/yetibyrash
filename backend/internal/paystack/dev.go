package paystack

import (
	"context"
	"net/url"
	"sync"
)

// DevProvider stands in for Paystack when no keys are set in development.
// Its "payment page" is served by the API at /api/v1/dev/pay/{reference}, with Pay and Cancel buttons.
// It is never used in production (see cmd/yetibyrash).
type DevProvider struct {
	mu   sync.Mutex
	pays map[string]*devPayment
}

type devPayment struct {
	Init
	paid bool
}

func NewDevProvider() *DevProvider { return &DevProvider{pays: map[string]*devPayment{}} }

func (d *DevProvider) Initialize(_ context.Context, in Init) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pays[in.Reference] = &devPayment{Init: in}
	return "/api/v1/dev/pay/" + url.PathEscape(in.Reference), nil
}

func (d *DevProvider) Verify(_ context.Context, reference string) (Payment, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	p, ok := d.pays[reference]
	if !ok {
		return Payment{Reference: reference}, nil
	}
	return Payment{Reference: reference, Paid: p.paid, AmountKobo: p.AmountKobo, Channel: "card", Raw: []byte(`{"dev":true}`)}, nil
}

// Lookup returns a started dev payment, for the fake payment page.
func (d *DevProvider) Lookup(reference string) (Init, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	p, ok := d.pays[reference]
	if !ok {
		return Init{}, false
	}
	return p.Init, true
}

// MarkPaid records the fake payment as successful.
func (d *DevProvider) MarkPaid(reference string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if p, ok := d.pays[reference]; ok {
		p.paid = true
	}
}
