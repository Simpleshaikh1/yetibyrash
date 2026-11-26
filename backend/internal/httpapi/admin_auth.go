package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"yetibyrash/backend/internal/staff"
)

// StaffService is what the admin sign-in and Staff screen need. *staff.Service satisfies it.
type StaffService interface {
	Login(ctx context.Context, email, password string) (staff.Member, string, error)
	Authenticate(ctx context.Context, token string) (staff.Member, error)
	Logout(ctx context.Context, token string) error
	Create(ctx context.Context, email, name string, role staff.Role, password string) (int64, error)
	List(ctx context.Context) ([]staff.Account, error)
	Update(ctx context.Context, id int64, name string, role staff.Role, active bool) error
	SetPassword(ctx context.Context, id int64, password string) error
}

const sessionCookie = "ybr_admin"

type ctxKey int

const memberKey ctxKey = 1

// currentMember returns the signed-in staff member set by requireStaff.
func currentMember(r *http.Request) staff.Member {
	m, _ := r.Context().Value(memberKey).(staff.Member)
	return m
}

// adminAuth bundles what the admin routes share.
type adminAuth struct {
	staff          StaffService
	secureCookies  bool     // true in production (HTTPS only)
	allowedOrigins []string // where the admin app is served from
	loginLimit     *rateLimiter
}

// checkOrigin rejects admin writes coming from other websites.
// The session cookie is SameSite=Lax as well; this is a second lock on the same door.
func (a *adminAuth) checkOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			origin := r.Header.Get("Origin")
			if origin != "" && !slices.Contains(a.allowedOrigins, origin) {
				writeError(w, http.StatusForbidden, "bad_origin", "This request came from another website and was blocked.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// requireStaff lets the request through only for a signed-in staff member
// whose role has at least one of the permissions.
func (a *adminAuth) requireStaff(perms ...staff.Permission) func(http.HandlerFunc) http.Handler {
	return func(next http.HandlerFunc) http.Handler {
		return a.checkOrigin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, _ := r.Cookie(sessionCookie)
			token := ""
			if c != nil {
				token = c.Value
			}
			m, err := a.staff.Authenticate(r.Context(), token)
			if errors.Is(err, staff.ErrNoSession) {
				writeError(w, http.StatusUnauthorized, "signed_out", "Please sign in again.")
				return
			}
			if err != nil {
				internalError(w, "authenticate", err)
				return
			}
			if len(perms) > 0 && !slices.ContainsFunc(perms, m.Role.Can) {
				writeError(w, http.StatusForbidden, "forbidden", "Your role can't do this. Ask the shop owner if you need access.")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), memberKey, m)))
		}))
	}
}

func (a *adminAuth) setSession(w http.ResponseWriter, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/api/admin",
		MaxAge:   maxAge,
		HttpOnly: true, // JavaScript can't read it
		Secure:   a.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

type meResponse struct {
	Staff       staff.Member       `json:"staff"`
	Permissions []staff.Permission `json:"permissions"`
}

// POST /api/admin/v1/auth/login {"email","password"}
func (a *adminAuth) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	key := clientIP(r) + "|" + strings.ToLower(strings.TrimSpace(in.Email))
	if !a.loginLimit.Allow(key) {
		writeError(w, http.StatusTooManyRequests, "too_many_attempts", "Too many sign-in attempts. Wait 15 minutes and try again.")
		return
	}
	m, token, err := a.staff.Login(r.Context(), in.Email, in.Password)
	if errors.Is(err, staff.ErrBadLogin) {
		writeError(w, http.StatusUnauthorized, "bad_login", "That email and password don't match.")
		return
	}
	if err != nil {
		internalError(w, "login", err)
		return
	}
	a.loginLimit.Reset(key)
	a.setSession(w, token, int(staff.SessionLength/time.Second))
	slog.Info("staff signed in", "staff_id", m.ID, "role", m.Role)
	writeJSON(w, http.StatusOK, meResponse{Staff: m, Permissions: m.Role.Permissions()})
}

// POST /api/admin/v1/auth/logout
func (a *adminAuth) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := a.staff.Logout(r.Context(), c.Value); err != nil {
			internalError(w, "logout", err)
			return
		}
	}
	a.setSession(w, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/admin/v1/auth/me
func (a *adminAuth) me(w http.ResponseWriter, r *http.Request) {
	m := currentMember(r)
	writeJSON(w, http.StatusOK, meResponse{Staff: m, Permissions: m.Role.Permissions()})
}

// ---------- Staff screen (owner only) ----------

func (a *adminAuth) listStaff(w http.ResponseWriter, r *http.Request) {
	list, err := a.staff.List(r.Context())
	if err != nil {
		internalError(w, "list staff", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"staff": list})
}

func (a *adminAuth) createStaff(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string     `json:"email"`
		Name     string     `json:"name"`
		Role     staff.Role `json:"role"`
		Password string     `json:"password"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	id, err := a.staff.Create(r.Context(), in.Email, in.Name, in.Role, in.Password)
	if a.staffError(w, err) {
		return
	}
	slog.Info("staff created", "by", currentMember(r).ID, "staff_id", id, "role", in.Role)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (a *adminAuth) updateStaff(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		Name   string     `json:"name"`
		Role   staff.Role `json:"role"`
		Active bool       `json:"active"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if a.staffError(w, a.staff.Update(r.Context(), id, in.Name, in.Role, in.Active)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *adminAuth) setStaffPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if a.staffError(w, a.staff.SetPassword(r.Context(), id, in.Password)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// staffError writes the response for a staff-service error. It returns true if there was one.
func (a *adminAuth) staffError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, staff.ErrInvalid):
		writeError(w, http.StatusBadRequest, "invalid", "Check the details: a valid email, a name, a role, and a password of at least 10 characters.")
	case errors.Is(err, staff.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken", "Another staff account already uses that email.")
	case errors.Is(err, staff.ErrLastOwner):
		writeError(w, http.StatusConflict, "last_owner", "The shop must keep at least one active owner.")
	case errors.Is(err, staff.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "That staff account doesn't exist.")
	default:
		internalError(w, "staff", err)
	}
	return true
}

// ---------- shared helpers ----------

// decodeJSON reads a JSON body of at most 1 MB into v. On failure it writes a 400 and returns false.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "bad_json", "The request couldn't be read. Reload the page and try again.")
		return false
	}
	return true
}
