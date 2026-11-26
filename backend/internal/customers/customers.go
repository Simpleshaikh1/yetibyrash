// Package customers runs optional shopper accounts: sign-in with a one-time code sent by
// SMS or email (no passwords), the account's orders, saved addresses and saved items.
package customers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/mail"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/db/sqlc"
	"yetibyrash/backend/internal/notify"
	"yetibyrash/backend/internal/orders"
	"yetibyrash/backend/internal/reviews"
)

const (
	SessionDays   = 60 // how long a sign-in lasts
	maxCodes      = 3  // codes per phone/email per 15 minutes
	maxAttempts   = 5  // wrong guesses per code
	codeValidMins = 10
)

type Service struct {
	pool   *pgxpool.Pool
	q      *sqlc.Queries
	mailer notify.Mailer
	sms    notify.SMS // nil = codes by email only
	// sending tracks codes still being sent in the background (Wait is for tests).
	sending sync.WaitGroup
}

func NewService(pool *pgxpool.Pool, mailer notify.Mailer, sms notify.SMS) *Service {
	return &Service{pool: pool, q: sqlc.New(pool), mailer: mailer, sms: sms}
}

// Customer is the signed-in shopper.
type Customer struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Phone          string `json:"phone"`
	Email          string `json:"email"`
	MarketingOptIn bool   `json:"marketingOptIn"`
}

func fromRow(id int64, name string, phone, email pgtype.Text, optIn bool) Customer {
	return Customer{ID: id, Name: name, Phone: phone.String, Email: email.String, MarketingOptIn: optIn}
}

// ---------- Sign-in ----------

// Target is a phone or email to send a code to, cleaned up.
type Target struct {
	Value   string `json:"value"` // "0803 000 0000" or "amaka@example.com"
	IsPhone bool   `json:"isPhone"`
}

// ParseTarget accepts a Nigerian mobile number or an email address.
func ParseTarget(s string) (Target, error) {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "@") {
		e := strings.ToLower(s)
		if a, err := mail.ParseAddress(e); err != nil || a.Address != e || !strings.Contains(e, ".") {
			return Target{}, apperr.NewInvalid("target", "Enter a valid email address.")
		}
		return Target{Value: e}, nil
	}
	p, ok := orders.NormalizePhone(s)
	if !ok {
		return Target{}, apperr.NewInvalid("target", "Enter your phone number (e.g. 0803 000 0000) or your email.")
	}
	return Target{Value: p, IsPhone: true}, nil
}

// SendCode sends a 6-digit sign-in code to a phone or email.
func (s *Service) SendCode(ctx context.Context, raw, ip string) (Target, error) {
	t, err := ParseTarget(raw)
	if err != nil {
		return t, err
	}
	if t.IsPhone && s.sms == nil {
		return t, apperr.NewInvalid("target", "We can't text codes yet. Use your email address instead.")
	}
	n, err := s.q.CountRecentSignInCodes(ctx, t.Value)
	if err != nil {
		return t, err
	}
	if n >= maxCodes {
		return t, &apperr.Conflict{Code: "too_many_codes", Message: "We've sent a few codes already. Use the latest one, or try again in 15 minutes."}
	}
	code := sixDigits()
	if err := s.q.InsertSignInCode(ctx, sqlc.InsertSignInCodeParams{Target: t.Value, CodeHash: TargetHash(t.Value, code), Ip: ip}); err != nil {
		return t, err
	}
	// Sent in the background, so the shopper isn't kept waiting on the mail or SMS
	// provider; if it fails, "Send a new code" tries again.
	text := fmt.Sprintf("Your Yetibyrash code is %s. It expires in %d minutes. Never share it with anyone.", code, codeValidMins)
	s.sending.Add(1)
	go func() {
		defer s.sending.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var err error
		if t.IsPhone {
			err = s.sms.SendSMS(ctx, t.Value, text)
		} else {
			err = s.mailer.Send(ctx, t.Value, "Your Yetibyrash sign-in code: "+code,
				"Hi,\n\n"+text+"\n\nIf you didn't ask for it, you can ignore this email.\n\nYetibyrash\n")
		}
		if err != nil {
			slog.Error("send sign-in code", "phone", t.IsPhone, "error", err)
		}
	}()
	return t, nil
}

// Wait blocks until codes being sent in the background have gone (for tests).
func (s *Service) Wait() { s.sending.Wait() }

// Verify checks a code. On success it signs the customer in (creating the account on first
// sign-in) and returns the session token for the cookie.
func (s *Service) Verify(ctx context.Context, raw, code string) (Customer, string, error) {
	t, err := ParseTarget(raw)
	if err != nil {
		return Customer{}, "", err
	}
	code = strings.Join(strings.Fields(code), "")
	var c Customer
	var token string
	wrong := false
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		row, err := q.LatestSignInCode(ctx, t.Value)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.ExpiresAt.Time.Before(time.Now())) {
			return apperr.NewInvalid("code", "That code has expired. Ask for a new one.")
		}
		if err != nil {
			return err
		}
		if row.Attempts >= maxAttempts {
			return apperr.NewInvalid("code", "Too many tries with that code. Ask for a new one.")
		}
		if subtle.ConstantTimeCompare(row.CodeHash, TargetHash(t.Value, code)) != 1 {
			wrong = true // counted below, and the count is kept
			return q.CountSignInAttempt(ctx, row.ID)
		}
		if err := q.UseSignInCode(ctx, row.ID); err != nil {
			return err
		}
		if c, err = findOrCreate(ctx, q, t); err != nil {
			return err
		}
		var hash []byte
		token, hash = newSessionToken()
		return q.InsertCustomerSession(ctx, sqlc.InsertCustomerSessionParams{
			TokenHash: hash, CustomerID: c.ID,
			ExpiresAt: pgtype.Timestamptz{Time: time.Now().AddDate(0, 0, SessionDays), Valid: true},
		})
	})
	if err != nil {
		return Customer{}, "", err
	}
	if wrong {
		return Customer{}, "", apperr.NewInvalid("code", "That code isn't right. Check it and try again.")
	}
	return c, token, nil
}

func findOrCreate(ctx context.Context, q *sqlc.Queries, t Target) (Customer, error) {
	if t.IsPhone {
		r, err := q.CustomerByPhone(ctx, pgtype.Text{String: t.Value, Valid: true})
		if err == nil {
			return fromRow(r.ID, r.Name, r.Phone, r.Email, r.MarketingOptIn), nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return Customer{}, err
		}
		n, err := q.InsertCustomer(ctx, sqlc.InsertCustomerParams{Phone: pgtype.Text{String: t.Value, Valid: true}})
		if err != nil {
			return Customer{}, err
		}
		return fromRow(n.ID, n.Name, n.Phone, n.Email, n.MarketingOptIn), nil
	}
	r, err := q.CustomerByEmail(ctx, pgtype.Text{String: t.Value, Valid: true})
	if err == nil {
		return fromRow(r.ID, r.Name, r.Phone, r.Email, r.MarketingOptIn), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Customer{}, err
	}
	n, err := q.InsertCustomer(ctx, sqlc.InsertCustomerParams{Email: pgtype.Text{String: t.Value, Valid: true}})
	if err != nil {
		return Customer{}, err
	}
	return fromRow(n.ID, n.Name, n.Phone, n.Email, n.MarketingOptIn), nil
}

// FromSession returns the customer for a session cookie, or ErrNotFound when signed out.
func (s *Service) FromSession(ctx context.Context, token string) (Customer, error) {
	if token == "" {
		return Customer{}, apperr.ErrNotFound
	}
	r, err := s.q.CustomerBySession(ctx, hashSession(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return Customer{}, apperr.ErrNotFound
	}
	if err != nil {
		return Customer{}, err
	}
	_ = s.q.TouchCustomer(ctx, r.ID)
	return fromRow(r.ID, r.Name, r.Phone, r.Email, r.MarketingOptIn), nil
}

func (s *Service) SignOut(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.q.DeleteCustomerSession(ctx, hashSession(token))
}

// ---------- Profile ----------

func (s *Service) UpdateProfile(ctx context.Context, c Customer, name string, optIn bool) error {
	name = strings.Join(strings.Fields(name), " ")
	if utf8.RuneCountInString(name) > 80 {
		return apperr.NewInvalid("name", "Keep your name under 80 characters.")
	}
	return s.q.UpdateCustomerProfile(ctx, sqlc.UpdateCustomerProfileParams{ID: c.ID, Name: name, MarketingOptIn: optIn})
}

// DeleteAccount removes the account, its addresses and saved items (NDPR). Orders are kept,
// as the law requires for sales records, but are no longer linked to an account.
func (s *Service) DeleteAccount(ctx context.Context, c Customer) error {
	return s.q.DeleteCustomer(ctx, c.ID)
}

// ---------- Orders ----------

type OrderSummary struct {
	Number    string    `json:"number"`
	Status    string    `json:"status"`
	TotalKobo int64     `json:"totalKobo"`
	Items     int       `json:"items"`
	Image     string    `json:"image"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Service) Orders(ctx context.Context, c Customer) ([]OrderSummary, error) {
	rows, err := s.q.CustomerOrders(ctx, sqlc.CustomerOrdersParams{CustomerID: pgtype.Int8{Int64: c.ID, Valid: true}, Phone: text(c.Phone), Email: text(c.Email)})
	if err != nil {
		return nil, err
	}
	out := make([]OrderSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, OrderSummary{Number: r.Number, Status: string(r.Status), TotalKobo: r.TotalKobo, Items: int(r.ItemCount), Image: r.ImageKey, CreatedAt: r.CreatedAt.Time})
	}
	return out, nil
}

// ReviewLink makes a fresh private review link for one of the customer's delivered orders.
func (s *Service) ReviewLink(ctx context.Context, c Customer, number string) (string, error) {
	o, err := s.q.CustomerOrderByNumber(ctx, sqlc.CustomerOrderByNumberParams{Number: number, CustomerID: pgtype.Int8{Int64: c.ID, Valid: true}, Phone: text(c.Phone), Email: text(c.Email)})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperr.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if o.Status != sqlc.OrderStatusDelivered {
		return "", &apperr.Conflict{Code: "not_delivered", Message: "You can review an order once it's delivered."}
	}
	token, hash := reviews.NewToken()
	if err := s.q.InsertReviewRequest(ctx, sqlc.InsertReviewRequestParams{OrderID: o.ID, TokenHash: hash}); err != nil {
		return "", err
	}
	return token, nil
}

// ---------- Addresses ----------

type Address struct {
	ID        int64  `json:"id"`
	StateID   int64  `json:"stateId"`
	StateName string `json:"stateName"`
	LgaID     int64  `json:"lgaId"`   // 0 when typed
	LgaText   string `json:"lgaText"` // typed LGA for states without a list
	LgaName   string `json:"lgaName"`
	Street    string `json:"street"`
	Landmark  string `json:"landmark"`
}

func (s *Service) Addresses(ctx context.Context, c Customer) ([]Address, error) {
	rows, err := s.q.ListAddresses(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	out := make([]Address, 0, len(rows))
	for _, r := range rows {
		out = append(out, Address{
			ID: r.ID, StateID: r.StateID, StateName: r.StateName, LgaID: r.LgaID.Int64, LgaText: r.LgaText,
			LgaName: r.LgaName, Street: r.Street, Landmark: r.Landmark,
		})
	}
	return out, nil
}

func (s *Service) DeleteAddress(ctx context.Context, c Customer, id int64) error {
	n, err := s.q.DeleteAddress(ctx, sqlc.DeleteAddressParams{ID: id, CustomerID: c.ID})
	if err != nil {
		return err
	}
	if n == 0 {
		return apperr.ErrNotFound
	}
	return nil
}

// ---------- Saved items ----------

func (s *Service) SavedIDs(ctx context.Context, c Customer) ([]int64, error) {
	ids, err := s.q.SavedProductIDs(ctx, c.ID)
	if ids == nil {
		ids = []int64{}
	}
	return ids, err
}

func (s *Service) Save(ctx context.Context, c Customer, productID int64) error {
	err := s.q.SaveItem(ctx, sqlc.SaveItemParams{CustomerID: c.ID, ProductID: productID})
	if isForeignKey(err) {
		return apperr.ErrNotFound
	}
	return err
}

func (s *Service) Unsave(ctx context.Context, c Customer, productID int64) error {
	return s.q.UnsaveItem(ctx, sqlc.UnsaveItemParams{CustomerID: c.ID, ProductID: productID})
}

// ---------- Helpers ----------

// TargetHash is how a sign-in code is stored: tied to where it was sent.
func TargetHash(target, code string) []byte {
	h := sha256.Sum256([]byte("otp:" + target + ":" + code))
	return h[:]
}

func sixDigits() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1_000_000))
	return fmt.Sprintf("%06d", n.Int64())
}

func newSessionToken() (string, []byte) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	t := base64.RawURLEncoding.EncodeToString(b)
	return t, hashSession(t)
}

func hashSession(t string) []byte {
	h := sha256.Sum256([]byte("session:" + t))
	return h[:]
}

func isForeignKey(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
