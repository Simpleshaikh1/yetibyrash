package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientIPBehindProxy(t *testing.T) {
	defer func(h int) { trustedProxyHops = h }(trustedProxyHops)
	req := func(xff string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.0.0.5:4444" // Render's proxy
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return r
	}

	trustedProxyHops = 0 // development: the direct connection
	if got := clientIP(req("1.2.3.4")); got != "10.0.0.5" {
		t.Errorf("no proxy: %s", got)
	}

	trustedProxyHops = 1 // Render
	if got := clientIP(req("102.89.4.7")); got != "102.89.4.7" {
		t.Errorf("one hop: %s", got)
	}
	// A visitor sends their own X-Forwarded-For to dodge rate limits; the proxy appends the real one.
	if got := clientIP(req("9.9.9.9, 102.89.4.7")); got != "102.89.4.7" {
		t.Errorf("spoofed: %s, want the address our proxy added", got)
	}
	if got := clientIP(req("")); got != "10.0.0.5" {
		t.Errorf("no header: %s", got)
	}
	if got := clientIP(req("not-an-ip")); got != "10.0.0.5" {
		t.Errorf("junk header: %s", got)
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := securityHeaders(SecurityOptions{HTTPSOnly: true, MediaOrigin: "https://media.yetibyrash.com"},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/p/gold-hoops", nil))
	csp := rec.Header().Get("Content-Security-Policy")
	for _, want := range []string{"script-src 'self'", "frame-ancestors 'none'", "https://media.yetibyrash.com", "https://fonts.gstatic.com"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP missing %q: %s", want, csp)
		}
	}
	if rec.Header().Get("Strict-Transport-Security") == "" || rec.Header().Get("X-Frame-Options") != "DENY" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("headers: %v", rec.Header())
	}
	if rec.Header().Get("Cache-Control") != "" {
		t.Errorf("page got API cache header")
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("API responses must not be cached: %q", rec.Header().Get("Cache-Control"))
	}

	dev := securityHeaders(SecurityOptions{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	rec = httptest.NewRecorder()
	dev.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS sent in development")
	}
}
