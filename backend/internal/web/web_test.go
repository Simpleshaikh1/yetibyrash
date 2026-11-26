package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestServesBothApps(t *testing.T) {
	root := t.TempDir()
	shop, admin := filepath.Join(root, "shop"), filepath.Join(root, "admin")
	writeFile(t, filepath.Join(shop, "index.html"), "shop page")
	writeFile(t, filepath.Join(shop, "assets", "app-abc123.js"), "shop js")
	writeFile(t, filepath.Join(admin, "index.html"), "admin page")
	writeFile(t, filepath.Join(admin, "assets", "app-def456.js"), "admin js")
	h := Handler(shop, admin)

	cases := []struct {
		path, wantBody, wantCache string
		wantStatus                int
	}{
		{"/", "shop page", "no-cache", 200},
		{"/p/gold-huggie-hoops", "shop page", "no-cache", 200}, // app route
		{"/assets/app-abc123.js", "shop js", "immutable", 200},
		{"/assets/missing.js", "", "", 404},
		{"/admin/", "admin page", "no-cache", 200},
		{"/admin/orders/12", "admin page", "no-cache", 200},
		{"/admin/assets/app-def456.js", "admin js", "immutable", 200},
		{"/../../etc/passwd", "", "", 400}, // climbing out of the folder is refused
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
		if rec.Code != c.wantStatus {
			t.Errorf("%s: status %d, want %d", c.path, rec.Code, c.wantStatus)
			continue
		}
		if c.wantStatus == 200 && !strings.Contains(rec.Body.String(), c.wantBody) {
			t.Errorf("%s: body %q, want %q", c.path, rec.Body.String(), c.wantBody)
		}
		if c.wantCache != "" && !strings.Contains(rec.Header().Get("Cache-Control"), c.wantCache) {
			t.Errorf("%s: cache %q, want %q", c.path, rec.Header().Get("Cache-Control"), c.wantCache)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin", nil))
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/admin/" {
		t.Errorf("/admin: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if got := rec.Header().Get("X-Robots-Tag"); got != "" {
		t.Errorf("redirect has X-Robots-Tag %q", got)
	}
}
