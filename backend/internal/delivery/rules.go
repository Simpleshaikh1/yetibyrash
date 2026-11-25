// Package delivery works out delivery fees, options and dates.
// Dates are calculated in Lagos time and skip non-dispatch days and public holidays.
package delivery

import (
	"fmt"
	"time"
)

// Lagos is the shop's time zone (West Africa Time, UTC+1, no daylight saving).
var Lagos = func() *time.Location {
	if loc, err := time.LoadLocation("Africa/Lagos"); err == nil {
		return loc
	}
	return time.FixedZone("WAT", 60*60) // fallback when the OS has no time zone database
}()

type Speed string

const (
	Standard Speed = "standard"
	Express  Speed = "express"
)

// Zone is a group of states sharing delivery fees and times.
type Zone struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	StandardFeeKobo int64  `json:"standardFeeKobo"`
	StandardDaysMin int    `json:"standardDaysMin"`
	StandardDaysMax int    `json:"standardDaysMax"`
	ExpressFeeKobo  *int64 `json:"expressFeeKobo"` // nil = no express in this zone
	ExpressDays     *int   `json:"expressDays"`    // 0 = same day if ordered before the cut-off
	FreeOverKobo    *int64 `json:"freeOverKobo"`   // standard is free at or above this subtotal
}

// Rules are the shop-wide timing rules.
type Rules struct {
	Cutoff       time.Duration         // time of day, e.g. 12h for 12 noon
	DispatchDays map[time.Weekday]bool // days parcels leave
	Holidays     map[string]bool       // "2026-12-25"
}

// Option is one way to deliver an order, as shown at checkout.
type Option struct {
	Speed   Speed  `json:"speed"`
	FeeKobo int64  `json:"feeKobo"`
	Free    bool   `json:"free"`
	From    string `json:"from"` // "2026-11-26"
	To      string `json:"to"`
	Label   string `json:"label"` // "Thu 26 Nov – Fri 27 Nov"
}

func (r Rules) workingDay(d time.Time) bool {
	return r.DispatchDays[d.Weekday()] && !r.Holidays[d.Format(time.DateOnly)]
}

// dispatchDay is the day an order placed at `now` leaves: today if it's a working day
// and before the cut-off, otherwise the next working day.
func (r Rules) dispatchDay(now time.Time) time.Time {
	now = now.In(Lagos)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, Lagos)
	if r.workingDay(day) && now.Sub(day) < r.Cutoff {
		return day
	}
	return r.addWorkingDays(day, 1)
}

// addWorkingDays moves n working days forward from d (n = 0 returns d).
func (r Rules) addWorkingDays(d time.Time, n int) time.Time {
	for i := 0; i < 400 && n > 0; i++ { // 400: guard against a calendar with no working days
		d = d.AddDate(0, 0, 1)
		if r.workingDay(d) {
			n--
		}
	}
	return d
}

// Options lists the delivery choices for a zone. expressHere says whether the
// customer's LGA gets express; subtotalKobo decides free standard delivery.
func (r Rules) Options(z Zone, expressHere bool, subtotalKobo int64, now time.Time) []Option {
	dispatch := r.dispatchDay(now)
	std := Option{Speed: Standard, FeeKobo: z.StandardFeeKobo}
	if z.FreeOverKobo != nil && subtotalKobo >= *z.FreeOverKobo {
		std.FeeKobo, std.Free = 0, true
	}
	std.setDates(r.addWorkingDays(dispatch, z.StandardDaysMin), r.addWorkingDays(dispatch, z.StandardDaysMax))
	opts := []Option{std}

	if expressHere && z.ExpressFeeKobo != nil && z.ExpressDays != nil {
		day := r.addWorkingDays(dispatch, *z.ExpressDays)
		ex := Option{Speed: Express, FeeKobo: *z.ExpressFeeKobo}
		ex.setDates(day, day)
		opts = append(opts, ex)
	}
	return opts
}

// CutoffToday returns today's cut-off time if it's a working day and it hasn't passed.
// The product page uses it for "Order within 3h 20m".
func (r Rules) CutoffToday(now time.Time) (time.Time, bool) {
	now = now.In(Lagos)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, Lagos)
	cut := day.Add(r.Cutoff)
	if !r.workingDay(day) || !now.Before(cut) {
		return time.Time{}, false
	}
	return cut, true
}

func (o *Option) setDates(from, to time.Time) {
	o.From = from.Format(time.DateOnly)
	o.To = to.Format(time.DateOnly)
	if from.Equal(to) {
		o.Label = from.Format("Mon 2 Jan")
	} else {
		o.Label = fmt.Sprintf("%s – %s", from.Format("Mon 2 Jan"), to.Format("Mon 2 Jan"))
	}
}
