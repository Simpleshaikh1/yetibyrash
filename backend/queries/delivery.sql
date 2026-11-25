-- name: ListZones :many
SELECT id, name, position, standard_fee_kobo, standard_days_min, standard_days_max,
       express_fee_kobo, express_days, free_over_kobo
FROM delivery_zones ORDER BY position, id;

-- name: ListStates :many
SELECT id, name, zone_id FROM states ORDER BY name;

-- name: ListLgas :many
SELECT id, state_id, name, delivered, express FROM lgas ORDER BY state_id, name;

-- name: GetState :one
SELECT s.id, s.name, s.zone_id FROM states s WHERE s.id = $1;

-- name: GetLga :one
SELECT id, state_id, name, delivered, express FROM lgas WHERE id = $1;

-- name: CountLgasInState :one
SELECT count(*)::int FROM lgas WHERE state_id = $1;

-- name: GetZone :one
SELECT id, name, position, standard_fee_kobo, standard_days_min, standard_days_max,
       express_fee_kobo, express_days, free_over_kobo
FROM delivery_zones WHERE id = $1;

-- name: GetDeliverySettings :one
SELECT express_cutoff, dispatch_days FROM delivery_settings WHERE id = 1;

-- name: ListUpcomingHolidays :many
SELECT day, name FROM holidays WHERE day >= current_date - 1 ORDER BY day;

-- name: UpdateZone :exec
UPDATE delivery_zones
SET name = $2, standard_fee_kobo = $3, standard_days_min = $4, standard_days_max = $5,
    express_fee_kobo = $6, express_days = $7, free_over_kobo = $8
WHERE id = $1;

-- name: SetStateZone :exec
UPDATE states SET zone_id = $2 WHERE id = $1;

-- name: InsertLga :one
INSERT INTO lgas (state_id, name, delivered, express) VALUES ($1, $2, $3, $4) RETURNING id;

-- name: UpdateLga :exec
UPDATE lgas SET name = $2, delivered = $3, express = $4 WHERE id = $1;

-- name: DeleteLga :exec
DELETE FROM lgas WHERE id = $1;

-- name: UpdateDeliverySettings :exec
UPDATE delivery_settings SET express_cutoff = $1, dispatch_days = $2 WHERE id = 1;

-- name: InsertHoliday :exec
INSERT INTO holidays (day, name) VALUES ($1, $2)
ON CONFLICT (day) DO UPDATE SET name = EXCLUDED.name;

-- name: DeleteHoliday :exec
DELETE FROM holidays WHERE day = $1;
