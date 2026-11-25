package httpapi_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"yetibyrash/backend/internal/cart"
	"yetibyrash/backend/internal/catalog"
	"yetibyrash/backend/internal/delivery"
	"yetibyrash/backend/internal/httpapi"
	"yetibyrash/backend/internal/orders"
	"yetibyrash/backend/internal/paystack"
	"yetibyrash/backend/internal/testdb"
)

const testSecret = "sk_test_fake_secret"

// fakePaystack imitates Paystack's initialize and verify endpoints.
type fakePaystack struct {
	mu   sync.Mutex
	paid map[string]int64 // reference → amount paid
	init map[string]int64 // reference → amount asked
}

func (f *fakePaystack) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /transaction/initialize", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+testSecret {
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]any{"status": false, "message": "Invalid key"})
			return
		}
		var in struct {
			Reference string `json:"reference"`
			Amount    int64  `json:"amount"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		f.mu.Lock()
		f.init[in.Reference] = in.Amount
		f.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"status": true, "data": map[string]any{
			"authorization_url": "https://checkout.paystack.test/" + in.Reference, "reference": in.Reference,
		}})
	})
	mux.HandleFunc("GET /transaction/verify/{ref}", func(w http.ResponseWriter, r *http.Request) {
		ref := r.PathValue("ref")
		f.mu.Lock()
		amount, paid := f.paid[ref]
		f.mu.Unlock()
		status := "abandoned"
		if paid {
			status = "success"
		}
		json.NewEncoder(w).Encode(map[string]any{"status": true, "data": map[string]any{
			"status": status, "reference": ref, "amount": amount, "channel": "card",
		}})
	})
	return mux
}

type shopper struct{ *client }

func (s shopper) json(method, path string, body any) (int, map[string]any) {
	return s.do(method, path, body, "")
}

// webhook sends a Paystack charge.success event, signed with secret.
func webhook(t *testing.T, base, secret, reference string, amount int64) int {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"event": "charge.success", "data": map[string]any{
		"status": "success", "reference": reference, "amount": amount, "channel": "card",
	}})
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write(body)
	req, _ := http.NewRequest("POST", base+"/api/webhooks/paystack", bytes.NewReader(body))
	req.Header.Set("x-paystack-signature", hex.EncodeToString(mac.Sum(nil)))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

func variantID(t *testing.T, c shopper, slug, colour, size string) (int64, int) {
	t.Helper()
	_, p := c.json("GET", "/api/v1/products/"+slug, nil)
	for _, v := range p["variants"].([]any) {
		m := v.(map[string]any)
		if m["colour"] == colour && m["size"] == size {
			return int64(m["id"].(float64)), int(m["stock"].(float64))
		}
	}
	t.Fatalf("no %s %s/%s", slug, colour, size)
	return 0, 0
}

func lagos(t *testing.T, c shopper, lga string) (stateID, lgaID float64) {
	t.Helper()
	_, loc := c.json("GET", "/api/v1/locations", nil)
	for _, s := range loc["states"].([]any) {
		st := s.(map[string]any)
		if st["name"] == "Lagos" {
			for _, l := range st["lgas"].([]any) {
				if l.(map[string]any)["name"] == lga {
					return st["id"].(float64), l.(map[string]any)["id"].(float64)
				}
			}
		}
	}
	t.Fatalf("no Lagos LGA %s", lga)
	return 0, 0
}

func orderStatus(t *testing.T, pool *pgxpool.Pool, number string) (status string, events int) {
	t.Helper()
	err := pool.QueryRow(context.Background(), `
		SELECT o.status::text, (SELECT count(*) FROM order_events e WHERE e.order_id = o.id AND e.status = 'paid')
		FROM orders o WHERE number = $1`, number).Scan(&status, &events)
	if err != nil {
		t.Fatal(err)
	}
	return
}

func TestCheckoutAndPayment(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()

	fake := &fakePaystack{paid: map[string]int64{}, init: map[string]int64{}}
	ps := httptest.NewServer(fake.handler())
	defer ps.Close()

	deliverySvc := delivery.NewService(pool)
	orderSvc := orders.NewService(pool, deliverySvc, paystack.NewClient(testSecret).WithBaseURL(ps.URL), "http://shop.test")
	srv := httptest.NewServer(httpapi.NewRouter(httpapi.Deps{
		DB: pool, Catalog: catalog.NewService(pool),
		Cart: cart.NewService(pool), Delivery: deliverySvc, Orders: orderSvc, PaystackSecret: testSecret,
	}))
	defer srv.Close()

	amaka := shopper{newClient(t, srv.URL)}

	// Espresso M joggers: the demo data has only 2 left.
	espM, stock := variantID(t, amaka, "ribbed-lounge-joggers", "Espresso", "M")
	if stock != 2 {
		t.Fatalf("demo stock for Espresso M = %d, want 2", stock)
	}
	if code, body := amaka.json("POST", "/api/v1/cart/items", map[string]any{"variantId": espM, "qty": 3}); code != 409 || errCode(body) != "not_enough_stock" {
		t.Fatalf("adding 3 of 2 = %d %v", code, body)
	}
	code, bag := amaka.json("POST", "/api/v1/cart/items", map[string]any{"variantId": espM, "qty": 2})
	if code != 200 || bag["count"].(float64) != 2 || bag["subtotalKobo"].(float64) != 2900000 {
		t.Fatalf("add to bag = %d %v", code, bag)
	}

	state, etiOsa := lagos(t, amaka, "Eti-Osa")
	_, alimosho := lagos(t, amaka, "Alimosho")

	// Quote: ₦29,000 is under Lagos's ₦40,000 free-delivery line. Eti-Osa has express, Alimosho doesn't.
	_, q := amaka.json("POST", "/api/v1/checkout/quote", map[string]any{"stateId": state, "lgaId": etiOsa})
	opts := q["options"].([]any)
	if len(opts) != 2 || opts[0].(map[string]any)["feeKobo"].(float64) != 250000 || opts[1].(map[string]any)["speed"] != "express" {
		t.Fatalf("Eti-Osa quote = %v", q)
	}
	if _, q := amaka.json("POST", "/api/v1/checkout/quote", map[string]any{"stateId": state, "lgaId": alimosho}); len(q["options"].([]any)) != 1 {
		t.Fatalf("Alimosho should have no express: %v", q)
	}

	form := map[string]any{
		"name": "Amaka Obi", "phone": "0803 000", "email": "amaka@example.test",
		"stateId": state, "lgaId": etiOsa, "street": "12 Admiralty Way, Lekki", "speed": "standard",
	}
	if code, body := amaka.json("POST", "/api/v1/checkout", form); code != 400 || body["error"].(map[string]any)["field"] != "phone" {
		t.Fatalf("bad phone = %d %v", code, body)
	}
	form["phone"] = "+234 803 000 0000"
	code, started := amaka.json("POST", "/api/v1/checkout", form)
	if code != 201 || !strings.HasPrefix(started["paymentUrl"].(string), "https://checkout.paystack.test/YBP-") {
		t.Fatalf("checkout = %d %v", code, started)
	}
	number := started["orderNumber"].(string)
	ref := strings.TrimPrefix(started["paymentUrl"].(string), "https://checkout.paystack.test/")
	if fake.init[ref] != 2900000+250000 {
		t.Fatalf("Paystack asked for %d kobo, want %d", fake.init[ref], 2900000+250000)
	}

	// Stock is reserved: another shopper can't buy the same 2.
	bisi := shopper{newClient(t, srv.URL)}
	if _, left := variantID(t, bisi, "ribbed-lounge-joggers", "Espresso", "M"); left != 0 {
		t.Fatalf("stock after checkout = %d, want 0 (reserved)", left)
	}
	if code, body := bisi.json("POST", "/api/v1/cart/items", map[string]any{"variantId": espM, "qty": 1}); code != 409 || errCode(body) != "sold_out" {
		t.Fatalf("second shopper add = %d %v", code, body)
	}

	// The order page needs the payment reference.
	if code, _ := amaka.json("GET", "/api/v1/orders/"+number+"?reference=WRONG", nil); code != 404 {
		t.Fatalf("order with wrong reference = %d, want 404", code)
	}
	_, view := amaka.json("GET", "/api/v1/orders/"+number+"?reference="+ref, nil)
	if view["status"] != "pending_payment" || view["phone"] != "0803 000 0000" {
		t.Fatalf("pending order = %v", view)
	}

	// Webhooks: a bad signature is refused; a good one marks the order paid, exactly once.
	if code := webhook(t, srv.URL, "wrong-secret", ref, 3150000); code != 401 {
		t.Fatalf("bad signature = %d, want 401", code)
	}
	for i := 0; i < 2; i++ {
		if code := webhook(t, srv.URL, testSecret, ref, 3150000); code != 200 {
			t.Fatalf("webhook %d = %d", i, code)
		}
	}
	if status, paidEvents := orderStatus(t, pool, number); status != "paid" || paidEvents != 1 {
		t.Fatalf("after webhooks: status=%s paid events=%d, want paid once", status, paidEvents)
	}
	if _, bag := amaka.json("GET", "/api/v1/cart", nil); len(bag["lines"].([]any)) != 0 {
		t.Fatalf("bag should be empty after payment: %v", bag)
	}
	if _, left := variantID(t, bisi, "ribbed-lounge-joggers", "Espresso", "M"); left != 0 {
		t.Fatalf("stock after payment = %d, want 0", left)
	}

	// An unpaid order is released after 30 minutes and its stock comes back.
	blush, before := variantID(t, bisi, "ribbed-lounge-joggers", "Blush", "S")
	bisi.json("POST", "/api/v1/cart/items", map[string]any{"variantId": blush, "qty": 1})
	form["name"], form["email"] = "Bisi K", "bisi@example.test"
	_, started2 := bisi.json("POST", "/api/v1/checkout", form)
	ref2 := strings.TrimPrefix(started2["paymentUrl"].(string), "https://checkout.paystack.test/")
	if _, mid := variantID(t, bisi, "ribbed-lounge-joggers", "Blush", "S"); mid != before-1 {
		t.Fatalf("reserved stock = %d, want %d", mid, before-1)
	}
	pool.Exec(ctx, `UPDATE orders SET created_at = now() - interval '31 minutes' WHERE payment_reference = $1`, ref2)
	if err := orderSvc.ReleaseStale(ctx); err != nil {
		t.Fatal(err)
	}
	if status, _ := orderStatus(t, pool, started2["orderNumber"].(string)); status != "abandoned" {
		t.Fatalf("stale order status = %s, want abandoned", status)
	}
	if _, after := variantID(t, bisi, "ribbed-lounge-joggers", "Blush", "S"); after != before {
		t.Fatalf("stock after release = %d, want %d", after, before)
	}
	// …and if the payment turns up late anyway, the order is paid and the stock taken again.
	webhook(t, srv.URL, testSecret, ref2, int64(started2Total(t, pool, ref2)))
	if status, _ := orderStatus(t, pool, started2["orderNumber"].(string)); status != "paid" {
		t.Fatalf("late payment: status = %s, want paid", status)
	}
	if _, late := variantID(t, bisi, "ribbed-lounge-joggers", "Blush", "S"); late != before-1 {
		t.Fatalf("stock after late payment = %d, want %d", late, before-1)
	}

	// A payment for the wrong amount doesn't mark the order paid.
	chika := shopper{newClient(t, srv.URL)}
	tote, _ := variantID(t, chika, "mini-structured-tote", "Tan", "")
	chika.json("POST", "/api/v1/cart/items", map[string]any{"variantId": tote, "qty": 1})
	form["name"], form["email"] = "Chika N", "chika@example.test"
	_, started3 := chika.json("POST", "/api/v1/checkout", form)
	ref3 := strings.TrimPrefix(started3["paymentUrl"].(string), "https://checkout.paystack.test/")
	webhook(t, srv.URL, testSecret, ref3, 100)
	if status, _ := orderStatus(t, pool, started3["orderNumber"].(string)); status != "pending_payment" {
		t.Fatalf("wrong amount: status = %s, want pending_payment", status)
	}
	// The shopper returning from Paystack triggers a verify; once Paystack says paid, the page shows paid.
	fake.mu.Lock()
	fake.paid[ref3] = int64(started2Total(t, pool, ref3))
	fake.mu.Unlock()
	pool.Exec(ctx, `DELETE FROM payments WHERE reference = $1`, ref3) // the wrong-amount record would block the retry
	if _, v := chika.json("GET", "/api/v1/orders/"+started3["orderNumber"].(string)+"?reference="+ref3, nil); v["status"] != "paid" {
		t.Fatalf("order page after verify = %v", v["status"])
	}

	// Money for an unknown reference is recorded for staff to sort out.
	webhook(t, srv.URL, testSecret, "YBP-UNKNOWN", 5000)
	var outcome string
	pool.QueryRow(ctx, `SELECT outcome FROM payments WHERE reference = 'YBP-UNKNOWN'`).Scan(&outcome)
	if outcome != orders.OutcomeNoOrder {
		t.Fatalf("unknown payment outcome = %q, want %q", outcome, orders.OutcomeNoOrder)
	}
}

func started2Total(t *testing.T, pool *pgxpool.Pool, ref string) int64 {
	t.Helper()
	var total int64
	if err := pool.QueryRow(context.Background(), `SELECT total_kobo FROM orders WHERE payment_reference = $1`, ref).Scan(&total); err != nil {
		t.Fatal(err)
	}
	return total
}
