package httpapi

import (
	"errors"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/cart"
	"yetibyrash/backend/internal/delivery"
	"yetibyrash/backend/internal/orders"
	"yetibyrash/backend/internal/paystack"
)

const cartCookie = "ybr_cart"

type shopHandlers struct {
	cart           *cart.Service
	delivery       *delivery.Service
	orders         *orders.Service
	paystackSecret string                // verifies webhooks; "" disables the webhook
	devPay         *paystack.DevProvider // non-nil only in development without Paystack keys
	secureCookies  bool
}

// writeAppError turns a service error into a response. Returns true if there was one.
func writeAppError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	var inv *apperr.Invalid
	var conf *apperr.Conflict
	switch {
	case errors.As(err, &inv):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "invalid", "message": inv.Message, "field": inv.Field}})
	case errors.As(err, &conf):
		writeError(w, http.StatusConflict, conf.Code, conf.Message)
	case errors.Is(err, apperr.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "We couldn't find that.")
	default:
		internalError(w, "shop", err)
	}
	return true
}

func (h *shopHandlers) cartID(r *http.Request) pgtype.UUID {
	c, err := r.Cookie(cartCookie)
	if err != nil {
		return pgtype.UUID{}
	}
	id, _ := cart.ParseID(c.Value)
	return id
}

func (h *shopHandlers) setCartCookie(w http.ResponseWriter, id pgtype.UUID) {
	http.SetCookie(w, &http.Cookie{
		Name: cartCookie, Value: uuidString(id), Path: "/api", MaxAge: int((30 * 24 * time.Hour).Seconds()),
		HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteLaxMode,
	})
}

func (h *shopHandlers) writeBag(w http.ResponseWriter, r *http.Request, id pgtype.UUID) {
	bag, err := h.cart.Get(r.Context(), id)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, bag)
}

// GET /api/v1/cart
func (h *shopHandlers) getCart(w http.ResponseWriter, r *http.Request) {
	h.writeBag(w, r, h.cartID(r))
}

// POST /api/v1/cart/items {"variantId", "qty"}
func (h *shopHandlers) addToCart(w http.ResponseWriter, r *http.Request) {
	var in struct {
		VariantID int64 `json:"variantId"`
		Qty       int   `json:"qty"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.Qty == 0 {
		in.Qty = 1
	}
	id, err := h.cart.Add(r.Context(), h.cartID(r), in.VariantID, in.Qty)
	if writeAppError(w, err) {
		return
	}
	h.setCartCookie(w, id)
	h.writeBag(w, r, id)
}

// PUT /api/v1/cart/items/{variantId} {"qty"} — qty 0 removes the line
func (h *shopHandlers) setCartQty(w http.ResponseWriter, r *http.Request) {
	variantID, ok := pathID(w, r, "variantId")
	if !ok {
		return
	}
	var in struct {
		Qty int `json:"qty"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	id := h.cartID(r)
	if writeAppError(w, h.cart.SetQty(r.Context(), id, variantID, in.Qty)) {
		return
	}
	h.writeBag(w, r, id)
}

// GET /api/v1/locations — states and LGAs for the address form
func (h *shopHandlers) locations(w http.ResponseWriter, r *http.Request) {
	states, err := h.delivery.Locations(r.Context(), false)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"states": states})
}

// GET /api/v1/delivery — fees and times for the product page and bag
func (h *shopHandlers) deliverySummary(w http.ResponseWriter, r *http.Request) {
	sum, err := h.delivery.Summary(r.Context())
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

// POST /api/v1/checkout/quote {"stateId", "lgaId", "lgaText"}
func (h *shopHandlers) quote(w http.ResponseWriter, r *http.Request) {
	var in struct {
		StateID int64  `json:"stateId"`
		LgaID   int64  `json:"lgaId"`
		LgaText string `json:"lgaText"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	q, err := h.orders.Quote(r.Context(), h.cartID(r), in.StateID, in.LgaID, in.LgaText)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, q)
}

// POST /api/v1/checkout — creates the order and returns the Paystack page to go to
func (h *shopHandlers) checkout(w http.ResponseWriter, r *http.Request) {
	var in orders.CheckoutInput
	if !decodeJSON(w, r, &in) {
		return
	}
	started, err := h.orders.Checkout(r.Context(), h.cartID(r), in)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, started)
}

// GET /api/v1/orders/{number}?reference=… — the confirmation page
func (h *shopHandlers) order(w http.ResponseWriter, r *http.Request) {
	v, err := h.orders.Order(r.Context(), r.PathValue("number"), r.URL.Query().Get("reference"))
	if errors.Is(err, apperr.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "We couldn't find that order. Check the link in your email or WhatsApp message.")
		return
	}
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// POST /api/webhooks/paystack — Paystack tells us about payments here.
// Signature failures get 401; everything else gets 200 so Paystack stops retrying.
func (h *shopHandlers) paystackWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_body", "Couldn't read the request.")
		return
	}
	if !paystack.ValidSignature(h.paystackSecret, body, r.Header.Get("x-paystack-signature")) {
		slog.Warn("paystack webhook with a bad signature", "ip", clientIP(r))
		writeError(w, http.StatusUnauthorized, "bad_signature", "Signature check failed.")
		return
	}
	ev, err := paystack.ParseEvent(body)
	if err != nil {
		slog.Warn("unreadable paystack webhook", "error", err)
		w.WriteHeader(http.StatusOK)
		return
	}
	if ev.Event == "charge.success" {
		if err := h.orders.ConfirmPayment(r.Context(), ev.Payment); err != nil {
			// 500 makes Paystack retry later, which is what we want if the database hiccupped.
			internalError(w, "confirm payment from webhook", err)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

// ---------- Development payment page (only without Paystack keys) ----------

var devPayPage = template.Must(template.New("pay").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Test payment</title>
<style>
body{margin:0;font-family:system-ui,sans-serif;background:#F7E7E2;color:#3A2420;display:grid;place-items:center;min-height:100vh;padding:16px;box-sizing:border-box}
.card{background:#FFF8F5;border-radius:24px;padding:28px;max-width:380px;width:100%;display:flex;flex-direction:column;gap:14px}
h1{margin:0;font-size:22px}.note{font-size:14px;color:#5A3E38;background:#FDF3E6;border-radius:12px;padding:10px 12px}
.amount{font-size:30px;font-weight:600}button{min-height:50px;border-radius:999px;border:0;font-size:16px;font-weight:500;cursor:pointer;width:100%}
.pay{background:#3A2420;color:#FFF8F5}.cancel{background:transparent;border:1px solid #EAC8BF;color:#3A2420}
</style></head><body><form class="card" method="post">
<h1>Test payment</h1>
<p class="note">Development only. No Paystack keys are set, so this page stands in for Paystack. No money moves.</p>
<div>Order {{.OrderNumber}} · {{.Email}}</div>
<div class="amount">{{.Amount}}</div>
<button class="pay" name="action" value="pay">Pay (test)</button>
<button class="cancel" name="action" value="cancel">Cancel and go back to the bag</button>
</form></body></html>`))

func (h *shopHandlers) devPayForm(w http.ResponseWriter, r *http.Request) {
	in, ok := h.devPay.Lookup(r.PathValue("reference"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = devPayPage.Execute(w, map[string]string{"OrderNumber": in.OrderNumber, "Email": in.Email, "Amount": nairaText(in.AmountKobo)})
}

func (h *shopHandlers) devPaySubmit(w http.ResponseWriter, r *http.Request) {
	ref := r.PathValue("reference")
	in, ok := h.devPay.Lookup(ref)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.FormValue("action") != "pay" {
		http.Redirect(w, r, in.CancelURL, http.StatusSeeOther)
		return
	}
	h.devPay.MarkPaid(ref)
	p, _ := h.devPay.Verify(r.Context(), ref)
	if err := h.orders.ConfirmPayment(r.Context(), p); err != nil {
		internalError(w, "confirm dev payment", err)
		return
	}
	// Same return link Paystack uses: the order page with ?reference=
	http.Redirect(w, r, in.CallbackURL+"?reference="+ref, http.StatusSeeOther)
}

func nairaText(kobo int64) string {
	s := strconv.FormatInt(kobo/100, 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return "₦" + s
}

func uuidString(id pgtype.UUID) string {
	v, _ := id.Value()
	s, _ := v.(string)
	return s
}
