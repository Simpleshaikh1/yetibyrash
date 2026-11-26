# Going live

Everything needed to put Yetibyrash on the internet, in order. Tick each box as you go.
Parts A and B can start now; C needs your domain; D is launch day.

The live shop is one Render web service (the shop at `/`, the admin at `/admin`, the API at
`/api`), a Render Postgres database, a nightly backup job, and two Cloudflare R2 buckets
(photos, backups). Render's blueprint file `render.yaml` sets most of it up for you.

---

## A. Accounts (start now; some need approval)

- [ ] **Paystack**: finish business verification (CAC, bank account for payouts, ID). Until
      it's approved, use the **test** secret key (`sk_test_...`); the shop works but no real
      money moves, and the server log warns you.
- [ ] **Termii**: create an account, add credit, register the sender ID **Yetibyrash**
      (1–2 days). Ask support to enable the **DND route** so texts reach numbers on
      Do-Not-Disturb, then set `TERMII_CHANNEL=dnd`.
- [ ] **Email sending**: create an account with Resend, Postmark or Zoho Mail. You'll verify
      your domain there in step C3. Note the SMTP host, port, username and password.
- [ ] **Cloudflare**: create a free account. In **R2**, create two buckets:
  - `yetibyrash-photos`: under Settings, turn on **Public access** (r2.dev address for now;
    a custom domain `media.yourdomain` later).
  - `yetibyrash-backups`: keep it **private**.
  - Under **R2 > Manage API tokens**, create a token with *Object Read & Write* on both
    buckets. Note the Account ID, Access Key ID and Secret Access Key.
- [ ] **Sentry** (error alerts): create a free account at sentry.io, add a **Go** project,
      copy its **DSN**.
- [ ] **UptimeRobot** (is the site up?): create a free account. You'll add the check in step C4.
- [ ] **Render**: create an account and connect it to the GitHub repository
      `Simpleshaikh1/yetibyrash`.

## B. First deploy (works before you have the domain)

1. [ ] **Copy your photos to R2.** In `backend/.env` on your laptop add:
   ```
   R2_ACCOUNT_ID=...
   R2_ACCESS_KEY_ID=...
   R2_SECRET_ACCESS_KEY=...
   R2_BUCKET=yetibyrash-photos
   MEDIA_PUBLIC_URL=https://pub-xxxx.r2.dev
   ```
   then in GoLand's terminal: `cd backend` and `go run ./cmd/mediasync -dry-run`, then
   `go run ./cmd/mediasync`.
2. [ ] **Create everything on Render**: *New + > Blueprint*, pick the repository. Render reads
   `render.yaml` and asks for each secret:

   | Setting | Value |
   | --- | --- |
   | `SHOP_URL`, `ADMIN_ORIGINS` | `https://yetibyrash.onrender.com` for now (your service's address) |
   | `PAYSTACK_SECRET_KEY` | test key until Paystack approves you |
   | `SMTP_ADDR`, `SMTP_USERNAME`, `SMTP_PASSWORD` | from your email provider |
   | `MAIL_FROM` | `Yetibyrash <orders@yourdomain>` |
   | `R2_*`, `MEDIA_PUBLIC_URL` | as in step 1 |
   | `BACKUP_BUCKET` | `yetibyrash-backups` |
   | `TERMII_API_KEY` | when Termii is ready (can be added later) |
   | `SENTRY_DSN` | from Sentry |

   Choose a **paid database plan**: the free one is deleted after 30 days.
3. [ ] **Watch the first deploy** (Render > yetibyrash > Logs). If a setting is missing or unsafe,
   the server stops and the log lists exactly which one, for example
   `"problem":"SHOP_URL must be the shop's https:// address"`. Fix it under *Environment* and
   Render redeploys.
4. [ ] **Create your owner account**: Render > yetibyrash > *Shell*, then
   `./createstaff -email you@yourdomain -name "Your Name" -role owner` and type a strong
   password (12+ characters).
5. [ ] **Sign in** at `https://yetibyrash.onrender.com/admin/`. Add staff, products and photos,
   delivery fees, and write the pages (About, Delivery and returns, Privacy policy, Terms).
6. [ ] **Paystack webhook**: Paystack dashboard > *Settings > API Keys & Webhooks* > Webhook URL:
   `https://yetibyrash.onrender.com/api/webhooks/paystack`.
7. [ ] **Run the backup once by hand**: Render > yetibyrash-backup > *Trigger Run*. Check the
   file appears in the `yetibyrash-backups` bucket.

## C. Your domain

1. [ ] Render > yetibyrash > *Settings > Custom Domains*: add `yourdomain.com` and
   `www.yourdomain.com`. Add the DNS records Render shows at your registrar (or move the
   domain's DNS to Cloudflare, which is free). Render sets up HTTPS on its own.
2. [ ] Change `SHOP_URL` and `ADMIN_ORIGINS` to `https://yourdomain.com`, and the Paystack
   webhook URL to `https://yourdomain.com/api/webhooks/paystack`.
3. [ ] Verify the domain at your email provider (it gives you SPF/DKIM DNS records), so order
   emails don't land in spam.
4. [ ] Optional: in Cloudflare R2, connect `media.yourdomain.com` to the photos bucket and set
   `MEDIA_PUBLIC_URL` to it.
5. [ ] UptimeRobot: add an HTTPS monitor for `https://yourdomain.com/api/health` (every 5
   minutes, alert by email/SMS). It shows "ok" only when the database answers too.

## D. Launch day

- [ ] Paystack approved: change `PAYSTACK_SECRET_KEY` to the **live** key (`sk_live_...`).
- [ ] Termii approved: `TERMII_API_KEY` set; sign in on the shop with your phone and get a text.
- [ ] Place a **real order** with a cheap item: pay, get the email and text, move it through
      Packed > Shipped > Delivered in the admin, check the Track page, then **refund it** in
      Paystack and cancel it in the admin.
- [ ] Check the Render log and Sentry show no errors.
- [ ] Announce!

---

## Looking after it

- **Deploying changes**: every push to `main` goes live automatically (Render builds the
  Dockerfile and swaps over with no downtime). Turn off *Auto-Deploy* to deploy by hand.
- **Backups**: Render's paid database keeps its own daily backups, and the backup job copies the
  database to R2 every night at 03:00 Lagos time, keeping 30 days.
- **Restoring a backup** (practice this once): create an empty database, then in the backup
  job's Shell run
  `RESTORE_DATABASE_URL=postgres://... restore.sh` (newest) or
  `restore.sh yetibyrash-2026-12-01T0200Z.sql.gz`. It prints counts of products, orders and
  staff so you can check it before switching the shop over.
- **Errors**: Sentry emails you when something breaks (unexpected errors, crashes, and
  messages that keep failing to send). UptimeRobot tells you if the site is down.
- **Keys**: if a key may have leaked, make a new one at the provider, update it on Render's
  *Environment* page, and delete the old one.
- **Staff leaving**: switch their account off on the Staff screen. That signs them out everywhere.
