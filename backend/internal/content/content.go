// Package content runs the homepage and site text the admin edits: hero banners,
// the announcement bar, homepage sections and simple pages (About, FAQ, policies).
package content

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/db/sqlc"
	"yetibyrash/backend/internal/media"
	"yetibyrash/backend/internal/promotions"
	"yetibyrash/backend/internal/staff"
)

type Service struct {
	pool    *pgxpool.Pool
	q       *sqlc.Queries
	storage media.Storage
}

func NewService(pool *pgxpool.Pool, storage media.Storage) *Service {
	return &Service{pool: pool, q: sqlc.New(pool), storage: storage}
}

func (s *Service) inTx(ctx context.Context, fn func(q *sqlc.Queries) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return fn(s.q.WithTx(tx)) })
}

// ---------- What the shop reads ----------

type Link struct {
	Text string `json:"text"`
	Link string `json:"link"`
}

type PageLink struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

// Site is what every shop page needs: the announcement bar and the footer's page links.
type Site struct {
	Announcements []Link     `json:"announcements"`
	Pages         []PageLink `json:"pages"`
}

func (s *Service) Site(ctx context.Context) (Site, error) {
	out := Site{Announcements: []Link{}, Pages: []PageLink{}}
	// A live sale's message comes first.
	sales, err := promotions.LiveSales(ctx, s.q)
	if err != nil {
		return out, err
	}
	for _, sale := range sales {
		if sale.Announcement != "" {
			out.Announcements = append(out.Announcements, Link{Text: sale.Announcement, Link: "/shop"})
		}
	}
	anns, err := s.q.LiveAnnouncements(ctx)
	if err != nil {
		return out, fmt.Errorf("announcements: %w", err)
	}
	for _, a := range anns {
		out.Announcements = append(out.Announcements, Link{Text: a.Text, Link: a.Link})
	}
	pages, err := s.q.ListPublishedPages(ctx)
	if err != nil {
		return out, fmt.Errorf("pages: %w", err)
	}
	for _, p := range pages {
		out.Pages = append(out.Pages, PageLink{Slug: p.Slug, Title: p.Title})
	}
	return out, nil
}

type PublicPage struct {
	Slug      string    `json:"slug"`
	Title     string    `json:"title"`
	Body      string    `json:"body"` // light Markdown
	UpdatedAt time.Time `json:"updatedAt"`
}

// Page returns a published page.
func (s *Service) Page(ctx context.Context, slug string) (PublicPage, error) {
	p, err := s.q.GetPublishedPage(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return PublicPage{}, apperr.ErrNotFound
	}
	if err != nil {
		return PublicPage{}, err
	}
	return PublicPage{Slug: p.Slug, Title: p.Title, Body: p.Body, UpdatedAt: p.UpdatedAt.Time}, nil
}

// ---------- Admin ----------

type Banner struct {
	ID         int64  `json:"id"`
	Eyebrow    string `json:"eyebrow"`
	Headline   string `json:"headline"`
	ButtonText string `json:"buttonText"`
	ButtonLink string `json:"buttonLink"`
	Image      string `json:"image"`
	Tier       string `json:"tier"`     // "" = both tiers
	StartsOn   string `json:"startsOn"` // "" = from now
	EndsOn     string `json:"endsOn"`   // last day shown, "" = no end
	Active     bool   `json:"active"`
	Live       bool   `json:"live"` // showing right now (read only)
}

type Announcement struct {
	ID       int64  `json:"id"`
	Text     string `json:"text"`
	Link     string `json:"link"`
	StartsOn string `json:"startsOn"`
	EndsOn   string `json:"endsOn"`
	Active   bool   `json:"active"`
	Live     bool   `json:"live"`
}

type Section struct {
	Key       string   `json:"key"`
	Title     string   `json:"title"`
	Visible   bool     `json:"visible"`
	ItemCount int      `json:"itemCount"`
	Lines     []string `json:"lines"`
}

type Page struct {
	ID        int64  `json:"id"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Published bool   `json:"published"`
	UpdatedAt string `json:"updatedAt"` // read only; ignored when saving
}

// Admin is everything on the Homepage and content screen.
type Admin struct {
	Banners       []Banner       `json:"banners"`
	Announcements []Announcement `json:"announcements"`
	Sections      []Section      `json:"sections"`
	Pages         []Page         `json:"pages"`
	// SaleMessages are announcements coming from live sales (edited under Promotions).
	SaleMessages []string `json:"saleMessages"`
}

func (s *Service) Admin(ctx context.Context) (Admin, error) {
	out := Admin{Banners: []Banner{}, Announcements: []Announcement{}, Sections: []Section{}, Pages: []Page{}, SaleMessages: []string{}}
	banners, err := s.q.ListBanners(ctx)
	if err != nil {
		return out, err
	}
	for _, b := range banners {
		x := Banner{
			ID: b.ID, Eyebrow: b.Eyebrow, Headline: b.Headline, ButtonText: b.ButtonText, ButtonLink: b.ButtonLink,
			Image: b.ImageKey, StartsOn: promotions.StartDay(b.StartsAt), EndsOn: promotions.EndDay(b.EndsAt),
			Active: b.Active, Live: promotions.Live(b.Active, b.StartsAt, b.EndsAt),
		}
		if b.Tier.Valid {
			x.Tier = string(b.Tier.ProductTier)
		}
		out.Banners = append(out.Banners, x)
	}
	anns, err := s.q.ListAnnouncements(ctx)
	if err != nil {
		return out, err
	}
	for _, a := range anns {
		out.Announcements = append(out.Announcements, Announcement{
			ID: a.ID, Text: a.Text, Link: a.Link, StartsOn: promotions.StartDay(a.StartsAt), EndsOn: promotions.EndDay(a.EndsAt),
			Active: a.Active, Live: promotions.Live(a.Active, a.StartsAt, a.EndsAt),
		})
	}
	secs, err := s.q.ListSections(ctx)
	if err != nil {
		return out, err
	}
	for _, x := range secs {
		out.Sections = append(out.Sections, Section{Key: x.Key, Title: x.Title, Visible: x.Visible, ItemCount: int(x.ItemCount), Lines: nonNil(x.Lines)})
	}
	pages, err := s.q.ListPages(ctx)
	if err != nil {
		return out, err
	}
	for _, p := range pages {
		out.Pages = append(out.Pages, Page{ID: p.ID, Slug: p.Slug, Title: p.Title, Body: p.Body, Published: p.Published, UpdatedAt: p.UpdatedAt.Time.Format(time.RFC3339)})
	}
	sales, err := promotions.LiveSales(ctx, s.q)
	if err != nil {
		return out, err
	}
	for _, sale := range sales {
		if sale.Announcement != "" {
			out.SaleMessages = append(out.SaleMessages, sale.Announcement)
		}
	}
	return out, nil
}

// ---------- Banners ----------

// SaveBanner creates (ID 0) or updates a banner's words, tier and dates. The photo is separate.
func (s *Service) SaveBanner(ctx context.Context, by staff.Member, b Banner) (int64, error) {
	b.Eyebrow = clean(b.Eyebrow)
	b.Headline = clean(b.Headline)
	b.ButtonText = clean(b.ButtonText)
	b.ButtonLink = strings.TrimSpace(b.ButtonLink)
	switch {
	case b.Headline == "" || utf8.RuneCountInString(b.Headline) > 80:
		return 0, apperr.NewInvalid("headline", "Write a headline of up to 80 characters.")
	case utf8.RuneCountInString(b.Eyebrow) > 40:
		return 0, apperr.NewInvalid("eyebrow", "Keep the small label under 40 characters.")
	case b.ButtonText == "" || utf8.RuneCountInString(b.ButtonText) > 24:
		return 0, apperr.NewInvalid("buttonText", "Write button text of up to 24 characters.")
	}
	if err := checkLink("buttonLink", b.ButtonLink, true); err != nil {
		return 0, err
	}
	tier, err := tierArg(b.Tier)
	if err != nil {
		return 0, err
	}
	start, err := promotions.DayStart("startsOn", b.StartsOn)
	if err != nil {
		return 0, err
	}
	end, err := promotions.DayEnd("endsOn", b.EndsOn)
	if err != nil {
		return 0, err
	}
	if err := promotions.CheckDays(start, end); err != nil {
		return 0, err
	}
	err = s.inTx(ctx, func(q *sqlc.Queries) error {
		if b.ID == 0 {
			id, err := q.InsertBanner(ctx, sqlc.InsertBannerParams{
				Eyebrow: b.Eyebrow, Headline: b.Headline, ButtonText: b.ButtonText, ButtonLink: b.ButtonLink,
				Tier: tier, StartsAt: start, EndsAt: end, Active: b.Active,
			})
			if err != nil {
				return err
			}
			b.ID = id
		} else {
			n, err := q.UpdateBanner(ctx, sqlc.UpdateBannerParams{
				ID: b.ID, Eyebrow: b.Eyebrow, Headline: b.Headline, ButtonText: b.ButtonText, ButtonLink: b.ButtonLink,
				Tier: tier, StartsAt: start, EndsAt: end, Active: b.Active,
			})
			if err != nil {
				return err
			}
			if n == 0 {
				return apperr.ErrNotFound
			}
		}
		return staff.Audit(ctx, q, by, "banner.save", "banner", b.ID, map[string]any{"headline": b.Headline})
	})
	return b.ID, err
}

// SetBannerImage stores a banner photo's phone and wide crops and swaps them in.
func (s *Service) SetBannerImage(ctx context.Context, by staff.Member, id int64, photo *media.ProcessedBanner) (string, error) {
	if _, err := s.q.GetBanner(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return "", apperr.ErrNotFound
	} else if err != nil {
		return "", err
	}
	key := fmt.Sprintf("banners/%d/%s", id, randomHex(6))
	for name, data := range photo.Files {
		if err := s.storage.Put(ctx, key+"-"+name+".jpg", bytes.NewReader(data)); err != nil {
			return "", fmt.Errorf("store banner photo: %w", err)
		}
	}
	var old string
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		var err error
		if old, err = q.SetBannerImage(ctx, sqlc.SetBannerImageParams{ID: id, ImageKey: key}); err != nil {
			return err
		}
		return staff.Audit(ctx, q, by, "banner.photo", "banner", id, map[string]any{"warnings": photo.Warnings})
	})
	if err != nil {
		s.deleteBannerFiles(ctx, key)
		return "", err
	}
	s.deleteBannerFiles(ctx, old)
	return key, nil
}

func (s *Service) DeleteBanner(ctx context.Context, by staff.Member, id int64) error {
	var key string
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		var err error
		key, err = q.DeleteBanner(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return err
		}
		return staff.Audit(ctx, q, by, "banner.delete", "banner", id, nil)
	})
	if err == nil {
		s.deleteBannerFiles(ctx, key)
	}
	return err
}

// OrderBanners sets the banners' order; the first live one for a tier is the hero.
func (s *Service) OrderBanners(ctx context.Context, by staff.Member, ids []int64) error {
	return s.inTx(ctx, func(q *sqlc.Queries) error {
		for i, id := range ids {
			if err := q.SetBannerPosition(ctx, sqlc.SetBannerPositionParams{ID: id, Position: int32(i + 1)}); err != nil {
				return err
			}
		}
		return staff.Audit(ctx, q, by, "banner.order", "banner", 0, ids)
	})
}

func (s *Service) deleteBannerFiles(ctx context.Context, key string) {
	if key == "" {
		return
	}
	for _, name := range []string{"phone", "wide"} {
		_ = s.storage.Delete(ctx, key+"-"+name+".jpg")
	}
}

// ---------- Announcements ----------

func (s *Service) SaveAnnouncement(ctx context.Context, by staff.Member, a Announcement) (int64, error) {
	a.Text = clean(a.Text)
	a.Link = strings.TrimSpace(a.Link)
	if a.Text == "" || utf8.RuneCountInString(a.Text) > 70 {
		return 0, apperr.NewInvalid("text", "Write a message of up to 70 characters, so it fits on a phone.")
	}
	if err := checkLink("link", a.Link, false); err != nil {
		return 0, err
	}
	start, err := promotions.DayStart("startsOn", a.StartsOn)
	if err != nil {
		return 0, err
	}
	end, err := promotions.DayEnd("endsOn", a.EndsOn)
	if err != nil {
		return 0, err
	}
	if err := promotions.CheckDays(start, end); err != nil {
		return 0, err
	}
	err = s.inTx(ctx, func(q *sqlc.Queries) error {
		if a.ID == 0 {
			id, err := q.InsertAnnouncement(ctx, sqlc.InsertAnnouncementParams{Text: a.Text, Link: a.Link, Active: a.Active, StartsAt: start, EndsAt: end})
			if err != nil {
				return err
			}
			a.ID = id
		} else {
			n, err := q.UpdateAnnouncement(ctx, sqlc.UpdateAnnouncementParams{ID: a.ID, Text: a.Text, Link: a.Link, Active: a.Active, StartsAt: start, EndsAt: end})
			if err != nil {
				return err
			}
			if n == 0 {
				return apperr.ErrNotFound
			}
		}
		return staff.Audit(ctx, q, by, "announcement.save", "announcement", a.ID, map[string]any{"text": a.Text})
	})
	return a.ID, err
}

func (s *Service) DeleteAnnouncement(ctx context.Context, by staff.Member, id int64) error {
	return s.inTx(ctx, func(q *sqlc.Queries) error {
		n, err := q.DeleteAnnouncement(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return apperr.ErrNotFound
		}
		return staff.Audit(ctx, q, by, "announcement.delete", "announcement", id, nil)
	})
}

func (s *Service) OrderAnnouncements(ctx context.Context, by staff.Member, ids []int64) error {
	return s.inTx(ctx, func(q *sqlc.Queries) error {
		for i, id := range ids {
			if err := q.SetAnnouncementPosition(ctx, sqlc.SetAnnouncementPositionParams{ID: id, Position: int32(i + 1)}); err != nil {
				return err
			}
		}
		return staff.Audit(ctx, q, by, "announcement.order", "announcement", 0, ids)
	})
}

// ---------- Homepage sections ----------

// SaveSections saves every section at once, in the order given.
func (s *Service) SaveSections(ctx context.Context, by staff.Member, secs []Section) error {
	for i := range secs {
		x := &secs[i]
		x.Title = clean(x.Title)
		if x.Title == "" || utf8.RuneCountInString(x.Title) > 40 {
			return apperr.NewInvalid("sections."+x.Key, "Give each section a title of up to 40 characters.")
		}
		if x.ItemCount == 0 {
			x.ItemCount = 6
		}
		if x.ItemCount < 2 || x.ItemCount > 12 {
			return apperr.NewInvalid("sections."+x.Key, "Show between 2 and 12 products.")
		}
		lines := []string{}
		for _, l := range x.Lines {
			if l = clean(l); l != "" {
				if utf8.RuneCountInString(l) > 40 {
					return apperr.NewInvalid("sections."+x.Key, "Keep each line under 40 characters.")
				}
				lines = append(lines, l)
			}
		}
		if len(lines) > 4 {
			return apperr.NewInvalid("sections."+x.Key, "The trust strip fits up to 4 lines.")
		}
		x.Lines = lines
	}
	return s.inTx(ctx, func(q *sqlc.Queries) error {
		for i, x := range secs {
			n, err := q.UpdateSection(ctx, sqlc.UpdateSectionParams{
				Key: x.Key, Title: x.Title, Position: int32(i + 1), Visible: x.Visible, ItemCount: int32(x.ItemCount), Lines: x.Lines,
			})
			if err != nil {
				return err
			}
			if n == 0 {
				return apperr.NewInvalid("sections", "Unknown section "+x.Key+".")
			}
		}
		return staff.Audit(ctx, q, by, "homepage.sections", "homepage", 0, secs)
	})
}

// ---------- Pages ----------

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func (s *Service) SavePage(ctx context.Context, by staff.Member, p Page) (int64, error) {
	p.Title = clean(p.Title)
	p.Slug = strings.TrimSpace(strings.ToLower(p.Slug))
	p.Body = strings.TrimSpace(strings.ReplaceAll(p.Body, "\r\n", "\n"))
	switch {
	case p.Title == "" || utf8.RuneCountInString(p.Title) > 60:
		return 0, apperr.NewInvalid("title", "Write a title of up to 60 characters.")
	case !slugRe.MatchString(p.Slug) || len(p.Slug) > 60:
		return 0, apperr.NewInvalid("slug", "Use lower-case letters, numbers and dashes, e.g. delivery-and-returns.")
	case utf8.RuneCountInString(p.Body) > 20000:
		return 0, apperr.NewInvalid("body", "This page is too long (20,000 characters at most).")
	case p.Published && p.Body == "":
		return 0, apperr.NewInvalid("body", "Write the page before publishing it.")
	}
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		if p.ID == 0 {
			id, err := q.InsertPage(ctx, sqlc.InsertPageParams{Slug: p.Slug, Title: p.Title, Body: p.Body, Published: p.Published})
			if err != nil {
				return err
			}
			p.ID = id
		} else {
			n, err := q.UpdatePage(ctx, sqlc.UpdatePageParams{ID: p.ID, Slug: p.Slug, Title: p.Title, Body: p.Body, Published: p.Published})
			if err != nil {
				return err
			}
			if n == 0 {
				return apperr.ErrNotFound
			}
		}
		return staff.Audit(ctx, q, by, "page.save", "page", p.ID, map[string]any{"slug": p.Slug, "published": p.Published})
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return 0, apperr.NewInvalid("slug", "Another page already uses this web address.")
	}
	return p.ID, err
}

func (s *Service) DeletePage(ctx context.Context, by staff.Member, id int64) error {
	return s.inTx(ctx, func(q *sqlc.Queries) error {
		n, err := q.DeletePage(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return apperr.ErrNotFound
		}
		return staff.Audit(ctx, q, by, "page.delete", "page", id, nil)
	})
}

// ---------- Helpers ----------

// checkLink allows paths on the shop ("/shop/bags") only, so a banner can't send shoppers elsewhere.
func checkLink(field, link string, required bool) error {
	if link == "" && !required {
		return nil
	}
	if !strings.HasPrefix(link, "/") || strings.HasPrefix(link, "//") || strings.ContainsAny(link, " \\") || len(link) > 200 {
		return apperr.NewInvalid(field, "Use a page on the shop, starting with /, e.g. /shop/bags.")
	}
	return nil
}

func tierArg(t string) (sqlc.NullProductTier, error) {
	switch t {
	case "":
		return sqlc.NullProductTier{}, nil
	case "everyday", "premium":
		return sqlc.NullProductTier{ProductTier: sqlc.ProductTier(t), Valid: true}, nil
	}
	return sqlc.NullProductTier{}, apperr.NewInvalid("tier", "Choose Everyday, Premium or both.")
}

func clean(s string) string { return strings.Join(strings.Fields(s), " ") }

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
