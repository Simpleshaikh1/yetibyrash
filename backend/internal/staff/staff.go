// Package staff handles admin accounts: passwords, sign-in sessions and roles.
package staff

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"yetibyrash/backend/internal/db/sqlc"
)

type Role string

const (
	RoleOwner         Role = "owner"
	RoleManager       Role = "manager"
	RolePacker        Role = "packer"
	RoleContentEditor Role = "content_editor"
)

func ValidRole(r Role) bool {
	return r == RoleOwner || r == RoleManager || r == RolePacker || r == RoleContentEditor
}

// Permission names one area of the admin. Roles grant permissions (see the Staff screen).
type Permission string

const (
	PermProducts   Permission = "products"   // product list, editor, photos
	PermCatalogue  Permission = "catalogue"  // categories, size sets, colours
	PermOrders     Permission = "orders"     // orders board (M4)
	PermStaff      Permission = "staff"      // staff accounts
	PermContent    Permission = "content"    // homepage, banners, announcements, pages
	PermPromotions Permission = "promotions" // sales, discount codes, free gifts
)

var rolePermissions = map[Role][]Permission{
	RoleOwner:         {PermProducts, PermCatalogue, PermOrders, PermStaff, PermContent, PermPromotions},
	RoleManager:       {PermProducts, PermCatalogue, PermOrders, PermContent, PermPromotions},
	RolePacker:        {PermOrders},
	RoleContentEditor: {PermProducts, PermContent},
}

func (r Role) Can(p Permission) bool {
	for _, x := range rolePermissions[r] {
		if x == p {
			return true
		}
	}
	return false
}

func (r Role) Permissions() []Permission { return append([]Permission{}, rolePermissions[r]...) }

// Member is a signed-in staff member.
type Member struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  Role   `json:"role"`
}

// Account is a staff account as listed on the Staff screen.
type Account struct {
	Member
	Active      bool       `json:"active"`
	LastLoginAt *time.Time `json:"lastLoginAt"`
	CreatedAt   time.Time  `json:"createdAt"`
}

var (
	ErrBadLogin   = errors.New("wrong email or password")
	ErrNoSession  = errors.New("not signed in")
	ErrEmailTaken = errors.New("email already used by another staff account")
	ErrInvalid    = errors.New("invalid input")
	ErrLastOwner  = errors.New("the shop must keep at least one active owner")
	ErrNotFound   = errors.New("staff account not found")
)

// SessionLength is the longest a sign-in lasts, however much it's used.
const SessionLength = 7 * 24 * time.Hour

// IdleTimeout ends a sign-in that hasn't been used for this long, so an admin left
// open on a shared or lost device signs itself out.
const IdleTimeout = 8 * time.Hour

type Service struct {
	q *sqlc.Queries
}

func NewService(db sqlc.DBTX) *Service { return &Service{q: sqlc.New(db)} }

// Login checks the email and password and starts a session.
// It returns the token to put in the cookie; only its hash is stored.
func (s *Service) Login(ctx context.Context, email, password string) (Member, string, error) {
	row, err := s.q.GetStaffByEmail(ctx, strings.TrimSpace(email))
	if errors.Is(err, pgx.ErrNoRows) {
		// Hash anyway so a wrong email takes as long as a wrong password.
		_, _ = CheckPassword(password, dummyHash)
		return Member{}, "", ErrBadLogin
	}
	if err != nil {
		return Member{}, "", fmt.Errorf("get staff: %w", err)
	}
	ok, err := CheckPassword(password, row.PasswordHash)
	if err != nil {
		return Member{}, "", err
	}
	if !ok || !row.Active {
		return Member{}, "", ErrBadLogin
	}

	token, hash, err := newToken()
	if err != nil {
		return Member{}, "", err
	}
	err = s.q.CreateSession(ctx, sqlc.CreateSessionParams{
		TokenHash: hash,
		StaffID:   row.ID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(SessionLength), Valid: true},
	})
	if err != nil {
		return Member{}, "", fmt.Errorf("create session: %w", err)
	}
	_ = s.q.TouchStaffLogin(ctx, row.ID)
	_ = s.q.DeleteExpiredSessions(ctx, IdleTimeout.Seconds())
	return Member{ID: row.ID, Email: row.Email, Name: row.Name, Role: Role(row.Role)}, token, nil
}

// Authenticate returns the staff member a session token belongs to.
func (s *Service) Authenticate(ctx context.Context, token string) (Member, error) {
	if token == "" {
		return Member{}, ErrNoSession
	}
	hash := hashToken(token)
	row, err := s.q.GetSessionStaff(ctx, sqlc.GetSessionStaffParams{TokenHash: hash, IdleSeconds: IdleTimeout.Seconds()})
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrNoSession
	}
	if err != nil {
		return Member{}, fmt.Errorf("get session: %w", err)
	}
	if err := s.q.TouchSession(ctx, hash); err != nil {
		return Member{}, fmt.Errorf("touch session: %w", err)
	}
	return Member{ID: row.ID, Email: row.Email, Name: row.Name, Role: Role(row.Role)}, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.q.DeleteSession(ctx, hashToken(token))
}

// Create adds a staff account. Used by the Staff screen and by the create-owner command.
func (s *Service) Create(ctx context.Context, email, name string, role Role, password string) (int64, error) {
	email = strings.TrimSpace(email)
	name = strings.TrimSpace(name)
	if _, err := mail.ParseAddress(email); err != nil || name == "" || !ValidRole(role) || len(password) < MinPasswordLength {
		return 0, ErrInvalid
	}
	hash, err := HashPassword(password)
	if err != nil {
		return 0, err
	}
	id, err := s.q.CreateStaff(ctx, sqlc.CreateStaffParams{Email: email, Name: name, PasswordHash: hash, Role: sqlc.StaffRole(role)})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return 0, ErrEmailTaken
	}
	return id, err
}

func (s *Service) List(ctx context.Context) ([]Account, error) {
	rows, err := s.q.ListStaff(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Account, 0, len(rows))
	for _, r := range rows {
		a := Account{
			Member:    Member{ID: r.ID, Email: r.Email, Name: r.Name, Role: Role(r.Role)},
			Active:    r.Active,
			CreatedAt: r.CreatedAt.Time,
		}
		if r.LastLoginAt.Valid {
			t := r.LastLoginAt.Time
			a.LastLoginAt = &t
		}
		out = append(out, a)
	}
	return out, nil
}

// Update changes a staff member's name, role or active flag.
// Deactivating someone signs them out everywhere. The last active owner can't be demoted or deactivated.
func (s *Service) Update(ctx context.Context, id int64, name string, role Role, active bool) error {
	name = strings.TrimSpace(name)
	if name == "" || !ValidRole(role) {
		return ErrInvalid
	}
	cur, err := s.q.GetStaffByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	losingOwner := Role(cur.Role) == RoleOwner && cur.Active && (role != RoleOwner || !active)
	if losingOwner {
		n, err := s.q.CountActiveOwners(ctx)
		if err != nil {
			return err
		}
		if n <= 1 {
			return ErrLastOwner
		}
	}
	if err := s.q.UpdateStaff(ctx, sqlc.UpdateStaffParams{ID: id, Name: name, Role: sqlc.StaffRole(role), Active: active}); err != nil {
		return err
	}
	if !active {
		return s.q.DeleteStaffSessions(ctx, id)
	}
	return nil
}

// SetPassword replaces a password and signs the account out everywhere.
func (s *Service) SetPassword(ctx context.Context, id int64, password string) error {
	if len(password) < MinPasswordLength {
		return ErrInvalid
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	if err := s.q.SetStaffPassword(ctx, sqlc.SetStaffPasswordParams{ID: id, PasswordHash: hash}); err != nil {
		return err
	}
	return s.q.DeleteStaffSessions(ctx, id)
}

// Audit records an admin change. details is any JSON-encodable value.
func Audit(ctx context.Context, q *sqlc.Queries, by Member, action, entity string, entityID int64, details any) error {
	b, err := json.Marshal(details)
	if err != nil {
		return err
	}
	return q.InsertAudit(ctx, sqlc.InsertAuditParams{
		StaffID:  pgtype.Int8{Int64: by.ID, Valid: by.ID != 0},
		Action:   action,
		Entity:   entity,
		EntityID: pgtype.Int8{Int64: entityID, Valid: entityID != 0},
		Details:  b,
	})
}

func newToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token), nil
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// dummyHash is checked when the email doesn't exist, so timing doesn't reveal which emails are staff.
var dummyHash = func() string {
	h, _ := HashPassword("not-a-real-password")
	return h
}()
