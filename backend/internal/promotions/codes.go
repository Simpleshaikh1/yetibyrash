package promotions

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"yetibyrash/backend/internal/db/sqlc"
)

// CodeResult is what a discount code gives a bag right now.
type CodeResult struct {
	Code         string `json:"code"`
	DiscountKobo int64  `json:"discountKobo"` // off the items
	FreeDelivery bool   `json:"freeDelivery"`
	Summary      string `json:"summary"` // "10% off", "₦5,000 off", "Free delivery"
	// Problem explains why the code gives nothing yet ("Spend ₦30,000 to use this code").
	// The code stays on the bag, so it starts working once the bag qualifies.
	Problem string `json:"problem"`
}

// CodeCheck is everything a code can depend on. Zone and customer are unknown in the bag
// and filled in at checkout.
type CodeCheck struct {
	SubtotalKobo int64
	ZoneID       int64  // 0 = not chosen yet
	Phone, Email string // "" = not entered yet
}

// NormalizeCode upper-cases a typed code and drops spaces.
func NormalizeCode(s string) string {
	return strings.ToUpper(strings.Join(strings.Fields(s), ""))
}

// ErrNoSuchCode means the code doesn't exist, is switched off, or is outside its dates.
var ErrNoSuchCode = errors.New("no such code")

// EvaluateCode works out a code against a bag. Pass a locking query (LockCode) inside the
// checkout transaction so the use count can't be overtaken.
func EvaluateCode(ctx context.Context, q *sqlc.Queries, code string, c CodeCheck, lock bool) (CodeResult, int64, error) {
	code = NormalizeCode(code)
	var row sqlc.GetCodeRow
	var err error
	if lock {
		var r sqlc.LockCodeRow
		r, err = q.LockCode(ctx, code)
		row = sqlc.GetCodeRow(r)
	} else {
		row, err = q.GetCode(ctx, code)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return CodeResult{}, 0, ErrNoSuchCode
	}
	if err != nil {
		return CodeResult{}, 0, fmt.Errorf("get code: %w", err)
	}
	now := time.Now()
	if !row.Active || (row.StartsAt.Valid && row.StartsAt.Time.After(now)) || (row.EndsAt.Valid && !row.EndsAt.Time.After(now)) {
		return CodeResult{}, 0, ErrNoSuchCode
	}

	res := CodeResult{Code: row.Code, Summary: Summary(row.Kind, row.Value)}
	switch {
	case row.MaxUses.Valid && row.Used >= row.MaxUses.Int32:
		res.Problem = "This code has been fully used."
	case c.SubtotalKobo < row.MinOrderKobo:
		res.Problem = fmt.Sprintf("Spend %s or more to use this code.", Naira(row.MinOrderKobo))
	case row.Kind == sqlc.DiscountKindFreeDelivery && row.ZoneID.Valid && c.ZoneID != 0 && c.ZoneID != row.ZoneID.Int64:
		res.Problem = "This code gives free delivery in another area only."
	}
	if res.Problem == "" && row.FirstOrderOnly && (c.Phone != "" || c.Email != "") {
		n, err := q.CountPaidOrdersFor(ctx, sqlc.CountPaidOrdersForParams{Phone: c.Phone, Email: c.Email})
		if err != nil {
			return CodeResult{}, 0, fmt.Errorf("count orders: %w", err)
		}
		if n > 0 {
			res.Problem = "This code is for a first order only."
		}
	}
	if res.Problem != "" {
		return res, row.ID, nil
	}

	switch row.Kind {
	case sqlc.DiscountKindPercent:
		res.DiscountKobo = c.SubtotalKobo * row.Value / 100 / 100 * 100
	case sqlc.DiscountKindFixed:
		res.DiscountKobo = row.Value
	case sqlc.DiscountKindFreeDelivery:
		res.FreeDelivery = true
	}
	// Never take the items below ₦100: Paystack needs something to charge.
	res.DiscountKobo = max(0, min(res.DiscountKobo, c.SubtotalKobo-10000))
	return res, row.ID, nil
}

// Summary describes a code's offer in a few words.
func Summary(kind sqlc.DiscountKind, value int64) string {
	switch kind {
	case sqlc.DiscountKindPercent:
		return fmt.Sprintf("%d%% off", value)
	case sqlc.DiscountKindFixed:
		return Naira(value) + " off"
	}
	return "Free delivery"
}

// Naira formats kobo as "₦14,500".
func Naira(kobo int64) string {
	s := fmt.Sprint(kobo / 100)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return "₦" + s
}
