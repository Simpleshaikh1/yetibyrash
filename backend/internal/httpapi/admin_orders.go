package httpapi

import (
	"errors"
	"net/http"
	"time"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/delivery"
	"yetibyrash/backend/internal/notify"
	"yetibyrash/backend/internal/orders"
	"yetibyrash/backend/internal/staff"
)

type adminOrderHandlers struct {
	orders   *orders.Service
	delivery *delivery.Service
	notify   *notify.Orders
}

// GET /api/admin/v1/overview
func (h adminOrderHandlers) overview(w http.ResponseWriter, r *http.Request) {
	ov, err := h.orders.Overview(r.Context())
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, ov)
}

// GET /api/admin/v1/orders?status=&search=
func (h adminOrderHandlers) list(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	switch status {
	case "", "paid", "packed", "shipped", "out_for_delivery", "delivered", "cancelled", "refunded":
	default:
		writeError(w, http.StatusBadRequest, "invalid_status", "That isn't an order status.")
		return
	}
	list, err := h.orders.Board(r.Context(), status, r.URL.Query().Get("search"))
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": list})
}

// GET /api/admin/v1/orders/{id}
func (h adminOrderHandlers) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	o, err := h.orders.AdminOrder(r.Context(), id)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, o)
}

// POST /api/admin/v1/orders/{id}/advance {"to","courierId","trackingNumber","note"}
func (h adminOrderHandlers) advance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in orders.AdvanceInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if writeAppError(w, h.orders.Advance(r.Context(), currentMember(r), id, in)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/admin/v1/orders/{id}/cancel {"reason"} — owners and managers only
func (h adminOrderHandlers) cancel(w http.ResponseWriter, r *http.Request) {
	m := currentMember(r)
	if m.Role != staff.RoleOwner && m.Role != staff.RoleManager {
		writeError(w, http.StatusForbidden, "forbidden", "Only the owner or a manager can cancel orders.")
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if writeAppError(w, h.orders.Cancel(r.Context(), m, id, in.Reason)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/admin/v1/couriers
func (h adminOrderHandlers) couriers(w http.ResponseWriter, r *http.Request) {
	list, err := h.delivery.Couriers(r.Context())
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"couriers": list})
}

// POST /api/admin/v1/couriers, PUT /api/admin/v1/couriers/{id}
func (h adminOrderHandlers) saveCourier(w http.ResponseWriter, r *http.Request) {
	id, ok := optionalID(w, r)
	if !ok {
		return
	}
	var c delivery.Courier
	if !decodeJSON(w, r, &c) {
		return
	}
	c.ID = id
	newID, err := h.delivery.SaveCourier(r.Context(), c)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": newID})
}

// GET /api/admin/v1/messages
func (h adminOrderHandlers) messages(w http.ResponseWriter, r *http.Request) {
	list, err := h.notify.Templates(r.Context())
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": list, "placeholders": notify.Placeholders})
}

// PUT /api/admin/v1/messages/{event}
func (h adminOrderHandlers) saveMessage(w http.ResponseWriter, r *http.Request) {
	var t notify.Template
	if !decodeJSON(w, r, &t) {
		return
	}
	t.Event = r.PathValue("event")
	if writeAppError(w, h.notify.SaveTemplate(r.Context(), t)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/admin/v1/messages/preview {"subject","body"} — fills placeholders with examples
func (h adminOrderHandlers) previewMessage(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
		SMS     string `json:"sms"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	s, b := notify.Preview(in.Subject, in.Body)
	writeJSON(w, http.StatusOK, map[string]string{"subject": s, "body": b, "sms": notify.PreviewSMS(in.SMS)})
}

// ---------- Shopper tracking ----------

type trackHandler struct {
	orders *orders.Service
	limit  *rateLimiter
}

// POST /api/v1/track {"number","phone"} — a POST so the phone number stays out of URLs and logs
func (h trackHandler) track(w http.ResponseWriter, r *http.Request) {
	if !h.limit.Allow(clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "too_many_attempts", "Too many tries. Wait a few minutes and try again.")
		return
	}
	var in struct {
		Number string `json:"number"`
		Phone  string `json:"phone"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	t, err := h.orders.Track(r.Context(), in.Number, in.Phone)
	if errors.Is(err, apperr.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "We couldn't find an order with that number and phone. Check both and try again.")
		return
	}
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func newTrackHandler(o *orders.Service) trackHandler {
	return trackHandler{orders: o, limit: newRateLimiter(20, 10*time.Minute)}
}
