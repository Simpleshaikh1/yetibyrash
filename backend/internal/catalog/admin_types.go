package catalog

import (
	"errors"
	"time"
)

// ValidationError is a problem with one field of what staff typed. The admin shows Message next to Field.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }

func invalid(field, msg string) error { return &ValidationError{Field: field, Message: msg} }

var (
	// ErrInUse: something can't be removed because products still use it.
	ErrInUse = errors.New("still in use")
	// ErrConflict: a name, SKU or similar is already taken.
	ErrConflict = errors.New("already exists")
)

type ProductStatus string

const (
	StatusDraft   ProductStatus = "draft"
	StatusVisible ProductStatus = "visible"
	StatusHidden  ProductStatus = "hidden"
)

// ---------- Products ----------

type AdminProductFilter struct {
	Search     string
	Status     ProductStatus
	CategoryID int64
}

type AdminProductRow struct {
	ID           int64         `json:"id"`
	Slug         string        `json:"slug"`
	Name         string        `json:"name"`
	Tier         Tier          `json:"tier"`
	Status       ProductStatus `json:"status"`
	Category     string        `json:"category"` // "Clothing › Joggers"
	MinPriceKobo int64         `json:"minPriceKobo"`
	MaxPriceKobo int64         `json:"maxPriceKobo"`
	TotalStock   int           `json:"totalStock"`
	OptionCount  int           `json:"optionCount"`
	SoldOutCount int           `json:"soldOutCount"`
	Image        string        `json:"image"` // photo key, "" if none
	UpdatedAt    time.Time     `json:"updatedAt"`
}

type Field struct {
	ID    int64  `json:"id"`
	Label string `json:"label"`
}

type DetailInput struct {
	FieldID int64  `json:"fieldId"`
	Value   string `json:"value"`
}

type VariantInput struct {
	ColourID      *int64 `json:"colourId"`
	SizeID        *int64 `json:"sizeId"`
	SKU           string `json:"sku"`
	PriceKobo     int64  `json:"priceKobo"`
	CompareAtKobo *int64 `json:"compareAtKobo"`
	Stock         int    `json:"stock"`
	Active        bool   `json:"active"`
}

// ProductInput is what the product editor sends when saving.
type ProductInput struct {
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	CategoryID      int64          `json:"categoryId"`
	Tier            Tier           `json:"tier"`
	Status          ProductStatus  `json:"status"`
	MaterialLine    string         `json:"materialLine"`
	Badges          []string       `json:"badges"`
	GiftBoxAllowed  bool           `json:"giftBoxAllowed"`
	PreorderAllowed bool           `json:"preorderAllowed"`
	Details         []DetailInput  `json:"details"`
	Variants        []VariantInput `json:"variants"`
}

type AdminVariant struct {
	ID int64 `json:"id"`
	VariantInput
}

type AdminImage struct {
	ID       int64    `json:"id"`
	ColourID *int64   `json:"colourId"`
	Key      string   `json:"key"`
	Kind     string   `json:"kind"`
	Width    int      `json:"width"`
	Warnings []string `json:"warnings"`
}

// AdminProduct is everything the product editor loads.
type AdminProduct struct {
	ID              int64          `json:"id"`
	Slug            string         `json:"slug"`
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	CategoryID      int64          `json:"categoryId"`
	Tier            Tier           `json:"tier"`
	Status          ProductStatus  `json:"status"`
	MaterialLine    string         `json:"materialLine"`
	Badges          []string       `json:"badges"`
	GiftBoxAllowed  bool           `json:"giftBoxAllowed"`
	PreorderAllowed bool           `json:"preorderAllowed"`
	UpdatedAt       time.Time      `json:"updatedAt"`
	Fields          []Field        `json:"fields"`
	Details         []DetailInput  `json:"details"`
	Variants        []AdminVariant `json:"variants"`
	Images          []AdminImage   `json:"images"`
}

// ---------- Catalogue setup ----------

type SetupSize struct {
	ID          int64  `json:"id"`
	Label       string `json:"label"`
	Measurement string `json:"measurement"`
	UsedBy      int    `json:"usedBy"` // product options using this size
}

type SetupSizeSet struct {
	ID           int64       `json:"id"`
	Name         string      `json:"name"`
	MeasureLabel string      `json:"measureLabel"`
	Sizes        []SetupSize `json:"sizes"`
}

type SetupColour struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Hex    string `json:"hex"`
	UsedBy int    `json:"usedBy"` // products using this colour
}

type SetupCategory struct {
	ID               int64           `json:"id"`
	ParentID         *int64          `json:"parentId"`
	Name             string          `json:"name"`
	Slug             string          `json:"slug"`
	DefaultSizeSetID *int64          `json:"defaultSizeSetId"`
	ReturnRule       string          `json:"returnRule"`
	ShowOnHome       bool            `json:"showOnHome"`
	Position         int             `json:"position"`
	Fields           []Field         `json:"fields"` // top categories only
	Children         []SetupCategory `json:"children"`
}

// Setup is everything the Catalogue setup screen and the product editor's dropdowns need.
type Setup struct {
	Categories []SetupCategory `json:"categories"`
	SizeSets   []SetupSizeSet  `json:"sizeSets"`
	Colours    []SetupColour   `json:"colours"`
	PriceBands []PriceBand     `json:"priceBands"`
}

type SizeInput struct {
	ID          int64  `json:"id"` // 0 = new size
	Label       string `json:"label"`
	Measurement string `json:"measurement"`
}

type SizeSetInput struct {
	Name         string      `json:"name"`
	MeasureLabel string      `json:"measureLabel"`
	Sizes        []SizeInput `json:"sizes"`
}

type ColourInput struct {
	Name string `json:"name"`
	Hex  string `json:"hex"`
}

type CategoryInput struct {
	ParentID         *int64 `json:"parentId"` // only used when creating
	Name             string `json:"name"`
	DefaultSizeSetID *int64 `json:"defaultSizeSetId"`
	ReturnRule       string `json:"returnRule"`
	ShowOnHome       bool   `json:"showOnHome"`
	Position         int    `json:"position"`
}

type FieldInput struct {
	ID    int64  `json:"id"` // 0 = new field
	Label string `json:"label"`
}
