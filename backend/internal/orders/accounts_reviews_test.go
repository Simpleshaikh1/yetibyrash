package orders_test

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/catalog"
	"yetibyrash/backend/internal/customers"
	"yetibyrash/backend/internal/jobs"
	"yetibyrash/backend/internal/media"
	"yetibyrash/backend/internal/notify"
	"yetibyrash/backend/internal/orders"
	"yetibyrash/backend/internal/reviews"
	"yetibyrash/backend/internal/staff"
)

var reviewLinkRe = regexp.MustCompile(`/review/([A-Za-z0-9_-]+)`)

func TestAccountCheckoutAndReview(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	packer := staff.Member{Name: "Packer", Role: staff.RolePacker}
	owner := staff.Member{Name: "Owner", Role: staff.RoleOwner}

	// A customer who signed in with their phone, before ever ordering.
	customerID := e.int(t, `INSERT INTO customers (phone) VALUES ('0803 000 0000') RETURNING id`)

	cartID, err := e.cart.Add(ctx, pgtype.UUID{}, e.variant(t, "BG-TOT-TAN"), 1)
	if err != nil {
		t.Fatal(err)
	}
	started, err := e.orders.Checkout(ctx, cartID, customerID, orders.CheckoutInput{
		Name: "Amaka Obi", Phone: "08030000000", Email: "amaka@example.test", StateID: e.lagos, LgaID: e.etiOsa,
		Street: "12 Admiralty Way", Speed: "standard", SaveAddress: true,
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
	orderID := e.int(t, `SELECT id FROM orders WHERE number = $1`, started.OrderNumber)

	// The order is linked, the address saved (once), and the account learned the email and name.
	if got := e.int(t, `SELECT customer_id FROM orders WHERE id = $1`, orderID); got != customerID {
		t.Fatalf("order customer = %d, want %d", got, customerID)
	}
	if n := e.int(t, `SELECT COUNT(*) FROM addresses WHERE customer_id = $1`, customerID); n != 1 {
		t.Fatalf("addresses = %d, want 1", n)
	}
	var email, name string
	e.pool.QueryRow(ctx, `SELECT email, name FROM customers WHERE id = $1`, customerID).Scan(&email, &name)
	if email != "amaka@example.test" || name != "Amaka Obi" {
		t.Fatalf("customer contact = %q %q", email, name)
	}
	acct := customers.NewService(e.pool, e.mail, nil)
	me := customers.Customer{ID: customerID, Phone: "0803 000 0000"}
	list, err := acct.Orders(ctx, me)
	if err != nil || len(list) != 1 || list[0].Number != started.OrderNumber {
		t.Fatalf("my orders = %+v, %v", list, err)
	}

	// Not delivered yet: no review link.
	var conflict *apperr.Conflict
	if _, err := acct.ReviewLink(ctx, me, started.OrderNumber); !errors.As(err, &conflict) {
		t.Fatalf("review before delivery: err = %v", err)
	}

	for _, step := range []orders.AdvanceInput{{To: "packed"}, {To: "shipped", CourierID: e.courier}, {To: "out_for_delivery"}, {To: "delivered"}} {
		if err := e.orders.Advance(ctx, packer, orderID, step); err != nil {
			t.Fatal(step.To, err)
		}
	}

	// Delivery queues the review request for 3 days later; bring it forward and run it.
	if n := e.int(t, `SELECT COUNT(*) FROM jobs WHERE kind = 'review_request' AND run_at > now() + interval '2 days'`); n != 1 {
		t.Fatalf("review request jobs = %d, want 1 waiting 3 days", n)
	}
	e.exec(t, `UPDATE jobs SET run_at = now() WHERE kind = 'review_request'`)
	runner := jobs.NewRunner(e.pool)
	runner.Handle(jobs.KindReviewRequest, notify.NewOrders(e.pool, e.mail, nil, "https://shop.test").HandleReviewRequest)
	runner.Handle(jobs.KindOrderEmail, func(context.Context, []byte) error { return nil })
	runner.Handle(jobs.KindOrderSMS, func(context.Context, []byte) error { return nil })
	for { // the runner takes 10 jobs at a time
		n, err := runner.RunOnce(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
	}
	m := e.mail.last(t)
	link := reviewLinkRe.FindStringSubmatch(m.body)
	if m.to != "amaka@example.test" || link == nil {
		t.Fatalf("review request email = %+v", m)
	}
	token := link[1]

	store, err := media.NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rv := reviews.NewService(e.pool, store)
	req, err := rv.Request(ctx, token)
	if err != nil || len(req.Items) != 1 || req.AuthorName != "Amaka O." {
		t.Fatalf("review page = %+v, %v", req, err)
	}
	if _, err := rv.Request(ctx, "not-a-real-token"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("bad token: err = %v", err)
	}
	item := req.Items[0].OrderItemID
	var invalid *apperr.Invalid
	if _, err := rv.Submit(ctx, token, reviews.Input{OrderItemID: item, Rating: 6}); !errors.As(err, &invalid) {
		t.Fatalf("rating 6: err = %v", err)
	}
	reviewID, err := rv.Submit(ctx, token, reviews.Input{OrderItemID: item, Rating: 5, Text: "Lovely tote, fits my laptop."})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rv.Submit(ctx, token, reviews.Input{OrderItemID: item, Rating: 4}); !errors.As(err, &conflict) {
		t.Fatalf("second review of the same item: err = %v", err)
	}

	// Nothing shows until staff approve it.
	cat := catalog.NewService(e.pool)
	if d, _ := cat.Product(ctx, "mini-structured-tote"); d.Rating.Count != 0 || len(d.Reviews) != 0 {
		t.Fatalf("waiting review is visible: %+v", d.Rating)
	}
	if err := rv.SetStatus(ctx, owner, reviewID, "published"); err != nil {
		t.Fatal(err)
	}
	if err := rv.Reply(ctx, owner, reviewID, "Thank you, Amaka!"); err != nil {
		t.Fatal(err)
	}
	d, err := cat.Product(ctx, "mini-structured-tote")
	if err != nil {
		t.Fatal(err)
	}
	if d.Rating.Count != 1 || d.Rating.Average != 5 || len(d.Reviews) != 1 || d.Reviews[0].Author != "Amaka O." || d.Reviews[0].Area != "Lagos" || d.Reviews[0].Reply != "Thank you, Amaka!" {
		t.Fatalf("product reviews = %+v %+v", d.Rating, d.Reviews)
	}

	// A signed-in customer can get a fresh review link for a delivered order.
	if tok, err := acct.ReviewLink(ctx, me, strings.ToLower(started.OrderNumber)); err != nil || tok == "" {
		t.Fatalf("review link: %q, %v", tok, err)
	}
}
