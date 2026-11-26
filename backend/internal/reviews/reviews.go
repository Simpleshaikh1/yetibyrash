package reviews

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/db/sqlc"
	"yetibyrash/backend/internal/media"
	"yetibyrash/backend/internal/staff"
)

// MaxPhotos is how many photos a buyer can add to one review.
const MaxPhotos = 3

type Service struct {
	pool    *pgxpool.Pool
	q       *sqlc.Queries
	storage media.Storage
}

func NewService(pool *pgxpool.Pool, storage media.Storage) *Service {
	return &Service{pool: pool, q: sqlc.New(pool), storage: storage}
}

// ---------- The buyer's review page ----------

type ItemReview struct {
	ID     int64    `json:"id"`
	Rating int      `json:"rating"`
	Status string   `json:"status"`
	Photos []string `json:"photos"`
}

type Item struct {
	OrderItemID int64       `json:"orderItemId"`
	Name        string      `json:"name"`
	Slug        string      `json:"slug"`
	Option      string      `json:"option"`
	Image       string      `json:"image"`
	Review      *ItemReview `json:"review"` // null until reviewed
}

// Request is what a review link opens: the order's items, each with any review left.
type Request struct {
	OrderNumber string `json:"orderNumber"`
	FirstName   string `json:"firstName"`
	AuthorName  string `json:"authorName"` // suggested: "Chioma E."
	Items       []Item `json:"items"`
}

func (s *Service) order(ctx context.Context, q *sqlc.Queries, token string) (sqlc.ReviewRequestOrderRow, error) {
	o, err := q.ReviewRequestOrder(ctx, HashToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return o, apperr.ErrNotFound
	}
	if err != nil {
		return o, err
	}
	if o.Status != sqlc.OrderStatusDelivered {
		return o, &apperr.Conflict{Code: "not_delivered", Message: "This order can't be reviewed."}
	}
	return o, nil
}

func (s *Service) Request(ctx context.Context, token string) (Request, error) {
	o, err := s.order(ctx, s.q, token)
	if err != nil {
		return Request{}, err
	}
	rows, err := s.q.ReviewableItems(ctx, o.ID)
	if err != nil {
		return Request{}, err
	}
	out := Request{OrderNumber: o.Number, FirstName: firstName(o.CustomerName), AuthorName: ShortName(o.CustomerName), Items: []Item{}}
	for _, r := range rows {
		it := Item{OrderItemID: r.ID, Name: r.ProductName, Slug: r.ProductSlug, Option: r.VariantLabel, Image: r.ImageKey}
		if r.ReviewID.Valid {
			photos := r.Photos
			if photos == nil {
				photos = []string{}
			}
			it.Review = &ItemReview{ID: r.ReviewID.Int64, Rating: int(r.Rating.Int32), Status: string(r.ReviewStatus.ReviewStatus), Photos: photos}
		}
		out.Items = append(out.Items, it)
	}
	return out, nil
}

type Input struct {
	OrderItemID int64  `json:"orderItemId"`
	Rating      int    `json:"rating"`
	Text        string `json:"text"`
	Name        string `json:"name"` // how the review is signed; "" = first name and initial
}

// Submit saves a review for one item of the order. It waits for staff approval.
func (s *Service) Submit(ctx context.Context, token string, in Input) (int64, error) {
	in.Text = strings.TrimSpace(strings.ReplaceAll(in.Text, "\r\n", "\n"))
	in.Name = strings.Join(strings.Fields(in.Name), " ")
	switch {
	case in.Rating < 1 || in.Rating > 5:
		return 0, apperr.NewInvalid("rating", "Tap the stars to choose a rating.")
	case utf8.RuneCountInString(in.Text) > 1000:
		return 0, apperr.NewInvalid("text", "Keep your review under 1,000 characters.")
	case utf8.RuneCountInString(in.Name) > 40:
		return 0, apperr.NewInvalid("name", "Keep the name under 40 characters.")
	}
	o, err := s.order(ctx, s.q, token)
	if err != nil {
		return 0, err
	}
	rows, err := s.q.ReviewableItems(ctx, o.ID)
	if err != nil {
		return 0, err
	}
	var item *sqlc.ReviewableItemsRow
	for i := range rows {
		if rows[i].ID == in.OrderItemID {
			item = &rows[i]
		}
	}
	if item == nil {
		return 0, apperr.ErrNotFound
	}
	if item.ReviewID.Valid {
		return 0, &apperr.Conflict{Code: "already_reviewed", Message: "You've already reviewed this item. Thank you!"}
	}
	name := in.Name
	if name == "" {
		name = ShortName(o.CustomerName)
	}
	id, err := s.q.InsertReview(ctx, sqlc.InsertReviewParams{
		ProductID: item.ProductID, OrderItemID: item.ID, Rating: int32(in.Rating), Text: in.Text,
		AuthorName: name, AuthorArea: o.StateName, VariantLabel: item.VariantLabel, OrderNumber: o.Number,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return 0, &apperr.Conflict{Code: "already_reviewed", Message: "You've already reviewed this item. Thank you!"}
	}
	return id, err
}

// AddPhoto adds a photo to a review that's still waiting for approval.
func (s *Service) AddPhoto(ctx context.Context, token string, reviewID int64, photo *media.Processed) (string, error) {
	o, err := s.order(ctx, s.q, token)
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf("reviews/%d/%s", reviewID, randomHex(6))
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		r, err := q.ReviewForPhotos(ctx, sqlc.ReviewForPhotosParams{ID: reviewID, OrderID: o.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return err
		}
		if len(r.Photos) >= MaxPhotos {
			return &apperr.Conflict{Code: "too_many_photos", Message: fmt.Sprintf("You can add up to %d photos.", MaxPhotos)}
		}
		for w, data := range photo.Files {
			if err := s.storage.Put(ctx, fmt.Sprintf("%s-%d.jpg", key, w), bytes.NewReader(data)); err != nil {
				return fmt.Errorf("store review photo: %w", err)
			}
		}
		return q.SetReviewPhotos(ctx, sqlc.SetReviewPhotosParams{ID: reviewID, Photos: append(r.Photos, key)})
	})
	if err != nil {
		for _, w := range media.Sizes {
			_ = s.storage.Delete(ctx, fmt.Sprintf("%s-%d.jpg", key, w))
		}
		return "", err
	}
	return key, nil
}

// ---------- Shop ----------

type Rating struct {
	Average float64 `json:"average"` // 0 when there are no reviews
	Count   int     `json:"count"`
}

type Published struct {
	ID          int64     `json:"id"`
	Rating      int       `json:"rating"`
	Text        string    `json:"text"`
	Photos      []string  `json:"photos"`
	Reply       string    `json:"reply"`
	Author      string    `json:"author"`
	Area        string    `json:"area"`
	Option      string    `json:"option"`
	PublishedAt time.Time `json:"publishedAt"`
}

// ---------- Admin ----------

type AdminReview struct {
	ID          int64     `json:"id"`
	Rating      int       `json:"rating"`
	Text        string    `json:"text"`
	Photos      []string  `json:"photos"`
	Status      string    `json:"status"`
	Reply       string    `json:"reply"`
	Author      string    `json:"author"`
	Area        string    `json:"area"`
	Option      string    `json:"option"`
	OrderNumber string    `json:"orderNumber"`
	Product     string    `json:"product"`
	ProductSlug string    `json:"productSlug"`
	CreatedAt   time.Time `json:"createdAt"`
}

type AdminList struct {
	Reviews   []AdminReview `json:"reviews"`
	Waiting   int           `json:"waiting"`
	Published int           `json:"published"`
	Hidden    int           `json:"hidden"`
	Average   float64       `json:"average"`
}

func validStatus(s string) bool { return s == "waiting" || s == "published" || s == "hidden" }

func (s *Service) Admin(ctx context.Context, status string) (AdminList, error) {
	if !validStatus(status) {
		status = "waiting"
	}
	rows, err := s.q.AdminListReviews(ctx, sqlc.ReviewStatus(status))
	if err != nil {
		return AdminList{}, err
	}
	c, err := s.q.ReviewCounts(ctx)
	if err != nil {
		return AdminList{}, err
	}
	out := AdminList{Reviews: []AdminReview{}, Waiting: int(c.Waiting), Published: int(c.Published), Hidden: int(c.Hidden), Average: c.Average}
	for _, r := range rows {
		photos := r.Photos
		if photos == nil {
			photos = []string{}
		}
		out.Reviews = append(out.Reviews, AdminReview{
			ID: r.ID, Rating: int(r.Rating), Text: r.Text, Photos: photos, Status: string(r.Status), Reply: r.Reply,
			Author: r.AuthorName, Area: r.AuthorArea, Option: r.VariantLabel, OrderNumber: r.OrderNumber,
			Product: r.ProductName, ProductSlug: r.ProductSlug, CreatedAt: r.CreatedAt.Time,
		})
	}
	return out, nil
}

// SetStatus approves (published), hides or returns a review to waiting.
func (s *Service) SetStatus(ctx context.Context, by staff.Member, id int64, status string) error {
	if !validStatus(status) {
		return apperr.NewInvalid("status", "Choose waiting, published or hidden.")
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		n, err := q.SetReviewStatus(ctx, sqlc.SetReviewStatusParams{ID: id, Status: sqlc.ReviewStatus(status)})
		if err != nil {
			return err
		}
		if n == 0 {
			return apperr.ErrNotFound
		}
		return staff.Audit(ctx, q, by, "review."+status, "review", id, nil)
	})
}

// Reply sets the shop's public reply ("" removes it).
func (s *Service) Reply(ctx context.Context, by staff.Member, id int64, reply string) error {
	reply = strings.TrimSpace(reply)
	if utf8.RuneCountInString(reply) > 600 {
		return apperr.NewInvalid("reply", "Keep the reply under 600 characters.")
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		n, err := q.SetReviewReply(ctx, sqlc.SetReviewReplyParams{ID: id, Reply: reply})
		if err != nil {
			return err
		}
		if n == 0 {
			return apperr.ErrNotFound
		}
		return staff.Audit(ctx, q, by, "review.reply", "review", id, map[string]string{"reply": reply})
	})
}

// ---------- Helpers ----------

// ShortName signs a review "Chioma E." from "Chioma Eze", so full names aren't published.
func ShortName(full string) string {
	parts := strings.Fields(full)
	switch len(parts) {
	case 0:
		return "A Yetibyrash customer"
	case 1:
		return parts[0]
	}
	last, _ := utf8.DecodeRuneInString(parts[len(parts)-1])
	return parts[0] + " " + strings.ToUpper(string(last)) + "."
}

func firstName(full string) string {
	if f := strings.Fields(full); len(f) > 0 {
		return f[0]
	}
	return ""
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
