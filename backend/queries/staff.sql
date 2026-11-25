-- name: GetStaffByEmail :one
SELECT id, email, name, password_hash, role, active
FROM staff WHERE lower(email) = lower(sqlc.arg(email)::text);

-- name: GetStaffByID :one
SELECT id, email, name, role, active, last_login_at, created_at
FROM staff WHERE id = $1;

-- name: ListStaff :many
SELECT id, email, name, role, active, last_login_at, created_at
FROM staff ORDER BY active DESC, name;

-- name: CreateStaff :one
INSERT INTO staff (email, name, password_hash, role)
VALUES (lower(sqlc.arg(email)::text), sqlc.arg(name), sqlc.arg(password_hash), sqlc.arg(role))
RETURNING id;

-- name: UpdateStaff :exec
UPDATE staff SET name = $2, role = $3, active = $4 WHERE id = $1;

-- name: SetStaffPassword :exec
UPDATE staff SET password_hash = $2 WHERE id = $1;

-- name: CountActiveOwners :one
SELECT count(*)::int FROM staff WHERE role = 'owner' AND active;

-- name: TouchStaffLogin :exec
UPDATE staff SET last_login_at = now() WHERE id = $1;

-- name: CreateSession :exec
INSERT INTO staff_sessions (token_hash, staff_id, expires_at) VALUES ($1, $2, $3);

-- name: GetSessionStaff :one
-- The signed-in staff member for a session token, if the session is live and the account active.
SELECT s.id, s.email, s.name, s.role
FROM staff_sessions ss
JOIN staff s ON s.id = ss.staff_id
WHERE ss.token_hash = $1 AND ss.expires_at > now() AND s.active;

-- name: DeleteSession :exec
DELETE FROM staff_sessions WHERE token_hash = $1;

-- name: DeleteStaffSessions :exec
DELETE FROM staff_sessions WHERE staff_id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM staff_sessions WHERE expires_at <= now();

-- name: InsertAudit :exec
INSERT INTO audit_log (staff_id, action, entity, entity_id, details)
VALUES ($1, $2, $3, $4, $5);
