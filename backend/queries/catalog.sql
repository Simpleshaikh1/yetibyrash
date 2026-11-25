-- Reads for the shop. Writes (admin product editor) arrive in M2.

-- name: ListCategories :many
SELECT id, parent_id, name, slug, position, show_on_home, return_rule
FROM categories
ORDER BY parent_id NULLS FIRST, position, name;

-- name: ListPriceBands :many
SELECT id, label, min_kobo, max_kobo
FROM price_bands
ORDER BY position;

-- name: ListProducts :many
-- Shop grid. Every filter is optional (NULL = don't filter).
-- A category filter matches the category itself and its sub-categories.
-- Price bands compare against the product's lowest variant price.
WITH filtered AS (
    SELECT p.id,
           p.slug,
           p.name,
           p.tier,
           p.material_line,
           p.badges,
           p.preorder_allowed,
           p.created_at,
           c.name                                AS category_name,
           MIN(v.price_kobo)::bigint             AS min_price_kobo,
           MAX(v.price_kobo)::bigint             AS max_price_kobo,
           COALESCE(MIN(v.compare_at_kobo), 0)::bigint AS compare_at_kobo, -- 0 = not on sale
           COUNT(DISTINCT v.colour_id)::int      AS colour_count,
           COALESCE(SUM(v.stock), 0)::int        AS total_stock
    FROM products p
    JOIN categories c ON c.id = p.category_id
    JOIN variants v ON v.product_id = p.id AND v.active
    WHERE p.status = 'visible'
      AND (sqlc.narg(category_slug)::text IS NULL
           OR c.slug = sqlc.narg(category_slug)::text
           OR c.parent_id = (SELECT pc.id FROM categories pc WHERE pc.slug = sqlc.narg(category_slug)::text))
      AND (sqlc.narg(tier)::product_tier IS NULL OR p.tier = sqlc.narg(tier)::product_tier)
      AND (sqlc.narg(colour)::text IS NULL OR EXISTS (
            SELECT 1 FROM variants cv JOIN colours co ON co.id = cv.colour_id
            WHERE cv.product_id = p.id AND cv.active AND co.name = sqlc.narg(colour)::text))
      AND (sqlc.narg(size)::text IS NULL OR EXISTS (
            SELECT 1 FROM variants sv JOIN sizes sz ON sz.id = sv.size_id
            WHERE sv.product_id = p.id AND sv.active AND sz.label = sqlc.narg(size)::text))
    GROUP BY p.id, c.name
)
SELECT id, slug, name, tier, material_line, badges, preorder_allowed, category_name,
       min_price_kobo, max_price_kobo, compare_at_kobo, colour_count, total_stock,
       COUNT(*) OVER ()::int AS total_count
FROM filtered
WHERE (sqlc.narg(min_kobo)::bigint IS NULL OR min_price_kobo >= sqlc.narg(min_kobo)::bigint)
  AND (sqlc.narg(max_kobo)::bigint IS NULL OR min_price_kobo <= sqlc.narg(max_kobo)::bigint)
ORDER BY
    CASE WHEN sqlc.arg(sort)::text = 'price_asc'  THEN min_price_kobo END ASC,
    CASE WHEN sqlc.arg(sort)::text = 'price_desc' THEN min_price_kobo END DESC,
    created_at DESC,
    id DESC
LIMIT sqlc.arg(page_limit)::int OFFSET sqlc.arg(page_offset)::int;

-- name: ListFilterColours :many
-- Colours that visible products in this category (or everywhere, if NULL) come in.
SELECT DISTINCT co.id, co.name, co.hex
FROM colours co
JOIN variants v ON v.colour_id = co.id AND v.active
JOIN products p ON p.id = v.product_id AND p.status = 'visible'
JOIN categories c ON c.id = p.category_id
WHERE sqlc.narg(category_slug)::text IS NULL
   OR c.slug = sqlc.narg(category_slug)::text
   OR c.parent_id = (SELECT pc.id FROM categories pc WHERE pc.slug = sqlc.narg(category_slug)::text)
ORDER BY co.name;

-- name: ListFilterSizes :many
-- Size labels used by visible products in this category, in size-set order.
SELECT sz.label, MIN(sz.position)::int AS position
FROM sizes sz
JOIN variants v ON v.size_id = sz.id AND v.active
JOIN products p ON p.id = v.product_id AND p.status = 'visible'
JOIN categories c ON c.id = p.category_id
WHERE sqlc.narg(category_slug)::text IS NULL
   OR c.slug = sqlc.narg(category_slug)::text
   OR c.parent_id = (SELECT pc.id FROM categories pc WHERE pc.slug = sqlc.narg(category_slug)::text)
GROUP BY sz.label
ORDER BY MIN(sz.position), sz.label;

-- name: GetProductBySlug :one
-- A visible product with its category, parent category and the return rule that applies.
SELECT p.id, p.slug, p.name, p.description, p.tier, p.material_line, p.badges,
       p.gift_box_allowed, p.preorder_allowed,
       c.id   AS category_id,
       c.name AS category_name,
       c.slug AS category_slug,
       pc.name AS parent_name,
       pc.slug AS parent_slug,
       COALESCE(NULLIF(c.return_rule, ''), pc.return_rule, '')::text AS return_rule
FROM products p
JOIN categories c ON c.id = p.category_id
LEFT JOIN categories pc ON pc.id = c.parent_id
WHERE p.slug = $1 AND p.status = 'visible';

-- name: ListVariantsForProduct :many
SELECT v.id, v.sku, v.price_kobo, v.compare_at_kobo, v.stock,
       v.colour_id, co.name AS colour_name, co.hex AS colour_hex,
       v.size_id, sz.label AS size_label, sz.size_set_id
FROM variants v
LEFT JOIN colours co ON co.id = v.colour_id
LEFT JOIN sizes sz ON sz.id = v.size_id
WHERE v.product_id = $1 AND v.active
ORDER BY v.position, co.name, sz.position;

-- name: GetSizeSet :one
SELECT id, name, measure_label FROM size_sets WHERE id = $1;

-- name: ListSizesInSet :many
SELECT id, label, measurement FROM sizes WHERE size_set_id = $1 ORDER BY position;

-- name: ListProductDetails :many
SELECT f.label, d.value
FROM product_details d
JOIN category_fields f ON f.id = d.field_id
WHERE d.product_id = $1
ORDER BY f.position;

-- name: ListProductImages :many
SELECT id, colour_id, storage_key, kind, width, height
FROM product_images
WHERE product_id = $1
ORDER BY position;

-- name: ListPairedProducts :many
-- "Complete the look" cards for a product.
SELECT p.id, p.slug, p.name, p.tier, p.material_line,
       MIN(v.price_kobo)::bigint AS min_price_kobo
FROM product_pairings pp
JOIN products p ON p.id = pp.paired_id AND p.status = 'visible'
JOIN variants v ON v.product_id = p.id AND v.active
WHERE pp.product_id = $1
GROUP BY p.id, pp.position
ORDER BY pp.position;
