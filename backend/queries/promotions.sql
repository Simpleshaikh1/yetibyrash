-- ---------- Sales ----------

-- name: LiveSales :many
SELECT id, name, kind, value, applies_to, tier, category_id, badge_text, announcement, exclude_on_sale, ends_at
FROM promotions
WHERE active AND starts_at <= now() AND ends_at > now()
ORDER BY id;

-- name: SaleTargets :many
-- What a sale needs to know about each product: its tier, category (and parent),
-- and whether it already has a compare-at price.
SELECT p.id, p.tier, p.category_id, COALESCE(c.parent_id, 0)::bigint AS parent_id,
       EXISTS (SELECT 1 FROM variants v WHERE v.product_id = p.id AND v.active
               AND v.compare_at_kobo IS NOT NULL AND v.compare_at_kobo > v.price_kobo)::bool AS on_sale
FROM products p JOIN categories c ON c.id = p.category_id
WHERE p.id = ANY (sqlc.arg(ids)::bigint[]);

-- name: ListPromotions :many
SELECT p.id, p.name, p.kind, p.value, p.applies_to, p.tier, p.category_id, p.starts_at, p.ends_at,
       p.badge_text, p.announcement, p.exclude_on_sale, p.active,
       COALESCE(c.name, '')::text AS category_name,
       (SELECT COUNT(*) FROM orders o WHERE o.promotion_id = p.id AND o.paid_at IS NOT NULL
            AND o.status NOT IN ('cancelled', 'refunded'))::int AS orders,
       (SELECT COALESCE(SUM(o.total_kobo), 0) FROM orders o WHERE o.promotion_id = p.id AND o.paid_at IS NOT NULL
            AND o.status NOT IN ('cancelled', 'refunded'))::bigint AS sales_kobo
FROM promotions p LEFT JOIN categories c ON c.id = p.category_id
ORDER BY p.ends_at DESC, p.id DESC;

-- name: InsertPromotion :one
INSERT INTO promotions (name, kind, value, applies_to, tier, category_id, starts_at, ends_at,
                        badge_text, announcement, exclude_on_sale, active)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING id;

-- name: UpdatePromotion :execrows
UPDATE promotions SET name = $2, kind = $3, value = $4, applies_to = $5, tier = $6, category_id = $7,
    starts_at = $8, ends_at = $9, badge_text = $10, announcement = $11, exclude_on_sale = $12, active = $13
WHERE id = $1;

-- name: DeletePromotion :execrows
-- Only sales no order used; used ones are switched off instead.
DELETE FROM promotions p WHERE p.id = $1 AND NOT EXISTS (SELECT 1 FROM orders o WHERE o.promotion_id = p.id);

-- ---------- Discount codes ----------

-- name: GetCode :one
SELECT id, code, kind, value, min_order_kobo, max_uses, used, first_order_only, zone_id, starts_at, ends_at, active
FROM discount_codes WHERE code = $1;

-- name: LockCode :one
SELECT id, code, kind, value, min_order_kobo, max_uses, used, first_order_only, zone_id, starts_at, ends_at, active
FROM discount_codes WHERE code = $1 FOR UPDATE;

-- name: UseCode :exec
UPDATE discount_codes SET used = used + 1 WHERE id = $1;

-- name: ReturnCode :exec
-- An unpaid or cancelled order gives its use back.
UPDATE discount_codes SET used = GREATEST(used - 1, 0) WHERE code = $1;

-- name: CountPaidOrdersFor :one
-- Has this customer (by phone or email) paid for an order before?
SELECT COUNT(*)::int FROM orders
WHERE paid_at IS NOT NULL AND status NOT IN ('cancelled', 'refunded')
  AND (customer_phone = sqlc.arg(phone) OR lower(customer_email) = lower(sqlc.arg(email)));

-- name: ListCodes :many
SELECT d.id, d.code, d.kind, d.value, d.min_order_kobo, d.max_uses, d.used, d.first_order_only,
       d.zone_id, COALESCE(z.name, '')::text AS zone_name, d.starts_at, d.ends_at, d.active
FROM discount_codes d LEFT JOIN delivery_zones z ON z.id = d.zone_id
ORDER BY d.active DESC, d.created_at DESC;

-- name: InsertCode :one
INSERT INTO discount_codes (code, kind, value, min_order_kobo, max_uses, first_order_only, zone_id, starts_at, ends_at, active)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id;

-- name: UpdateCode :execrows
UPDATE discount_codes SET code = $2, kind = $3, value = $4, min_order_kobo = $5, max_uses = $6,
    first_order_only = $7, zone_id = $8, starts_at = $9, ends_at = $10, active = $11
WHERE id = $1;

-- name: SetCartCode :exec
UPDATE carts SET discount_code = $2, updated_at = now() WHERE id = $1;

-- name: GetCartCode :one
SELECT discount_code FROM carts WHERE id = $1;

-- ---------- Gift with purchase ----------

-- name: LiveGift :one
-- The newest live offer that still has gifts and stock left.
SELECT g.id, g.threshold_kobo, g.variant_id, g.quantity, g.given,
       p.name AS product_name, p.slug AS product_slug, v.sku, v.stock,
       COALESCE(co.name, '')::text AS colour, COALESCE(sz.label, '')::text AS size,
       COALESCE((SELECT im.storage_key FROM product_images im WHERE im.product_id = p.id
                 ORDER BY im.position, im.id LIMIT 1), '')::text AS image_key
FROM gift_offers g
JOIN variants v ON v.id = g.variant_id
JOIN products p ON p.id = v.product_id
LEFT JOIN colours co ON co.id = v.colour_id
LEFT JOIN sizes sz ON sz.id = v.size_id
WHERE g.active AND g.given < g.quantity AND v.stock > 0
  AND (g.starts_at IS NULL OR g.starts_at <= now()) AND (g.ends_at IS NULL OR g.ends_at > now())
ORDER BY g.created_at DESC
LIMIT 1;

-- name: LockGift :one
SELECT id, given, quantity FROM gift_offers WHERE id = $1 FOR UPDATE;

-- name: GiveGift :exec
UPDATE gift_offers SET given = given + 1 WHERE id = $1;

-- name: ReturnGift :exec
UPDATE gift_offers SET given = GREATEST(given - 1, 0) WHERE id = $1;

-- name: ListGifts :many
SELECT g.id, g.threshold_kobo, g.variant_id, g.quantity, g.given, g.starts_at, g.ends_at, g.active,
       p.name AS product_name, v.sku,
       COALESCE(co.name, '')::text AS colour, COALESCE(sz.label, '')::text AS size, v.stock
FROM gift_offers g
JOIN variants v ON v.id = g.variant_id
JOIN products p ON p.id = v.product_id
LEFT JOIN colours co ON co.id = v.colour_id
LEFT JOIN sizes sz ON sz.id = v.size_id
ORDER BY g.created_at DESC;

-- name: InsertGift :one
INSERT INTO gift_offers (threshold_kobo, variant_id, quantity, starts_at, ends_at, active)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: UpdateGift :execrows
UPDATE gift_offers SET threshold_kobo = $2, variant_id = $3, quantity = $4, starts_at = $5, ends_at = $6, active = $7
WHERE id = $1;

-- name: VariantBySKU :one
-- Staff pick the gift by its SKU.
SELECT v.id, p.name AS product_name, COALESCE(co.name, '')::text AS colour, COALESCE(sz.label, '')::text AS size, v.stock
FROM variants v JOIN products p ON p.id = v.product_id
LEFT JOIN colours co ON co.id = v.colour_id
LEFT JOIN sizes sz ON sz.id = v.size_id
WHERE upper(v.sku) = upper($1);

-- name: GetOrderOffers :one
-- What an order took from promotions, so an unpaid or cancelled order can give it back.
SELECT discount_code, gift_offer_id FROM orders WHERE id = $1;

-- name: RetakeCode :exec
-- A late payment for an abandoned order takes its code use again.
UPDATE discount_codes SET used = used + 1 WHERE code = $1;
