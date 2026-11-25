-- name: GetStoreSetting :one
SELECT key, value, updated_at FROM store_settings WHERE key = $1;

-- name: ListStoreSettings :many
SELECT key, value, updated_at FROM store_settings ORDER BY key;
