// Package catalog serves categories and products to the shop.
// Money is always in kobo (₦1 = 100 kobo); the frontend formats it.
package catalog

import (
	"errors"
	"time"
)

// ErrNotFound means the product or category doesn't exist or isn't visible.
var ErrNotFound = errors.New("not found")

type Tier string

const (
	TierEveryday Tier = "everyday"
	TierPremium  Tier = "premium"
)

// ParseTier accepts "", "everyday" or "premium". "" means both tiers.
func ParseTier(s string) (Tier, bool) {
	switch Tier(s) {
	case "", TierEveryday, TierPremium:
		return Tier(s), true
	}
	return "", false
}

// Sort orders for the shop grid.
const (
	SortNewest    = "newest"
	SortPriceAsc  = "price_asc"
	SortPriceDesc = "price_desc"
)

func ValidSort(s string) bool {
	return s == SortNewest || s == SortPriceAsc || s == SortPriceDesc
}

// PageSize is how many products one page of the shop grid shows.
const PageSize = 24

type Category struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Slug       string     `json:"slug"`
	ShowOnHome bool       `json:"showOnHome"`
	Children   []Category `json:"children"`
}

// ProductCard is one product in a grid.
type ProductCard struct {
	ID           int64    `json:"id"`
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Tier         Tier     `json:"tier"`
	MaterialLine string   `json:"materialLine"`
	CategoryName string   `json:"categoryName"`
	Badges       []string `json:"badges"`
	// PriceKobo is the lowest option price; MaxPriceKobo the highest (they differ when sizes cost more).
	PriceKobo     int64  `json:"priceKobo"`
	MaxPriceKobo  int64  `json:"maxPriceKobo"`
	CompareAtKobo *int64 `json:"compareAtKobo"` // old price when on sale, else null
	SaleBadge     string `json:"saleBadge"`     // a running sale's badge, e.g. "20% OFF"
	Rating        Rating `json:"rating"`        // published reviews
	ColourCount   int    `json:"colourCount"`
	InStock       bool   `json:"inStock"`
	Preorder      bool   `json:"preorder"`
	Image         string `json:"image"` // main photo key, "" if none; files are /media/<key>-<400|800|1600>.jpg
}

type ProductFilter struct {
	CategorySlug string
	Tier         Tier
	Colour       string // colour name, e.g. "Espresso"
	Size         string // size label, e.g. "M"
	BandID       int64  // price band id, 0 = any price
	Sort         string
	Page         int // starts at 1
}

type Colour struct {
	Name string `json:"name"`
	Hex  string `json:"hex"`
}

type PriceBand struct {
	ID      int64  `json:"id"`
	Label   string `json:"label"`
	MinKobo int64  `json:"minKobo"`
	MaxKobo *int64 `json:"maxKobo"`
}

// FilterOptions are the choices the shop's filter sheet offers for the current category.
type FilterOptions struct {
	Colours    []Colour    `json:"colours"`
	Sizes      []string    `json:"sizes"`
	PriceBands []PriceBand `json:"priceBands"`
}

type ProductList struct {
	Items    []ProductCard `json:"items"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
	Filters  FilterOptions `json:"filters"`
}

type CategoryRef struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type Size struct {
	Label       string `json:"label"`
	Measurement string `json:"measurement"`
}

type SizeSet struct {
	Name         string `json:"name"`
	MeasureLabel string `json:"measureLabel"`
	Sizes        []Size `json:"sizes"`
}

// Variant is one buyable option. Colour and Size are "" when the product doesn't vary by them.
type Variant struct {
	ID            int64  `json:"id"`
	SKU           string `json:"sku"`
	Colour        string `json:"colour"`
	Size          string `json:"size"`
	PriceKobo     int64  `json:"priceKobo"`
	CompareAtKobo *int64 `json:"compareAtKobo"`
	Stock         int    `json:"stock"`
}

type Detail struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type Image struct {
	ID       int64  `json:"id"`
	ColourID *int64 `json:"colourId"`
	Key      string `json:"key"`
	Kind     string `json:"kind"`
}

type ProductDetail struct {
	ID              int64         `json:"id"`
	Slug            string        `json:"slug"`
	Name            string        `json:"name"`
	Description     string        `json:"description"`
	Tier            Tier          `json:"tier"`
	MaterialLine    string        `json:"materialLine"`
	Badges          []string      `json:"badges"`
	SaleBadge       string        `json:"saleBadge"`
	GiftBoxAllowed  bool          `json:"giftBoxAllowed"`
	PreorderAllowed bool          `json:"preorderAllowed"`
	Category        CategoryRef   `json:"category"`
	Parent          *CategoryRef  `json:"parent"`
	ReturnRule      string        `json:"returnRule"`
	Colours         []Colour      `json:"colours"` // in the order the shop shows them
	SizeSet         *SizeSet      `json:"sizeSet"` // null for one-size products
	Variants        []Variant     `json:"variants"`
	Details         []Detail      `json:"details"`
	Images          []Image       `json:"images"`
	Pairings        []ProductCard `json:"pairings"`
	// LowStockThreshold: show "Only X left" when a variant's stock is at or below this.
	LowStockThreshold int      `json:"lowStockThreshold"`
	Rating            Rating   `json:"rating"`
	Reviews           []Review `json:"reviews"` // newest published, up to 20
}

// Rating is a product's published reviews in short.
type Rating struct {
	Average float64 `json:"average"` // 0 when there are none
	Count   int     `json:"count"`
}

// Review is a published review on the product page.
type Review struct {
	ID          int64     `json:"id"`
	Rating      int       `json:"rating"`
	Text        string    `json:"text"`
	Photos      []string  `json:"photos"` // photo keys: /media/<key>-<400|800|1600>.jpg
	Reply       string    `json:"reply"`  // the shop's public reply
	Author      string    `json:"author"` // "Chioma E."
	Area        string    `json:"area"`   // "Enugu"
	Option      string    `json:"option"` // "M / Espresso"
	PublishedAt time.Time `json:"publishedAt"`
}

// HomeReview is one quote in the homepage reviews strip.
type HomeReview struct {
	ID          int64  `json:"id"`
	Text        string `json:"text"`
	Author      string `json:"author"`
	Area        string `json:"area"`
	Product     string `json:"product"`
	ProductSlug string `json:"productSlug"`
}

// Home is the homepage for one tier. Sections lists the visible blocks in the order the
// admin set; the shop draws each block from the matching field below.
type Home struct {
	Tagline     string        `json:"tagline"`
	Sections    []HomeSection `json:"sections"`
	Hero        *Banner       `json:"hero"` // null = show the plain wordmark hero
	Categories  []Category    `json:"categories"`
	NewIn       []ProductCard `json:"newIn"`
	BestSellers []ProductCard `json:"bestSellers"`
	Budgets     []PriceBand   `json:"budgets"` // gift shop by budget
	Reviews     []HomeReview  `json:"reviews"` // 5-star quotes
}

// HomeSection is one homepage block: hero, categories, new_in, best_sellers, gift_budget or trust.
type HomeSection struct {
	Key   string   `json:"key"`
	Title string   `json:"title"`
	Lines []string `json:"lines"` // trust strip only
}

// Banner is a hero banner. Image files: /media/<image>-phone.jpg (4:5) and -wide.jpg (16:7).
type Banner struct {
	Eyebrow    string `json:"eyebrow"`
	Headline   string `json:"headline"`
	ButtonText string `json:"buttonText"`
	ButtonLink string `json:"buttonLink"`
	Image      string `json:"image"`
}
