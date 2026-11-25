package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"yetibyrash/backend/internal/catalog"
	"yetibyrash/backend/internal/httpapi"
	"yetibyrash/backend/internal/media"
	"yetibyrash/backend/internal/staff"
	"yetibyrash/backend/internal/testdb"
)

const adminOrigin = "http://localhost:5174"

// client is a browser-like test client: it keeps cookies and sends the admin's Origin header.
type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func newClient(t *testing.T, base string) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: base, http: &http.Client{Jar: jar}}
}

func (c *client) do(method, path string, body any, origin string) (int, map[string]any) {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.base+path, r)
	req.Header.Set("Content-Type", "application/json")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	return c.send(req)
}

func (c *client) send(req *http.Request) (int, map[string]any) {
	c.t.Helper()
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	out := map[string]any{}
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func errCode(body map[string]any) string {
	e, _ := body["error"].(map[string]any)
	s, _ := e["code"].(string)
	return s
}

func TestAdminEndToEnd(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()

	storage, err := media.NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	staffSvc := staff.NewService(pool)
	if _, err := staffSvc.Create(ctx, "owner@example.test", "Owner", staff.RoleOwner, "owner-password-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := staffSvc.Create(ctx, "editor@example.test", "Editor", staff.RoleContentEditor, "editor-password-1"); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(httpapi.NewRouter(httpapi.Deps{
		DB:             pool,
		Catalog:        catalog.NewService(pool),
		Staff:          staffSvc,
		Admin:          catalog.NewAdminService(pool, storage),
		Media:          storage.Handler(),
		AllowedOrigins: []string{adminOrigin},
	}))
	defer srv.Close()

	owner := newClient(t, srv.URL)

	// Signed out: admin endpoints refuse.
	if code, body := owner.do("GET", "/api/admin/v1/auth/me", nil, ""); code != 401 || errCode(body) != "signed_out" {
		t.Fatalf("me while signed out = %d %v", code, body)
	}
	// Wrong password.
	if code, _ := owner.do("POST", "/api/admin/v1/auth/login", map[string]string{"email": "owner@example.test", "password": "nope"}, adminOrigin); code != 401 {
		t.Fatalf("wrong password = %d, want 401", code)
	}
	// Login from another website is blocked.
	if code, body := owner.do("POST", "/api/admin/v1/auth/login", map[string]string{"email": "owner@example.test", "password": "owner-password-1"}, "https://evil.example"); code != 403 || errCode(body) != "bad_origin" {
		t.Fatalf("login from other origin = %d %v", code, body)
	}
	// Right password, email in different case.
	code, body := owner.do("POST", "/api/admin/v1/auth/login", map[string]string{"email": "OWNER@example.test", "password": "owner-password-1"}, adminOrigin)
	if code != 200 {
		t.Fatalf("login = %d %v", code, body)
	}
	if code, body := owner.do("GET", "/api/admin/v1/auth/me", nil, ""); code != 200 || body["staff"].(map[string]any)["role"] != "owner" {
		t.Fatalf("me = %d %v", code, body)
	}

	// Load the dropdowns the editor needs.
	code, setup := owner.do("GET", "/api/admin/v1/setup", nil, "")
	if code != 200 {
		t.Fatalf("setup = %d", code)
	}
	var joggersID, clothingSizesID, espressoID, blushID float64
	for _, top := range setup["categories"].([]any) {
		for _, ch := range top.(map[string]any)["children"].([]any) {
			if ch.(map[string]any)["slug"] == "joggers" {
				joggersID = ch.(map[string]any)["id"].(float64)
			}
		}
	}
	for _, s := range setup["sizeSets"].([]any) {
		if s.(map[string]any)["name"] == "Clothing sizes (UK)" {
			clothingSizesID = s.(map[string]any)["id"].(float64)
		}
	}
	sizeIDs := map[string]float64{}
	for _, s := range setup["sizeSets"].([]any) {
		if s.(map[string]any)["id"].(float64) == clothingSizesID {
			for _, sz := range s.(map[string]any)["sizes"].([]any) {
				sizeIDs[sz.(map[string]any)["label"].(string)] = sz.(map[string]any)["id"].(float64)
			}
		}
	}
	for _, c := range setup["colours"].([]any) {
		switch c.(map[string]any)["name"] {
		case "Espresso":
			espressoID = c.(map[string]any)["id"].(float64)
		case "Blush":
			blushID = c.(map[string]any)["id"].(float64)
		}
	}
	if joggersID == 0 || espressoID == 0 || len(sizeIDs) != 5 {
		t.Fatalf("setup missing data: joggers=%v espresso=%v sizes=%v", joggersID, espressoID, sizeIDs)
	}

	// Create a product: invalid first (no name), then valid.
	product := map[string]any{
		"name": "", "categoryId": joggersID, "tier": "everyday", "status": "visible",
		"variants": []map[string]any{},
	}
	if code, body := owner.do("POST", "/api/admin/v1/products", product, adminOrigin); code != 400 || body["error"].(map[string]any)["field"] != "name" {
		t.Fatalf("create without name = %d %v", code, body)
	}
	product["name"] = "Wide-leg lounge pants"
	product["materialLine"] = "Cotton-blend rib"
	product["variants"] = []map[string]any{
		{"colourId": espressoID, "sizeId": sizeIDs["S"], "sku": "wl-esp-s", "priceKobo": 1600000, "stock": 4, "active": true},
		{"colourId": espressoID, "sizeId": sizeIDs["M"], "sku": "WL-ESP-M", "priceKobo": 1600000, "stock": 0, "active": true},
	}
	code, body = owner.do("POST", "/api/admin/v1/products", product, adminOrigin)
	if code != 201 {
		t.Fatalf("create = %d %v", code, body)
	}
	id := int(body["id"].(float64))
	path := "/api/admin/v1/products/" + itoa(id)

	// Edit: add a Blush option, drop Espresso M. The shop should see 2 options, not 3.
	product["variants"] = []map[string]any{
		{"colourId": espressoID, "sizeId": sizeIDs["S"], "sku": "WL-ESP-S", "priceKobo": 1650000, "stock": 4, "active": true},
		{"colourId": blushID, "sizeId": sizeIDs["S"], "sku": "WL-BLU-S", "priceKobo": 1650000, "compareAtKobo": 1900000, "stock": 2, "active": true},
	}
	if code, body := owner.do("PUT", path, product, adminOrigin); code != 204 {
		t.Fatalf("update = %d %v", code, body)
	}
	code, got := owner.do("GET", path, nil, "")
	if code != 200 || got["slug"] != "wide-leg-lounge-pants" {
		t.Fatalf("get = %d slug=%v", code, got["slug"])
	}
	active := 0
	for _, v := range got["variants"].([]any) {
		if v.(map[string]any)["active"] == true {
			active++
		}
	}
	if len(got["variants"].([]any)) != 3 || active != 2 {
		t.Fatalf("variants = %d (active %d), want 3 kept with 2 active", len(got["variants"].([]any)), active)
	}

	// A duplicate SKU from another product is refused with the field named.
	dup := product
	dup["name"] = "Copy"
	dup["variants"] = []map[string]any{{"colourId": espressoID, "sizeId": sizeIDs["S"], "sku": "WL-ESP-S", "priceKobo": 100, "stock": 1, "active": true}}
	if code, body := owner.do("POST", "/api/admin/v1/products", dup, adminOrigin); code != 400 || !strings.Contains(body["error"].(map[string]any)["message"].(string), "already used") {
		t.Fatalf("duplicate sku = %d %v", code, body)
	}

	// Upload a photo; it shows on the shop card.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("photo", "pants.png")
	img := image.NewRGBA(image.Rect(0, 0, 1300, 1400))
	for y := 0; y < 1400; y += 1 {
		for x := 0; x < 1300; x++ {
			img.Set(x, y, color.RGBA{230, 200, 190, 255})
		}
	}
	png.Encode(fw, img)
	mw.WriteField("kind", "studio")
	mw.Close()
	req, _ := http.NewRequest("POST", srv.URL+path+"/images", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Origin", adminOrigin)
	code, photo := owner.send(req)
	if code != 201 || photo["key"] == "" {
		t.Fatalf("upload = %d %v", code, photo)
	}
	res, err := http.Get(srv.URL + "/media/" + photo["key"].(string) + "-800.jpg")
	if err != nil || res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("photo file: err=%v status=%v type=%v", err, res.StatusCode, res.Header.Get("Content-Type"))
	}
	res.Body.Close()

	code, shop := owner.do("GET", "/api/v1/products?category=joggers", nil, "")
	found := false
	for _, it := range shop["items"].([]any) {
		m := it.(map[string]any)
		if m["slug"] == "wide-leg-lounge-pants" {
			found = true
			if m["image"] != photo["key"] || m["priceKobo"].(float64) != 1650000 {
				t.Fatalf("shop card = %v", m)
			}
		}
	}
	if code != 200 || !found {
		t.Fatalf("new product not in shop: %d %v", code, shop)
	}

	// A content editor can edit products but not catalogue setup or staff.
	editor := newClient(t, srv.URL)
	if code, _ := editor.do("POST", "/api/admin/v1/auth/login", map[string]string{"email": "editor@example.test", "password": "editor-password-1"}, adminOrigin); code != 200 {
		t.Fatalf("editor login = %d", code)
	}
	if code, _ := editor.do("GET", path, nil, ""); code != 200 {
		t.Fatalf("editor get product = %d", code)
	}
	if code, body := editor.do("POST", "/api/admin/v1/colours", map[string]string{"name": "Mint", "hex": "#AAFFCC"}, adminOrigin); code != 403 || errCode(body) != "forbidden" {
		t.Fatalf("editor add colour = %d %v", code, body)
	}
	if code, _ := editor.do("GET", "/api/admin/v1/staff", nil, ""); code != 403 {
		t.Fatalf("editor list staff = %d", code)
	}

	// Removing a size that products use is refused.
	sizes := []map[string]any{{"id": sizeIDs["XS"], "label": "XS"}, {"id": sizeIDs["M"], "label": "M"}}
	if code, body := owner.do("PUT", "/api/admin/v1/size-sets/"+itoa(int(clothingSizesID)), map[string]any{"name": "Clothing sizes (UK)", "sizes": sizes}, adminOrigin); code != 409 || errCode(body) != "in_use" {
		t.Fatalf("remove used size = %d %v", code, body)
	}

	// The last owner can't be deactivated.
	code, list := owner.do("GET", "/api/admin/v1/staff", nil, "")
	var ownerID float64
	for _, s := range list["staff"].([]any) {
		if s.(map[string]any)["role"] == "owner" {
			ownerID = s.(map[string]any)["id"].(float64)
		}
	}
	if code, body := owner.do("PUT", "/api/admin/v1/staff/"+itoa(int(ownerID)), map[string]any{"name": "Owner", "role": "owner", "active": false}, adminOrigin); code != 409 || errCode(body) != "last_owner" {
		t.Fatalf("deactivate last owner = %d %v", code, body)
	}

	// Sign out ends the session.
	if code, _ := owner.do("POST", "/api/admin/v1/auth/logout", nil, adminOrigin); code != 204 {
		t.Fatalf("logout = %d", code)
	}
	if code, _ := owner.do("GET", "/api/admin/v1/auth/me", nil, ""); code != 401 {
		t.Fatalf("me after logout = %d, want 401", code)
	}
}

func TestLoginRateLimit(t *testing.T) {
	pool := testdb.New(t)
	srv := httptest.NewServer(httpapi.NewRouter(httpapi.Deps{
		DB: pool, Catalog: catalog.NewService(pool), Staff: staff.NewService(pool),
		Admin: catalog.NewAdminService(pool, nil), AllowedOrigins: []string{adminOrigin},
	}))
	defer srv.Close()

	c := newClient(t, srv.URL)
	for i := 0; i < 5; i++ {
		c.do("POST", "/api/admin/v1/auth/login", map[string]string{"email": "x@example.test", "password": "wrong"}, adminOrigin)
	}
	if code, body := c.do("POST", "/api/admin/v1/auth/login", map[string]string{"email": "x@example.test", "password": "wrong"}, adminOrigin); code != 429 || errCode(body) != "too_many_attempts" {
		t.Fatalf("6th attempt = %d %v, want 429", code, body)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }
