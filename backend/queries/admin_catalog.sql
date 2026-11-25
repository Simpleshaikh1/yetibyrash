-- Admin reads and writes for products and catalogue setup.
-- Unlike the shop queries, these see draft and hidden products too.

-- ---------- Products ----------

-- name: AdminListProducts :many
SELECT p.id, p.slug, p.name, p.tier, p.status, p.updated_at,
       c.name AS category_name,
       pc.name AS parent_name,
       COALESCE(MIN(v.price_kobo) FILTER (WHERE v.active), 0)::bigint AS min_price_kobo,
       COALESCE(MAX(v.price_kobo) FILTER (WHERE v.active), 0)::bigint AS max_price_kobo,
       COALESCE(SUM(v.stock) FILTER (WHERE v.active), 0)::int        AS total_stock,
       COUNT(v.id) FILTER (WHERE v.active)::int                     AS option_count,
       COUNT(v.id) FILTER (WHERE v.active AND v.stock = 0)::int     AS sold_out_count,
       COALESCE((SELECT im.storage_key FROM product_images im WHERE im.product_id = p.id
                 ORDER BY im.position, im.id LIMIT 1), '')::text    AS image_key -- '' = no photo yet
FROM products p
JOIN categories c ON c.id = p.category_id
LEFT JOIN categories pc ON pc.id = c.parent_id
LEFT JOIN variants v ON v.product_id = p.id
WHERE (sqlc.narg(search)::text IS NULL OR p.name ILIKE '%' || sqlc.narg(search)::text || '%'
       OR EXISTS (SELECT 1 FROM variants sv WHERE sv.product_id = p.id AND sv.sku ILIKE '%' || sqlc.narg(search)::text || '%'))
  AND (sqlc.narg(status)::product_status IS NULL OR p.status = sqlc.narg(status)::product_status)
  AND (sqlc.narg(category_id)::bigint IS NULL OR c.id = sqlc.narg(category_id)::bigint OR c.parent_id = sqlc.narg(category_id)::bigint)
GROUP BY p.id, c.name, pc.name
ORDER BY p.updated_at DESC, p.id DESC
LIMIT 200;

-- name: AdminGetProduct :one
SELECT p.id, p.slug, p.name, p.description, p.category_id, p.tier, p.material_line, p.status,
       p.badges, p.gift_box_allowed, p.preorder_allowed, p.updated_at
FROM products p WHERE p.id = $1;

-- name: SlugExists :one
SELECT EXISTS (SELECT 1 FROM products WHERE slug = $1);

-- name: InsertProduct :one
INSERT INTO products (slug, name, description, category_id, tier, material_line, status, badges,
                      gift_box_allowed, preorder_allowed)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id;

-- name: UpdateProduct :exec
UPDATE products
SET name = $2, description = $3, category_id = $4, tier = $5, material_line = $6, status = $7,
    badges = $8, gift_box_allowed = $9, preorder_allowed = $10, updated_at = now()
WHERE id = $1;

-- name: AdminListVariants :many
-- Every variant, including ones switched off, so the editor can bring them back.
SELECT v.id, v.colour_id, v.size_id, v.sku, v.price_kobo, v.compare_at_kobo, v.stock, v.active
FROM variants v
LEFT JOIN sizes sz ON sz.id = v.size_id
WHERE v.product_id = $1
ORDER BY v.position, v.id;

-- name: UpsertVariant :one
-- One row per colour + size; saving the same option again updates it.
INSERT INTO variants (product_id, colour_id, size_id, sku, price_kobo, compare_at_kobo, stock, active, position)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (product_id, COALESCE(colour_id, 0), COALESCE(size_id, 0))
DO UPDATE SET sku = EXCLUDED.sku, price_kobo = EXCLUDED.price_kobo, compare_at_kobo = EXCLUDED.compare_at_kobo,
              stock = EXCLUDED.stock, active = EXCLUDED.active, position = EXCLUDED.position
RETURNING id;

-- name: DeactivateOtherVariants :exec
-- Options removed in the editor are switched off, not deleted, so past orders keep pointing at them.
UPDATE variants SET active = false
WHERE product_id = $1 AND NOT (id = ANY (sqlc.arg(keep_ids)::bigint[]));

-- name: SkuTakenByOtherProduct :one
SELECT EXISTS (SELECT 1 FROM variants WHERE sku = $1 AND product_id <> $2);

-- name: DeleteProductDetails :exec
DELETE FROM product_details WHERE product_id = $1;

-- name: InsertProductDetail :exec
INSERT INTO product_details (product_id, field_id, value) VALUES ($1, $2, $3);

-- name: AdminListProductDetails :many
SELECT field_id, value FROM product_details WHERE product_id = $1;

-- name: ListFieldsForCategory :many
-- A product's detail fields: its category's own, or its parent category's.
SELECT f.id, f.label
FROM category_fields f
JOIN categories c ON c.id = sqlc.arg(category_id)::bigint
WHERE f.category_id = COALESCE(c.parent_id, c.id)
ORDER BY f.position;

-- name: CategoryExists :one
SELECT EXISTS (SELECT 1 FROM categories WHERE id = $1);

-- ---------- Photos ----------

-- name: AdminListImages :many
SELECT id, colour_id, storage_key, kind, width, height, warnings
FROM product_images WHERE product_id = $1 ORDER BY position, id;

-- name: InsertImage :one
INSERT INTO product_images (product_id, colour_id, storage_key, kind, position, width, height, warnings)
VALUES ($1, $2, $3, $4,
        (SELECT COALESCE(MAX(position), -1) + 1 FROM product_images WHERE product_id = $1),
        $5, $6, $7)
RETURNING id;

-- name: DeleteImage :one
DELETE FROM product_images WHERE id = $1 AND product_id = $2 RETURNING storage_key;

-- name: MoveImageFirst :exec
UPDATE product_images
SET position = CASE WHEN id = sqlc.arg(image_id) THEN 0 ELSE position + 1 END
WHERE product_id = sqlc.arg(product_id);

-- name: TouchProduct :exec
UPDATE products SET updated_at = now() WHERE id = $1;

-- ---------- Catalogue setup ----------

-- name: AdminListCategories :many
SELECT id, parent_id, name, slug, position, default_size_set_id, return_rule, show_on_home
FROM categories ORDER BY parent_id NULLS FIRST, position, name;

-- name: AdminListCategoryFields :many
SELECT id, category_id, label FROM category_fields ORDER BY category_id, position;

-- name: UpdateCategory :exec
UPDATE categories
SET name = $2, default_size_set_id = $3, return_rule = $4, show_on_home = $5, position = $6
WHERE id = $1;

-- name: InsertCategory :one
INSERT INTO categories (parent_id, name, slug, position, default_size_set_id, return_rule)
VALUES ($1, $2, $3,
        (SELECT COALESCE(MAX(position), 0) + 1 FROM categories WHERE parent_id IS NOT DISTINCT FROM $1),
        $4, $5)
RETURNING id;

-- name: CategorySlugExists :one
SELECT EXISTS (SELECT 1 FROM categories WHERE slug = $1);

-- name: UpsertCategoryField :one
INSERT INTO category_fields (category_id, label, position) VALUES ($1, $2, $3)
ON CONFLICT (category_id, label) DO UPDATE SET position = EXCLUDED.position
RETURNING id;

-- name: RenameCategoryField :exec
UPDATE category_fields SET label = $3, position = $4 WHERE id = $1 AND category_id = $2;

-- name: DeleteOtherCategoryFields :exec
DELETE FROM category_fields
WHERE category_id = $1 AND NOT (id = ANY (sqlc.arg(keep_ids)::bigint[]));

-- name: AdminListSizeSets :many
SELECT id, name, measure_label FROM size_sets ORDER BY name;

-- name: AdminListSizes :many
SELECT sz.id, sz.size_set_id, sz.label, sz.measurement,
       (SELECT count(*) FROM variants v WHERE v.size_id = sz.id AND v.active)::int AS used_by
FROM sizes sz ORDER BY sz.size_set_id, sz.position;

-- name: InsertSizeSet :one
INSERT INTO size_sets (name, measure_label) VALUES ($1, $2) RETURNING id;

-- name: UpdateSizeSet :exec
UPDATE size_sets SET name = $2, measure_label = $3 WHERE id = $1;

-- name: InsertSize :one
INSERT INTO sizes (size_set_id, label, measurement, position) VALUES ($1, $2, $3, $4) RETURNING id;

-- name: UpdateSize :exec
UPDATE sizes SET label = $3, measurement = $4, position = $5 WHERE id = $1 AND size_set_id = $2;

-- name: DeleteOtherSizes :exec
DELETE FROM sizes WHERE size_set_id = $1 AND NOT (id = ANY (sqlc.arg(keep_ids)::bigint[]));

-- name: AdminListColours :many
SELECT co.id, co.name, co.hex,
       (SELECT count(DISTINCT v.product_id) FROM variants v WHERE v.colour_id = co.id AND v.active)::int AS used_by
FROM colours co ORDER BY co.name;

-- name: InsertColour :one
INSERT INTO colours (name, hex) VALUES ($1, $2) RETURNING id;

-- name: UpdateColour :exec
UPDATE colours SET name = $2, hex = $3 WHERE id = $1;

-- name: DeleteColour :exec
DELETE FROM colours WHERE id = $1;
