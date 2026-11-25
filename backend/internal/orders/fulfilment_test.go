package orders_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/cart"
	"yetibyrash/backend/internal/delivery"
	"yetibyrash/backend/internal/jobs"
	"yetibyrash/backend/internal/notify"
	"yetibyrash/backend/internal/orders"
	"yetibyrash/backend/internal/paystack"
	"yetibyrash/backend/internal/staff"
	"yetibyrash/backend/internal/testdb"
)

type sentMail struct{ to, subject, body string }

// fakeMailer records emails instead of sending them. failNext makes the next send fail.
type fakeMailer struct {
	mu       sync.Mutex
	sent     []sentMail
	failNext bool
}

func (m *fakeMailer) Send(_ context.Context, to, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failNext {
		m.failNext = false
		return errors.New("mail server down")
	}
	m.sent = append(m.sent, sentMail{to, subject, body})
	return nil
}

func (m *fakeMailer) last(t *testing.T) sentMail {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sent) == 0 {
		t.Fatal("no email sent")
	}
	return m.sent[len(m.sent)-1]
}

func (m *fakeMailer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

type env struct {
	pool    *pgxpool.Pool
	orders  *orders.Service
	cart    *cart.Service
	dev     *paystack.DevProvider
	runner  *jobs.Runner
	mail    *fakeMailer
	lagos   int64
	etiOsa  int64
	courier int64
}

func setup(t *testing.T) *env {
	pool := testdb.New(t)
	ctx := context.Background()
	dev := paystack.NewDevProvider()
	mail := &fakeMailer{}
	e := &env{
		pool: pool, dev: dev, mail: mail, cart: cart.NewService(pool),
		orders: orders.NewService(pool, delivery.NewService(pool), dev, "https://shop.test"),
		runner: jobs.NewRunner(pool),
	}
	e.runner.Handle(jobs.KindOrderEmail, notify.NewOrders(pool, mail, "https://shop.test").HandleJob)
	if err := pool.QueryRow(ctx, `SELECT s.id, l.id FROM states s JOIN lgas l ON l.state_id = s.id WHERE s.name = 'Lagos' AND l.name = 'Eti-Osa'`).Scan(&e.lagos, &e.etiOsa); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM couriers ORDER BY id LIMIT 1`).Scan(&e.courier); err != nil {
		t.Fatal(err)
	}
	return e
}

// paidOrder buys one Tan mini tote as a new shopper and pays for it. Returns the order id and number.
func (e *env) paidOrder(t *testing.T, email string) (int64, string) {
	t.Helper()
	ctx := context.Background()
	var variant int64
	if err := e.pool.QueryRow(ctx, `SELECT v.id FROM variants v JOIN products p ON p.id = v.product_id JOIN colours c ON c.id = v.colour_id
		WHERE p.slug = 'mini-structured-tote' AND c.name = 'Tan'`).Scan(&variant); err != nil {
		t.Fatal(err)
	}
	cartID, err := e.cart.Add(ctx, pgtype.UUID{}, variant, 1)
	if err != nil {
		t.Fatal(err)
	}
	started, err := e.orders.Checkout(ctx, cartID, orders.CheckoutInput{
		Name: "Amaka Obi", Phone: "08030000000", Email: email, StateID: e.lagos, LgaID: e.etiOsa,
		Street: "12 Admiralty Way", Speed: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}
	ref := strings.TrimPrefix(started.PaymentURL, "/api/v1/dev/pay/")
	e.dev.MarkPaid(ref)
	p, _ := e.dev.Verify(ctx, ref)
	if err := e.orders.ConfirmPayment(ctx, p); err != nil {
		t.Fatal(err)
	}
	var id int64
	e.pool.QueryRow(ctx, `SELECT id FROM orders WHERE number = $1`, started.OrderNumber).Scan(&id)
	return id, started.OrderNumber
}

func (e *env) runJobs(t *testing.T) {
	t.Helper()
	if _, err := e.runner.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestFulfilment(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	packer := staff.Member{ID: 0, Name: "Packer", Role: staff.RolePacker}

	id, number := e.paidOrder(t, "amaka@example.test")

	// Paying queues the "we've got your order" email.
	e.runJobs(t)
	m := e.mail.last(t)
	if m.to != "amaka@example.test" || m.subject != "We've got your order, Amaka" || !strings.Contains(m.body, number) || !strings.Contains(m.body, "₦32,000") {
		t.Fatalf("paid email = %+v", m)
	}

	board, err := e.orders.Board(ctx, "", "")
	if err != nil || len(board) != 1 || board[0].Status != "paid" || board[0].Area != "Eti-Osa, Lagos" {
		t.Fatalf("board = %+v, %v", board, err)
	}

	// Steps can't be skipped.
	var conflict *apperr.Conflict
	if err := e.orders.Advance(ctx, packer, id, orders.AdvanceInput{To: "shipped"}); !errors.As(err, &conflict) {
		t.Fatalf("skipping to shipped: err = %v, want a conflict", err)
	}

	// Packed, with a note the customer sees.
	if err := e.orders.Advance(ctx, packer, id, orders.AdvanceInput{To: "packed", Note: "Wrapped in our blush gift paper."}); err != nil {
		t.Fatal(err)
	}
	e.runJobs(t)
	if m := e.mail.last(t); m.subject != "Your order "+number+" is packed" || !strings.Contains(m.body, "blush gift paper") {
		t.Fatalf("packed email = %+v", m)
	}

	// Shipping needs a courier.
	var invalid *apperr.Invalid
	if err := e.orders.Advance(ctx, packer, id, orders.AdvanceInput{To: "shipped"}); !errors.As(err, &invalid) || invalid.Field != "courierId" {
		t.Fatalf("ship without courier: err = %v", err)
	}
	if err := e.orders.Advance(ctx, packer, id, orders.AdvanceInput{To: "shipped", CourierID: e.courier, TrackingNumber: "LAG-88213"}); err != nil {
		t.Fatal(err)
	}
	e.runJobs(t)
	if m := e.mail.last(t); !strings.Contains(m.body, "Own rider (Lagos)") || !strings.Contains(m.body, "LAG-88213") || !strings.Contains(m.body, "https://shop.test/track?number="+number) {
		t.Fatalf("shipped email = %+v", m)
	}

	// Tracking: the phone number must match (any usual format); staff-only notes stay hidden.
	if _, err := e.orders.Track(ctx, number, "0803 111 1111"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("track with wrong phone: err = %v, want not found", err)
	}
	tr, err := e.orders.Track(ctx, strings.ToLower(number), "+234 803 000 0000")
	if err != nil {
		t.Fatal(err)
	}
	if tr.Headline != "On its way" || tr.Courier != "Own rider (Lagos)" || tr.TrackingNumber != "LAG-88213" {
		t.Fatalf("tracking = %+v", tr)
	}
	done := 0
	for _, s := range tr.Steps {
		if s.Done {
			done++
		}
		if s.Current && s.Status != "shipped" {
			t.Fatalf("current step = %s, want shipped", s.Status)
		}
	}
	if done != 3 || len(tr.Steps) != 5 {
		t.Fatalf("steps done = %d of %d, want 3 of 5", done, len(tr.Steps))
	}
	notes := []string{}
	for _, n := range tr.Notes {
		notes = append(notes, n.Text)
	}
	if !strings.Contains(strings.Join(notes, "|"), "blush gift paper") {
		t.Fatalf("customer notes = %v", notes)
	}

	// A shipped order can't be cancelled here.
	if err := e.orders.Cancel(ctx, packer, id, "Changed mind"); !errors.As(err, &conflict) {
		t.Fatalf("cancel shipped: err = %v", err)
	}

	// A message switched off isn't sent: turn off "delivered", then deliver.
	e.pool.Exec(ctx, `UPDATE message_templates SET active = false WHERE event = 'delivered'`)
	before := e.mail.count()
	e.orders.Advance(ctx, packer, id, orders.AdvanceInput{To: "out_for_delivery"})
	e.orders.Advance(ctx, packer, id, orders.AdvanceInput{To: "delivered"})
	e.runJobs(t)
	if got := e.mail.count() - before; got != 1 {
		t.Fatalf("emails for out-for-delivery + switched-off delivered = %d, want 1", got)
	}

	// Cancelling a paid order returns its stock and emails the reason.
	stock := func() (n int) {
		e.pool.QueryRow(ctx, `SELECT v.stock FROM variants v JOIN products p ON p.id = v.product_id JOIN colours c ON c.id = v.colour_id
			WHERE p.slug = 'mini-structured-tote' AND c.name = 'Tan'`).Scan(&n)
		return
	}
	id2, number2 := e.paidOrder(t, "bisi@example.test")
	afterBuy := stock()
	if err := e.orders.Cancel(ctx, staff.Member{Role: staff.RoleManager}, id2, "Sorry, this piece was damaged in our studio."); err != nil {
		t.Fatal(err)
	}
	if stock() != afterBuy+1 {
		t.Fatalf("stock after cancel = %d, want %d", stock(), afterBuy+1)
	}
	e.runJobs(t) // sends bisi's paid email and the cancellation
	if m := e.mail.last(t); m.to != "bisi@example.test" || m.subject != "Order "+number2+" has been cancelled" || !strings.Contains(m.body, "damaged") {
		t.Fatalf("cancel email = %+v", m)
	}

	// A failed send is retried later instead of being lost.
	e.paidOrder(t, "chika@example.test")
	e.mail.failNext = true
	before = e.mail.count()
	e.runJobs(t)
	if e.mail.count() != before {
		t.Fatal("email counted as sent although the mail server failed")
	}
	e.pool.Exec(ctx, `UPDATE jobs SET run_at = now() WHERE done_at IS NULL`)
	e.runJobs(t)
	if m := e.mail.last(t); m.to != "chika@example.test" {
		t.Fatalf("retried email went to %s", m.to)
	}

	// Overview counts.
	ov, err := e.orders.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ov.OrdersToday != 2 || ov.SalesTodayKobo != 2*3450000 /* ₦32,000 + ₦2,500 delivery, cancelled order excluded */ || ov.ToPack != 1 || len(ov.Last7Days) != 7 {
		t.Fatalf("overview = ordersToday %d, sales %d, toPack %d, days %d", ov.OrdersToday, ov.SalesTodayKobo, ov.ToPack, len(ov.Last7Days))
	}
}
