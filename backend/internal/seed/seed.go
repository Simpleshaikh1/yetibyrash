// Package seed fills a development database with example products.
// It never runs in production: the real catalogue comes from the admin.
package seed

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Demo inserts the example products if the products table is empty.
// It reports whether it inserted anything.
func Demo(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM products`).Scan(&count); err != nil {
		return false, fmt.Errorf("count products: %w", err)
	}
	if count > 0 {
		return false, nil
	}

	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		ids := map[string]int64{}
		for i, p := range demoProducts {
			id, err := insertProduct(ctx, tx, p, i)
			if err != nil {
				return fmt.Errorf("%s: %w", p.Slug, err)
			}
			ids[p.Slug] = id
		}
		for _, p := range demoProducts {
			for pos, other := range p.CompleteWith {
				otherID, ok := ids[other]
				if !ok {
					return fmt.Errorf("%s: complete-the-look product %q is not in the demo data", p.Slug, other)
				}
				if _, err := tx.Exec(ctx,
					`INSERT INTO product_pairings (product_id, paired_id, position) VALUES ($1, $2, $3)`,
					ids[p.Slug], otherID, pos); err != nil {
					return fmt.Errorf("%s: pairing: %w", p.Slug, err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// Reset deletes every product (and, through cascades, their variants, details and pairings).
func Reset(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `DELETE FROM products`)
	return err
}

func insertProduct(ctx context.Context, tx pgx.Tx, p demoProduct, index int) (int64, error) {
	tier := "everyday"
	if p.Premium {
		tier = "premium"
	}
	badges := p.Badges
	if badges == nil {
		badges = []string{}
	}

	var id int64
	// created_at is staggered so "newest" order is the order of demoProducts.
	err := tx.QueryRow(ctx, `
		INSERT INTO products (slug, name, description, category_id, tier, material_line, status, badges,
		                      gift_box_allowed, preorder_allowed, created_at)
		VALUES ($1, $2, $3, (SELECT id FROM categories WHERE slug = $4), $5, $6, 'visible', $7, $8, $9,
		        now() - make_interval(mins => $10))
		RETURNING id`,
		p.Slug, p.Name, p.Description, p.Category, tier, p.Material, badges, p.GiftBox, p.Preorder, index,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert product: %w", err)
	}

	for _, d := range p.Details {
		// Fields live on the top category; a product in a sub-category uses its parent's fields.
		tag, err := tx.Exec(ctx, `
			INSERT INTO product_details (product_id, field_id, value)
			SELECT $1, f.id, $3
			FROM category_fields f
			JOIN categories c ON c.slug = $2
			WHERE f.label = $4 AND f.category_id = COALESCE(c.parent_id, c.id)`,
			id, p.Category, d[1], d[0])
		if err != nil {
			return 0, fmt.Errorf("detail %q: %w", d[0], err)
		}
		if tag.RowsAffected() == 0 {
			return 0, fmt.Errorf("detail %q: no such field for category %q", d[0], p.Category)
		}
	}

	sizes, err := sizeLabels(ctx, tx, p)
	if err != nil {
		return 0, err
	}
	colours := p.Colours
	if len(colours) == 0 {
		colours = []string{""}
	}
	if len(sizes) == 0 {
		sizes = []string{""}
	}

	pos := 0
	for ci, colour := range colours {
		for si, size := range sizes {
			key := colour + "/" + size
			stock := 3 + (ci*7+si*5)%12 // varied but repeatable
			switch {
			case matches(p.SoldOut, colour, size):
				stock = 0
			case matches(p.LowStock, colour, size):
				stock = 2
			}
			price := p.PriceKobo
			if sp, ok := p.SizePrices[size]; ok {
				price = sp
			}
			var was *int64
			if p.WasKobo > 0 {
				was = &p.WasKobo
			}
			_, err := tx.Exec(ctx, `
				INSERT INTO variants (product_id, colour_id, size_id, sku, price_kobo, compare_at_kobo, stock, position)
				VALUES ($1,
				        (SELECT id FROM colours WHERE name = NULLIF($2, '')),
				        (SELECT sz.id FROM sizes sz JOIN size_sets ss ON ss.id = sz.size_set_id
				          WHERE ss.name = $3 AND sz.label = NULLIF($4, '')),
				        $5, $6, $7, $8, $9)`,
				id, colour, p.SizeSet, size, sku(p.SKU, colour, size), price, was, stock, pos)
			if err != nil {
				return 0, fmt.Errorf("variant %s: %w", key, err)
			}
			pos++
		}
	}
	return id, nil
}

// sizeLabels returns the sizes to create variants for: the listed ones, or the whole set.
func sizeLabels(ctx context.Context, tx pgx.Tx, p demoProduct) ([]string, error) {
	if p.SizeSet == "" {
		return nil, nil
	}
	rows, err := tx.Query(ctx, `
		SELECT sz.label FROM sizes sz JOIN size_sets ss ON ss.id = sz.size_set_id
		WHERE ss.name = $1 ORDER BY sz.position`, p.SizeSet)
	if err != nil {
		return nil, fmt.Errorf("sizes for %q: %w", p.SizeSet, err)
	}
	all, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("size set %q has no sizes", p.SizeSet)
	}
	if p.Sizes == nil {
		return all, nil
	}
	for _, s := range p.Sizes {
		if !slices.Contains(all, s) {
			return nil, fmt.Errorf("size %q is not in %q", s, p.SizeSet)
		}
	}
	return p.Sizes, nil
}

// matches reports whether an option list contains this colour/size.
// Entries can be "Colour/Size", "Colour" (every size) or "/Size" (every colour).
func matches(list []string, colour, size string) bool {
	for _, e := range list {
		c, s, hasSize := strings.Cut(e, "/")
		if (c == "" || c == colour) && (!hasSize || s == size) {
			return true
		}
	}
	return false
}

// sku builds e.g. "JG-RIB-ESP-M" from a prefix, colour and size.
func sku(prefix, colour, size string) string {
	parts := []string{prefix}
	if colour != "" {
		code := strings.ToUpper(strings.ReplaceAll(colour, " ", ""))
		if len(code) > 3 {
			code = code[:3]
		}
		parts = append(parts, code)
	}
	if size != "" {
		parts = append(parts, strings.Trim(size, `"`))
	}
	return strings.Join(parts, "-")
}
