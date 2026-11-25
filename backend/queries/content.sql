-- ---------- Banners ----------

-- name: LiveBanner :one
-- The hero for a tier: the first active banner inside its dates, for that tier or both.
SELECT id, eyebrow, headline, button_text, button_link, image_key, tier
FROM banners
WHERE active
  AND (starts_at IS NULL OR starts_at <= now())
  AND (ends_at IS NULL OR ends_at > now())
  AND (tier IS NULL OR sqlc.narg(tier)::product_tier IS NULL OR tier = sqlc.narg(tier)::product_tier)
ORDER BY (tier IS NULL), position, id
LIMIT 1;

-- name: ListBanners :many
SELECT id, eyebrow, headline, button_text, button_link, image_key, tier, starts_at, ends_at, position, active
FROM banners ORDER BY position, id;

-- name: GetBanner :one
SELECT id, eyebrow, headline, button_text, button_link, image_key, tier, starts_at, ends_at, position, active
FROM banners WHERE id = $1;

-- name: InsertBanner :one
INSERT INTO banners (eyebrow, headline, button_text, button_link, tier, starts_at, ends_at, active, position)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, (SELECT COALESCE(MAX(position), 0) + 1 FROM banners))
RETURNING id;

-- name: UpdateBanner :execrows
UPDATE banners SET eyebrow = $2, headline = $3, button_text = $4, button_link = $5,
    tier = $6, starts_at = $7, ends_at = $8, active = $9
WHERE id = $1;

-- name: SetBannerImage :one
-- Returns the old key so its files can be deleted.
UPDATE banners b SET image_key = $2 FROM banners old
WHERE b.id = $1 AND old.id = b.id
RETURNING old.image_key;

-- name: SetBannerPosition :exec
UPDATE banners SET position = $2 WHERE id = $1;

-- name: DeleteBanner :one
DELETE FROM banners WHERE id = $1 RETURNING image_key;

-- ---------- Announcements ----------

-- name: LiveAnnouncements :many
SELECT text, link FROM announcements
WHERE active AND (starts_at IS NULL OR starts_at <= now()) AND (ends_at IS NULL OR ends_at > now())
ORDER BY position, id;

-- name: ListAnnouncements :many
SELECT id, text, link, position, active, starts_at, ends_at FROM announcements ORDER BY position, id;

-- name: InsertAnnouncement :one
INSERT INTO announcements (text, link, active, starts_at, ends_at, position)
VALUES ($1, $2, $3, $4, $5, (SELECT COALESCE(MAX(position), 0) + 1 FROM announcements))
RETURNING id;

-- name: UpdateAnnouncement :execrows
UPDATE announcements SET text = $2, link = $3, active = $4, starts_at = $5, ends_at = $6 WHERE id = $1;

-- name: SetAnnouncementPosition :exec
UPDATE announcements SET position = $2 WHERE id = $1;

-- name: DeleteAnnouncement :execrows
DELETE FROM announcements WHERE id = $1;

-- ---------- Homepage sections ----------

-- name: ListSections :many
SELECT key, title, position, visible, item_count, lines FROM homepage_sections ORDER BY position, key;

-- name: UpdateSection :execrows
UPDATE homepage_sections SET title = $2, position = $3, visible = $4, item_count = $5, lines = $6 WHERE key = $1;

-- ---------- Pages ----------

-- name: ListPages :many
SELECT id, slug, title, body, published, position, updated_at FROM pages ORDER BY position, id;

-- name: ListPublishedPages :many
SELECT slug, title FROM pages WHERE published ORDER BY position, id;

-- name: GetPublishedPage :one
SELECT slug, title, body, updated_at FROM pages WHERE slug = $1 AND published;

-- name: InsertPage :one
INSERT INTO pages (slug, title, body, published, position)
VALUES ($1, $2, $3, $4, (SELECT COALESCE(MAX(position), 0) + 1 FROM pages))
RETURNING id;

-- name: UpdatePage :execrows
UPDATE pages SET slug = $2, title = $3, body = $4, published = $5, updated_at = now() WHERE id = $1;

-- name: DeletePage :execrows
DELETE FROM pages WHERE id = $1;

-- ---------- Homepage product rows ----------

-- name: BestSellerIDs :many
-- Most items sold in paid orders over the last 30 days, for a tier (NULL = both).
SELECT p.id
FROM order_items i
JOIN orders o ON o.id = i.order_id
JOIN variants v ON v.id = i.variant_id
JOIN products p ON p.id = v.product_id
WHERE o.status IN ('paid', 'packed', 'shipped', 'out_for_delivery', 'delivered')
  AND o.paid_at > now() - interval '30 days'
  AND NOT i.is_gift
  AND p.status = 'visible'
  AND (sqlc.narg(tier)::product_tier IS NULL OR p.tier = sqlc.narg(tier)::product_tier)
GROUP BY p.id
ORDER BY SUM(i.qty) DESC, p.id
LIMIT sqlc.arg(max_items)::int;

-- name: ProductCardsByIDs :many
-- Product cards for a list of products (order is restored in Go).
SELECT p.id, p.slug, p.name, p.tier, p.material_line, p.badges, p.preorder_allowed,
       c.name                                AS category_name,
       MIN(v.price_kobo)::bigint             AS min_price_kobo,
       MAX(v.price_kobo)::bigint             AS max_price_kobo,
       COALESCE(MIN(v.compare_at_kobo), 0)::bigint AS compare_at_kobo,
       COUNT(DISTINCT v.colour_id)::int      AS colour_count,
       COALESCE(SUM(v.stock), 0)::int        AS total_stock,
       COALESCE((SELECT im.storage_key FROM product_images im WHERE im.product_id = p.id
                 ORDER BY im.position, im.id LIMIT 1), '')::text AS image_key
FROM products p
JOIN categories c ON c.id = p.category_id
JOIN variants v ON v.product_id = p.id AND v.active
WHERE p.id = ANY (sqlc.arg(ids)::bigint[]) AND p.status = 'visible'
GROUP BY p.id, c.name;
