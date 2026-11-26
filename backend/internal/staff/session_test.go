package staff_test

import (
	"context"
	"errors"
	"testing"

	"yetibyrash/backend/internal/staff"
	"yetibyrash/backend/internal/testdb"
)

func TestSignInEndsAfterEightIdleHours(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	svc := staff.NewService(pool)
	if _, err := svc.Create(ctx, "packer@example.test", "Packer", staff.RolePacker, "a long test password 1"); err != nil {
		t.Fatal(err)
	}
	signIn := func() string {
		t.Helper()
		_, token, err := svc.Login(ctx, "packer@example.test", "a long test password 1")
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	setLastUse := func(ago, sinceLogin string) {
		t.Helper()
		if _, err := pool.Exec(ctx, `UPDATE staff_sessions SET last_seen_at = now() - $1::interval, expires_at = created_at + interval '7 days' - $2::interval`, ago, sinceLogin); err != nil {
			t.Fatal(err)
		}
	}

	// Used 7 hours ago: still signed in, and using it resets the clock.
	token := signIn()
	setLastUse("7 hours", "0 hours")
	if _, err := svc.Authenticate(ctx, token); err != nil {
		t.Fatalf("7 hours idle: %v", err)
	}
	var fresh bool
	pool.QueryRow(ctx, `SELECT bool_and(last_seen_at > now() - interval '1 minute') FROM staff_sessions`).Scan(&fresh)
	if !fresh {
		t.Fatal("using the admin didn't reset the idle clock")
	}

	// Not used for 9 hours: signed out.
	setLastUse("9 hours", "0 hours")
	if _, err := svc.Authenticate(ctx, token); !errors.Is(err, staff.ErrNoSession) {
		t.Fatalf("9 hours idle: err = %v, want signed out", err)
	}

	// Used every hour but signed in 7 days ago: the 7-day limit still applies.
	token = signIn()
	setLastUse("1 hour", "7 days")
	if _, err := svc.Authenticate(ctx, token); !errors.Is(err, staff.ErrNoSession) {
		t.Fatalf("7 days after sign-in: err = %v, want signed out", err)
	}
}
