package catalog

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"yetibyrash/backend/internal/db/sqlc"
	"yetibyrash/backend/internal/media"
	"yetibyrash/backend/internal/staff"
)

// AdminService is the admin's side of the catalogue: product editor, photos and catalogue setup.
// Every change is written to the audit log with the staff member who made it.
type AdminService struct {
	pool    *pgxpool.Pool
	q       *sqlc.Queries
	storage media.Storage
}

func NewAdminService(pool *pgxpool.Pool, storage media.Storage) *AdminService {
	return &AdminService{pool: pool, q: sqlc.New(pool), storage: storage}
}

// inTx runs fn in a transaction: everything is saved, or nothing is.
func (s *AdminService) inTx(ctx context.Context, fn func(q *sqlc.Queries) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return fn(s.q.WithTx(tx)) })
}

// ---------- Products ----------

func (s *AdminService) ListProducts(ctx context.Context, f AdminProductFilter) ([]AdminProductRow, error) {
	params := sqlc.AdminListProductsParams{Search: text(strings.TrimSpace(f.Search))}
	if f.Status != "" {
		params.Status = sqlc.NullProductStatus{ProductStatus: sqlc.ProductStatus(f.Status), Valid: true}
	}
	if f.CategoryID != 0 {
		params.CategoryID = pgtype.Int8{Int64: f.CategoryID, Valid: true}
	}
	rows, err := s.q.AdminListProducts(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	out := make([]AdminProductRow, 0, len(rows))
	for _, r := range rows {
		cat := r.CategoryName
		if r.ParentName.Valid {
			cat = r.ParentName.String + " › " + r.CategoryName
		}
		out = append(out, AdminProductRow{
			ID: r.ID, Slug: r.Slug, Name: r.Name, Tier: Tier(r.Tier), Status: ProductStatus(r.Status),
			Category: cat, MinPriceKobo: r.MinPriceKobo, MaxPriceKobo: r.MaxPriceKobo,
			TotalStock: int(r.TotalStock), OptionCount: int(r.OptionCount), SoldOutCount: int(r.SoldOutCount),
			Image: r.ImageKey, UpdatedAt: r.UpdatedAt.Time,
		})
	}
	return out, nil
}

func (s *AdminService) GetProduct(ctx context.Context, id int64) (AdminProduct, error) {
	p, err := s.q.AdminGetProduct(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminProduct{}, ErrNotFound
	}
	if err != nil {
		return AdminProduct{}, fmt.Errorf("get product: %w", err)
	}
	out := AdminProduct{
		ID: p.ID, Slug: p.Slug, Name: p.Name, Description: p.Description, CategoryID: p.CategoryID,
		Tier: Tier(p.Tier), Status: ProductStatus(p.Status), MaterialLine: p.MaterialLine, Badges: nonNil(p.Badges),
		GiftBoxAllowed: p.GiftBoxAllowed, PreorderAllowed: p.PreorderAllowed, UpdatedAt: p.UpdatedAt.Time,
		Fields: []Field{}, Details: []DetailInput{}, Variants: []AdminVariant{}, Images: []AdminImage{},
	}

	fields, err := s.q.ListFieldsForCategory(ctx, p.CategoryID)
	if err != nil {
		return AdminProduct{}, err
	}
	for _, f := range fields {
		out.Fields = append(out.Fields, Field{ID: f.ID, Label: f.Label})
	}
	details, err := s.q.AdminListProductDetails(ctx, id)
	if err != nil {
		return AdminProduct{}, err
	}
	for _, d := range details {
		out.Details = append(out.Details, DetailInput{FieldID: d.FieldID, Value: d.Value})
	}
	variants, err := s.q.AdminListVariants(ctx, id)
	if err != nil {
		return AdminProduct{}, err
	}
	for _, v := range variants {
		out.Variants = append(out.Variants, AdminVariant{ID: v.ID, VariantInput: VariantInput{
			ColourID: nullInt(v.ColourID), SizeID: nullInt(v.SizeID), SKU: v.Sku, PriceKobo: v.PriceKobo,
			CompareAtKobo: nullInt(v.CompareAtKobo), Stock: int(v.Stock), Active: v.Active,
		}})
	}
	images, err := s.q.AdminListImages(ctx, id)
	if err != nil {
		return AdminProduct{}, err
	}
	for _, im := range images {
		out.Images = append(out.Images, AdminImage{ID: im.ID, ColourID: nullInt(im.ColourID), Key: im.StorageKey,
			Kind: im.Kind, Width: int(im.Width), Warnings: nonNil(im.Warnings)})
	}
	return out, nil
}

// CreateProduct saves a new product and returns its id.
func (s *AdminService) CreateProduct(ctx context.Context, by staff.Member, in ProductInput) (int64, error) {
	if err := s.validateProduct(ctx, &in); err != nil {
		return 0, err
	}
	var id int64
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		slug, err := uniqueSlug(ctx, q.SlugExists, slugify(in.Name))
		if err != nil {
			return err
		}
		id, err = q.InsertProduct(ctx, sqlc.InsertProductParams{
			Slug: slug, Name: in.Name, Description: in.Description, CategoryID: in.CategoryID,
			Tier: sqlc.ProductTier(in.Tier), MaterialLine: in.MaterialLine, Status: sqlc.ProductStatus(in.Status),
			Badges: in.Badges, GiftBoxAllowed: in.GiftBoxAllowed, PreorderAllowed: in.PreorderAllowed,
		})
		if err != nil {
			return err
		}
		if err := saveDetailsAndVariants(ctx, q, id, in); err != nil {
			return err
		}
		return staff.Audit(ctx, q, by, "product.create", "product", id, map[string]any{"name": in.Name, "status": in.Status})
	})
	return id, friendly(err)
}

// UpdateProduct saves the product editor. Options left out are switched off, not deleted.
func (s *AdminService) UpdateProduct(ctx context.Context, by staff.Member, id int64, in ProductInput) error {
	if err := s.validateProduct(ctx, &in); err != nil {
		return err
	}
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		if _, err := q.AdminGetProduct(ctx, id); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		err := q.UpdateProduct(ctx, sqlc.UpdateProductParams{
			ID: id, Name: in.Name, Description: in.Description, CategoryID: in.CategoryID,
			Tier: sqlc.ProductTier(in.Tier), MaterialLine: in.MaterialLine, Status: sqlc.ProductStatus(in.Status),
			Badges: in.Badges, GiftBoxAllowed: in.GiftBoxAllowed, PreorderAllowed: in.PreorderAllowed,
		})
		if err != nil {
			return err
		}
		if err := saveDetailsAndVariants(ctx, q, id, in); err != nil {
			return err
		}
		return staff.Audit(ctx, q, by, "product.update", "product", id, map[string]any{
			"name": in.Name, "status": in.Status, "options": len(in.Variants),
		})
	})
	return friendly(err)
}

func saveDetailsAndVariants(ctx context.Context, q *sqlc.Queries, productID int64, in ProductInput) error {
	if err := q.DeleteProductDetails(ctx, productID); err != nil {
		return err
	}
	for _, d := range in.Details {
		if err := q.InsertProductDetail(ctx, sqlc.InsertProductDetailParams{ProductID: productID, FieldID: d.FieldID, Value: d.Value}); err != nil {
			return err
		}
	}

	keep := make([]int64, 0, len(in.Variants))
	for i, v := range in.Variants {
		if taken, err := q.SkuTakenByOtherProduct(ctx, sqlc.SkuTakenByOtherProductParams{Sku: v.SKU, ProductID: productID}); err != nil {
			return err
		} else if taken {
			return invalid(fmt.Sprintf("variants.%d.sku", i), fmt.Sprintf("SKU %s is already used by another product.", v.SKU))
		}
		vid, err := q.UpsertVariant(ctx, sqlc.UpsertVariantParams{
			ProductID: productID, ColourID: pgInt(v.ColourID), SizeID: pgInt(v.SizeID), Sku: v.SKU,
			PriceKobo: v.PriceKobo, CompareAtKobo: pgInt(v.CompareAtKobo), Stock: int32(v.Stock), Active: v.Active,
			Position: int32(i),
		})
		if err != nil {
			return err
		}
		keep = append(keep, vid)
	}
	return q.DeactivateOtherVariants(ctx, sqlc.DeactivateOtherVariantsParams{ProductID: productID, KeepIds: keep})
}

var allowedBadges = map[string]bool{"New": true, "Bestseller": true, "Handmade": true}

// validateProduct checks and tidies the editor's input. Messages are shown to staff as written.
func (s *AdminService) validateProduct(ctx context.Context, in *ProductInput) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.MaterialLine = strings.TrimSpace(in.MaterialLine)

	if in.Name == "" {
		return invalid("name", "Give the product a name.")
	}
	if utf8.RuneCountInString(in.Name) > 120 {
		return invalid("name", "Keep the name under 120 characters.")
	}
	if utf8.RuneCountInString(in.MaterialLine) > 80 {
		return invalid("materialLine", "Keep the line under the name under 80 characters.")
	}
	if in.Tier != TierEveryday && in.Tier != TierPremium {
		return invalid("tier", "Choose Everyday or Premium.")
	}
	if in.Status != StatusDraft && in.Status != StatusVisible && in.Status != StatusHidden {
		return invalid("status", "Choose draft, visible or hidden.")
	}
	ok, err := s.q.CategoryExists(ctx, in.CategoryID)
	if err != nil {
		return err
	}
	if !ok {
		return invalid("categoryId", "Choose a category.")
	}

	badges := []string{}
	for _, b := range in.Badges {
		if !allowedBadges[b] {
			return invalid("badges", fmt.Sprintf("Unknown badge %q.", b))
		}
		badges = append(badges, b)
	}
	in.Badges = badges

	// Details must belong to this category's fields; blank ones are dropped.
	fields, err := s.q.ListFieldsForCategory(ctx, in.CategoryID)
	if err != nil {
		return err
	}
	allowed := map[int64]bool{}
	for _, f := range fields {
		allowed[f.ID] = true
	}
	details := []DetailInput{}
	for _, d := range in.Details {
		d.Value = strings.TrimSpace(d.Value)
		if d.Value == "" {
			continue
		}
		if !allowed[d.FieldID] {
			return invalid("details", "A detail doesn't belong to this category. Reload the editor and try again.")
		}
		details = append(details, d)
	}
	in.Details = details

	seen := map[string]bool{}
	skus := map[string]bool{}
	active := 0
	for i := range in.Variants {
		v := &in.Variants[i]
		field := fmt.Sprintf("variants.%d", i)
		v.SKU = strings.ToUpper(strings.TrimSpace(v.SKU))
		combo := idKey(v.ColourID) + "/" + idKey(v.SizeID)
		if seen[combo] {
			return invalid(field, "The same colour and size appears twice.")
		}
		seen[combo] = true
		if v.SKU == "" || len(v.SKU) > 40 || !skuPattern.MatchString(v.SKU) {
			return invalid(field+".sku", "SKU must be letters, numbers and dashes, up to 40 characters.")
		}
		if skus[v.SKU] {
			return invalid(field+".sku", fmt.Sprintf("SKU %s is used twice.", v.SKU))
		}
		skus[v.SKU] = true
		if v.PriceKobo <= 0 || v.PriceKobo > 100_000_000*100 {
			return invalid(field+".price", "Enter a price above ₦0.")
		}
		if v.CompareAtKobo != nil && *v.CompareAtKobo <= v.PriceKobo {
			return invalid(field+".compareAt", "The \"was\" price must be higher than the price.")
		}
		if v.Stock < 0 || v.Stock > 100_000 {
			return invalid(field+".stock", "Stock must be between 0 and 100,000.")
		}
		if v.Active {
			active++
		}
	}
	if in.Status == StatusVisible && active == 0 {
		return invalid("variants", "A visible product needs at least one option switched on.")
	}
	return nil
}

var skuPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]*$`)

// ---------- Photos ----------

// AddImage processes an uploaded photo, stores its sizes and adds it to the product.
func (s *AdminService) AddImage(ctx context.Context, by staff.Member, productID int64, colourID *int64, kind string, photo *media.Processed) (AdminImage, error) {
	if kind != "studio" && kind != "real_life" {
		kind = "studio"
	}
	if _, err := s.q.AdminGetProduct(ctx, productID); errors.Is(err, pgx.ErrNoRows) {
		return AdminImage{}, ErrNotFound
	} else if err != nil {
		return AdminImage{}, err
	}

	key := fmt.Sprintf("products/%d/%s", productID, randomHex(6))
	for w, data := range photo.Files {
		if err := s.storage.Put(ctx, imageFile(key, w), bytes.NewReader(data)); err != nil {
			return AdminImage{}, fmt.Errorf("store photo: %w", err)
		}
	}

	var id int64
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		var err error
		id, err = q.InsertImage(ctx, sqlc.InsertImageParams{
			ProductID: productID, ColourID: pgInt(colourID), StorageKey: key, Kind: kind,
			Width: int32(photo.Side), Height: int32(photo.Side), Warnings: photo.Warnings,
		})
		if err != nil {
			return err
		}
		if err := q.TouchProduct(ctx, productID); err != nil {
			return err
		}
		return staff.Audit(ctx, q, by, "product.photo.add", "product", productID, map[string]any{"image": id, "warnings": photo.Warnings})
	})
	if err != nil {
		s.deleteFiles(ctx, key)
		return AdminImage{}, friendly(err)
	}
	return AdminImage{ID: id, ColourID: colourID, Key: key, Kind: kind, Width: photo.Side, Warnings: photo.Warnings}, nil
}

func (s *AdminService) DeleteImage(ctx context.Context, by staff.Member, productID, imageID int64) error {
	var key string
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		var err error
		key, err = q.DeleteImage(ctx, sqlc.DeleteImageParams{ID: imageID, ProductID: productID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := q.TouchProduct(ctx, productID); err != nil {
			return err
		}
		return staff.Audit(ctx, q, by, "product.photo.delete", "product", productID, map[string]any{"image": imageID})
	})
	if err != nil {
		return err
	}
	s.deleteFiles(ctx, key) // after the commit, so a failed delete never leaves a row without files
	return nil
}

// MakeImageMain moves a photo to the front: it becomes the one shown in the shop grid.
func (s *AdminService) MakeImageMain(ctx context.Context, by staff.Member, productID, imageID int64) error {
	return s.inTx(ctx, func(q *sqlc.Queries) error {
		if err := q.MoveImageFirst(ctx, sqlc.MoveImageFirstParams{ImageID: imageID, ProductID: productID}); err != nil {
			return err
		}
		if err := q.TouchProduct(ctx, productID); err != nil {
			return err
		}
		return staff.Audit(ctx, q, by, "product.photo.main", "product", productID, map[string]any{"image": imageID})
	})
}

func (s *AdminService) deleteFiles(ctx context.Context, key string) {
	for _, w := range media.Sizes {
		_ = s.storage.Delete(ctx, imageFile(key, w))
	}
}

// imageFile is the stored file for one size, e.g. "products/12/ab12cd-800.jpg".
func imageFile(key string, width int) string { return key + "-" + strconv.Itoa(width) + ".jpg" }

// ---------- Catalogue setup ----------

func (s *AdminService) Setup(ctx context.Context) (Setup, error) {
	out := Setup{Categories: []SetupCategory{}, SizeSets: []SetupSizeSet{}, Colours: []SetupColour{}}

	fields, err := s.q.AdminListCategoryFields(ctx)
	if err != nil {
		return out, err
	}
	fieldsByCat := map[int64][]Field{}
	for _, f := range fields {
		fieldsByCat[f.CategoryID] = append(fieldsByCat[f.CategoryID], Field{ID: f.ID, Label: f.Label})
	}

	cats, err := s.q.AdminListCategories(ctx)
	if err != nil {
		return out, err
	}
	index := map[int64]int{}
	for _, c := range cats {
		sc := SetupCategory{
			ID: c.ID, ParentID: nullInt(c.ParentID), Name: c.Name, Slug: c.Slug,
			DefaultSizeSetID: nullInt(c.DefaultSizeSetID), ReturnRule: c.ReturnRule, ShowOnHome: c.ShowOnHome,
			Position: int(c.Position), Fields: []Field{}, Children: []SetupCategory{},
		}
		if !c.ParentID.Valid {
			if f := fieldsByCat[c.ID]; f != nil {
				sc.Fields = f
			}
			index[c.ID] = len(out.Categories)
			out.Categories = append(out.Categories, sc)
		} else if i, ok := index[c.ParentID.Int64]; ok {
			out.Categories[i].Children = append(out.Categories[i].Children, sc)
		}
	}

	sets, err := s.q.AdminListSizeSets(ctx)
	if err != nil {
		return out, err
	}
	sizes, err := s.q.AdminListSizes(ctx)
	if err != nil {
		return out, err
	}
	sizesBySet := map[int64][]SetupSize{}
	for _, sz := range sizes {
		sizesBySet[sz.SizeSetID] = append(sizesBySet[sz.SizeSetID], SetupSize{ID: sz.ID, Label: sz.Label, Measurement: sz.Measurement, UsedBy: int(sz.UsedBy)})
	}
	for _, set := range sets {
		ss := SetupSizeSet{ID: set.ID, Name: set.Name, MeasureLabel: set.MeasureLabel, Sizes: sizesBySet[set.ID]}
		if ss.Sizes == nil {
			ss.Sizes = []SetupSize{}
		}
		out.SizeSets = append(out.SizeSets, ss)
	}

	colours, err := s.q.AdminListColours(ctx)
	if err != nil {
		return out, err
	}
	for _, c := range colours {
		out.Colours = append(out.Colours, SetupColour{ID: c.ID, Name: c.Name, Hex: c.Hex, UsedBy: int(c.UsedBy)})
	}

	bands, err := s.q.ListPriceBands(ctx)
	if err != nil {
		return out, err
	}
	out.PriceBands = make([]PriceBand, 0, len(bands))
	for _, b := range bands {
		out.PriceBands = append(out.PriceBands, PriceBand{ID: b.ID, Label: b.Label, MinKobo: b.MinKobo, MaxKobo: nullInt(b.MaxKobo)})
	}
	return out, nil
}

// SaveSizeSet creates (id 0) or replaces a size set and its sizes, in the order given.
// Removing a size that products still use is refused.
func (s *AdminService) SaveSizeSet(ctx context.Context, by staff.Member, id int64, in SizeSetInput) (int64, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.MeasureLabel = strings.TrimSpace(in.MeasureLabel)
	if in.Name == "" {
		return 0, invalid("name", "Give the size set a name.")
	}
	if len(in.Sizes) == 0 {
		return 0, invalid("sizes", "Add at least one size.")
	}
	labels := map[string]bool{}
	for i := range in.Sizes {
		in.Sizes[i].Label = strings.TrimSpace(in.Sizes[i].Label)
		in.Sizes[i].Measurement = strings.TrimSpace(in.Sizes[i].Measurement)
		l := in.Sizes[i].Label
		if l == "" {
			return 0, invalid(fmt.Sprintf("sizes.%d.label", i), "Every size needs a label, e.g. M or 7.")
		}
		if labels[strings.ToLower(l)] {
			return 0, invalid(fmt.Sprintf("sizes.%d.label", i), fmt.Sprintf("Size %s is listed twice.", l))
		}
		labels[strings.ToLower(l)] = true
	}

	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		var err error
		if id == 0 {
			id, err = q.InsertSizeSet(ctx, sqlc.InsertSizeSetParams{Name: in.Name, MeasureLabel: in.MeasureLabel})
		} else {
			err = q.UpdateSizeSet(ctx, sqlc.UpdateSizeSetParams{ID: id, Name: in.Name, MeasureLabel: in.MeasureLabel})
		}
		if err != nil {
			return err
		}
		// Rename existing sizes out of the way first, so swapping two labels doesn't hit the unique rule.
		for _, sz := range in.Sizes {
			if sz.ID != 0 {
				if err := q.UpdateSize(ctx, sqlc.UpdateSizeParams{ID: sz.ID, SizeSetID: id, Label: "~" + strconv.FormatInt(sz.ID, 10), Measurement: sz.Measurement, Position: 0}); err != nil {
					return err
				}
			}
		}
		keep := []int64{}
		for pos, sz := range in.Sizes {
			if sz.ID != 0 {
				if err := q.UpdateSize(ctx, sqlc.UpdateSizeParams{ID: sz.ID, SizeSetID: id, Label: sz.Label, Measurement: sz.Measurement, Position: int32(pos + 1)}); err != nil {
					return err
				}
				keep = append(keep, sz.ID)
				continue
			}
			newID, err := q.InsertSize(ctx, sqlc.InsertSizeParams{SizeSetID: id, Label: sz.Label, Measurement: sz.Measurement, Position: int32(pos + 1)})
			if err != nil {
				return err
			}
			keep = append(keep, newID)
		}
		if err := q.DeleteOtherSizes(ctx, sqlc.DeleteOtherSizesParams{SizeSetID: id, KeepIds: keep}); err != nil {
			return inUse(err, "A size you removed is still used by product options. Switch those options off first.")
		}
		return staff.Audit(ctx, q, by, "size_set.save", "size_set", id, in)
	})
	return id, friendly(err)
}

var hexPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func (s *AdminService) SaveColour(ctx context.Context, by staff.Member, id int64, in ColourInput) (int64, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Hex = strings.ToUpper(strings.TrimSpace(in.Hex))
	if in.Name == "" {
		return 0, invalid("name", "Give the colour a name.")
	}
	if !hexPattern.MatchString(in.Hex) {
		return 0, invalid("hex", "Pick a colour for the swatch.")
	}
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		var err error
		if id == 0 {
			id, err = q.InsertColour(ctx, sqlc.InsertColourParams{Name: in.Name, Hex: in.Hex})
		} else {
			err = q.UpdateColour(ctx, sqlc.UpdateColourParams{ID: id, Name: in.Name, Hex: in.Hex})
		}
		if err != nil {
			return err
		}
		return staff.Audit(ctx, q, by, "colour.save", "colour", id, in)
	})
	return id, friendly(err)
}

func (s *AdminService) DeleteColour(ctx context.Context, by staff.Member, id int64) error {
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		if err := q.DeleteColour(ctx, id); err != nil {
			return inUse(err, "Products still use this colour. Remove it from those products first.")
		}
		return staff.Audit(ctx, q, by, "colour.delete", "colour", id, nil)
	})
	return friendly(err)
}

// SaveCategory creates (id 0) or edits a category. New categories go under in.ParentID, or at the top.
func (s *AdminService) SaveCategory(ctx context.Context, by staff.Member, id int64, in CategoryInput) (int64, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.ReturnRule = strings.TrimSpace(in.ReturnRule)
	if in.Name == "" {
		return 0, invalid("name", "Give the category a name.")
	}
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		var err error
		if id == 0 {
			slug, err := uniqueSlug(ctx, q.CategorySlugExists, slugify(in.Name))
			if err != nil {
				return err
			}
			id, err = q.InsertCategory(ctx, sqlc.InsertCategoryParams{
				ParentID: pgInt(in.ParentID), Name: in.Name, Slug: slug,
				DefaultSizeSetID: pgInt(in.DefaultSizeSetID), ReturnRule: in.ReturnRule,
			})
			if err != nil {
				return err
			}
		} else {
			err = q.UpdateCategory(ctx, sqlc.UpdateCategoryParams{
				ID: id, Name: in.Name, DefaultSizeSetID: pgInt(in.DefaultSizeSetID), ReturnRule: in.ReturnRule,
				ShowOnHome: in.ShowOnHome, Position: int32(in.Position),
			})
			if err != nil {
				return err
			}
		}
		return staff.Audit(ctx, q, by, "category.save", "category", id, in)
	})
	return id, friendly(err)
}

// SaveCategoryFields replaces a top category's product detail fields, in order.
// Removing a field also removes what products had filled in for it.
func (s *AdminService) SaveCategoryFields(ctx context.Context, by staff.Member, categoryID int64, fields []FieldInput) error {
	seen := map[string]bool{}
	for i := range fields {
		fields[i].Label = strings.TrimSpace(fields[i].Label)
		l := strings.ToLower(fields[i].Label)
		if l == "" {
			return invalid(fmt.Sprintf("fields.%d", i), "Every field needs a label.")
		}
		if seen[l] {
			return invalid(fmt.Sprintf("fields.%d", i), fmt.Sprintf("%s is listed twice.", fields[i].Label))
		}
		seen[l] = true
	}
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		keep := []int64{}
		for pos, f := range fields {
			if f.ID != 0 {
				if err := q.RenameCategoryField(ctx, sqlc.RenameCategoryFieldParams{ID: f.ID, CategoryID: categoryID, Label: "~" + strconv.FormatInt(f.ID, 10), Position: int32(pos + 1)}); err != nil {
					return err
				}
			}
		}
		for pos, f := range fields {
			if f.ID != 0 {
				if err := q.RenameCategoryField(ctx, sqlc.RenameCategoryFieldParams{ID: f.ID, CategoryID: categoryID, Label: f.Label, Position: int32(pos + 1)}); err != nil {
					return err
				}
				keep = append(keep, f.ID)
				continue
			}
			id, err := q.UpsertCategoryField(ctx, sqlc.UpsertCategoryFieldParams{CategoryID: categoryID, Label: f.Label, Position: int32(pos + 1)})
			if err != nil {
				return err
			}
			keep = append(keep, id)
		}
		if err := q.DeleteOtherCategoryFields(ctx, sqlc.DeleteOtherCategoryFieldsParams{CategoryID: categoryID, KeepIds: keep}); err != nil {
			return err
		}
		return staff.Audit(ctx, q, by, "category.fields", "category", categoryID, fields)
	})
	return friendly(err)
}

// ---------- helpers ----------

// friendly turns database rule violations into errors the API can explain.
func friendly(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return fmt.Errorf("%w: %s", ErrConflict, pgErr.ConstraintName)
		case "23503": // foreign_key_violation
			return fmt.Errorf("%w: %s", ErrInUse, pgErr.ConstraintName)
		}
	}
	return err
}

// inUse replaces a foreign-key error with a message staff understand.
func inUse(err error, msg string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return fmt.Errorf("%w: %s", ErrInUse, msg)
	}
	return err
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer("’", "", "'", "", "&", " and ").Replace(s)
	s = strings.Trim(nonSlug.ReplaceAllString(s, "-"), "-")
	if s == "" {
		s = "item"
	}
	if len(s) > 80 {
		s = strings.Trim(s[:80], "-")
	}
	return s
}

// uniqueSlug adds -2, -3… until the slug is free.
func uniqueSlug(ctx context.Context, exists func(context.Context, string) (bool, error), base string) (string, error) {
	slug := base
	for i := 2; ; i++ {
		taken, err := exists(ctx, slug)
		if err != nil {
			return "", err
		}
		if !taken {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

func pgInt(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func idKey(v *int64) string {
	if v == nil {
		return "-"
	}
	return strconv.FormatInt(*v, 10)
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
