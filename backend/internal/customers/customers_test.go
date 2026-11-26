package customers_test

import (
	"context"
	"errors"
	"regexp"
	"sync"
	"testing"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/customers"
	"yetibyrash/backend/internal/testdb"
)

// outbox records emails and texts instead of sending them.
type outbox struct {
	mu   sync.Mutex
	last string
}

func (o *outbox) Send(_ context.Context, to, subject, body string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.last = subject + "\n" + body
	return nil
}

func (o *outbox) SendSMS(_ context.Context, phone, text string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.last = text
	return nil
}

var svcWait = func() {}

var codeRe = regexp.MustCompile(`\b\d{6}\b`)

func (o *outbox) code(t *testing.T) string {
	t.Helper()
	svcWait()
	o.mu.Lock()
	defer o.mu.Unlock()
	c := codeRe.FindString(o.last)
	if c == "" {
		t.Fatalf("no code in %q", o.last)
	}
	return c
}

func TestSignInByEmailAndPhone(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	box := &outbox{}
	svc := customers.NewService(pool, box, box)
	svcWait = svc.Wait

	// Email: the address is cleaned up, a wrong code is refused and counted.
	target, err := svc.SendCode(ctx, "  Amaka@Example.TEST ", "1.2.3.4")
	if err != nil || target.Value != "amaka@example.test" || target.IsPhone {
		t.Fatalf("send code: %+v, %v", target, err)
	}
	code := box.code(t)
	var invalid *apperr.Invalid
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	if _, _, err := svc.Verify(ctx, "amaka@example.test", wrong); !errors.As(err, &invalid) || invalid.Field != "code" {
		t.Fatalf("wrong code: err = %v", err)
	}
	var attempts int
	pool.QueryRow(ctx, `SELECT attempts FROM otp_codes WHERE target = 'amaka@example.test'`).Scan(&attempts)
	if attempts != 1 {
		t.Fatalf("attempts = %d, want the wrong guess counted", attempts)
	}
	c, token, err := svc.Verify(ctx, "amaka@example.test", code)
	if err != nil || c.Email != "amaka@example.test" || token == "" {
		t.Fatalf("verify: %+v, %q, %v", c, token, err)
	}
	// A code works once.
	if _, _, err := svc.Verify(ctx, "amaka@example.test", code); err == nil {
		t.Fatal("a used code worked again")
	}
	me, err := svc.FromSession(ctx, token)
	if err != nil || me.ID != c.ID {
		t.Fatalf("session: %+v, %v", me, err)
	}
	if err := svc.SignOut(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.FromSession(ctx, token); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("after sign-out: err = %v", err)
	}

	// Phone: any usual format; the same account comes back next time.
	if _, err := svc.SendCode(ctx, "+234 803 555 0101", ""); err != nil {
		t.Fatal(err)
	}
	p1, _, err := svc.Verify(ctx, "08035550101", box.code(t))
	if err != nil || p1.Phone != "0803 555 0101" {
		t.Fatalf("phone sign-in: %+v, %v", p1, err)
	}
	if _, err := svc.SendCode(ctx, "0803 555 0101", ""); err != nil {
		t.Fatal(err)
	}
	p2, _, err := svc.Verify(ctx, "0803 555 0101", box.code(t))
	if err != nil || p2.ID != p1.ID {
		t.Fatalf("second sign-in made a new account: %+v vs %+v", p2, p1)
	}

	// No more than 3 codes per 15 minutes to one phone.
	var conflict *apperr.Conflict
	if _, err := svc.SendCode(ctx, "0803 555 0101", ""); err != nil {
		t.Fatalf("third code: %v", err)
	}
	if _, err := svc.SendCode(ctx, "0803 555 0101", ""); !errors.As(err, &conflict) {
		t.Fatalf("fourth code: err = %v, want too many", err)
	}

	// Without an SMS provider, phone codes are refused with a pointer to email.
	noSMS := customers.NewService(pool, box, nil)
	if _, err := noSMS.SendCode(ctx, "0809 000 0000", ""); !errors.As(err, &invalid) {
		t.Fatalf("phone code without SMS: err = %v", err)
	}
}
