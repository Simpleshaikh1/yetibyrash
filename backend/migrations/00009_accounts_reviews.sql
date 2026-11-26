-- +goose Up
-- M6: customer accounts (one-time code sign-in, addresses, saved items), reviews,
-- and SMS alongside email.

-- A customer is created the first time they sign in. Accounts are optional:
-- guests still check out, and their orders show up once they sign in with the
-- same phone or email.
CREATE TABLE customers (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name             text NOT NULL DEFAULT '',
    phone            text UNIQUE,           -- "0803 000 0000"
    email            text UNIQUE,           -- lower case
    marketing_opt_in boolean NOT NULL DEFAULT false,
    created_at       timestamptz NOT NULL DEFAULT now(),
    last_seen_at     timestamptz NOT NULL DEFAULT now(),
    CHECK (phone IS NOT NULL OR email IS NOT NULL)
);

CREATE TABLE customer_sessions (
    token_hash  bytea PRIMARY KEY,          -- sha256 of the cookie value
    customer_id bigint NOT NULL REFERENCES customers (id) ON DELETE CASCADE,
    expires_at  timestamptz NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX customer_sessions_customer ON customer_sessions (customer_id);

-- One-time sign-in codes. Only a hash is kept; a code works once, for 10 minutes, 5 tries.
CREATE TABLE otp_codes (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    target     text NOT NULL,               -- the phone or email it was sent to
    code_hash  bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    attempts   int NOT NULL DEFAULT 0,
    used_at    timestamptz,
    ip         text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX otp_codes_target ON otp_codes (target, created_at DESC);

CREATE TABLE addresses (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    customer_id bigint NOT NULL REFERENCES customers (id) ON DELETE CASCADE,
    state_id    bigint NOT NULL REFERENCES states (id) ON DELETE RESTRICT,
    lga_id      bigint REFERENCES lgas (id) ON DELETE SET NULL,
    lga_text    text NOT NULL DEFAULT '',   -- for states without a listed LGA
    street      text NOT NULL,
    landmark    text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX addresses_customer ON addresses (customer_id);

CREATE TABLE saved_items (
    customer_id bigint NOT NULL REFERENCES customers (id) ON DELETE CASCADE,
    product_id  bigint NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (customer_id, product_id)
);

ALTER TABLE orders ADD COLUMN customer_id bigint REFERENCES customers (id) ON DELETE SET NULL;

-- ---------- Reviews ----------

CREATE TYPE review_status AS ENUM ('waiting', 'published', 'hidden');

-- The private link in the "how did you like it?" message. Only its hash is kept.
CREATE TABLE review_requests (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id   bigint NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- One review per bought item, so only real buyers can review. Nothing shows until approved.
CREATE TABLE reviews (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id    bigint NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    order_item_id bigint NOT NULL UNIQUE REFERENCES order_items (id) ON DELETE CASCADE,
    rating        int NOT NULL CHECK (rating BETWEEN 1 AND 5),
    text          text NOT NULL DEFAULT '',
    photos        text[] NOT NULL DEFAULT '{}', -- photo keys, up to 3
    status        review_status NOT NULL DEFAULT 'waiting',
    reply         text NOT NULL DEFAULT '',     -- the shop's public reply
    author_name   text NOT NULL,                -- "Chioma E."
    author_area   text NOT NULL DEFAULT '',     -- "Enugu"
    variant_label text NOT NULL DEFAULT '',     -- "M / Espresso"
    order_number  text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    published_at  timestamptz,
    CHECK (cardinality(photos) <= 3)
);
CREATE INDEX reviews_product ON reviews (product_id, published_at DESC) WHERE status = 'published';
CREATE INDEX reviews_status ON reviews (status, created_at DESC);

-- A homepage strip of 5-star reviews (hidden until there are some).
INSERT INTO homepage_sections (key, title, position, item_count) VALUES ('reviews', 'What our customers say', 7, 6);

-- ---------- Messages: SMS text, and the review request ----------

ALTER TABLE message_templates ALTER COLUMN event TYPE text USING event::text;
ALTER TABLE message_templates ADD COLUMN sms text NOT NULL DEFAULT '';

UPDATE message_templates SET sms = 'Yetibyrash: we got your order {order_number} ({total}). Track it: {track_url}' WHERE event = 'paid';
UPDATE message_templates SET sms = 'Yetibyrash: order {order_number} is on its way with {courier}. Track it: {track_url}' WHERE event = 'shipped';
UPDATE message_templates SET sms = 'Yetibyrash: order {order_number} arrives today. Please keep your phone close; the rider will call.' WHERE event = 'out_for_delivery';
UPDATE message_templates SET sms = 'Yetibyrash: order {order_number} was cancelled. {note}' WHERE event = 'cancelled';

INSERT INTO message_templates (event, subject, body, sms) VALUES
('review_request', 'How are you finding your Yetibyrash pieces?',
'Hi {first_name},

We hope you''re loving order {order_number}. Would you tell other shoppers what you think?
It takes a minute, and you can add photos.

Leave a review: {review_link}

Thank you!
Yetibyrash · Elegance meets affordability',
'Yetibyrash: how do you like order {order_number}? Leave a quick review: {review_link}');

-- +goose Down
DELETE FROM message_templates WHERE event = 'review_request';
ALTER TABLE message_templates DROP COLUMN sms;
ALTER TABLE message_templates ALTER COLUMN event TYPE order_status USING event::order_status;
DELETE FROM homepage_sections WHERE key = 'reviews';
DROP TABLE reviews;
DROP TABLE review_requests;
DROP TYPE review_status;
ALTER TABLE orders DROP COLUMN customer_id;
DROP TABLE saved_items;
DROP TABLE addresses;
DROP TABLE otp_codes;
DROP TABLE customer_sessions;
DROP TABLE customers;
