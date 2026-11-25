-- +goose Up
-- Starting catalogue set-up, matching the admin's Catalogue setup screen.
-- Staff can change all of this later; it is here so a new shop is usable straight away.

INSERT INTO size_sets (name, measure_label) VALUES
    ('Clothing sizes (UK)', 'UK size · bust · waist · hips'),
    ('Ring sizes (US)',     'Inner diameter'),
    ('Bracelet sizes',      'Wrist size'),
    ('Anklet lengths',      'Length'),
    ('Necklace lengths',    'Where it sits'),
    ('Tights sizes',        'Height and hips'),
    ('Panty sizes',         'UK dress size · hips'),
    ('One size',            '');

INSERT INTO sizes (size_set_id, label, measurement, position)
SELECT s.id, v.label, v.measurement, v.position
FROM size_sets s
JOIN (VALUES
    ('Clothing sizes (UK)', 'XS', 'UK 6 · bust 78 · waist 60 · hips 86 cm', 1),
    ('Clothing sizes (UK)', 'S',  'UK 8 · bust 82 · waist 64 · hips 90 cm', 2),
    ('Clothing sizes (UK)', 'M',  'UK 10–12 · bust 88 · waist 70 · hips 96 cm', 3),
    ('Clothing sizes (UK)', 'L',  'UK 14 · bust 94 · waist 76 · hips 102 cm', 4),
    ('Clothing sizes (UK)', 'XL', 'UK 16 · bust 100 · waist 82 · hips 108 cm', 5),
    ('Ring sizes (US)', '5', '15.7 mm', 1),
    ('Ring sizes (US)', '6', '16.5 mm', 2),
    ('Ring sizes (US)', '7', '17.3 mm', 3),
    ('Ring sizes (US)', '8', '18.2 mm', 4),
    ('Ring sizes (US)', '9', '19.0 mm', 5),
    ('Bracelet sizes', 'S', 'Wrist up to 15 cm', 1),
    ('Bracelet sizes', 'M', 'Wrist 15–17 cm', 2),
    ('Bracelet sizes', 'L', 'Wrist 17–19 cm', 3),
    ('Anklet lengths', '9"',  '23 cm', 1),
    ('Anklet lengths', '10"', '25 cm', 2),
    ('Anklet lengths', '11"', '28 cm', 3),
    ('Necklace lengths', '16"', 'Collarbone', 1),
    ('Necklace lengths', '18"', 'Just below collarbone', 2),
    ('Necklace lengths', '20"', 'Above the bust', 3),
    ('Necklace lengths', '24"', 'Below the bust', 4),
    ('Tights sizes', 'S',   'Height 150–160 cm · hips 85–95 cm', 1),
    ('Tights sizes', 'M',   'Height 155–170 cm · hips 92–102 cm', 2),
    ('Tights sizes', 'L',   'Height 165–175 cm · hips 100–110 cm', 3),
    ('Tights sizes', 'XL',  'Height 170–180 cm · hips 108–120 cm', 4),
    ('Tights sizes', 'XXL', 'Height 170–185 cm · hips 118–130 cm', 5),
    ('Panty sizes', 'S',  'UK 8 · hips 86–91 cm', 1),
    ('Panty sizes', 'M',  'UK 10–12 · hips 91–99 cm', 2),
    ('Panty sizes', 'L',  'UK 14 · hips 99–104 cm', 3),
    ('Panty sizes', 'XL', 'UK 16–18 · hips 104–114 cm', 4)
) AS v (set_name, label, measurement, position) ON v.set_name = s.name;

INSERT INTO colours (name, hex) VALUES
    ('Espresso',  '#3A2420'),
    ('Blush',     '#EAC8BF'),
    ('Ivory',     '#F3E9DC'),
    ('Tan',       '#B07A4F'),
    ('Cream',     '#EFE4D2'),
    ('Gold',      '#C9A24B'),
    ('Silver',    '#C7C7CC'),
    ('Rose gold', '#C98F8F'),
    ('Black',     '#1E1A1A'),
    ('Grey marl', '#9A9A9E'),
    ('Nude 1',    '#E6C1A6'),
    ('Nude 2',    '#B9845E'),
    ('Nude 3',    '#7A4E33'),
    ('Ruby',      '#9B2335');

-- Top categories
INSERT INTO categories (name, slug, position, show_on_home, default_size_set_id, return_rule)
SELECT v.name, v.slug, v.position, true, s.id, v.return_rule
FROM (VALUES
    ('Jewelry',   'jewelry',   1, NULL,                  '7 days, unworn'),
    ('Watches',   'watches',   2, 'One size',            '7 days, with tags'),
    ('Bags',      'bags',      3, 'One size',            '7 days, unused with tags'),
    ('Clothing',  'clothing',  4, 'Clothing sizes (UK)', '7 days, unworn with tags'),
    ('Intimates', 'intimates', 5, NULL,                  'No returns once opened')
) AS v (name, slug, position, size_set, return_rule)
LEFT JOIN size_sets s ON s.name = v.size_set;

-- Sub-categories
INSERT INTO categories (parent_id, name, slug, position, default_size_set_id, return_rule)
SELECT p.id, v.name, v.slug, v.position, s.id, v.return_rule
FROM (VALUES
    ('jewelry',   'Necklaces',              'necklaces',            1, 'Necklace lengths',    '7 days, unworn'),
    ('jewelry',   'Earrings',               'earrings',             2, 'One size',            'No returns (hygiene)'),
    ('jewelry',   'Bracelets',              'bracelets',            3, 'Bracelet sizes',      '7 days, unworn'),
    ('jewelry',   'Rings',                  'rings',                4, 'Ring sizes (US)',     '7 days, unworn'),
    ('jewelry',   'Anklets',                'anklets',              5, 'Anklet lengths',      '7 days, unworn'),
    ('jewelry',   'Bead pieces',            'bead-pieces',          6, 'Bracelet sizes',      '7 days, unworn'),
    ('bags',      'Handbags and totes',     'handbags-and-totes',   1, 'One size',            '7 days, unused with tags'),
    ('bags',      'Shoulder and crossbody', 'shoulder-and-crossbody', 2, 'One size',          '7 days, unused with tags'),
    ('bags',      'Clutches and evening',   'clutches-and-evening', 3, 'One size',            '7 days, unused with tags'),
    ('bags',      'Mini bags',              'mini-bags',            4, 'One size',            '7 days, unused with tags'),
    ('clothing',  'Joggers',                'joggers',              1, 'Clothing sizes (UK)', '7 days, unworn with tags'),
    ('clothing',  'Tops',                   'tops',                 2, 'Clothing sizes (UK)', '7 days, unworn with tags'),
    ('clothing',  'Shorts and skirts',      'shorts-and-skirts',    3, 'Clothing sizes (UK)', '7 days, unworn with tags'),
    ('clothing',  'Co-ord sets',            'co-ord-sets',          4, 'Clothing sizes (UK)', '7 days, unworn with tags'),
    ('intimates', 'Tights',                 'tights',               1, 'Tights sizes',        'No returns once opened'),
    ('intimates', 'Panties',                'panties',              2, 'Panty sizes',         'No returns once opened')
) AS v (parent_slug, name, slug, position, size_set, return_rule)
JOIN categories p ON p.slug = v.parent_slug
LEFT JOIN size_sets s ON s.name = v.size_set;

INSERT INTO category_fields (category_id, label, position)
SELECT c.id, v.label, v.position
FROM categories c
JOIN (VALUES
    ('jewelry',   'Metal', 1), ('jewelry', 'Plating', 2), ('jewelry', 'Stone', 3), ('jewelry', 'Hypoallergenic', 4),
    ('watches',   'Case size', 1), ('watches', 'Strap', 2), ('watches', 'Movement', 3),
    ('watches',   'Water resistance', 4), ('watches', 'Warranty', 5), ('watches', 'In the box', 6),
    ('bags',      'Dimensions (W × H × D)', 1), ('bags', 'Strap drop', 2), ('bags', 'Material', 3),
    ('bags',      'Closure', 4), ('bags', 'Inside', 5), ('bags', 'Fits', 6),
    ('clothing',  'Fabric', 1), ('clothing', 'Fit', 2), ('clothing', 'Inside leg (size M)', 3), ('clothing', 'Model wears', 4),
    ('intimates', 'Fabric', 1), ('intimates', 'Denier', 2), ('intimates', 'Pack size', 3), ('intimates', 'Washing', 4)
) AS v (slug, label, position) ON v.slug = c.slug;

INSERT INTO price_bands (label, min_kobo, max_kobo, position) VALUES
    ('Under ₦10,000',     0,        999999,  1),
    ('₦10,000 – ₦30,000', 1000000,  3000000, 2),
    ('₦30,000 – ₦80,000', 3000001,  8000000, 3),
    ('Over ₦80,000',      8000001,  NULL,    4);

-- +goose Down
DELETE FROM price_bands;
DELETE FROM category_fields;
DELETE FROM categories WHERE parent_id IS NOT NULL;
DELETE FROM categories;
DELETE FROM colours;
DELETE FROM sizes;
DELETE FROM size_sets;
