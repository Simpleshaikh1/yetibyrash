-- ---------- One-time codes ----------

-- name: CountRecentSignInCodes :one
-- How many codes went to this phone/email recently (to stop code flooding).
SELECT COUNT(*)::int FROM otp_codes WHERE target = $1 AND created_at > now() - interval '15 minutes';

-- name: InsertSignInCode :exec
INSERT INTO otp_codes (target, code_hash, expires_at, ip) VALUES ($1, $2, now() + interval '10 minutes', $3);

-- name: LatestSignInCode :one
-- The newest unused code for a target, locked while it's checked.
SELECT id, code_hash, expires_at, attempts FROM otp_codes
WHERE target = $1 AND used_at IS NULL
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE;

-- name: CountSignInAttempt :exec
UPDATE otp_codes SET attempts = attempts + 1 WHERE id = $1;

-- name: UseSignInCode :exec
UPDATE otp_codes SET used_at = now() WHERE id = $1;

-- ---------- Customers and sessions ----------

-- name: CustomerByPhone :one
SELECT id, name, phone, email, marketing_opt_in FROM customers WHERE phone = $1;

-- name: CustomerByEmail :one
SELECT id, name, phone, email, marketing_opt_in FROM customers WHERE email = $1;

-- name: InsertCustomer :one
INSERT INTO customers (phone, email) VALUES ($1, $2) RETURNING id, name, phone, email, marketing_opt_in;

-- name: InsertCustomerSession :exec
INSERT INTO customer_sessions (token_hash, customer_id, expires_at) VALUES ($1, $2, $3);

-- name: CustomerBySession :one
SELECT c.id, c.name, c.phone, c.email, c.marketing_opt_in
FROM customer_sessions s JOIN customers c ON c.id = s.customer_id
WHERE s.token_hash = $1 AND s.expires_at > now();

-- name: TouchCustomer :exec
UPDATE customers SET last_seen_at = now() WHERE id = $1;

-- name: DeleteCustomerSession :exec
DELETE FROM customer_sessions WHERE token_hash = $1;

-- name: UpdateCustomerProfile :exec
UPDATE customers SET name = $2, marketing_opt_in = $3 WHERE id = $1;

-- name: SetCustomerContact :exec
-- Fills in the phone or email the customer didn't sign in with, from their checkout.
UPDATE customers SET phone = COALESCE(phone, sqlc.narg(phone)), email = COALESCE(email, sqlc.narg(email)),
    name = CASE WHEN name = '' THEN sqlc.arg(name) ELSE name END
WHERE id = sqlc.arg(id);

-- name: DeleteCustomer :exec
-- NDPR: a customer can delete their account. Their orders stay (needed for accounts) but lose the link.
DELETE FROM customers WHERE id = $1;

-- ---------- Orders ----------

-- name: CustomerOrders :many
-- Orders placed while signed in, or as a guest with the same phone or email.
SELECT o.number, o.status, o.total_kobo, o.created_at, o.paid_at,
       (SELECT COALESCE(SUM(qty), 0) FROM order_items i WHERE i.order_id = o.id AND NOT i.is_gift)::int AS item_count,
       COALESCE((SELECT i.image_key FROM order_items i WHERE i.order_id = o.id ORDER BY i.id LIMIT 1), '')::text AS image_key
FROM orders o
WHERE o.status NOT IN ('pending_payment', 'abandoned')
  AND (o.customer_id = sqlc.arg(customer_id)
       OR (sqlc.narg(phone)::text IS NOT NULL AND o.customer_phone = sqlc.narg(phone)::text)
       OR (sqlc.narg(email)::text IS NOT NULL AND lower(o.customer_email) = sqlc.narg(email)::text))
ORDER BY o.created_at DESC
LIMIT 50;

-- name: CustomerOrderByNumber :one
SELECT o.id, o.number, o.status FROM orders o
WHERE o.number = upper(sqlc.arg(number))
  AND (o.customer_id = sqlc.arg(customer_id)
       OR (sqlc.narg(phone)::text IS NOT NULL AND o.customer_phone = sqlc.narg(phone)::text)
       OR (sqlc.narg(email)::text IS NOT NULL AND lower(o.customer_email) = sqlc.narg(email)::text));

-- name: SetOrderCustomer :exec
UPDATE orders SET customer_id = $2 WHERE id = $1;

-- ---------- Addresses ----------

-- name: ListAddresses :many
SELECT a.id, a.state_id, s.name AS state_name, a.lga_id, COALESCE(l.name, a.lga_text)::text AS lga_name,
       a.lga_text, a.street, a.landmark
FROM addresses a JOIN states s ON s.id = a.state_id LEFT JOIN lgas l ON l.id = a.lga_id
WHERE a.customer_id = $1
ORDER BY a.created_at DESC;

-- name: FindAddress :one
SELECT id FROM addresses
WHERE customer_id = $1 AND state_id = $2 AND lga_id IS NOT DISTINCT FROM $3 AND lga_text = $4 AND lower(street) = lower($5)
LIMIT 1;

-- name: InsertAddress :one
INSERT INTO addresses (customer_id, state_id, lga_id, lga_text, street, landmark)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING id;

-- name: CountAddresses :one
SELECT COUNT(*)::int FROM addresses WHERE customer_id = $1;

-- name: DeleteAddress :execrows
DELETE FROM addresses WHERE id = $1 AND customer_id = $2;

-- ---------- Saved items ----------

-- name: SavedProductIDs :many
SELECT product_id FROM saved_items WHERE customer_id = $1 ORDER BY created_at DESC;

-- name: SaveItem :exec
INSERT INTO saved_items (customer_id, product_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: UnsaveItem :exec
DELETE FROM saved_items WHERE customer_id = $1 AND product_id = $2;
