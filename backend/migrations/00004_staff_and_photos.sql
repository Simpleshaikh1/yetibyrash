-- +goose Up
-- Staff accounts for the admin, their login sessions, and a log of every admin change.

CREATE TYPE staff_role AS ENUM ('owner', 'manager', 'packer', 'content_editor');

CREATE TABLE staff (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email         text NOT NULL,
    name          text NOT NULL,
    password_hash text NOT NULL,
    role          staff_role NOT NULL,
    active        boolean NOT NULL DEFAULT true,
    last_login_at timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX staff_email ON staff (lower(email));

-- Only a hash of the session token is stored, so a leaked database can't be used to log in.
CREATE TABLE staff_sessions (
    token_hash bytea PRIMARY KEY,
    staff_id   bigint NOT NULL REFERENCES staff (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);
CREATE INDEX staff_sessions_staff ON staff_sessions (staff_id);

CREATE TABLE audit_log (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    staff_id   bigint REFERENCES staff (id) ON DELETE SET NULL,
    action     text NOT NULL, -- e.g. "product.update"
    entity     text NOT NULL, -- e.g. "product"
    entity_id  bigint,
    details    jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_entity ON audit_log (entity, entity_id);

-- Photo quality warnings shown on the admin's photo tiles ("small", "dark").
ALTER TABLE product_images
    ADD COLUMN warnings   text[] NOT NULL DEFAULT '{}',
    ADD COLUMN created_at timestamptz NOT NULL DEFAULT now();

-- +goose Down
ALTER TABLE product_images DROP COLUMN warnings, DROP COLUMN created_at;
DROP TABLE audit_log;
DROP TABLE staff_sessions;
DROP TABLE staff;
DROP TYPE staff_role;
