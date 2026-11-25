-- name: CreateCart :one
INSERT INTO carts DEFAULT VALUES RETURNING id;

-- name: CartExists :one
SELECT EXISTS (SELECT 1 FROM carts WHERE id = $1);

-- name: TouchCart :exec
UPDATE carts SET updated_at = now() WHERE id = $1;

-- name: GetCartLines :many
-- Everything the bag shows, with live price and stock. "available" is false when the
-- option was switched off or the product hidden since it was added.
SELECT ci.variant_id, ci.qty,
       v.sku, v.price_kobo, v.compare_at_kobo, v.stock,
       (v.active AND p.status = 'visible')::bool AS available,
       p.id AS product_id, p.slug, p.name,
       COALESCE(co.name, '')::text AS colour,
       COALESCE(sz.label, '')::text AS size,
       COALESCE((SELECT im.storage_key FROM product_images im WHERE im.product_id = p.id
                 ORDER BY im.position, im.id LIMIT 1), '')::text AS image_key
FROM cart_items ci
JOIN variants v ON v.id = ci.variant_id
JOIN products p ON p.id = v.product_id
LEFT JOIN colours co ON co.id = v.colour_id
LEFT JOIN sizes sz ON sz.id = v.size_id
WHERE ci.cart_id = $1
ORDER BY ci.added_at, ci.variant_id;

-- name: GetVariantForCart :one
SELECT v.id, v.stock, (v.active AND p.status = 'visible')::bool AS available
FROM variants v JOIN products p ON p.id = v.product_id
WHERE v.id = $1;

-- name: AddCartItem :one
-- Adding something already in the bag adds to its quantity (up to 20).
INSERT INTO cart_items (cart_id, variant_id, qty) VALUES ($1, $2, $3)
ON CONFLICT (cart_id, variant_id) DO UPDATE SET qty = LEAST(cart_items.qty + EXCLUDED.qty, 20)
RETURNING qty;

-- name: SetCartItemQty :execrows
UPDATE cart_items SET qty = $3 WHERE cart_id = $1 AND variant_id = $2;

-- name: DeleteCartItem :exec
DELETE FROM cart_items WHERE cart_id = $1 AND variant_id = $2;

-- name: DeleteCart :exec
DELETE FROM carts WHERE id = $1;
