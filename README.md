# Yetibyrash

Elegance meets affordability. Online shop for jewelry, watches, bags, women's clothing and intimates in Nigeria.

- `backend/` Go API (one binary: API, background jobs, and later the built React apps)
- `frontend/shop` customer site · `frontend/admin` staff dashboard · `frontend/ui` shared tokens and helpers
- `design/` canvas source and design system · `docs/backend-plan.md` the plan

## First-time setup

Needs Go 1.27+, Node 22+, Docker Desktop (running).

```powershell
cd frontend
npm install
```

## Run everything (GoLand)

Open the `Yetibyrash` folder as the project, then pick **Yetibyrash (all)** in the run configurations and press Run. It starts:

| Config | What | Address |
| --- | --- | --- |
| `db` | Postgres and Mailpit in Docker | Postgres `localhost:5432`, test email inbox http://localhost:8025 |
| `api` | Go API (migrates the database on start) | http://localhost:8090/api/health |
| `shop` | Customer site | http://localhost:5173 |
| `admin` | Staff dashboard | http://localhost:5174/admin/ |

The API waits up to 30 seconds for the database, so starting everything at once is fine.

**Example products:** run **seed demo products** once (or `cd backend; go run ./cmd/seed -reset`) to fill the shop with 14 example products. `-reset` replaces any products already there. It refuses to run when `APP_ENV=production`.

**Admin sign-in:** the seed also creates a development owner account if there are no staff yet. Its email and password are `DEV_OWNER_EMAIL` and `DEV_OWNER_PASSWORD` in `backend/.env.example`. Sign in at http://localhost:5174/admin/. Add other staff on the **Staff** screen.

**Photos:** upload them in the admin (Products → a product → 2. Photos → + Add photos). They're saved in `backend/var/media` (ignored by git) and served at `/media/...`.

**Payments:** without Paystack keys, checkout uses a built-in **test payment page** (development only; no money moves). To use Paystack's test mode instead, put your keys in `backend/.env` (never in `.env.example`, which is committed):

```
PAYSTACK_SECRET_KEY=sk_test_...
SHOP_URL=http://localhost:5173
```

Paystack can't reach your computer's webhook during local development; the order page still confirms payment by asking Paystack when the shopper returns. Unpaid orders release their stock after 30 minutes.

**Orders and customer emails:** paid orders appear on the admin **Orders** board. Move each one along: Packed → Shipped (pick a courier; couriers are set up on the **Delivery** screen) → Out for delivery → Delivered. Owners and managers can cancel an order while it's Paid or Packed, which puts the stock back. Every step emails the customer. The wording is edited on **Customer messages**. Locally, those emails land in Mailpit at http://localhost:8025 instead of reaching anyone. Customers follow their order at http://localhost:5173/track using the order number and their phone number.

For real email, set `SMTP_ADDR`, `SMTP_USERNAME`, `SMTP_PASSWORD` and `MAIL_FROM` in `backend/.env`. Emails are sent from a queue, so a mail outage only delays them (they retry for several hours).

**First owner on a real server** (no seed there):

```powershell
cd backend; go run ./cmd/createstaff -email you@example.com -name "Your Name" -role owner
```

It asks for the password on the next line, so it never lands in your shell history.

## Show the running shop to someone else

Everything runs on your computer, so `localhost` links only work for you. A Cloudflare quick tunnel gives a temporary public `https://….trycloudflare.com` link to your running shop (free, no account).

1. Install once: `winget install --id Cloudflare.cloudflared`
2. Start everything as usual, then in a new terminal: `cloudflared tunnel --url http://localhost:5173`
3. Copy the `https://….trycloudflare.com` address it prints and put it in `backend/.env` as `SHOP_URL=https://….trycloudflare.com`, then rerun the `api`. (Without this, the test payment sends them back to *your* localhost.)
4. Send them the link. Press Ctrl+C in that terminal to stop sharing. The address changes every time.

Anyone with the link can use the shop while it's running, so stop the tunnel when they're done. Avoid sharing the admin this way.

## Without GoLand

```powershell
docker compose up -d
```

```powershell
cd backend; go run ./cmd/yetibyrash
```

```powershell
cd frontend; npm run dev:shop
```

## Tests

```powershell
cd backend; go test ./...
```

The catalogue tests create a throwaway database in the local Postgres, so start Docker first (they are skipped if Postgres isn't running).

## Settings

The API reads environment variables, with development defaults for all of them. To override locally, copy `backend/.env.example` to `backend/.env`. Port 8080 is avoided because Apache already uses it on this machine.

## Database queries (sqlc)

SQL lives in `backend/queries/`. Generate the Go code with Docker:

```powershell
cd backend; docker run --rm -v "${PWD}:/src" -w /src sqlc/sqlc:1.27.0 generate
```
