package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/catalog"
	"yetibyrash/backend/internal/customers"
	"yetibyrash/backend/internal/media"
	"yetibyrash/backend/internal/reviews"
)

const customerCookie = "ybr_session"

// CardLoader gives product cards by id (the catalogue service).
type CardLoader interface {
	CardsByID(ctx context.Context, ids []int64) ([]catalog.ProductCard, error)
}

type accountHandlers struct {
	c             *customers.Service
	cards         CardLoader
	secureCookies bool
	codeLimit     *rateLimiter // sending codes, per IP
	verifyLimit   *rateLimiter // checking codes, per IP
}

func newAccountHandlers(c *customers.Service, cards CardLoader, secure bool) *accountHandlers {
	return &accountHandlers{
		c: c, cards: cards, secureCookies: secure,
		codeLimit:   newRateLimiter(10, 15*time.Minute),
		verifyLimit: newRateLimiter(30, 15*time.Minute),
	}
}

func sessionToken(r *http.Request) string {
	if c, err := r.Cookie(customerCookie); err == nil {
		return c.Value
	}
	return ""
}

// customerID is the signed-in shopper's id, 0 for a guest.
func (h *accountHandlers) customerID(r *http.Request) int64 {
	if h == nil {
		return 0
	}
	c, err := h.c.FromSession(r.Context(), sessionToken(r))
	if err != nil {
		return 0
	}
	return c.ID
}

// signedIn wraps account routes: it answers 401 when nobody is signed in.
func (h *accountHandlers) signedIn(next func(http.ResponseWriter, *http.Request, customers.Customer)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := h.c.FromSession(r.Context(), sessionToken(r))
		if errors.Is(err, apperr.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "signed_out", "Please sign in.")
			return
		}
		if writeAppError(w, err) {
			return
		}
		next(w, r, c)
	}
}

// POST /api/v1/auth/code {"target"} — sends a sign-in code by SMS or email
func (h *accountHandlers) sendCode(w http.ResponseWriter, r *http.Request) {
	if !h.codeLimit.Allow(clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many tries. Please wait 15 minutes.")
		return
	}
	var in struct {
		Target string `json:"target"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	t, err := h.c.SendCode(r.Context(), in.Target, clientIP(r))
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// POST /api/v1/auth/verify {"target", "code"} — signs in
func (h *accountHandlers) verify(w http.ResponseWriter, r *http.Request) {
	if !h.verifyLimit.Allow(clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many tries. Please wait 15 minutes.")
		return
	}
	var in struct {
		Target string `json:"target"`
		Code   string `json:"code"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	c, token, err := h.c.Verify(r.Context(), in.Target, in.Code)
	if writeAppError(w, err) {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: customerCookie, Value: token, Path: "/api", MaxAge: customers.SessionDays * 24 * 3600,
		HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"customer": c})
}

// POST /api/v1/auth/logout
func (h *accountHandlers) logout(w http.ResponseWriter, r *http.Request) {
	_ = h.c.SignOut(r.Context(), sessionToken(r))
	http.SetCookie(w, &http.Cookie{Name: customerCookie, Value: "", Path: "/api", MaxAge: -1, HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/me — the signed-in customer and their saved product ids, or null
func (h *accountHandlers) me(w http.ResponseWriter, r *http.Request) {
	c, err := h.c.FromSession(r.Context(), sessionToken(r))
	if errors.Is(err, apperr.ErrNotFound) {
		writeJSON(w, http.StatusOK, map[string]any{"customer": nil, "saved": []int64{}})
		return
	}
	if writeAppError(w, err) {
		return
	}
	ids, err := h.c.SavedIDs(r.Context(), c)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"customer": c, "saved": ids})
}

// PUT /api/v1/me {"name", "marketingOptIn"}
func (h *accountHandlers) updateMe(w http.ResponseWriter, r *http.Request, c customers.Customer) {
	var in struct {
		Name           string `json:"name"`
		MarketingOptIn bool   `json:"marketingOptIn"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if writeAppError(w, h.c.UpdateProfile(r.Context(), c, in.Name, in.MarketingOptIn)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/v1/me — deletes the account (orders are kept for the shop's records)
func (h *accountHandlers) deleteMe(w http.ResponseWriter, r *http.Request, c customers.Customer) {
	if writeAppError(w, h.c.DeleteAccount(r.Context(), c)) {
		return
	}
	http.SetCookie(w, &http.Cookie{Name: customerCookie, Value: "", Path: "/api", MaxAge: -1, HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func (h *accountHandlers) orders(w http.ResponseWriter, r *http.Request, c customers.Customer) {
	list, err := h.c.Orders(r.Context(), c)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": list})
}

// POST /api/v1/me/orders/{number}/review — a private link to review a delivered order
func (h *accountHandlers) reviewLink(w http.ResponseWriter, r *http.Request, c customers.Customer) {
	token, err := h.c.ReviewLink(r.Context(), c, r.PathValue("number"))
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *accountHandlers) addresses(w http.ResponseWriter, r *http.Request, c customers.Customer) {
	list, err := h.c.Addresses(r.Context(), c)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"addresses": list})
}

func (h *accountHandlers) deleteAddress(w http.ResponseWriter, r *http.Request, c customers.Customer) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if writeAppError(w, h.c.DeleteAddress(r.Context(), c, id)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/me/saved — saved products as cards
func (h *accountHandlers) saved(w http.ResponseWriter, r *http.Request, c customers.Customer) {
	ids, err := h.c.SavedIDs(r.Context(), c)
	if writeAppError(w, err) {
		return
	}
	cards, err := h.cards.CardsByID(r.Context(), ids)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": cards})
}

func (h *accountHandlers) save(w http.ResponseWriter, r *http.Request, c customers.Customer) {
	id, ok := pathID(w, r, "productId")
	if !ok {
		return
	}
	if writeAppError(w, h.c.Save(r.Context(), c, id)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *accountHandlers) unsave(w http.ResponseWriter, r *http.Request, c customers.Customer) {
	id, ok := pathID(w, r, "productId")
	if !ok {
		return
	}
	if writeAppError(w, h.c.Unsave(r.Context(), c, id)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- Reviews ----------

type reviewHandlers struct{ rv *reviews.Service }

// GET /api/v1/reviews/{token} — the review page for an order
func (h reviewHandlers) request(w http.ResponseWriter, r *http.Request) {
	v, err := h.rv.Request(r.Context(), r.PathValue("token"))
	if errors.Is(err, apperr.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "This review link has expired or isn't right. Sign in to your account to review your orders.")
		return
	}
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// POST /api/v1/reviews/{token} {"orderItemId", "rating", "text", "name"}
func (h reviewHandlers) submit(w http.ResponseWriter, r *http.Request) {
	var in reviews.Input
	if !decodeJSON(w, r, &in) {
		return
	}
	id, err := h.rv.Submit(r.Context(), r.PathValue("token"), in)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

// POST /api/v1/reviews/{token}/{id}/photos — multipart form: photo
func (h reviewHandlers) photo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "We couldn't find that review.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, media.MaxUploadBytes+1<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", "That photo is too big. Photos can be up to 15 MB.")
		return
	}
	file, _, err := r.FormFile("photo")
	if err != nil {
		writeError(w, http.StatusBadRequest, "no_photo", "Choose a photo to upload.")
		return
	}
	defer file.Close()
	photo, err := media.Process(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "not_an_image", "That file isn't a photo. Upload a JPEG, PNG or WebP.")
		return
	}
	key, err := h.rv.AddPhoto(r.Context(), r.PathValue("token"), id, photo)
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"photo": key})
}

// ---------- Admin: reviews ----------

func (h reviewHandlers) adminList(w http.ResponseWriter, r *http.Request) {
	v, err := h.rv.Admin(r.Context(), r.URL.Query().Get("status"))
	if writeAppError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h reviewHandlers) adminStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		Status string `json:"status"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if writeAppError(w, h.rv.SetStatus(r.Context(), currentMember(r), id, in.Status)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h reviewHandlers) adminReply(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		Reply string `json:"reply"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if writeAppError(w, h.rv.Reply(r.Context(), currentMember(r), id, in.Reply)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
