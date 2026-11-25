-- +goose Up
-- Bags (carts), orders and Paystack payments.

CREATE TABLE carts (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE cart_items (
    cart_id    uuid   NOT NULL REFERENCES carts (id) ON DELETE CASCADE,
    variant_id bigint NOT NULL REFERENCES variants (id) ON DELETE CASCADE,
    qty        int    NOT NULL CHECK (qty BETWEEN 1 AND 20),
    added_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (cart_id, variant_id)
);

CREATE TYPE order_status AS ENUM (
    'pending_payment', -- waiting for Paystack; stock is reserved
    'paid', 'packed', 'shipped', 'out_for_delivery', 'delivered',
    'abandoned',       -- no payment within 30 minutes; stock released
    'cancelled', 'refunded'
);

CREATE TYPE delivery_speed AS ENUM ('standard', 'express');

CREATE SEQUENCE order_number_seq START 20001;

CREATE TABLE orders (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    number            text   NOT NULL UNIQUE DEFAULT 'YB-' || nextval('order_number_seq'),
    status            order_status NOT NULL DEFAULT 'pending_payment',
    cart_id           uuid,        -- emptied once the order is paid
    payment_reference text   NOT NULL UNIQUE,

    -- Copied at checkout, so the order never changes if the customer or catalogue does.
    customer_name     text   NOT NULL,
    customer_phone    text   NOT NULL,
    customer_email    text   NOT NULL,
    state_name        text   NOT NULL,
    lga_name          text   NOT NULL,
    street            text   NOT NULL,
    landmark          text   NOT NULL DEFAULT '',
    zone_name         text   NOT NULL,
    speed             delivery_speed NOT NULL,

    subtotal_kobo     bigint NOT NULL CHECK (subtotal_kobo > 0),
    delivery_kobo     bigint NOT NULL CHECK (delivery_kobo >= 0),
    total_kobo        bigint NOT NULL CHECK (total_kobo = subtotal_kobo + delivery_kobo),

    expected_from     date   NOT NULL,
    expected_to       date   NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    paid_at           timestamptz
);
CREATE INDEX orders_pending ON orders (created_at) WHERE status = 'pending_payment';

CREATE TABLE order_items (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id      bigint NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    variant_id    bigint NOT NULL REFERENCES variants (id) ON DELETE RESTRICT,
    product_name  text   NOT NULL,
    product_slug  text   NOT NULL,
    variant_label text   NOT NULL, -- "Espresso / M"
    sku           text   NOT NULL,
    image_key     text   NOT NULL DEFAULT '',
    unit_kobo     bigint NOT NULL CHECK (unit_kobo > 0),
    qty           int    NOT NULL CHECK (qty > 0)
);
CREATE INDEX order_items_order ON order_items (order_id);

-- What happened to an order and when. The shopper's tracker reads this.
CREATE TABLE order_events (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id   bigint NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    status     order_status NOT NULL,
    note       text NOT NULL DEFAULT '',
    staff_id   bigint REFERENCES staff (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX order_events_order ON order_events (order_id);

-- Every payment Paystack tells us about, matched to an order or not.
CREATE TABLE payments (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reference   text   NOT NULL UNIQUE,
    order_id    bigint REFERENCES orders (id) ON DELETE SET NULL,
    amount_kobo bigint NOT NULL,
    channel     text   NOT NULL DEFAULT '',
    -- success | no_order (paid but we have no such order) | amount_mismatch
    outcome     text   NOT NULL,
    payload     jsonb  NOT NULL DEFAULT '{}',
    received_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE payments;
DROP TABLE order_events;
DROP TABLE order_items;
DROP TABLE orders;
DROP SEQUENCE order_number_seq;
DROP TYPE delivery_speed;
DROP TYPE order_status;
DROP TABLE cart_items;
DROP TABLE carts;
