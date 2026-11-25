-- +goose Up
-- Delivery zones, states, LGAs and the rules that decide fees and delivery dates.

CREATE TABLE delivery_zones (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name              text   NOT NULL UNIQUE,
    position          int    NOT NULL DEFAULT 0,
    standard_fee_kobo bigint NOT NULL CHECK (standard_fee_kobo >= 0),
    standard_days_min int    NOT NULL CHECK (standard_days_min >= 0),
    standard_days_max int    NOT NULL CHECK (standard_days_max >= standard_days_min),
    -- Express is offered only when express_fee_kobo is set, and only in LGAs marked express.
    express_fee_kobo  bigint CHECK (express_fee_kobo IS NULL OR express_fee_kobo >= 0),
    express_days      int    CHECK (express_days IS NULL OR express_days >= 0), -- 0 = same day if ordered before the cut-off
    -- Standard delivery is free when the bag costs at least this much. NULL = never free.
    free_over_kobo    bigint CHECK (free_over_kobo IS NULL OR free_over_kobo > 0)
);

CREATE TABLE states (
    id      bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name    text   NOT NULL UNIQUE,
    zone_id bigint NOT NULL REFERENCES delivery_zones (id) ON DELETE RESTRICT
);

-- LGAs are listed where we know them (Lagos and Abuja to start). For a state with none listed,
-- checkout asks for the LGA or town as text instead.
CREATE TABLE lgas (
    id        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    state_id  bigint  NOT NULL REFERENCES states (id) ON DELETE CASCADE,
    name      text    NOT NULL,
    delivered boolean NOT NULL DEFAULT true,  -- false = we don't deliver here
    express   boolean NOT NULL DEFAULT false, -- express offered here (if the zone has express)
    UNIQUE (state_id, name)
);

-- One row of shop-wide delivery rules.
CREATE TABLE delivery_settings (
    id             int  PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    express_cutoff time NOT NULL DEFAULT '12:00',
    -- ISO weekdays we dispatch on: 1 = Monday … 7 = Sunday.
    dispatch_days  int[] NOT NULL DEFAULT '{1,2,3,4,5,6}'
);

CREATE TABLE holidays (
    day  date PRIMARY KEY,
    name text NOT NULL
);

INSERT INTO delivery_settings DEFAULT VALUES;

INSERT INTO delivery_zones (name, position, standard_fee_kobo, standard_days_min, standard_days_max, express_fee_kobo, express_days, free_over_kobo) VALUES
    ('Lagos',                       1, 250000, 1, 2, 400000, 0,    4000000),
    ('South-West',                  2, 400000, 2, 4, NULL,   NULL, 6000000),
    ('Abuja and North-Central',     3, 450000, 2, 4, 700000, 1,    6000000),
    ('South-South and South-East',  4, 500000, 3, 5, NULL,   NULL, 7500000),
    ('North',                       5, 600000, 4, 6, NULL,   NULL, 8000000);

INSERT INTO states (name, zone_id)
SELECT v.name, z.id
FROM (VALUES
    ('Lagos', 'Lagos'),
    ('Ogun', 'South-West'), ('Oyo', 'South-West'), ('Osun', 'South-West'), ('Ondo', 'South-West'), ('Ekiti', 'South-West'),
    ('FCT (Abuja)', 'Abuja and North-Central'), ('Niger', 'Abuja and North-Central'), ('Kogi', 'Abuja and North-Central'),
    ('Kwara', 'Abuja and North-Central'), ('Nasarawa', 'Abuja and North-Central'), ('Benue', 'Abuja and North-Central'),
    ('Plateau', 'Abuja and North-Central'),
    ('Rivers', 'South-South and South-East'), ('Delta', 'South-South and South-East'), ('Edo', 'South-South and South-East'),
    ('Akwa Ibom', 'South-South and South-East'), ('Cross River', 'South-South and South-East'), ('Bayelsa', 'South-South and South-East'),
    ('Anambra', 'South-South and South-East'), ('Enugu', 'South-South and South-East'), ('Imo', 'South-South and South-East'),
    ('Abia', 'South-South and South-East'), ('Ebonyi', 'South-South and South-East'),
    ('Kano', 'North'), ('Kaduna', 'North'), ('Katsina', 'North'), ('Sokoto', 'North'), ('Kebbi', 'North'),
    ('Zamfara', 'North'), ('Jigawa', 'North'), ('Bauchi', 'North'), ('Gombe', 'North'), ('Adamawa', 'North'),
    ('Taraba', 'North'), ('Borno', 'North'), ('Yobe', 'North')
) AS v (name, zone)
JOIN delivery_zones z ON z.name = v.zone;

INSERT INTO lgas (state_id, name, express)
SELECT s.id, v.name, v.express
FROM (VALUES
    ('Lagos', 'Agege', false), ('Lagos', 'Ajeromi-Ifelodun', false), ('Lagos', 'Alimosho', false),
    ('Lagos', 'Amuwo-Odofin', false), ('Lagos', 'Apapa', false), ('Lagos', 'Badagry', false),
    ('Lagos', 'Epe', false), ('Lagos', 'Eti-Osa', true), ('Lagos', 'Ibeju-Lekki', false),
    ('Lagos', 'Ifako-Ijaiye', false), ('Lagos', 'Ikeja', true), ('Lagos', 'Ikorodu', false),
    ('Lagos', 'Kosofe', false), ('Lagos', 'Lagos Island', true), ('Lagos', 'Lagos Mainland', false),
    ('Lagos', 'Mushin', false), ('Lagos', 'Ojo', false), ('Lagos', 'Oshodi-Isolo', false),
    ('Lagos', 'Shomolu', false), ('Lagos', 'Surulere', true),
    ('FCT (Abuja)', 'Abaji', false), ('FCT (Abuja)', 'Abuja Municipal (AMAC)', true), ('FCT (Abuja)', 'Bwari', false),
    ('FCT (Abuja)', 'Gwagwalada', false), ('FCT (Abuja)', 'Kuje', false), ('FCT (Abuja)', 'Kwali', false)
) AS v (state, name, express)
JOIN states s ON s.name = v.state;

-- +goose Down
DROP TABLE holidays;
DROP TABLE delivery_settings;
DROP TABLE lgas;
DROP TABLE states;
DROP TABLE delivery_zones;
