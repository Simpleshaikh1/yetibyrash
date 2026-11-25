-- +goose Up
-- Shop-wide settings the admin edits in Settings › Store details
-- (WhatsApp number, low-stock warning level, socials, logo).
CREATE TABLE store_settings (
    key        text PRIMARY KEY,
    value      jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO store_settings (key, value) VALUES
    ('shop_name',           '"Yetibyrash"'),
    ('tagline',             '"Elegance meets affordability"'),
    ('low_stock_threshold', '6');

-- +goose Down
DROP TABLE store_settings;
