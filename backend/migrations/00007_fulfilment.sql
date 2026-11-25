-- +goose Up
-- Packing and shipping orders, customer messages, and a queue for background work.

-- Who carries parcels. tracking_url may contain {tracking_number}, e.g. https://track.example/?n={tracking_number}
CREATE TABLE couriers (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name         text    NOT NULL UNIQUE,
    tracking_url text    NOT NULL DEFAULT '',
    active       boolean NOT NULL DEFAULT true
);
INSERT INTO couriers (name) VALUES ('Own rider (Lagos)'), ('Courier partner (interstate)');

ALTER TABLE orders
    ADD COLUMN courier_id      bigint REFERENCES couriers (id) ON DELETE SET NULL,
    ADD COLUMN tracking_number text NOT NULL DEFAULT '',
    ADD COLUMN status_at       timestamptz NOT NULL DEFAULT now(); -- when the current status was set

CREATE INDEX orders_status ON orders (status, status_at);

-- public = the shopper sees this note on their tracker; otherwise it's for staff only.
ALTER TABLE order_events ADD COLUMN public boolean NOT NULL DEFAULT false;
UPDATE order_events SET public = true WHERE status = 'paid' AND note LIKE 'Paid with Paystack%';

-- What the shopper is told at each step. Placeholders: {first_name} {order_number} {total}
-- {items} {delivery_dates} {courier} {tracking_number} {tracking_link} {track_url} {note}
CREATE TABLE message_templates (
    event   order_status PRIMARY KEY,
    subject text    NOT NULL,
    body    text    NOT NULL,
    active  boolean NOT NULL DEFAULT true
);

INSERT INTO message_templates (event, subject, body) VALUES
('paid', 'We''ve got your order, {first_name}',
'Hi {first_name},

Thank you for shopping with Yetibyrash! We received {total} for order {order_number}.

{items}

Expected delivery: {delivery_dates}.
We''ll message you when it''s packed and on its way.

Track your order: {track_url}

Yetibyrash · Elegance meets affordability'),
('packed', 'Your order {order_number} is packed',
'Hi {first_name},

Your Yetibyrash pieces are wrapped and ready to go. Order {order_number}.
{note}
Track your order: {track_url}'),
('shipped', 'Order {order_number} is on its way',
'Hi {first_name},

Order {order_number} has left our studio with {courier}. Expected {delivery_dates}.
Tracking number: {tracking_number} {tracking_link}
{note}
Track your order: {track_url}'),
('out_for_delivery', 'Arriving today: order {order_number}',
'Hi {first_name},

Your order is out for delivery today. Please keep your phone close; the rider will call.
{note}
Track your order: {track_url}'),
('delivered', 'Delivered! How did we do?',
'Hi {first_name},

Order {order_number} has been delivered. We hope you love it!
If anything isn''t right, reply to this email or message us on WhatsApp.
{note}'),
('cancelled', 'Order {order_number} has been cancelled',
'Hi {first_name},

Order {order_number} has been cancelled.
{note}
If you paid, your refund of {total} will be sent back to the card or account you paid with.');

-- Background work (sending emails now; review requests and more later).
-- The worker takes due jobs with FOR UPDATE SKIP LOCKED, so several workers never take the same one.
CREATE TABLE jobs (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind       text  NOT NULL,
    payload    jsonb NOT NULL DEFAULT '{}',
    run_at     timestamptz NOT NULL DEFAULT now(),
    attempts   int   NOT NULL DEFAULT 0,
    last_error text  NOT NULL DEFAULT '',
    done_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX jobs_due ON jobs (run_at) WHERE done_at IS NULL;

-- +goose Down
DROP TABLE jobs;
DROP TABLE message_templates;
ALTER TABLE order_events DROP COLUMN public;
DROP INDEX orders_status;
ALTER TABLE orders DROP COLUMN courier_id, DROP COLUMN tracking_number, DROP COLUMN status_at;
DROP TABLE couriers;
