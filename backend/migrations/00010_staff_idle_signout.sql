-- +goose Up
-- Admin sign-ins end after 8 hours without use (as well as 7 days after signing in).
ALTER TABLE staff_sessions ADD COLUMN last_seen_at timestamptz NOT NULL DEFAULT now();

-- +goose Down
ALTER TABLE staff_sessions DROP COLUMN last_seen_at;
