-- ---------- Admin orders ----------

-- name: AdminListOrders :many
-- Paid orders and later. Delivered and cancelled ones only from the last `recent_days` days.
SELECT o.id, o.number, o.status, o.status_at, o.created_at, o.paid_at,
       o.customer_name, o.customer_phone, o.state_name, o.lga_name, o.speed, o.total_kobo,
       (SELECT COALESCE(SUM(qty), 0) FROM order_items i WHERE i.order_id = o.id)::int AS item_count
FROM orders o
WHERE o.status NOT IN ('pending_payment', 'abandoned')
  AND (sqlc.narg(status)::order_status IS NULL OR o.status = sqlc.narg(status)::order_status)
  AND (o.status NOT IN ('delivered', 'cancelled', 'refunded')
       OR o.status_at > now() - make_interval(days => sqlc.arg(recent_days)::int))
  AND (sqlc.narg(search)::text IS NULL
       OR o.number ILIKE '%' || sqlc.narg(search)::text || '%'
       OR o.customer_name ILIKE '%' || sqlc.narg(search)::text || '%'
       OR regexp_replace(o.customer_phone, '\D', '', 'g') LIKE '%' || regexp_replace(sqlc.narg(search)::text, '\D', '', 'g') || '%')
ORDER BY o.paid_at NULLS LAST, o.id
LIMIT 300;

-- name: AdminGetOrder :one
SELECT o.id, o.number, o.status, o.status_at, o.payment_reference, o.created_at, o.paid_at,
       o.customer_name, o.customer_phone, o.customer_email,
       o.state_name, o.lga_name, o.street, o.landmark, o.zone_name, o.speed,
       o.subtotal_kobo, o.delivery_kobo, o.total_kobo, o.expected_from, o.expected_to,
       o.discount_kobo, o.discount_code,
       o.courier_id, COALESCE(c.name, '')::text AS courier_name, COALESCE(c.tracking_url, '')::text AS courier_tracking_url,
       o.tracking_number
FROM orders o LEFT JOIN couriers c ON c.id = o.courier_id
WHERE o.id = $1;

-- name: LockOrder :one
SELECT id, number, status FROM orders WHERE id = $1 FOR UPDATE;

-- name: SetOrderShipping :exec
UPDATE orders SET courier_id = $2, tracking_number = $3 WHERE id = $1;

-- name: AdminListOrderEvents :many
SELECT e.status, e.note, e.public, e.created_at, COALESCE(s.name, '')::text AS staff_name
FROM order_events e LEFT JOIN staff s ON s.id = e.staff_id
WHERE e.order_id = $1 ORDER BY e.created_at, e.id;

-- name: ListOrderPayments :many
SELECT reference, amount_kobo, channel, outcome, received_at FROM payments WHERE order_id = $1 ORDER BY received_at;

-- ---------- Couriers ----------

-- name: ListCouriers :many
SELECT id, name, tracking_url, active FROM couriers ORDER BY active DESC, name;

-- name: GetCourier :one
SELECT id, name, tracking_url, active FROM couriers WHERE id = $1;

-- name: InsertCourier :one
INSERT INTO couriers (name, tracking_url, active) VALUES ($1, $2, $3) RETURNING id;

-- name: UpdateCourier :exec
UPDATE couriers SET name = $2, tracking_url = $3, active = $4 WHERE id = $1;

-- ---------- Shopper tracking ----------

-- name: GetOrderForTracking :one
SELECT o.id, o.number, o.status, o.customer_phone, o.customer_name, o.state_name, o.lga_name, o.speed,
       o.expected_from, o.expected_to, o.created_at,
       COALESCE(c.name, '')::text AS courier_name, COALESCE(c.tracking_url, '')::text AS courier_tracking_url,
       o.tracking_number,
       (SELECT COALESCE(SUM(qty), 0) FROM order_items i WHERE i.order_id = o.id)::int AS item_count
FROM orders o LEFT JOIN couriers c ON c.id = o.courier_id
WHERE o.number = upper(sqlc.arg(number)::text);

-- ---------- Customer messages ----------

-- name: ListTemplates :many
SELECT event, subject, body, active FROM message_templates
ORDER BY array_position(ARRAY['paid','packed','shipped','out_for_delivery','delivered','cancelled']::order_status[], event);

-- name: GetTemplate :one
SELECT event, subject, body, active FROM message_templates WHERE event = $1;

-- name: UpdateTemplate :exec
UPDATE message_templates SET subject = $2, body = $3, active = $4 WHERE event = $1;

-- ---------- Jobs ----------

-- name: EnqueueJob :exec
INSERT INTO jobs (kind, payload) VALUES ($1, $2);

-- name: ClaimDueJobs :many
-- Locks up to 10 due jobs; other workers skip them until this transaction ends.
SELECT id, kind, payload, attempts FROM jobs
WHERE done_at IS NULL AND run_at <= now() AND attempts < sqlc.arg(max_attempts)::int
ORDER BY run_at, id
LIMIT 10
FOR UPDATE SKIP LOCKED;

-- name: MarkJobDone :exec
UPDATE jobs SET done_at = now(), attempts = attempts + 1, last_error = '' WHERE id = $1;

-- name: MarkJobFailed :exec
UPDATE jobs SET attempts = attempts + 1, last_error = $2,
       run_at = now() + make_interval(secs => sqlc.arg(retry_in_seconds)::int)
WHERE id = $1;

-- name: CountFailedJobs :one
SELECT count(*)::int FROM jobs WHERE done_at IS NULL AND attempts >= sqlc.arg(max_attempts)::int;

-- ---------- Overview ----------

-- name: SalesSince :one
SELECT COALESCE(SUM(total_kobo), 0)::bigint AS total_kobo, count(*)::int AS orders
FROM orders
WHERE paid_at >= sqlc.arg(since)::timestamptz AND status NOT IN ('cancelled', 'refunded');

-- name: DailySales :many
-- Paid totals for each of the last 7 Lagos days, oldest first (days with no sales included).
WITH days AS (
    SELECT generate_series((now() AT TIME ZONE 'Africa/Lagos')::date - 6,
                           (now() AT TIME ZONE 'Africa/Lagos')::date, interval '1 day')::date AS day
)
SELECT d.day::date AS day,
       COALESCE(SUM(o.total_kobo), 0)::bigint AS total_kobo
FROM days d
LEFT JOIN orders o ON (o.paid_at AT TIME ZONE 'Africa/Lagos')::date = d.day
                  AND o.status NOT IN ('cancelled', 'refunded')
GROUP BY d.day ORDER BY d.day;

-- name: CountOrdersByStatus :many
SELECT status, count(*)::int AS n FROM orders
WHERE status IN ('paid', 'packed', 'shipped', 'out_for_delivery')
GROUP BY status;

-- name: LateToPack :many
-- Paid more than a day ago and still not packed.
SELECT id, number, customer_name, paid_at FROM orders
WHERE status = 'paid' AND paid_at < now() - interval '24 hours'
ORDER BY paid_at LIMIT 10;

-- name: LowStock :many
SELECT p.id AS product_id, p.name,
       trim(both ' /' from COALESCE(co.name, '') || ' / ' || COALESCE(sz.label, ''))::text AS option,
       v.stock
FROM variants v
JOIN products p ON p.id = v.product_id AND p.status = 'visible'
LEFT JOIN colours co ON co.id = v.colour_id
LEFT JOIN sizes sz ON sz.id = v.size_id
WHERE v.active AND v.stock <= sqlc.arg(threshold)::int
ORDER BY v.stock, p.name
LIMIT 50;

-- name: ProblemPayments :many
-- Money that arrived but didn't mark an order paid.
SELECT reference, order_id, amount_kobo, outcome, received_at FROM payments
WHERE outcome <> 'success' ORDER BY received_at DESC LIMIT 10;
