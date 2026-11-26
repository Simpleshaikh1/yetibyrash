// Package httpapi holds the HTTP routes, handlers and middleware.
package httpapi

import (
	"context"
	"net/http"
	"time"

	"yetibyrash/backend/internal/cart"
	"yetibyrash/backend/internal/content"
	"yetibyrash/backend/internal/customers"
	"yetibyrash/backend/internal/delivery"
	"yetibyrash/backend/internal/notify"
	"yetibyrash/backend/internal/orders"
	"yetibyrash/backend/internal/paystack"
	"yetibyrash/backend/internal/promotions"
	"yetibyrash/backend/internal/reviews"
	"yetibyrash/backend/internal/staff"
)

// Pinger is anything that can check the database is reachable.
// *pgxpool.Pool satisfies it; tests pass a fake.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps is everything the handlers need. It grows as milestones add features.
type Deps struct {
	DB      Pinger
	Catalog CatalogService
	Staff   StaffService
	Admin   AdminCatalog
	Media   http.Handler // serves /media/<key> (product photos)
	Version string

	Cart     *cart.Service
	Delivery *delivery.Service
	Orders   *orders.Service
	// PaystackSecret verifies webhooks. DevPay is set only in development without Paystack keys.
	PaystackSecret string
	DevPay         *paystack.DevProvider
	Notify         *notify.Orders // customer message templates
	Content        *content.Service
	Promotions     *promotions.Service
	Customers      *customers.Service
	Reviews        *reviews.Service
	Cards          CardLoader // product cards by id, for saved items

	SecureCookies  bool     // send cookies over HTTPS only (production)
	AllowedOrigins []string // admin app addresses allowed to make changes
}

// NewRouter registers every route and wraps them in the shared middleware.
func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", healthHandler(d))

	// Shop catalogue
	mux.HandleFunc("GET /api/v1/home", homeHandler(d.Catalog))
	mux.HandleFunc("GET /api/v1/categories", categoriesHandler(d.Catalog))
	mux.HandleFunc("GET /api/v1/products", productsHandler(d.Catalog))
	mux.HandleFunc("GET /api/v1/products/{slug}", productHandler(d.Catalog))

	var accounts *accountHandlers
	if d.Customers != nil && d.Cards != nil {
		accounts = newAccountHandlers(d.Customers, d.Cards, d.SecureCookies)
		a := accounts
		mux.HandleFunc("POST /api/v1/auth/code", a.sendCode)
		mux.HandleFunc("POST /api/v1/auth/verify", a.verify)
		mux.HandleFunc("POST /api/v1/auth/logout", a.logout)
		mux.HandleFunc("GET /api/v1/me", a.me)
		mux.HandleFunc("PUT /api/v1/me", a.signedIn(a.updateMe))
		mux.HandleFunc("DELETE /api/v1/me", a.signedIn(a.deleteMe))
		mux.HandleFunc("GET /api/v1/me/orders", a.signedIn(a.orders))
		mux.HandleFunc("POST /api/v1/me/orders/{number}/review", a.signedIn(a.reviewLink))
		mux.HandleFunc("GET /api/v1/me/addresses", a.signedIn(a.addresses))
		mux.HandleFunc("DELETE /api/v1/me/addresses/{id}", a.signedIn(a.deleteAddress))
		mux.HandleFunc("GET /api/v1/me/saved", a.signedIn(a.saved))
		mux.HandleFunc("PUT /api/v1/me/saved/{productId}", a.signedIn(a.save))
		mux.HandleFunc("DELETE /api/v1/me/saved/{productId}", a.signedIn(a.unsave))
	}
	if d.Reviews != nil {
		rh := reviewHandlers{rv: d.Reviews}
		mux.HandleFunc("GET /api/v1/reviews/{token}", rh.request)
		mux.HandleFunc("POST /api/v1/reviews/{token}", rh.submit)
		mux.HandleFunc("POST /api/v1/reviews/{token}/{id}/photos", rh.photo)
	}

	if d.Orders != nil {
		registerShopOrders(mux, d, accounts)
	}
	if d.Content != nil {
		sh := siteHandlers{c: d.Content}
		mux.HandleFunc("GET /api/v1/site", sh.site)
		mux.HandleFunc("GET /api/v1/pages/{slug}", sh.page)
	}

	if d.Media != nil {
		mux.Handle("GET /media/", d.Media)
	}

	if d.Staff != nil {
		registerAdmin(mux, d)
	}

	// Unknown /api paths get a JSON 404 instead of Go's plain-text one.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "There is no API endpoint at this address.")
	})

	return recoverPanics(logRequests(mux))
}

func registerShopOrders(mux *http.ServeMux, d Deps, accounts *accountHandlers) {
	h := &shopHandlers{
		cart: d.Cart, delivery: d.Delivery, orders: d.Orders, accounts: accounts,
		paystackSecret: d.PaystackSecret, devPay: d.DevPay, secureCookies: d.SecureCookies,
	}
	mux.HandleFunc("GET /api/v1/cart", h.getCart)
	mux.HandleFunc("POST /api/v1/cart/items", h.addToCart)
	mux.HandleFunc("PUT /api/v1/cart/items/{variantId}", h.setCartQty)
	mux.HandleFunc("POST /api/v1/cart/code", h.applyCode)
	mux.HandleFunc("DELETE /api/v1/cart/code", h.removeCode)
	mux.HandleFunc("GET /api/v1/locations", h.locations)
	mux.HandleFunc("GET /api/v1/delivery", h.deliverySummary)
	mux.HandleFunc("POST /api/v1/checkout/quote", h.quote)
	mux.HandleFunc("POST /api/v1/checkout", h.checkout)
	mux.HandleFunc("GET /api/v1/orders/{number}", h.order)
	mux.HandleFunc("POST /api/webhooks/paystack", h.paystackWebhook)
	mux.HandleFunc("POST /api/v1/track", newTrackHandler(d.Orders).track)
	if d.DevPay != nil {
		mux.HandleFunc("GET /api/v1/dev/pay/{reference}", h.devPayForm)
		mux.HandleFunc("POST /api/v1/dev/pay/{reference}", h.devPaySubmit)
	}
}

func registerAdmin(mux *http.ServeMux, d Deps) {
	a := &adminAuth{
		staff:          d.Staff,
		secureCookies:  d.SecureCookies,
		allowedOrigins: d.AllowedOrigins,
		loginLimit:     newRateLimiter(5, 15*time.Minute),
	}
	h := adminCatalogHandlers{c: d.Admin}

	anyStaff := a.requireStaff()
	products := a.requireStaff(staff.PermProducts)
	catalogue := a.requireStaff(staff.PermCatalogue)
	productsOrCatalogue := a.requireStaff(staff.PermProducts, staff.PermCatalogue)
	owner := a.requireStaff(staff.PermStaff)

	const p = "/api/admin/v1"
	mux.Handle("POST "+p+"/auth/login", a.checkOrigin(http.HandlerFunc(a.login)))
	mux.Handle("POST "+p+"/auth/logout", a.checkOrigin(http.HandlerFunc(a.logout)))
	mux.Handle("GET "+p+"/auth/me", anyStaff(a.me))

	mux.Handle("GET "+p+"/setup", productsOrCatalogue(h.setup))

	mux.Handle("GET "+p+"/products", products(h.listProducts))
	mux.Handle("POST "+p+"/products", products(h.createProduct))
	mux.Handle("GET "+p+"/products/{id}", products(h.getProduct))
	mux.Handle("PUT "+p+"/products/{id}", products(h.updateProduct))
	mux.Handle("POST "+p+"/products/{id}/images", products(h.uploadImage))
	mux.Handle("DELETE "+p+"/products/{id}/images/{imageId}", products(h.deleteImage))
	mux.Handle("POST "+p+"/products/{id}/images/{imageId}/main", products(h.makeImageMain))

	mux.Handle("POST "+p+"/size-sets", catalogue(h.saveSizeSet))
	mux.Handle("PUT "+p+"/size-sets/{id}", catalogue(h.saveSizeSet))
	mux.Handle("POST "+p+"/colours", catalogue(h.saveColour))
	mux.Handle("PUT "+p+"/colours/{id}", catalogue(h.saveColour))
	mux.Handle("DELETE "+p+"/colours/{id}", catalogue(h.deleteColour))
	mux.Handle("POST "+p+"/categories", catalogue(h.saveCategory))
	mux.Handle("PUT "+p+"/categories/{id}", catalogue(h.saveCategory))
	mux.Handle("PUT "+p+"/categories/{id}/fields", catalogue(h.saveCategoryFields))

	if d.Delivery != nil {
		dh := adminDeliveryHandlers{d: d.Delivery}
		mux.Handle("GET "+p+"/delivery", catalogue(dh.get))
		mux.Handle("PUT "+p+"/delivery/zones/{id}", catalogue(dh.updateZone))
		mux.Handle("PUT "+p+"/delivery/states/{id}", catalogue(dh.setStateZone))
		mux.Handle("POST "+p+"/delivery/states/{id}/lgas", catalogue(dh.addLga))
		mux.Handle("PUT "+p+"/delivery/lgas/{id}", catalogue(dh.updateLga))
		mux.Handle("DELETE "+p+"/delivery/lgas/{id}", catalogue(dh.deleteLga))
		mux.Handle("PUT "+p+"/delivery/settings", catalogue(dh.updateSettings))
		mux.Handle("POST "+p+"/delivery/holidays", catalogue(dh.addHoliday))
		mux.Handle("DELETE "+p+"/delivery/holidays/{day}", catalogue(dh.deleteHoliday))
	}

	if d.Orders != nil && d.Delivery != nil && d.Notify != nil {
		oh := adminOrderHandlers{orders: d.Orders, delivery: d.Delivery, notify: d.Notify}
		ordersPerm := a.requireStaff(staff.PermOrders)
		mux.Handle("GET "+p+"/overview", anyStaff(oh.overview))
		mux.Handle("GET "+p+"/orders", ordersPerm(oh.list))
		mux.Handle("GET "+p+"/orders/{id}", ordersPerm(oh.get))
		mux.Handle("POST "+p+"/orders/{id}/advance", ordersPerm(oh.advance))
		mux.Handle("POST "+p+"/orders/{id}/cancel", ordersPerm(oh.cancel))
		mux.Handle("GET "+p+"/couriers", a.requireStaff(staff.PermOrders, staff.PermCatalogue)(oh.couriers))
		mux.Handle("POST "+p+"/couriers", catalogue(oh.saveCourier))
		mux.Handle("PUT "+p+"/couriers/{id}", catalogue(oh.saveCourier))
		mux.Handle("GET "+p+"/messages", catalogue(oh.messages))
		mux.Handle("PUT "+p+"/messages/{event}", catalogue(oh.saveMessage))
		mux.Handle("POST "+p+"/messages/preview", catalogue(oh.previewMessage))
	}

	if d.Content != nil {
		ch := adminContentHandlers{c: d.Content}
		contentPerm := a.requireStaff(staff.PermContent)
		mux.Handle("GET "+p+"/content", contentPerm(ch.get))
		mux.Handle("POST "+p+"/content/banners", contentPerm(ch.saveBanner))
		mux.Handle("PUT "+p+"/content/banners/{id}", contentPerm(ch.saveBanner))
		mux.Handle("DELETE "+p+"/content/banners/{id}", contentPerm(ch.deleteBanner))
		mux.Handle("POST "+p+"/content/banners/{id}/image", contentPerm(ch.bannerImage))
		mux.Handle("PUT "+p+"/content/banner-order", contentPerm(ch.orderBanners))
		mux.Handle("POST "+p+"/content/announcements", contentPerm(ch.saveAnnouncement))
		mux.Handle("PUT "+p+"/content/announcements/{id}", contentPerm(ch.saveAnnouncement))
		mux.Handle("DELETE "+p+"/content/announcements/{id}", contentPerm(ch.deleteAnnouncement))
		mux.Handle("PUT "+p+"/content/announcement-order", contentPerm(ch.orderAnnouncements))
		mux.Handle("PUT "+p+"/content/sections", contentPerm(ch.saveSections))
		mux.Handle("POST "+p+"/content/pages", contentPerm(ch.savePage))
		mux.Handle("PUT "+p+"/content/pages/{id}", contentPerm(ch.savePage))
		mux.Handle("DELETE "+p+"/content/pages/{id}", contentPerm(ch.deletePage))
	}
	if d.Promotions != nil {
		ph := adminPromoHandlers{p: d.Promotions}
		promoPerm := a.requireStaff(staff.PermPromotions)
		mux.Handle("GET "+p+"/promotions", promoPerm(ph.get))
		mux.Handle("POST "+p+"/promotions/sales", promoPerm(ph.saveSale))
		mux.Handle("PUT "+p+"/promotions/sales/{id}", promoPerm(ph.saveSale))
		mux.Handle("DELETE "+p+"/promotions/sales/{id}", promoPerm(ph.deleteSale))
		mux.Handle("POST "+p+"/promotions/codes", promoPerm(ph.saveCode))
		mux.Handle("PUT "+p+"/promotions/codes/{id}", promoPerm(ph.saveCode))
		mux.Handle("POST "+p+"/promotions/gifts", promoPerm(ph.saveGift))
		mux.Handle("PUT "+p+"/promotions/gifts/{id}", promoPerm(ph.saveGift))
	}

	if d.Reviews != nil {
		rh := reviewHandlers{rv: d.Reviews}
		mux.Handle("GET "+p+"/reviews", products(rh.adminList))
		mux.Handle("PUT "+p+"/reviews/{id}/status", products(rh.adminStatus))
		mux.Handle("PUT "+p+"/reviews/{id}/reply", products(rh.adminReply))
	}

	mux.Handle("GET "+p+"/staff", owner(a.listStaff))
	mux.Handle("POST "+p+"/staff", owner(a.createStaff))
	mux.Handle("PUT "+p+"/staff/{id}", owner(a.updateStaff))
	mux.Handle("PUT "+p+"/staff/{id}/password", owner(a.setStaffPassword))
}
