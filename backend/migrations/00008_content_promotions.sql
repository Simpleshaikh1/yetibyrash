-- +goose Up
-- M5: homepage content (banners, announcements, sections, pages) and promotions
-- (sales, discount codes, free gift with purchase).

-- Hero banners. The homepage shows the first live banner for the shopper's tier.
CREATE TABLE banners (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    eyebrow     text NOT NULL DEFAULT '',          -- small label above the headline
    headline    text NOT NULL,
    button_text text NOT NULL DEFAULT 'Shop now',
    button_link text NOT NULL DEFAULT '/shop',     -- a path on the shop, e.g. /shop/clothing
    image_key   text NOT NULL DEFAULT '',          -- files: /media/<key>-phone.jpg and -wide.jpg
    tier        product_tier,                      -- NULL = both tiers
    starts_at   timestamptz,                       -- NULL = from now
    ends_at     timestamptz,                       -- NULL = no end
    position    int  NOT NULL DEFAULT 0,
    active      boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (ends_at IS NULL OR starts_at IS NULL OR ends_at > starts_at)
);

-- Short messages in the bar at the top of every shop page.
CREATE TABLE announcements (
    id        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    text      text NOT NULL,
    link      text NOT NULL DEFAULT '',
    position  int  NOT NULL DEFAULT 0,
    active    boolean NOT NULL DEFAULT true,
    starts_at timestamptz,
    ends_at   timestamptz
);

INSERT INTO announcements (text, link, position) VALUES
    ('Delivery to all 36 states and FCT', '/track', 1),
    ('Secure payment with Paystack', '', 2);

-- The homepage blocks, in order. Staff rename, reorder and hide them; they can't add new kinds.
CREATE TABLE homepage_sections (
    key        text PRIMARY KEY,  -- hero | categories | new_in | best_sellers | gift_budget | trust
    title      text NOT NULL,
    position   int  NOT NULL,
    visible    boolean NOT NULL DEFAULT true,
    item_count int  NOT NULL DEFAULT 6 CHECK (item_count BETWEEN 2 AND 12), -- product rows only
    lines      text[] NOT NULL DEFAULT '{}'                                 -- trust strip only
);

INSERT INTO homepage_sections (key, title, position, lines) VALUES
    ('hero',         'Hero banner',        1, '{}'),
    ('categories',   'Shop by category',   2, '{}'),
    ('new_in',       'New in',             3, '{}'),
    ('best_sellers', 'Best sellers',       4, '{}'),
    ('gift_budget',  'Gift shop by budget', 5, '{}'),
    ('trust',        'Why shop with us',   6, ARRAY['Secure payment with Paystack', 'Discreet packaging', 'Delivery to all 36 states']);

-- Simple pages: About, FAQ, policies. Body is light Markdown (headings, lists, bold, links).
CREATE TABLE pages (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug       text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    title      text NOT NULL,
    body       text NOT NULL DEFAULT '',
    published  boolean NOT NULL DEFAULT false,
    position   int  NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Drafts: the owner writes the real wording before publishing.
INSERT INTO pages (slug, title, position) VALUES
    ('about', 'About us', 1),
    ('faq', 'FAQ', 2),
    ('contact', 'Contact', 3),
    ('delivery-and-returns', 'Delivery and returns', 4),
    ('jewelry-care', 'Jewelry care', 5),
    ('privacy', 'Privacy policy', 6),
    ('terms', 'Terms of sale', 7);

CREATE TYPE discount_kind AS ENUM ('percent', 'fixed', 'free_delivery');
CREATE TYPE promotion_scope AS ENUM ('all', 'tier', 'category');

-- Sales: lower prices for a time. When two overlap on a product, the bigger saving wins.
CREATE TABLE promotions (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name            text NOT NULL,
    kind            discount_kind NOT NULL CHECK (kind IN ('percent', 'fixed')),
    value           bigint NOT NULL CHECK (value > 0), -- percent (1–90) or kobo off each item
    applies_to      promotion_scope NOT NULL DEFAULT 'all',
    tier            product_tier,
    category_id     bigint REFERENCES categories (id) ON DELETE RESTRICT, -- includes its sub-categories
    starts_at       timestamptz NOT NULL,
    ends_at         timestamptz NOT NULL,
    badge_text      text NOT NULL DEFAULT '',  -- on product cards, e.g. "20% OFF · BLACK FRIDAY"
    announcement    text NOT NULL DEFAULT '',  -- shown in the top bar while the sale is live
    exclude_on_sale boolean NOT NULL DEFAULT true, -- skip products that already have a compare-at price
    active          boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (ends_at > starts_at),
    CHECK (kind <> 'percent' OR value <= 90),
    CHECK ((applies_to = 'tier') = (tier IS NOT NULL)),
    CHECK ((applies_to = 'category') = (category_id IS NOT NULL))
);

CREATE TABLE discount_codes (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code             text NOT NULL UNIQUE CHECK (code ~ '^[A-Z0-9]{3,20}$'),
    kind             discount_kind NOT NULL,
    value            bigint NOT NULL DEFAULT 0, -- percent or kobo; unused for free_delivery
    min_order_kobo   bigint NOT NULL DEFAULT 0 CHECK (min_order_kobo >= 0),
    max_uses         int CHECK (max_uses IS NULL OR max_uses > 0),
    used             int NOT NULL DEFAULT 0 CHECK (used >= 0), -- counted at checkout, given back if unpaid
    first_order_only boolean NOT NULL DEFAULT false,
    zone_id          bigint REFERENCES delivery_zones (id) ON DELETE RESTRICT, -- free delivery only here
    starts_at        timestamptz,
    ends_at          timestamptz,
    active           boolean NOT NULL DEFAULT true,
    created_at       timestamptz NOT NULL DEFAULT now(),
    CHECK (kind = 'free_delivery' OR value > 0),
    CHECK (kind <> 'percent' OR value <= 90)
);

-- Free gift once the bag reaches a spend. One live offer at a time (the newest wins).
CREATE TABLE gift_offers (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    threshold_kobo bigint NOT NULL CHECK (threshold_kobo > 0),
    variant_id     bigint NOT NULL REFERENCES variants (id) ON DELETE RESTRICT,
    quantity       int NOT NULL CHECK (quantity > 0), -- how many gifts to give in total
    given          int NOT NULL DEFAULT 0 CHECK (given >= 0),
    starts_at      timestamptz,
    ends_at        timestamptz,
    active         boolean NOT NULL DEFAULT true,
    created_at     timestamptz NOT NULL DEFAULT now()
);

-- Orders remember what they got.
ALTER TABLE orders
    ADD COLUMN discount_kobo   bigint NOT NULL DEFAULT 0 CHECK (discount_kobo >= 0),
    ADD COLUMN discount_code   text   NOT NULL DEFAULT '',
    ADD COLUMN promotion_id    bigint REFERENCES promotions (id) ON DELETE SET NULL,
    ADD COLUMN gift_offer_id   bigint REFERENCES gift_offers (id) ON DELETE SET NULL;
ALTER TABLE orders DROP CONSTRAINT orders_check;
ALTER TABLE orders ADD CONSTRAINT orders_total_kobo_check
    CHECK (total_kobo = subtotal_kobo - discount_kobo + delivery_kobo AND total_kobo > 0);

-- The free gift is an order line at ₦0, so packing lists and restocking include it.
ALTER TABLE order_items
    ADD COLUMN is_gift boolean NOT NULL DEFAULT false;
ALTER TABLE order_items DROP CONSTRAINT order_items_unit_kobo_check;
ALTER TABLE order_items ADD CONSTRAINT order_items_unit_kobo_check
    CHECK (unit_kobo > 0 OR (is_gift AND unit_kobo = 0));

-- A code typed into the bag stays with the bag.
ALTER TABLE carts ADD COLUMN discount_code text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE carts DROP COLUMN discount_code;
DELETE FROM order_items WHERE is_gift;
ALTER TABLE order_items DROP CONSTRAINT order_items_unit_kobo_check;
ALTER TABLE order_items ADD CONSTRAINT order_items_unit_kobo_check CHECK (unit_kobo > 0);
ALTER TABLE order_items DROP COLUMN is_gift;
ALTER TABLE orders DROP CONSTRAINT orders_total_kobo_check;
ALTER TABLE orders DROP COLUMN gift_offer_id, DROP COLUMN promotion_id, DROP COLUMN discount_code, DROP COLUMN discount_kobo;
ALTER TABLE orders ADD CONSTRAINT orders_check CHECK (total_kobo = subtotal_kobo + delivery_kobo);
DROP TABLE gift_offers;
DROP TABLE discount_codes;
DROP TABLE promotions;
DROP TYPE promotion_scope;
DROP TYPE discount_kind;
DROP TABLE pages;
DROP TABLE homepage_sections;
DROP TABLE announcements;
DROP TABLE banners;
