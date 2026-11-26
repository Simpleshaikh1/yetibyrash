package httpapi

import (
	"net"
	"net/http"
	"strings"
)

// trustedProxyHops is how many proxies sit in front of the app (Render: 1). Set by NewRouter.
var trustedProxyHops = 0

// clientIP is the visitor's address, used for rate limits. Behind a proxy it's the address
// the proxy saw, read from the right end of X-Forwarded-For: a visitor can put anything at
// the left end, but not in the entries our own proxy appends.
func clientIP(r *http.Request) string {
	if trustedProxyHops > 0 {
		parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		if i := len(parts) - trustedProxyHops; i >= 0 && r.Header.Get("X-Forwarded-For") != "" {
			if ip := strings.TrimSpace(parts[i]); net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// SecurityOptions shape the security headers.
type SecurityOptions struct {
	HTTPSOnly   bool   // production: tell browsers to always use https (HSTS)
	MediaOrigin string // where photos load from, e.g. https://media.yetibyrash.com ("" = this site)
}

// securityHeaders adds headers that stop the site being framed, sniffed or made to load
// scripts from anywhere but itself.
func securityHeaders(o SecurityOptions, next http.Handler) http.Handler {
	img := "'self' data: blob:"
	if o.MediaOrigin != "" {
		img += " " + o.MediaOrigin
	}
	csp := strings.Join([]string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
		"font-src 'self' https://fonts.gstatic.com",
		"img-src " + img,
		"connect-src 'self'",
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"object-src 'none'",
	}, "; ")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		if o.HTTPSOnly {
			h.Set("Strict-Transport-Security", "max-age=15552000; includeSubDomains")
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("X-Robots-Tag", "noindex")
			h.Set("Cache-Control", "no-store") // personal data: never kept by browsers or proxies
		}
		next.ServeHTTP(w, r)
	})
}
