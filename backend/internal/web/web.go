// Package web serves the built shop and admin apps from the same server as the API,
// so the whole site is one program on one address.
package web

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Handler serves the shop at / and the admin at /admin/. Both are single-page apps:
// a path that isn't a file gets the app's index.html, and the app shows the right page.
// shopDir and adminDir are the Vite build folders (frontend/shop/dist, frontend/admin/dist).
func Handler(shopDir, adminDir string) http.Handler {
	shop := spa{dir: shopDir}
	admin := spa{dir: adminDir}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p := r.URL.Path
		switch {
		case p == "/admin":
			http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
		case strings.HasPrefix(p, "/admin/"):
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
			admin.serve(w, r, strings.TrimPrefix(p, "/admin"))
		default:
			shop.serve(w, r, p)
		}
	})
}

type spa struct{ dir string }

func (s spa) serve(w http.ResponseWriter, r *http.Request, p string) {
	clean := path.Clean("/" + p)
	file := filepath.Join(s.dir, filepath.FromSlash(clean))
	if info, err := os.Stat(file); err == nil && !info.IsDir() && clean != "/index.html" {
		if strings.HasPrefix(clean, "/assets/") {
			// Vite puts a content hash in these names, so they can be cached for good.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		http.ServeFile(w, r, file)
		return
	}
	if strings.HasPrefix(clean, "/assets/") || path.Ext(clean) != "" {
		// A missing script, style or image: a real 404, not the app page.
		http.NotFound(w, r)
		return
	}
	// The app page itself is never cached, so a new release shows up straight away.
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, filepath.Join(s.dir, "index.html"))
}
