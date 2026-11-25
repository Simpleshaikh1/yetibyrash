package delivery

import (
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }

var monToSat = map[time.Weekday]bool{
	time.Monday: true, time.Tuesday: true, time.Wednesday: true,
	time.Thursday: true, time.Friday: true, time.Saturday: true,
}

var lagosZone = Zone{
	Name: "Lagos", StandardFeeKobo: 250000, StandardDaysMin: 1, StandardDaysMax: 2,
	ExpressFeeKobo: ptr(int64(400000)), ExpressDays: ptr(0), FreeOverKobo: ptr(int64(4000000)),
}

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, Lagos)
	if err != nil {
		panic(err)
	}
	return t
}

func TestOptionsBeforeCutoff(t *testing.T) {
	r := Rules{Cutoff: 12 * time.Hour, DispatchDays: monToSat, Holidays: map[string]bool{}}
	// Wednesday 25 Nov 2026, 10:00: dispatched today.
	opts := r.Options(lagosZone, true, 2650000, at("2026-11-25 10:00"))
	if len(opts) != 2 {
		t.Fatalf("got %d options, want standard and express", len(opts))
	}
	std, ex := opts[0], opts[1]
	if std.FeeKobo != 250000 || std.From != "2026-11-26" || std.To != "2026-11-27" {
		t.Fatalf("standard = %+v", std)
	}
	if ex.FeeKobo != 400000 || ex.From != "2026-11-25" || ex.Label != "Wed 25 Nov" {
		t.Fatalf("express = %+v, want same day", ex)
	}
}

func TestOptionsAfterCutoffSkipsSunday(t *testing.T) {
	r := Rules{Cutoff: 12 * time.Hour, DispatchDays: monToSat, Holidays: map[string]bool{}}
	// Saturday 28 Nov, 15:00: past cut-off, Sunday is closed, so dispatch is Monday 30 Nov.
	opts := r.Options(lagosZone, true, 100000, at("2026-11-28 15:00"))
	if opts[1].From != "2026-11-30" {
		t.Fatalf("express = %s, want Monday 30 Nov", opts[1].From)
	}
	if opts[0].From != "2026-12-01" || opts[0].To != "2026-12-02" {
		t.Fatalf("standard = %s–%s, want 1–2 Dec", opts[0].From, opts[0].To)
	}
}

func TestHolidaysAreSkipped(t *testing.T) {
	r := Rules{Cutoff: 12 * time.Hour, DispatchDays: monToSat, Holidays: map[string]bool{"2026-12-25": true, "2026-12-26": true}}
	// Thursday 24 Dec, 09:00. Dispatch today; 25 (Christmas), 26 (Boxing Day) and 27 (Sunday) are skipped.
	opts := r.Options(lagosZone, false, 100000, at("2026-12-24 09:00"))
	if opts[0].From != "2026-12-28" || opts[0].To != "2026-12-29" {
		t.Fatalf("standard = %s–%s, want 28–29 Dec", opts[0].From, opts[0].To)
	}
}

func TestFreeDeliveryAndNoExpress(t *testing.T) {
	r := Rules{Cutoff: 12 * time.Hour, DispatchDays: monToSat, Holidays: map[string]bool{}}
	opts := r.Options(lagosZone, false, 4000000, at("2026-11-25 10:00"))
	if len(opts) != 1 {
		t.Fatalf("got %d options; express must not show in an LGA without express", len(opts))
	}
	if !opts[0].Free || opts[0].FeeKobo != 0 {
		t.Fatalf("standard at the threshold should be free: %+v", opts[0])
	}
}

func TestCutoffToday(t *testing.T) {
	r := Rules{Cutoff: 12 * time.Hour, DispatchDays: monToSat, Holidays: map[string]bool{}}
	if cut, ok := r.CutoffToday(at("2026-11-25 08:40")); !ok || cut.Hour() != 12 {
		t.Fatalf("Wednesday morning: cut=%v ok=%v", cut, ok)
	}
	if _, ok := r.CutoffToday(at("2026-11-25 12:00")); ok {
		t.Fatal("at noon the cut-off has passed")
	}
	if _, ok := r.CutoffToday(at("2026-11-29 09:00")); ok {
		t.Fatal("no cut-off on Sunday")
	}
}
