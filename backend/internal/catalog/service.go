package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"yetibyrash/backend/internal/db/sqlc"
	"yetibyrash/backend/internal/promotions"
)

// Service reads the catalogue from Postgres.
type Service struct {
	q *sqlc.Queries
}

func NewService(db sqlc.DBTX) *Service {
	return &Service{q: sqlc.New(db)}
}

// Categories returns the top categories, each with its sub-categories.
func (s *Service) Categories(ctx context.Context) ([]Category, error) {
	rows, err := s.q.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	// Rows come parents first, so every child finds its parent already placed.
	var tops []Category
	index := map[int64]int{}
	for _, r := range rows {
		c := Category{ID: r.ID, Name: r.Name, Slug: r.Slug, ShowOnHome: r.ShowOnHome, Children: []Category{}}
		if !r.ParentID.Valid {
			index[r.ID] = len(tops)
			tops = append(tops, c)
			continue
		}
		if i, ok := index[r.ParentID.Int64]; ok {
			tops[i].Children = append(tops[i].Children, c)
		}
	}
	if tops == nil {
		tops = []Category{}
	}
	return tops, nil
}

// ListProducts returns one page of the shop grid plus the filter choices for that category.
func (s *Service) ListProducts(ctx context.Context, f ProductFilter) (ProductList, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Sort == "" {
		f.Sort = SortNewest
	}

	bands, err := s.priceBands(ctx)
	if err != nil {
		return ProductList{}, err
	}

	params := sqlc.ListProductsParams{
		CategorySlug: text(f.CategorySlug),
		Colour:       text(f.Colour),
		Size:         text(f.Size),
		Sort:         f.Sort,
		PageLimit:    PageSize,
		PageOffset:   int32((f.Page - 1) * PageSize),
	}
	if f.Tier != "" {
		params.Tier = sqlc.NullProductTier{ProductTier: sqlc.ProductTier(f.Tier), Valid: true}
	}
	if f.BandID != 0 {
		band, ok := findBand(bands, f.BandID)
		if !ok {
			return ProductList{}, fmt.Errorf("price band %d: %w", f.BandID, ErrNotFound)
		}
		params.MinKobo = pgtype.Int8{Int64: band.MinKobo, Valid: true}
		if band.MaxKobo != nil {
			params.MaxKobo = pgtype.Int8{Int64: *band.MaxKobo, Valid: true}
		}
	}

	rows, err := s.q.ListProducts(ctx, params)
	if err != nil {
		return ProductList{}, fmt.Errorf("list products: %w", err)
	}

	list := ProductList{Items: make([]ProductCard, 0, len(rows)), Page: f.Page, PageSize: PageSize}
	for _, r := range rows {
		list.Total = int(r.TotalCount)
		list.Items = append(list.Items, ProductCard{
			ID:            r.ID,
			Slug:          r.Slug,
			Name:          r.Name,
			Tier:          Tier(r.Tier),
			MaterialLine:  r.MaterialLine,
			CategoryName:  r.CategoryName,
			Badges:        nonNil(r.Badges),
			PriceKobo:     r.MinPriceKobo,
			MaxPriceKobo:  r.MaxPriceKobo,
			CompareAtKobo: positive(r.CompareAtKobo),
			ColourCount:   int(r.ColourCount),
			InStock:       r.TotalStock > 0,
			Preorder:      r.PreorderAllowed,
			Image:         r.ImageKey,
		})
	}

	if err := s.applySales(ctx, list.Items); err != nil {
		return ProductList{}, err
	}
	list.Filters, err = s.filterOptions(ctx, f.CategorySlug, bands)
	if err != nil {
		return ProductList{}, err
	}
	return list, nil
}

// Product returns everything the product page needs.
func (s *Service) Product(ctx context.Context, slug string) (ProductDetail, error) {
	p, err := s.q.GetProductBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProductDetail{}, ErrNotFound
	}
	if err != nil {
		return ProductDetail{}, fmt.Errorf("get product %q: %w", slug, err)
	}

	d := ProductDetail{
		ID:              p.ID,
		Slug:            p.Slug,
		Name:            p.Name,
		Description:     p.Description,
		Tier:            Tier(p.Tier),
		MaterialLine:    p.MaterialLine,
		Badges:          nonNil(p.Badges),
		GiftBoxAllowed:  p.GiftBoxAllowed,
		PreorderAllowed: p.PreorderAllowed,
		Category:        CategoryRef{Name: p.CategoryName, Slug: p.CategorySlug},
		ReturnRule:      p.ReturnRule,
		Colours:         []Colour{},
		Variants:        []Variant{},
		Details:         []Detail{},
		Images:          []Image{},
		Pairings:        []ProductCard{},
	}
	if p.ParentSlug.Valid {
		d.Parent = &CategoryRef{Name: p.ParentName.String, Slug: p.ParentSlug.String}
	}

	variants, err := s.q.ListVariantsForProduct(ctx, p.ID)
	if err != nil {
		return ProductDetail{}, fmt.Errorf("list variants: %w", err)
	}
	sales, err := promotions.LoadSales(ctx, s.q, []int64{p.ID})
	if err != nil {
		return ProductDetail{}, err
	}
	seenColour := map[string]bool{}
	var sizeSetID int64
	saleBadge := ""
	for _, v := range variants {
		price := sales.Apply(p.ID, v.PriceKobo, v.CompareAtKobo.Int64)
		if price.Badge != "" {
			saleBadge = price.Badge
		}
		d.Variants = append(d.Variants, Variant{
			ID:            v.ID,
			SKU:           v.Sku,
			Colour:        v.ColourName.String,
			Size:          v.SizeLabel.String,
			PriceKobo:     price.Kobo,
			CompareAtKobo: positive(price.CompareAtKobo),
			Stock:         int(v.Stock),
		})
		if v.ColourName.Valid && !seenColour[v.ColourName.String] {
			seenColour[v.ColourName.String] = true
			d.Colours = append(d.Colours, Colour{Name: v.ColourName.String, Hex: v.ColourHex.String})
		}
		if v.SizeSetID.Valid {
			sizeSetID = v.SizeSetID.Int64
		}
	}

	d.SaleBadge = saleBadge

	if sizeSetID != 0 {
		set, err := s.q.GetSizeSet(ctx, sizeSetID)
		if err != nil {
			return ProductDetail{}, fmt.Errorf("get size set: %w", err)
		}
		sizes, err := s.q.ListSizesInSet(ctx, sizeSetID)
		if err != nil {
			return ProductDetail{}, fmt.Errorf("list sizes: %w", err)
		}
		d.SizeSet = &SizeSet{Name: set.Name, MeasureLabel: set.MeasureLabel, Sizes: make([]Size, 0, len(sizes))}
		for _, sz := range sizes {
			d.SizeSet.Sizes = append(d.SizeSet.Sizes, Size{Label: sz.Label, Measurement: sz.Measurement})
		}
	}

	details, err := s.q.ListProductDetails(ctx, p.ID)
	if err != nil {
		return ProductDetail{}, fmt.Errorf("list details: %w", err)
	}
	for _, x := range details {
		d.Details = append(d.Details, Detail{Label: x.Label, Value: x.Value})
	}

	images, err := s.q.ListProductImages(ctx, p.ID)
	if err != nil {
		return ProductDetail{}, fmt.Errorf("list images: %w", err)
	}
	for _, im := range images {
		d.Images = append(d.Images, Image{ID: im.ID, ColourID: nullInt(im.ColourID), Key: im.StorageKey, Kind: im.Kind})
	}

	pairs, err := s.q.ListPairedProducts(ctx, p.ID)
	if err != nil {
		return ProductDetail{}, fmt.Errorf("list pairings: %w", err)
	}
	for _, x := range pairs {
		d.Pairings = append(d.Pairings, ProductCard{
			ID: x.ID, Slug: x.Slug, Name: x.Name, Tier: Tier(x.Tier), MaterialLine: x.MaterialLine,
			Badges: []string{}, PriceKobo: x.MinPriceKobo, MaxPriceKobo: x.MinPriceKobo, InStock: true,
			Image: x.ImageKey,
		})
	}
	if err := s.applySales(ctx, d.Pairings); err != nil {
		return ProductDetail{}, err
	}

	d.Reviews = []Review{}
	revs, err := s.q.ProductReviews(ctx, p.ID)
	if err != nil {
		return ProductDetail{}, fmt.Errorf("reviews: %w", err)
	}
	for _, r := range revs {
		photos := r.Photos
		if photos == nil {
			photos = []string{}
		}
		d.Reviews = append(d.Reviews, Review{ID: r.ID, Rating: int(r.Rating), Text: r.Text, Photos: photos, Reply: r.Reply,
			Author: r.AuthorName, Area: r.AuthorArea, Option: r.VariantLabel, PublishedAt: r.PublishedAt.Time})
	}
	ratings, err := s.ratings(ctx, []int64{p.ID})
	if err != nil {
		return ProductDetail{}, err
	}
	d.Rating = ratings[p.ID]

	d.LowStockThreshold, err = s.lowStockThreshold(ctx)
	if err != nil {
		return ProductDetail{}, err
	}
	return d, nil
}

// Home returns what the homepage shows for a tier ("" = both tiers).
func (s *Service) Home(ctx context.Context, tier Tier) (Home, error) {
	home := Home{
		Tagline: "Elegance meets affordability", Sections: []HomeSection{}, Categories: []Category{},
		NewIn: []ProductCard{}, BestSellers: []ProductCard{}, Budgets: []PriceBand{}, Reviews: []HomeReview{},
	}
	if raw, err := s.q.GetStoreSetting(ctx, "tagline"); err == nil {
		var t string
		if json.Unmarshal(raw.Value, &t) == nil && t != "" {
			home.Tagline = t
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Home{}, fmt.Errorf("get tagline: %w", err)
	}

	sections, err := s.q.ListSections(ctx)
	if err != nil {
		return Home{}, fmt.Errorf("list sections: %w", err)
	}
	var tierArg sqlc.NullProductTier
	if tier != "" {
		tierArg = sqlc.NullProductTier{ProductTier: sqlc.ProductTier(tier), Valid: true}
	}
	for _, sec := range sections {
		if !sec.Visible {
			continue
		}
		home.Sections = append(home.Sections, HomeSection{Key: sec.Key, Title: sec.Title, Lines: nonNil(sec.Lines)})
		n := int(sec.ItemCount)
		switch sec.Key {
		case "hero":
			b, err := s.q.LiveBanner(ctx, tierArg)
			if err == nil {
				home.Hero = &Banner{Eyebrow: b.Eyebrow, Headline: b.Headline, ButtonText: b.ButtonText, ButtonLink: b.ButtonLink, Image: b.ImageKey}
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return Home{}, fmt.Errorf("live banner: %w", err)
			}
		case "categories":
			cats, err := s.Categories(ctx)
			if err != nil {
				return Home{}, err
			}
			for _, c := range cats {
				if c.ShowOnHome {
					home.Categories = append(home.Categories, c)
				}
			}
		case "new_in":
			newIn, err := s.ListProducts(ctx, ProductFilter{Tier: tier, Sort: SortNewest, Page: 1})
			if err != nil {
				return Home{}, err
			}
			home.NewIn = newIn.Items[:min(n, len(newIn.Items))]
		case "best_sellers":
			ids, err := s.q.BestSellerIDs(ctx, sqlc.BestSellerIDsParams{Tier: tierArg, MaxItems: int32(n)})
			if err != nil {
				return Home{}, fmt.Errorf("best sellers: %w", err)
			}
			if home.BestSellers, err = s.CardsByID(ctx, ids); err != nil {
				return Home{}, err
			}
		case "reviews":
			rows, err := s.q.HomeReviews(ctx, int32(n))
			if err != nil {
				return Home{}, fmt.Errorf("home reviews: %w", err)
			}
			for _, r := range rows {
				home.Reviews = append(home.Reviews, HomeReview{ID: r.ID, Text: r.Text, Author: r.AuthorName, Area: r.AuthorArea, Product: r.ProductName, ProductSlug: r.ProductSlug})
			}
		case "gift_budget":
			if home.Budgets, err = s.priceBands(ctx); err != nil {
				return Home{}, err
			}
		}
	}
	return home, nil
}

// CardsByID returns product cards in the order of ids, skipping hidden products.
func (s *Service) CardsByID(ctx context.Context, ids []int64) ([]ProductCard, error) {
	out := []ProductCard{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.q.ProductCardsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("product cards: %w", err)
	}
	byID := map[int64]sqlc.ProductCardsByIDsRow{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	for _, id := range ids {
		r, ok := byID[id]
		if !ok {
			continue
		}
		out = append(out, ProductCard{
			ID: r.ID, Slug: r.Slug, Name: r.Name, Tier: Tier(r.Tier), MaterialLine: r.MaterialLine,
			CategoryName: r.CategoryName, Badges: nonNil(r.Badges), PriceKobo: r.MinPriceKobo,
			MaxPriceKobo: r.MaxPriceKobo, CompareAtKobo: positive(r.CompareAtKobo), ColourCount: int(r.ColourCount),
			InStock: r.TotalStock > 0, Preorder: r.PreorderAllowed, Image: r.ImageKey,
		})
	}
	return out, s.applySales(ctx, out)
}

// ratings returns published-review averages and counts by product.
func (s *Service) ratings(ctx context.Context, ids []int64) (map[int64]Rating, error) {
	rows, err := s.q.ProductRatings(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("ratings: %w", err)
	}
	out := map[int64]Rating{}
	for _, r := range rows {
		out[r.ProductID] = Rating{Average: float64(int(r.Average*10+0.5)) / 10, Count: int(r.Count)}
	}
	return out, nil
}

// applySales swaps in sale prices and the sale badge, and adds star ratings.
func (s *Service) applySales(ctx context.Context, cards []ProductCard) error {
	if len(cards) == 0 {
		return nil
	}
	ids := make([]int64, len(cards))
	for i, c := range cards {
		ids[i] = c.ID
	}
	ratings, err := s.ratings(ctx, ids)
	if err != nil {
		return err
	}
	for i := range cards {
		cards[i].Rating = ratings[cards[i].ID]
	}
	sales, err := promotions.LoadSales(ctx, s.q, ids)
	if err != nil {
		return err
	}
	for i := range cards {
		c := &cards[i]
		var compareAt int64
		if c.CompareAtKobo != nil {
			compareAt = *c.CompareAtKobo
		}
		low := sales.Apply(c.ID, c.PriceKobo, compareAt)
		if low.SaleID == 0 {
			continue
		}
		c.PriceKobo = low.Kobo
		c.CompareAtKobo = positive(low.CompareAtKobo)
		c.MaxPriceKobo = sales.Apply(c.ID, c.MaxPriceKobo, 0).Kobo
		c.SaleBadge = low.Badge
	}
	return nil
}

func (s *Service) priceBands(ctx context.Context) ([]PriceBand, error) {
	rows, err := s.q.ListPriceBands(ctx)
	if err != nil {
		return nil, fmt.Errorf("list price bands: %w", err)
	}
	bands := make([]PriceBand, 0, len(rows))
	for _, r := range rows {
		bands = append(bands, PriceBand{ID: r.ID, Label: r.Label, MinKobo: r.MinKobo, MaxKobo: nullInt(r.MaxKobo)})
	}
	return bands, nil
}

func (s *Service) filterOptions(ctx context.Context, categorySlug string, bands []PriceBand) (FilterOptions, error) {
	opts := FilterOptions{Colours: []Colour{}, Sizes: []string{}, PriceBands: bands}
	colours, err := s.q.ListFilterColours(ctx, text(categorySlug))
	if err != nil {
		return opts, fmt.Errorf("list filter colours: %w", err)
	}
	for _, c := range colours {
		opts.Colours = append(opts.Colours, Colour{Name: c.Name, Hex: c.Hex})
	}
	sizes, err := s.q.ListFilterSizes(ctx, text(categorySlug))
	if err != nil {
		return opts, fmt.Errorf("list filter sizes: %w", err)
	}
	for _, sz := range sizes {
		opts.Sizes = append(opts.Sizes, sz.Label)
	}
	return opts, nil
}

// lowStockThreshold reads the admin's "Show 'Only X left' at" setting (default 6).
func (s *Service) lowStockThreshold(ctx context.Context) (int, error) {
	raw, err := s.q.GetStoreSetting(ctx, "low_stock_threshold")
	if errors.Is(err, pgx.ErrNoRows) {
		return 6, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get low stock threshold: %w", err)
	}
	var n int
	if err := json.Unmarshal(raw.Value, &n); err != nil {
		return 6, nil
	}
	return n, nil
}

func findBand(bands []PriceBand, id int64) (PriceBand, bool) {
	for _, b := range bands {
		if b.ID == id {
			return b, true
		}
	}
	return PriceBand{}, false
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

func nullInt(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func positive(v int64) *int64 {
	if v <= 0 {
		return nil
	}
	return &v
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
