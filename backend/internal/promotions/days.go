package promotions

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/delivery"
)

// Staff pick whole days in Lagos time. A start day begins at midnight; an end day is
// included, so it ends at the following midnight.

// DayStart turns "2026-12-01" into midnight that day in Lagos. "" means no start.
func DayStart(field, day string) (pgtype.Timestamptz, error) {
	if day == "" {
		return pgtype.Timestamptz{}, nil
	}
	t, err := time.ParseInLocation(time.DateOnly, day, delivery.Lagos)
	if err != nil {
		return pgtype.Timestamptz{}, apperr.NewInvalid(field, "Pick a date.")
	}
	return pgtype.Timestamptz{Time: t, Valid: true}, nil
}

// DayEnd turns an inclusive last day into the midnight after it. "" means no end.
func DayEnd(field, day string) (pgtype.Timestamptz, error) {
	t, err := DayStart(field, day)
	if err != nil || !t.Valid {
		return t, err
	}
	t.Time = t.Time.AddDate(0, 0, 1)
	return t, nil
}

// StartDay is the reverse of DayStart: "2026-12-01", or "" when unset.
func StartDay(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.In(delivery.Lagos).Format(time.DateOnly)
}

// EndDay is the reverse of DayEnd: the last included day, or "" when unset.
func EndDay(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.In(delivery.Lagos).AddDate(0, 0, -1).Format(time.DateOnly)
}

// CheckDays makes sure an end isn't before its start.
func CheckDays(start, end pgtype.Timestamptz) error {
	if start.Valid && end.Valid && !end.Time.After(start.Time) {
		return apperr.NewInvalid("endsOn", "The last day can't be before the first day.")
	}
	return nil
}

// Live reports whether now falls inside optional start and end times.
func Live(active bool, start, end pgtype.Timestamptz) bool {
	now := time.Now()
	return active && (!start.Valid || !start.Time.After(now)) && (!end.Valid || end.Time.After(now))
}
