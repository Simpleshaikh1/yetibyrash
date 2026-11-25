-- name: LockVariantsForCheckout :many
-- Locks the options being bought, in id order so two checkouts can't deadlock.
SELECT v.id, v.stock, v.sku, v.price_kobo, (v.active AND p.status = 'visible')::bool AS available
FROM variants v JOIN products p ON p.id = v.product_id
WHERE v.id = ANY (sqlc.arg(ids)::bigint[])
ORDER BY v.id
FOR UPDATE OF v;

-- name: ChangeStock :exec
-- Negative delta takes stock (checkout); positive returns it (abandoned order).
-- Never goes below zero.
UPDATE variants SET stock = GREATEST(stock + sqlc.arg(delta)::int, 0) WHERE id = sqlc.arg(id);

-- name: InsertOrder :one
INSERT INTO orders (cart_id, payment_reference, customer_name, customer_phone, customer_email,
                    state_name, lga_name, street, landmark, zone_name, speed,
                    subtotal_kobo, delivery_kobo, total_kobo, expected_from, expected_to)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING id, number;

-- name: InsertOrderItem :exec
INSERT INTO order_items (order_id, variant_id, product_name, product_slug, variant_label, sku, image_key, unit_kobo, qty)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: InsertOrderEvent :exec
INSERT INTO order_events (order_id, status, note, staff_id) VALUES ($1, $2, $3, $4);

-- name: GetOrderByReferenceForUpdate :one
SELECT id, number, status, cart_id, total_kobo
FROM orders WHERE payment_reference = $1
FOR UPDATE;

-- name: GetOrderByNumber :one
SELECT id, number, status, payment_reference, customer_name, customer_phone, customer_email,
       state_name, lga_name, street, landmark, zone_name, speed,
       subtotal_kobo, delivery_kobo, total_kobo, expected_from, expected_to, created_at, paid_at
FROM orders WHERE number = $1;

-- name: ListOrderItems :many
SELECT variant_id, product_name, product_slug, variant_label, sku, image_key, unit_kobo, qty
FROM order_items WHERE order_id = $1 ORDER BY id;

-- name: ListOrderEvents :many
SELECT status, note, created_at FROM order_events WHERE order_id = $1 ORDER BY created_at, id;

-- name: MarkOrderPaid :exec
UPDATE orders SET status = 'paid', paid_at = now() WHERE id = $1;

-- name: SetOrderStatus :exec
UPDATE orders SET status = $2 WHERE id = $1;

-- name: ListStalePendingOrders :many
-- Orders still waiting for payment after the reservation time.
SELECT id, number, payment_reference FROM orders
WHERE status = 'pending_payment' AND created_at < now() - make_interval(mins => sqlc.arg(minutes)::int)
ORDER BY created_at
LIMIT 50;

-- name: InsertPayment :one
-- Paystack can tell us about the same payment twice (webhook and verify); the first one wins.
INSERT INTO payments (reference, order_id, amount_kobo, channel, outcome, payload)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (reference) DO NOTHING
RETURNING id;
