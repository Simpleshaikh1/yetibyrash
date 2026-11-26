-- ---------- Review requests ----------

-- name: InsertReviewRequest :exec
INSERT INTO review_requests (order_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '60 days');

-- name: ReviewRequestOrder :one
-- The order behind a review link that hasn't expired.
SELECT o.id, o.number, o.status, o.customer_name, o.lga_name, o.state_name
FROM review_requests r JOIN orders o ON o.id = r.order_id
WHERE r.token_hash = $1 AND r.expires_at > now();

-- name: ReviewableItems :many
-- An order's items (not the free gift), with any review already left.
SELECT i.id, v.product_id, i.product_name, i.product_slug, i.variant_label, i.image_key,
       r.id AS review_id, r.rating, r.status AS review_status, r.photos
FROM order_items i
JOIN variants v ON v.id = i.variant_id
LEFT JOIN reviews r ON r.order_item_id = i.id
WHERE i.order_id = $1 AND NOT i.is_gift
ORDER BY i.id;

-- name: InsertReview :one
INSERT INTO reviews (product_id, order_item_id, rating, text, author_name, author_area, variant_label, order_number)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id;

-- name: ReviewForPhotos :one
-- A waiting review that belongs to the order behind a link (photos can be added until approved).
SELECT r.id, r.photos FROM reviews r JOIN order_items i ON i.id = r.order_item_id
WHERE r.id = $1 AND i.order_id = $2 AND r.status = 'waiting'
FOR UPDATE OF r;

-- name: SetReviewPhotos :exec
UPDATE reviews SET photos = $2 WHERE id = $1;

-- ---------- Shop ----------

-- name: ProductRatings :many
-- Average stars and count of published reviews, for product cards.
SELECT product_id, AVG(rating)::float8 AS average, COUNT(*)::int AS count
FROM reviews WHERE status = 'published' AND product_id = ANY (sqlc.arg(ids)::bigint[])
GROUP BY product_id;

-- name: ProductReviews :many
SELECT id, rating, text, photos, reply, author_name, author_area, variant_label, published_at
FROM reviews WHERE product_id = $1 AND status = 'published'
ORDER BY published_at DESC
LIMIT 20;

-- name: HomeReviews :many
-- Recent 5-star reviews for the homepage strip.
SELECT r.id, r.text, r.author_name, r.author_area, p.name AS product_name, p.slug AS product_slug
FROM reviews r JOIN products p ON p.id = r.product_id
WHERE r.status = 'published' AND r.rating = 5 AND r.text <> '' AND p.status = 'visible'
ORDER BY r.published_at DESC
LIMIT sqlc.arg(max_items)::int;

-- ---------- Admin ----------

-- name: AdminListReviews :many
SELECT r.id, r.rating, r.text, r.photos, r.status, r.reply, r.author_name, r.author_area, r.variant_label,
       r.order_number, r.created_at, p.name AS product_name, p.slug AS product_slug
FROM reviews r JOIN products p ON p.id = r.product_id
WHERE r.status = $1
ORDER BY r.created_at DESC
LIMIT 200;

-- name: ReviewCounts :one
SELECT COUNT(*) FILTER (WHERE status = 'waiting')::int AS waiting,
       COUNT(*) FILTER (WHERE status = 'published')::int AS published,
       COUNT(*) FILTER (WHERE status = 'hidden')::int AS hidden,
       COALESCE(AVG(rating) FILTER (WHERE status = 'published'), 0)::float8 AS average
FROM reviews;

-- name: SetReviewStatus :execrows
UPDATE reviews SET status = $2,
    published_at = CASE WHEN $2 = 'published'::review_status THEN COALESCE(published_at, now()) ELSE published_at END
WHERE id = $1;

-- name: SetReviewReply :execrows
UPDATE reviews SET reply = $2 WHERE id = $1;
