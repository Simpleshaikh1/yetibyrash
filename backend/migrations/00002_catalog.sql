-- +goose Up
-- Catalogue: everything the admin's Catalogue setup and product editor control.

CREATE TYPE product_tier AS ENUM ('everyday', 'premium');
CREATE TYPE product_status AS ENUM ('draft', 'visible', 'hidden');

-- A named list of sizes, e.g. "Clothing sizes (UK)": XS, S, M, L, XL.
CREATE TABLE size_sets (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name          text NOT NULL UNIQUE,
    measure_label text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sizes (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    size_set_id bigint NOT NULL REFERENCES size_sets (id) ON DELETE CASCADE,
    label       text NOT NULL,
    measurement text NOT NULL DEFAULT '',
    position    int  NOT NULL DEFAULT 0,
    UNIQUE (size_set_id, label)
);

CREATE TABLE colours (
    id   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name text NOT NULL UNIQUE,
    hex  text NOT NULL CHECK (hex ~ '^#[0-9A-Fa-f]{6}$')
);

-- Two levels: top categories (Jewelry, Bags…) and their sub-categories (Rings, Totes…).
CREATE TABLE categories (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    parent_id           bigint REFERENCES categories (id) ON DELETE RESTRICT,
    name                text NOT NULL,
    slug                text NOT NULL UNIQUE,
    position            int  NOT NULL DEFAULT 0,
    default_size_set_id bigint REFERENCES size_sets (id) ON DELETE SET NULL,
    return_rule         text NOT NULL DEFAULT '',
    show_on_home        boolean NOT NULL DEFAULT false,
    created_at          timestamptz NOT NULL DEFAULT now()
);

-- Extra product details a category asks for (bags: Dimensions, Strap drop…).
-- Defined on top categories; sub-categories use their parent's fields.
CREATE TABLE category_fields (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    category_id bigint NOT NULL REFERENCES categories (id) ON DELETE CASCADE,
    label       text NOT NULL,
    position    int  NOT NULL DEFAULT 0,
    UNIQUE (category_id, label)
);

-- Price filter options in the shop, e.g. "Under ₦10,000". max_kobo NULL = no upper limit.
CREATE TABLE price_bands (
    id       bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    label    text   NOT NULL,
    min_kobo bigint NOT NULL DEFAULT 0,
    max_kobo bigint,
    position int    NOT NULL DEFAULT 0
);

CREATE TABLE products (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug             text NOT NULL UNIQUE,
    name             text NOT NULL,
    description      text NOT NULL DEFAULT '',
    category_id      bigint NOT NULL REFERENCES categories (id) ON DELETE RESTRICT,
    tier             product_tier   NOT NULL DEFAULT 'everyday',
    material_line    text NOT NULL DEFAULT '',
    status           product_status NOT NULL DEFAULT 'draft',
    badges           text[] NOT NULL DEFAULT '{}',
    gift_box_allowed boolean NOT NULL DEFAULT false,
    preorder_allowed boolean NOT NULL DEFAULT false,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX products_listing ON products (status, tier, category_id);

CREATE TABLE product_details (
    product_id bigint NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    field_id   bigint NOT NULL REFERENCES category_fields (id) ON DELETE CASCADE,
    value      text NOT NULL,
    PRIMARY KEY (product_id, field_id)
);

-- One sellable option: a colour and/or size of a product, with its own price and stock.
CREATE TABLE variants (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id      bigint NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    colour_id       bigint REFERENCES colours (id) ON DELETE RESTRICT,
    size_id         bigint REFERENCES sizes (id) ON DELETE RESTRICT,
    sku             text   NOT NULL UNIQUE,
    price_kobo      bigint NOT NULL CHECK (price_kobo > 0),
    compare_at_kobo bigint CHECK (compare_at_kobo IS NULL OR compare_at_kobo > price_kobo),
    stock           int    NOT NULL DEFAULT 0 CHECK (stock >= 0),
    active          boolean NOT NULL DEFAULT true,
    position        int    NOT NULL DEFAULT 0
);
-- A product can't have two variants for the same colour + size.
CREATE UNIQUE INDEX variants_one_per_option ON variants (product_id, COALESCE(colour_id, 0), COALESCE(size_id, 0));
CREATE INDEX variants_product ON variants (product_id);

CREATE TABLE product_images (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_id  bigint NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    colour_id   bigint REFERENCES colours (id) ON DELETE SET NULL,
    storage_key text NOT NULL,
    kind        text NOT NULL DEFAULT 'studio' CHECK (kind IN ('studio', 'real_life')),
    position    int  NOT NULL DEFAULT 0,
    width       int  NOT NULL DEFAULT 0,
    height      int  NOT NULL DEFAULT 0
);

-- "Complete the look": products shown under another product.
CREATE TABLE product_pairings (
    product_id bigint NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    paired_id  bigint NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    position   int    NOT NULL DEFAULT 0,
    PRIMARY KEY (product_id, paired_id),
    CHECK (product_id <> paired_id)
);

-- +goose Down
DROP TABLE product_pairings;
DROP TABLE product_images;
DROP TABLE variants;
DROP TABLE product_details;
DROP TABLE products;
DROP TABLE price_bands;
DROP TABLE category_fields;
DROP TABLE categories;
DROP TABLE colours;
DROP TABLE sizes;
DROP TABLE size_sets;
DROP TYPE product_status;
DROP TYPE product_tier;
