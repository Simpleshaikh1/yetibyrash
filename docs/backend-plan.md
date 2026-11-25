# Yetibyrash backend plan

Go API + two React apps (shop and admin) in one repository, worked on in one GoLand project.

This machine already has Go 1.27, Node 22 / npm 10 and Docker 26, which is everything needed.

---

## 1. Repository layout (one GoLand project)

```
Yetibyrash/
├─ backend/                  Go module
│  ├─ cmd/
│  │  └─ yetibyrash/main.go  one binary: API + background worker + serves built React apps
│  ├─ internal/
│  │  ├─ config/             env vars → typed config
│  │  ├─ httpapi/            router, middleware, JSON helpers, error format
│  │  ├─ catalog/            categories, size sets, colours, products, variants, images
│  │  ├─ pricing/            THE one place prices, discounts and fees are calculated
│  │  ├─ cart/
│  │  ├─ delivery/           zones, states, LGAs, fees, "get it by" dates
│  │  ├─ checkout/           turns a cart into a pending order + Paystack payment
│  │  ├─ paystack/           Paystack client + webhook verification
│  │  ├─ orders/             status changes, tracking, order events
│  │  ├─ promotions/         sales, discount codes, gift with purchase
│  │  ├─ reviews/
│  │  ├─ content/            banners, homepage sections, pages, size guides, looks
│  │  ├─ customers/          optional accounts, OTP sign-in, addresses
│  │  ├─ staff/              admin users, roles, sessions, audit log
│  │  ├─ media/              photo upload, square crop, resize, quality warnings
│  │  ├─ notify/             email + WhatsApp/SMS, message templates
│  │  ├─ jobs/               background job queue in Postgres
│  │  └─ db/                 sqlc-generated query code
│  ├─ migrations/            numbered SQL files (goose)
│  ├─ queries/               .sql files sqlc reads
│  ├─ seed/                  Nigerian states + LGAs, size sets, colours, demo products
│  ├─ api.http               GoLand HTTP Client requests for every endpoint
│  ├─ sqlc.yaml
│  └─ go.mod
├─ frontend/                 npm workspaces
│  ├─ shop/                  React + Vite + TypeScript (customers, phone first)
│  ├─ admin/                 React + Vite + TypeScript (staff dashboard)
│  └─ ui/                    shared tokens + components from the design system
├─ design/                   (already here) canvas source + design system
├─ docs/                     this plan, later API notes
├─ docker-compose.yml        Postgres, Mailpit (fake inbox), MinIO (fake S3)
├─ .run/                     GoLand run configurations, committed
└─ Makefile                  migrate, sqlc, seed, test, build
```

**Why two React apps:** the admin code (product editor, staff tools) never ships to shoppers' phones, so the shop stays small and fast on mobile data. Both share `frontend/ui`, so the look stays identical.

**Why one Go binary:** one thing to deploy. In production it serves the API at `/api`, the shop at `/`, and the admin at `/admin` (the built React files are embedded with `embed.FS`).

### GoLand setup

GoLand handles Go, TypeScript/React, SQL and HTTP requests in the same window.

| Run configuration | What it runs |
| --- | --- |
| `db` | `docker compose up` (Postgres :5432, Mailpit :8025, MinIO :9000) |
| `api` | Go Build of `backend/cmd/yetibyrash` on :8080 |
| `shop` | npm `dev` in `frontend/shop` on :5173 |
| `admin` | npm `dev` in `frontend/admin` on :5174 |
| `Yetibyrash (all)` | Compound: starts all four |

- Vite proxies `/api` to `:8080` in development, so there is no CORS setup and cookies just work.
- Connect the **Database** tool window to the local Postgres to browse tables and run queries.
- Use `backend/api.http` (GoLand HTTP Client) to call endpoints without the frontend.
- Commit `.run/` so the configurations survive a fresh clone.

---

## 2. Technology choices

| Need | Choice | Why |
| --- | --- | --- |
| HTTP routing | Standard library `net/http` (method + path patterns) | No framework to learn; enough for this API |
| Database | PostgreSQL 16 | Transactions for stock and payments, full-text search, JSON columns |
| DB driver | `pgx/v5` | Fast, native Postgres types |
| Queries | `sqlc` | You write SQL, it generates typed Go functions. No ORM magic |
| Migrations | `goose` | Plain `.sql` files, runs from Go on startup |
| Logging | `log/slog` | Built in, structured |
| Passwords | `golang.org/x/crypto/argon2` (argon2id) | Current best practice |
| Images | `disintegration/imaging` (pure Go) | Square crop + resizes without cgo, works on Windows |
| File storage | S3-compatible: MinIO locally, Cloudflare R2 in production | Cheap, no egress fees |
| Email | Resend or Postmark (Mailpit locally) | Reliable delivery |
| SMS / WhatsApp | Termii (Nigerian provider) or Meta WhatsApp Cloud API | WhatsApp templates need Meta approval; start with SMS + email |
| Background jobs | A `jobs` table + worker goroutine (`FOR UPDATE SKIP LOCKED`) | No Redis needed |
| TS types from Go | `tygo` | Go structs → TypeScript types, so the React apps match the API |

### Money rule

Store every amount as **kobo** in a `BIGINT` (₦14,500 = `1450000`). Paystack also uses kobo. Never use floats for money. Format as `₦14,500` only in the frontend.

### Time rule

Store times in UTC. Calculate delivery dates, cut-offs and "Order within 3h 20m" in `Africa/Lagos` time, skipping Sundays and Nigerian public holidays (a `holidays` table the admin can edit).

---

## 3. Data model

Tables grouped by area. `id` is a `bigint` identity unless noted. Every table has `created_at`/`updated_at`.

### Catalogue (everything the admin's Catalogue setup controls)
- `categories` — `parent_id`, `name`, `slug`, `position`, `default_size_set_id`, `return_rule`, `show_on_home`, `icon`, `filters text[]`
- `category_fields` — per-category product details: `category_id`, `key`, `label`, `position` (bags: dimensions, strap drop, closure, what fits; watches: case size, movement…)
- `size_sets` — `name` ("Clothing sizes (UK)"), `measure_label`, `guide_page_id`
- `sizes` — `size_set_id`, `label` ("M"), `measurement` ("UK 10–12 · bust 88…"), `position`
- `colours` — `name`, `hex`
- `materials` — `name`
- `price_bands` — `label`, `min_kobo`, `max_kobo`, `position`

### Products
- `products` — `slug`, `name`, `description`, `category_id`, `tier` (`everyday`|`premium`), `material_line`, `status` (`draft`|`visible`|`hidden`), `publish_at`, `badges text[]`, `gift_box_allowed`, `preorder_allowed`
- `product_details` — `product_id`, `field_id`, `value` (fills the category's fields)
- `variants` — `product_id`, `colour_id`, `size_id` (null for one-size), `sku` (unique), `price_kobo`, `compare_at_kobo`, `stock`, `active`
- `product_images` — `product_id`, `colour_id` (optional), `storage_key`, `kind` (`studio`|`real_life`), `position`, `width`, `height`, `warnings text[]`
- `product_pairings` — "Complete the look" links
- `looks`, `look_items` — named sets with a set price

### Customers and carts
- `customers` — `name`, `phone` (unique), `email`, `marketing_opt_in`
- `addresses` — `customer_id`, `state`, `lga_id`, `street`, `landmark`
- `otp_codes` — `target`, `code_hash`, `expires_at`, `attempts`
- `carts` — `token` (uuid, kept in a cookie), `customer_id` (optional)
- `cart_items` — `cart_id`, `variant_id`, `qty`, `gift_box`
- `stock_holds` — `variant_id`, `order_id`, `qty`, `expires_at` (30 minutes while paying)

### Orders and payments
- `orders` — `number` ("YB-20418", unique), copied customer name/phone/email, copied address, `zone_id`, `speed` (`standard`|`express`), `subtotal_kobo`, `discount_kobo`, `delivery_kobo`, `bulky_kobo`, `gift_kobo`, `total_kobo`, `promotion_id`, `discount_code`, `status`, `courier_id`, `tracking_number`, `expected_from`, `expected_to`, `gift_note`, `paid_at`
- `order_items` — **copies** of name, variant label, SKU and unit price at the time of purchase, `qty`, `gift_box`. (If a product is renamed or repriced later, old orders don't change.)
- `order_events` — `order_id`, `status`, `note`, `staff_id`, `created_at`. This is what the bead tracker shows.
- `payments` — `reference` (unique), `order_id` (nullable), `amount_kobo`, `channel` (card/bank_transfer/ussd), `status`, `paystack_payload jsonb`

### Delivery
- `delivery_zones` — `name`, `standard_fee_kobo`, `standard_days_min/max`, `express_fee_kobo`, `express_label`, `free_over_kobo`
- `states` — 36 states + FCT, `zone_id`
- `lgas` — `state_id`, `name`, `delivered`, `express`
- `delivery_settings` (one row) — `express_cutoff`, `dispatch_days`, `preorder_extra_days`, `bulky_fee_kobo`
- `holidays` — `date`, `name`
- `couriers` — `name`, `tracking_url_pattern`

### Promotions and reviews
- `promotions` — `name`, `kind` (percent/fixed), `value`, `applies_to` (all/tier/category), `target`, `starts_at`, `ends_at`, `badge_text`, `banner_id`, `exclude_on_sale`
- `discount_codes` — `code`, `kind`, `value`, `min_order_kobo`, `max_uses`, `used`, `first_order_only`, `active`
- `gift_offers` — `threshold_kobo`, `variant_id`, `stock`, `given`
- `reviews` — `product_id`, `order_item_id` (proves a real buyer), `rating`, `text`, `photos text[]`, `status` (waiting/published/hidden), `reply`

### Content
- `banners` — eyebrow, headline, button text/link, phone and desktop images, `tier`, `starts_at`, `ends_at`
- `homepage_sections` — `key`, `position`, `visible`, `config jsonb`
- `announcements` — rotating top-bar messages
- `pages` — `slug`, `title`, `body` (Markdown): About, FAQ, policies, size guides

### Staff, messages, system
- `staff` — `email`, `name`, `password_hash`, `role` (owner/manager/packer/content_editor), `active`
- `sessions` — staff login sessions (token hash, expiry)
- `audit_log` — who changed what and when, with before/after
- `message_templates` — `event` (paid, packed, shipped…), `channel`, `subject`, `body` with `{first_name}` placeholders
- `store_settings` — key/value: low-stock warning level, WhatsApp number, socials, logo
- `jobs` — `kind`, `payload jsonb`, `run_at`, `attempts`, `last_error`, `done_at`

---

## 4. Checkout and payment flow

The most important flow. **The server always calculates the total**; the browser never sends a price.

1. Shopper taps **Checkout**. Shop calls `POST /api/v1/checkout/quote` with the address and delivery speed. Server returns items, discount, delivery fee, bulky fee, total and the "get it by" date.
2. Shopper taps **Pay with Paystack**. Shop calls `POST /api/v1/checkout`.
   In one database transaction the server:
   - recalculates the total with `pricing.Quote` (same function as step 1),
   - checks stock and creates 30-minute `stock_holds`,
   - creates the order with status `pending_payment` and a unique Paystack `reference`,
   - calls Paystack *Initialize Transaction* with the amount in kobo.
   It returns Paystack's `access_code`; the shop opens the Paystack popup.
3. Shopper pays (card, bank transfer or USSD).
4. Paystack calls `POST /api/webhooks/paystack` (`charge.success`). The server:
   - checks the `x-paystack-signature` header (HMAC-SHA512 of the body with the secret key); rejects if wrong,
   - finds the order by `reference` and checks the amount matches `total_kobo` exactly,
   - in one transaction: marks the order `paid`, turns stock holds into real stock decreases, records the payment, adds an `order_events` row, queues the "Payment received" email + SMS/WhatsApp,
   - does nothing if this webhook was already processed (Paystack can send it twice).
5. The shop's Paystack callback lands on the confirmation page, which calls `GET /api/v1/orders/{number}/status`. That endpoint also calls Paystack *Verify Transaction* if the webhook hasn't arrived yet, so the shopper never sees "pending" for long.
6. A job releases stock holds and marks the order `abandoned` if no payment arrives within 30 minutes, and can send the "your bag is still waiting" message.

**"Paid but no order saved"** (the admin alert in the designs): a webhook with a reference the database doesn't know, or an amount that doesn't match. It is stored in `payments` with no order, shown on the Overview and Payments screens, and the admin can create the order from it or refund it in Paystack.

---

## 5. Order status rules

```
pending_payment → paid → packed → shipped → out_for_delivery → delivered
pending_payment → abandoned            (no payment in 30 min)
paid/packed     → cancelled → refunded (Owner only)
```

- Statuses only move forward one step at a time (the admin board's "Move to …").
- Every change writes an `order_events` row (shown on the tracker) and an `audit_log` row (who did it).
- Each change queues the customer message from `message_templates` for that step.
- **Shipped** requires a courier; the tracking number is optional.
- 3 days after **delivered**, a job sends the review request.

---

## 6. API outline

JSON over HTTPS. Errors are always `{"error": {"code": "out_of_stock", "message": "Only 2 left in size M."}}`.

### Shop (public, `/api/v1`)
| Method + path | Purpose |
| --- | --- |
| `GET /home?tier=everyday` | Hero, sections, announcements, New in, best sellers for the chosen tier |
| `GET /categories` | Menu tree with filters |
| `GET /products?category=&tier=&band=&colour=&size=&sort=&page=` | Shop grid |
| `GET /products/{slug}` | Product page: variants, images, details, size set, reviews summary, pairings, "get it by" |
| `GET /search?q=` | Postgres full-text + trigram search |
| `GET /locations/states`, `GET /locations/states/{id}/lgas` | Address dropdowns |
| `GET /cart`, `POST /cart/items`, `PATCH /cart/items/{id}`, `DELETE /cart/items/{id}` | Bag (cart token in a cookie) |
| `POST /cart/code` | Apply a discount code |
| `POST /checkout/quote` | Fees, total, delivery date |
| `POST /checkout` | Create order + Paystack transaction |
| `GET /orders/{number}/status` | Confirmation page |
| `GET /track?number=&phone=` | Track order without login (rate limited) |
| `POST /auth/otp`, `POST /auth/verify`, `POST /auth/logout` | Optional customer sign-in by one-time code |
| `GET /me/orders`, `GET/POST /me/addresses`, `GET/POST/DELETE /me/saved` | Account |
| `POST /reviews` | Submit a review (signed link from the review request) |

### Webhooks
`POST /api/webhooks/paystack`

### Admin (`/api/admin/v1`, staff session cookie, checked per role)
| Area | Endpoints |
| --- | --- |
| Auth | login, logout, me |
| Overview | today's numbers, needs-attention list, 7-day sales |
| Orders | list/board, detail, `POST /orders/{id}/advance`, courier + tracking, notes, packing slips |
| Products | list, create, update, hide/show, bulk actions, variants table, `POST /products/{id}/images` |
| Catalogue | categories, category fields, size sets + sizes, colours, materials, price bands |
| Content | banners, homepage sections, announcements, looks, pages, size guides |
| Promotions | promotions, discount codes, gift offers |
| Reviews | list by status, approve, hide, reply |
| Delivery | zones, states, LGAs, settings, holidays, couriers |
| Settings | message templates (+ send test), store details, staff + roles, payments list, create order from payment, audit log |

**Roles** (matches the Staff screen): Owner = everything; Manager = everything except staff and payments; Packer = orders only; Content editor = products, photos, content.

---

## 7. Photos

1. Admin uploads a photo to `POST /api/admin/v1/products/{id}/images`.
2. Server checks it is a real JPEG/PNG/WebP, max 15 MB.
3. It creates a centred square crop and three sizes (400, 800, 1600 px), stores them in object storage, and saves warnings: "smaller than 1200 px", "too dark" (average brightness below a threshold).
4. The admin sees the warnings on the photo tile; nothing is blocked.

---

## 8. Security checklist

- Totals, discounts and fees only ever come from `pricing.Quote` on the server.
- Paystack secret key only on the server (environment variable), never in React code. The public key is the only one the shop sees.
- Webhook signature checked on every call; amount checked against the order.
- Staff passwords hashed with argon2id; sessions in `HttpOnly`, `Secure`, `SameSite=Lax` cookies; check the `Origin` header on admin writes.
- Rate limits on OTP requests, sign-in and order tracking lookups.
- Every admin change goes to `audit_log`.
- Uploaded files are checked by content, not file name.
- NDPR (Nigeria Data Protection Regulation): privacy policy page, only collect what delivery needs, let customers ask for their data to be deleted.

---

## 9. Testing

- Unit tests for `pricing`, `delivery` (fees, cut-offs, holidays) and order status rules: these are where money and dates go wrong.
- Integration tests against the Docker Postgres for checkout + webhook (including a duplicate webhook and a wrong amount).
- Paystack **test mode** with test cards; use a tunnel (cloudflared or ngrok) so Paystack can reach the webhook on your machine.
- Run tests from GoLand's gutter icons or `go test ./...`.

---

## 10. Build order

Each milestone ends with something you can click through.

| # | Milestone | Done when |
| --- | --- | --- |
| M0 | Skeleton | `docker compose up`, migrations run, `GET /api/health` works, the compound run configuration starts everything |
| M1 | Catalogue | Seed data loads (states, LGAs, size sets, colours, demo products); shop Home, Shop and Product pages show real data |
| M2 | Admin basics | Staff login + roles; product editor with variants and photo upload; catalogue setup screens |
| M3 | Bag and checkout | Cart, delivery quote, Paystack test payment, webhook, order created and stock reduced |
| M4 | Orders | Admin orders board, status changes, tracker page, email messages via the job queue |
| M5 | Content and promotions | Banners, homepage sections, pages, sales, codes, gift with purchase |
| M6 | Accounts and reviews | OTP sign-in, saved addresses and items, review requests and moderation, SMS/WhatsApp |
| M7 | Launch | Production hosting, Paystack live keys, backups, error monitoring, domain + HTTPS |

---

## 11. Decisions still open

1. **Hosting:** a small VPS (cheapest, more to manage) or a managed platform (Render, Railway, Fly.io) with managed Postgres.
2. **Messages:** start with email + SMS (Termii), add WhatsApp once templates are approved by Meta?
3. **Couriers:** own riders in Lagos plus a courier for other states, entered by hand; courier API integration later.
4. **Paystack account:** activating live payments needs business details and documents. Start this early so it is ready by M7.
