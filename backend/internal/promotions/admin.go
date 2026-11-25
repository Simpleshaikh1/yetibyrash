package promotions

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/db/sqlc"
	"yetibyrash/backend/internal/staff"
)

// Service is the admin's Promotions screen.
type Service struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool, q: sqlc.New(pool)} }

func (s *Service) inTx(ctx context.Context, fn func(q *sqlc.Queries) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return fn(s.q.WithTx(tx)) })
}

type AdminSale struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`  // percent | fixed
	Value         int64  `json:"value"` // percent, or kobo off each item
	AppliesTo     string `json:"appliesTo"`
	Tier          string `json:"tier"`
	CategoryID    int64  `json:"categoryId"`
	CategoryName  string `json:"categoryName"`
	StartsOn      string `json:"startsOn"`
	EndsOn        string `json:"endsOn"` // last day of the sale
	Badge         string `json:"badge"`
	Announcement  string `json:"announcement"`
	ExcludeOnSale bool   `json:"excludeOnSale"`
	Active        bool   `json:"active"`
	State         string `json:"state"` // live | scheduled | ended | off
	EndsInDays    int    `json:"endsInDays"`
	Orders        int    `json:"orders"`
	SalesKobo     int64  `json:"salesKobo"`
}

type AdminCode struct {
	ID             int64  `json:"id"`
	Code           string `json:"code"`
	Kind           string `json:"kind"` // percent | fixed | free_delivery
	Value          int64  `json:"value"`
	MinOrderKobo   int64  `json:"minOrderKobo"`
	MaxUses        *int   `json:"maxUses"` // null = unlimited
	Used           int    `json:"used"`
	FirstOrderOnly bool   `json:"firstOrderOnly"`
	ZoneID         int64  `json:"zoneId"` // free delivery only in this zone; 0 = anywhere
	ZoneName       string `json:"zoneName"`
	StartsOn       string `json:"startsOn"`
	EndsOn         string `json:"endsOn"`
	Active         bool   `json:"active"`
	Summary        string `json:"summary"`
}

type AdminGift struct {
	ID            int64  `json:"id"`
	ThresholdKobo int64  `json:"thresholdKobo"`
	SKU           string `json:"sku"`
	ProductName   string `json:"productName"`
	Option        string `json:"option"`
	Stock         int    `json:"stock"`
	Quantity      int    `json:"quantity"`
	Given         int    `json:"given"`
	StartsOn      string `json:"startsOn"`
	EndsOn        string `json:"endsOn"`
	Active        bool   `json:"active"`
	Live          bool   `json:"live"`
}

type Choice struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type Admin struct {
	Sales      []AdminSale `json:"sales"`
	Codes      []AdminCode `json:"codes"`
	Gifts      []AdminGift `json:"gifts"`
	Categories []Choice    `json:"categories"` // "Clothing", "Clothing › Joggers"
	Zones      []Choice    `json:"zones"`
}

func (s *Service) Admin(ctx context.Context) (Admin, error) {
	out := Admin{Sales: []AdminSale{}, Codes: []AdminCode{}, Gifts: []AdminGift{}, Categories: []Choice{}, Zones: []Choice{}}
	now := time.Now()

	sales, err := s.q.ListPromotions(ctx)
	if err != nil {
		return out, fmt.Errorf("list promotions: %w", err)
	}
	for _, p := range sales {
		a := AdminSale{
			ID: p.ID, Name: p.Name, Kind: string(p.Kind), Value: p.Value, AppliesTo: string(p.AppliesTo),
			CategoryName: p.CategoryName, StartsOn: StartDay(p.StartsAt), EndsOn: EndDay(p.EndsAt),
			Badge: p.BadgeText, Announcement: p.Announcement, ExcludeOnSale: p.ExcludeOnSale, Active: p.Active,
			Orders: int(p.Orders), SalesKobo: p.SalesKobo,
		}
		if p.Tier.Valid {
			a.Tier = string(p.Tier.ProductTier)
		}
		if p.CategoryID.Valid {
			a.CategoryID = p.CategoryID.Int64
		}
		switch {
		case !p.Active:
			a.State = "off"
		case p.EndsAt.Time.Before(now):
			a.State = "ended"
		case p.StartsAt.Time.After(now):
			a.State = "scheduled"
		default:
			a.State = "live"
			a.EndsInDays = int(math.Ceil(p.EndsAt.Time.Sub(now).Hours() / 24))
		}
		out.Sales = append(out.Sales, a)
	}

	codes, err := s.q.ListCodes(ctx)
	if err != nil {
		return out, fmt.Errorf("list codes: %w", err)
	}
	for _, c := range codes {
		a := AdminCode{
			ID: c.ID, Code: c.Code, Kind: string(c.Kind), Value: c.Value, MinOrderKobo: c.MinOrderKobo, Used: int(c.Used),
			FirstOrderOnly: c.FirstOrderOnly, ZoneName: c.ZoneName, StartsOn: StartDay(c.StartsAt), EndsOn: EndDay(c.EndsAt),
			Active: c.Active, Summary: Summary(c.Kind, c.Value),
		}
		if c.MaxUses.Valid {
			n := int(c.MaxUses.Int32)
			a.MaxUses = &n
		}
		if c.ZoneID.Valid {
			a.ZoneID = c.ZoneID.Int64
		}
		out.Codes = append(out.Codes, a)
	}

	gifts, err := s.q.ListGifts(ctx)
	if err != nil {
		return out, fmt.Errorf("list gifts: %w", err)
	}
	for _, g := range gifts {
		out.Gifts = append(out.Gifts, AdminGift{
			ID: g.ID, ThresholdKobo: g.ThresholdKobo, SKU: g.Sku, ProductName: g.ProductName,
			Option: joinNonEmpty(" / ", g.Colour, g.Size), Stock: int(g.Stock), Quantity: int(g.Quantity), Given: int(g.Given),
			StartsOn: StartDay(g.StartsAt), EndsOn: EndDay(g.EndsAt), Active: g.Active,
			Live: Live(g.Active, g.StartsAt, g.EndsAt) && g.Given < g.Quantity && g.Stock > 0,
		})
	}

	cats, err := s.q.ListCategories(ctx)
	if err != nil {
		return out, err
	}
	names := map[int64]string{}
	for _, c := range cats {
		name := c.Name
		if c.ParentID.Valid {
			name = names[c.ParentID.Int64] + " › " + c.Name
		}
		names[c.ID] = name
		out.Categories = append(out.Categories, Choice{ID: c.ID, Name: name})
	}
	zones, err := s.q.ListZones(ctx)
	if err != nil {
		return out, err
	}
	for _, z := range zones {
		out.Zones = append(out.Zones, Choice{ID: z.ID, Name: z.Name})
	}
	return out, nil
}

// ---------- Sales ----------

func (s *Service) SaveSale(ctx context.Context, by staff.Member, a AdminSale) (int64, error) {
	a.Name = clean(a.Name)
	a.Badge = clean(a.Badge)
	a.Announcement = clean(a.Announcement)
	if a.Name == "" || utf8.RuneCountInString(a.Name) > 60 {
		return 0, apperr.NewInvalid("name", "Name the sale (up to 60 characters), e.g. Black Friday week.")
	}
	if err := checkValue(a.Kind, a.Value, false); err != nil {
		return 0, err
	}
	if utf8.RuneCountInString(a.Badge) > 30 {
		return 0, apperr.NewInvalid("badge", "Keep the badge under 30 characters so it fits on a product card.")
	}
	if utf8.RuneCountInString(a.Announcement) > 70 {
		return 0, apperr.NewInvalid("announcement", "Keep the announcement under 70 characters.")
	}
	params := sqlc.InsertPromotionParams{
		Name: a.Name, Kind: sqlc.DiscountKind(a.Kind), Value: a.Value, AppliesTo: sqlc.PromotionScope(a.AppliesTo),
		BadgeText: a.Badge, Announcement: a.Announcement, ExcludeOnSale: a.ExcludeOnSale, Active: a.Active,
	}
	switch a.AppliesTo {
	case "all":
	case "tier":
		if a.Tier != "everyday" && a.Tier != "premium" {
			return 0, apperr.NewInvalid("tier", "Choose Everyday or Premium.")
		}
		params.Tier = sqlc.NullProductTier{ProductTier: sqlc.ProductTier(a.Tier), Valid: true}
	case "category":
		if a.CategoryID == 0 {
			return 0, apperr.NewInvalid("categoryId", "Choose a category.")
		}
		params.CategoryID = pgtype.Int8{Int64: a.CategoryID, Valid: true}
	default:
		return 0, apperr.NewInvalid("appliesTo", "Choose what the sale applies to.")
	}
	var err error
	if a.StartsOn == "" {
		return 0, apperr.NewInvalid("startsOn", "Pick the first day of the sale.")
	}
	if a.EndsOn == "" {
		return 0, apperr.NewInvalid("endsOn", "Pick the last day of the sale. Sales always end, so prices go back on their own.")
	}
	if params.StartsAt, err = DayStart("startsOn", a.StartsOn); err != nil {
		return 0, err
	}
	if params.EndsAt, err = DayEnd("endsOn", a.EndsOn); err != nil {
		return 0, err
	}
	if err := CheckDays(params.StartsAt, params.EndsAt); err != nil {
		return 0, err
	}

	err = s.inTx(ctx, func(q *sqlc.Queries) error {
		if a.ID == 0 {
			id, err := q.InsertPromotion(ctx, params)
			if err != nil {
				return err
			}
			a.ID = id
		} else {
			n, err := q.UpdatePromotion(ctx, sqlc.UpdatePromotionParams{
				ID: a.ID, Name: params.Name, Kind: params.Kind, Value: params.Value, AppliesTo: params.AppliesTo,
				Tier: params.Tier, CategoryID: params.CategoryID, StartsAt: params.StartsAt, EndsAt: params.EndsAt,
				BadgeText: params.BadgeText, Announcement: params.Announcement, ExcludeOnSale: params.ExcludeOnSale, Active: params.Active,
			})
			if err != nil {
				return err
			}
			if n == 0 {
				return apperr.ErrNotFound
			}
		}
		return staff.Audit(ctx, q, by, "promotion.save", "promotion", a.ID, map[string]any{"name": a.Name, "kind": a.Kind, "value": a.Value})
	})
	if isFK(err) {
		return 0, apperr.NewInvalid("categoryId", "That category no longer exists.")
	}
	return a.ID, err
}

// DeleteSale removes a sale no order used. Used sales are kept for the record; switch them off.
func (s *Service) DeleteSale(ctx context.Context, by staff.Member, id int64) error {
	return s.inTx(ctx, func(q *sqlc.Queries) error {
		n, err := q.DeletePromotion(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return &apperr.Conflict{Code: "in_use", Message: "Orders used this sale, so it's kept for your records. Switch it off instead."}
		}
		return staff.Audit(ctx, q, by, "promotion.delete", "promotion", id, nil)
	})
}

// ---------- Codes ----------

func (s *Service) SaveCode(ctx context.Context, by staff.Member, c AdminCode) (int64, error) {
	c.Code = NormalizeCode(c.Code)
	if len(c.Code) < 3 || len(c.Code) > 20 || strings.Trim(c.Code, "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") != "" {
		return 0, apperr.NewInvalid("code", "Use 3 to 20 letters and numbers, e.g. WELCOME10.")
	}
	if c.Kind == "free_delivery" {
		c.Value = 0
	} else if err := checkValue(c.Kind, c.Value, true); err != nil {
		return 0, err
	}
	if c.ZoneID != 0 && c.Kind != "free_delivery" {
		c.ZoneID = 0
	}
	if c.MinOrderKobo < 0 {
		return 0, apperr.NewInvalid("minOrderKobo", "Enter a minimum spend, or 0 for none.")
	}
	if c.Kind == "fixed" && c.MinOrderKobo < c.Value+10000 {
		return 0, apperr.NewInvalid("minOrderKobo", fmt.Sprintf("Set a minimum spend above %s, or the code could make an order free.", Naira(c.Value)))
	}
	if c.MaxUses != nil && *c.MaxUses < 1 {
		return 0, apperr.NewInvalid("maxUses", "Allow at least 1 use, or leave it empty for no limit.")
	}
	start, err := DayStart("startsOn", c.StartsOn)
	if err != nil {
		return 0, err
	}
	end, err := DayEnd("endsOn", c.EndsOn)
	if err != nil {
		return 0, err
	}
	if err := CheckDays(start, end); err != nil {
		return 0, err
	}
	var maxUses pgtype.Int4
	if c.MaxUses != nil {
		maxUses = pgtype.Int4{Int32: int32(*c.MaxUses), Valid: true}
	}
	zone := pgtype.Int8{Int64: c.ZoneID, Valid: c.ZoneID != 0}

	err = s.inTx(ctx, func(q *sqlc.Queries) error {
		if c.ID == 0 {
			id, err := q.InsertCode(ctx, sqlc.InsertCodeParams{
				Code: c.Code, Kind: sqlc.DiscountKind(c.Kind), Value: c.Value, MinOrderKobo: c.MinOrderKobo, MaxUses: maxUses,
				FirstOrderOnly: c.FirstOrderOnly, ZoneID: zone, StartsAt: start, EndsAt: end, Active: c.Active,
			})
			if err != nil {
				return err
			}
			c.ID = id
		} else {
			n, err := q.UpdateCode(ctx, sqlc.UpdateCodeParams{
				ID: c.ID, Code: c.Code, Kind: sqlc.DiscountKind(c.Kind), Value: c.Value, MinOrderKobo: c.MinOrderKobo, MaxUses: maxUses,
				FirstOrderOnly: c.FirstOrderOnly, ZoneID: zone, StartsAt: start, EndsAt: end, Active: c.Active,
			})
			if err != nil {
				return err
			}
			if n == 0 {
				return apperr.ErrNotFound
			}
		}
		return staff.Audit(ctx, q, by, "code.save", "discount_code", c.ID, map[string]any{"code": c.Code, "kind": c.Kind, "value": c.Value})
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return 0, apperr.NewInvalid("code", "That code already exists.")
	}
	return c.ID, err
}

// ---------- Gifts ----------

func (s *Service) SaveGift(ctx context.Context, by staff.Member, g AdminGift) (int64, error) {
	if g.ThresholdKobo < 100000 {
		return 0, apperr.NewInvalid("thresholdKobo", "Set a spend of at least ₦1,000.")
	}
	if g.Quantity < 1 {
		return 0, apperr.NewInvalid("quantity", "Say how many gifts to give.")
	}
	v, err := s.q.VariantBySKU(ctx, strings.TrimSpace(g.SKU))
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, apperr.NewInvalid("sku", "No product option has that SKU. Copy it from the product's options.")
	}
	if err != nil {
		return 0, err
	}
	start, err := DayStart("startsOn", g.StartsOn)
	if err != nil {
		return 0, err
	}
	end, err := DayEnd("endsOn", g.EndsOn)
	if err != nil {
		return 0, err
	}
	if err := CheckDays(start, end); err != nil {
		return 0, err
	}
	err = s.inTx(ctx, func(q *sqlc.Queries) error {
		if g.ID == 0 {
			id, err := q.InsertGift(ctx, sqlc.InsertGiftParams{
				ThresholdKobo: g.ThresholdKobo, VariantID: v.ID, Quantity: int32(g.Quantity), StartsAt: start, EndsAt: end, Active: g.Active,
			})
			if err != nil {
				return err
			}
			g.ID = id
		} else {
			n, err := q.UpdateGift(ctx, sqlc.UpdateGiftParams{
				ID: g.ID, ThresholdKobo: g.ThresholdKobo, VariantID: v.ID, Quantity: int32(g.Quantity), StartsAt: start, EndsAt: end, Active: g.Active,
			})
			if err != nil {
				return err
			}
			if n == 0 {
				return apperr.ErrNotFound
			}
		}
		return staff.Audit(ctx, q, by, "gift.save", "gift_offer", g.ID, map[string]any{"sku": g.SKU, "threshold": g.ThresholdKobo})
	})
	return g.ID, err
}

// ---------- Helpers ----------

func checkValue(kind string, value int64, code bool) error {
	switch kind {
	case "percent":
		if value < 1 || value > 90 {
			return apperr.NewInvalid("value", "Enter a percentage from 1 to 90.")
		}
	case "fixed":
		if value < 10000 {
			return apperr.NewInvalid("value", "Enter an amount of at least ₦100.")
		}
	default:
		if code {
			return apperr.NewInvalid("kind", "Choose a percentage, an amount off, or free delivery.")
		}
		return apperr.NewInvalid("kind", "Choose a percentage or an amount off.")
	}
	return nil
}

func isFK(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

func clean(s string) string { return strings.Join(strings.Fields(s), " ") }
