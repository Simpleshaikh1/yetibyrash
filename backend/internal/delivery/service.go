package delivery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"yetibyrash/backend/internal/apperr"
	"yetibyrash/backend/internal/db/sqlc"
)

type Service struct {
	q   *sqlc.Queries
	now func() time.Time
}

func NewService(db sqlc.DBTX) *Service { return &Service{q: sqlc.New(db), now: time.Now} }

type Lga struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Express   bool   `json:"express"`
	Delivered bool   `json:"delivered"`
}

type State struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	ZoneID   int64  `json:"zoneId"`
	ZoneName string `json:"zoneName"`
	Lgas     []Lga  `json:"lgas"` // empty: checkout asks for the LGA or town as text
}

// Rules loads the shop-wide timing rules.
func (s *Service) Rules(ctx context.Context) (Rules, error) {
	set, err := s.q.GetDeliverySettings(ctx)
	if err != nil {
		return Rules{}, fmt.Errorf("delivery settings: %w", err)
	}
	r := Rules{
		Cutoff:       time.Duration(set.ExpressCutoff.Microseconds) * time.Microsecond,
		DispatchDays: map[time.Weekday]bool{},
		Holidays:     map[string]bool{},
	}
	for _, d := range set.DispatchDays {
		r.DispatchDays[time.Weekday(d%7)] = true // ISO 7 = Sunday = Go's 0
	}
	hols, err := s.q.ListUpcomingHolidays(ctx)
	if err != nil {
		return Rules{}, fmt.Errorf("holidays: %w", err)
	}
	for _, h := range hols {
		r.Holidays[h.Day.Time.Format(time.DateOnly)] = true
	}
	return r, nil
}

func (s *Service) Zones(ctx context.Context) ([]Zone, error) {
	rows, err := s.q.ListZones(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Zone, 0, len(rows))
	for _, z := range rows {
		out = append(out, zoneFrom(z))
	}
	return out, nil
}

func zoneFrom(z sqlc.DeliveryZone) Zone {
	out := Zone{
		ID: z.ID, Name: z.Name, StandardFeeKobo: z.StandardFeeKobo,
		StandardDaysMin: int(z.StandardDaysMin), StandardDaysMax: int(z.StandardDaysMax),
	}
	if z.ExpressFeeKobo.Valid {
		out.ExpressFeeKobo = &z.ExpressFeeKobo.Int64
	}
	if z.ExpressDays.Valid {
		d := int(z.ExpressDays.Int32)
		out.ExpressDays = &d
	}
	if z.FreeOverKobo.Valid {
		out.FreeOverKobo = &z.FreeOverKobo.Int64
	}
	return out
}

// Locations lists every state with its LGAs. withHidden includes LGAs we don't deliver to (admin).
func (s *Service) Locations(ctx context.Context, withHidden bool) ([]State, error) {
	zones, err := s.Zones(ctx)
	if err != nil {
		return nil, err
	}
	zoneName := map[int64]string{}
	for _, z := range zones {
		zoneName[z.ID] = z.Name
	}
	states, err := s.q.ListStates(ctx)
	if err != nil {
		return nil, err
	}
	lgas, err := s.q.ListLgas(ctx)
	if err != nil {
		return nil, err
	}
	byState := map[int64][]Lga{}
	for _, l := range lgas {
		if l.Delivered || withHidden {
			byState[l.StateID] = append(byState[l.StateID], Lga{ID: l.ID, Name: l.Name, Express: l.Express, Delivered: l.Delivered})
		}
	}
	out := make([]State, 0, len(states))
	for _, st := range states {
		ls := byState[st.ID]
		if ls == nil {
			ls = []Lga{}
		}
		out = append(out, State{ID: st.ID, Name: st.Name, ZoneID: st.ZoneID, ZoneName: zoneName[st.ZoneID], Lgas: ls})
	}
	return out, nil
}

// Destination is where an order goes, after checking it against the delivery tables.
type Destination struct {
	StateName   string
	LgaName     string
	Zone        Zone
	ExpressHere bool
}

// Resolve checks the state and LGA the customer picked. States with listed LGAs need one of them;
// other states take the LGA or town as typed.
func (s *Service) Resolve(ctx context.Context, stateID int64, lgaID int64, lgaText string) (Destination, error) {
	st, err := s.q.GetState(ctx, stateID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Destination{}, apperr.NewInvalid("stateId", "Choose your state.")
	}
	if err != nil {
		return Destination{}, err
	}
	z, err := s.q.GetZone(ctx, st.ZoneID)
	if err != nil {
		return Destination{}, err
	}
	d := Destination{StateName: st.Name, Zone: zoneFrom(z)}

	listed, err := s.q.CountLgasInState(ctx, stateID)
	if err != nil {
		return Destination{}, err
	}
	if listed > 0 {
		l, err := s.q.GetLga(ctx, lgaID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && l.StateID != stateID) {
			return Destination{}, apperr.NewInvalid("lgaId", "Choose your LGA.")
		}
		if err != nil {
			return Destination{}, err
		}
		if !l.Delivered {
			return Destination{}, apperr.NewInvalid("lgaId", fmt.Sprintf("We don't deliver to %s yet. Message us on WhatsApp and we'll try to help.", l.Name))
		}
		d.LgaName, d.ExpressHere = l.Name, l.Express
		return d, nil
	}
	lgaText = strings.TrimSpace(lgaText)
	if lgaText == "" || len(lgaText) > 80 {
		return Destination{}, apperr.NewInvalid("lgaText", "Enter your LGA or town.")
	}
	d.LgaName = lgaText
	return d, nil
}

// Options returns the delivery choices for a destination and bag subtotal.
func (s *Service) Options(ctx context.Context, d Destination, subtotalKobo int64) ([]Option, error) {
	r, err := s.Rules(ctx)
	if err != nil {
		return nil, err
	}
	return r.Options(d.Zone, d.ExpressHere, subtotalKobo, s.now()), nil
}

// ZoneSummary is one zone as the product page and bag describe it.
type ZoneSummary struct {
	Name         string   `json:"name"`
	FeeKobo      int64    `json:"feeKobo"`
	Days         string   `json:"days"` // "1–2"
	FreeOverKobo *int64   `json:"freeOverKobo"`
	States       []string `json:"states"`
}

// Summary is what the product page and bag show about delivery.
type Summary struct {
	CutoffAt *time.Time    `json:"cutoffAt"` // today's cut-off if still ahead, for "Order within…"
	Lagos    *Option       `json:"lagos"`    // standard delivery to Lagos if ordered now
	Zones    []ZoneSummary `json:"zones"`
}

func (s *Service) Summary(ctx context.Context) (Summary, error) {
	r, err := s.Rules(ctx)
	if err != nil {
		return Summary{}, err
	}
	zones, err := s.Zones(ctx)
	if err != nil {
		return Summary{}, err
	}
	states, err := s.q.ListStates(ctx)
	if err != nil {
		return Summary{}, err
	}
	now := s.now()
	out := Summary{Zones: []ZoneSummary{}}
	if cut, ok := r.CutoffToday(now); ok {
		out.CutoffAt = &cut
	}
	for _, z := range zones {
		zs := ZoneSummary{Name: z.Name, FeeKobo: z.StandardFeeKobo, FreeOverKobo: z.FreeOverKobo, States: []string{}}
		zs.Days = fmt.Sprint(z.StandardDaysMin)
		if z.StandardDaysMax != z.StandardDaysMin {
			zs.Days += "–" + fmt.Sprint(z.StandardDaysMax)
		}
		for _, st := range states {
			if st.ZoneID == z.ID {
				zs.States = append(zs.States, st.Name)
			}
		}
		out.Zones = append(out.Zones, zs)
		if z.Name == "Lagos" {
			opt := r.Options(z, false, 0, now)[0]
			out.Lagos = &opt
		}
	}
	return out, nil
}

// ---------- Couriers ----------

type Courier struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	TrackingURL string `json:"trackingUrl"` // may contain {tracking_number}
	Active      bool   `json:"active"`
}

func (s *Service) Couriers(ctx context.Context) ([]Courier, error) {
	rows, err := s.q.ListCouriers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Courier, 0, len(rows))
	for _, c := range rows {
		out = append(out, Courier{ID: c.ID, Name: c.Name, TrackingURL: c.TrackingUrl, Active: c.Active})
	}
	return out, nil
}

// SaveCourier adds (id 0) or edits a courier. Couriers are switched off rather than deleted,
// because past orders point at them.
func (s *Service) SaveCourier(ctx context.Context, c Courier) (int64, error) {
	c.Name = strings.TrimSpace(c.Name)
	c.TrackingURL = strings.TrimSpace(c.TrackingURL)
	if c.Name == "" {
		return 0, apperr.NewInvalid("name", "Give the courier a name.")
	}
	if c.TrackingURL != "" && !strings.HasPrefix(c.TrackingURL, "https://") && !strings.HasPrefix(c.TrackingURL, "http://") {
		return 0, apperr.NewInvalid("trackingUrl", "The tracking link must start with https://")
	}
	if c.ID == 0 {
		return s.q.InsertCourier(ctx, sqlc.InsertCourierParams{Name: c.Name, TrackingUrl: c.TrackingURL, Active: c.Active})
	}
	return c.ID, s.q.UpdateCourier(ctx, sqlc.UpdateCourierParams{ID: c.ID, Name: c.Name, TrackingUrl: c.TrackingURL, Active: c.Active})
}

// ---------- Admin ----------

type Settings struct {
	ExpressCutoff string `json:"expressCutoff"` // "12:00"
	DispatchDays  []int  `json:"dispatchDays"`  // ISO weekdays, 1 = Monday
}

type Holiday struct {
	Day  string `json:"day"` // "2026-12-25"
	Name string `json:"name"`
}

type AdminView struct {
	Zones    []Zone    `json:"zones"`
	States   []State   `json:"states"`
	Settings Settings  `json:"settings"`
	Holidays []Holiday `json:"holidays"`
}

func (s *Service) Admin(ctx context.Context) (AdminView, error) {
	var v AdminView
	var err error
	if v.Zones, err = s.Zones(ctx); err != nil {
		return v, err
	}
	if v.States, err = s.Locations(ctx, true); err != nil {
		return v, err
	}
	set, err := s.q.GetDeliverySettings(ctx)
	if err != nil {
		return v, err
	}
	mins := set.ExpressCutoff.Microseconds / 60_000_000
	v.Settings = Settings{ExpressCutoff: fmt.Sprintf("%02d:%02d", mins/60, mins%60), DispatchDays: []int{}}
	for _, d := range set.DispatchDays {
		v.Settings.DispatchDays = append(v.Settings.DispatchDays, int(d))
	}
	hols, err := s.q.ListUpcomingHolidays(ctx)
	if err != nil {
		return v, err
	}
	v.Holidays = []Holiday{}
	for _, h := range hols {
		v.Holidays = append(v.Holidays, Holiday{Day: h.Day.Time.Format(time.DateOnly), Name: h.Name})
	}
	return v, nil
}

func (s *Service) UpdateZone(ctx context.Context, z Zone) error {
	z.Name = strings.TrimSpace(z.Name)
	switch {
	case z.Name == "":
		return apperr.NewInvalid("name", "Give the zone a name.")
	case z.StandardFeeKobo < 0:
		return apperr.NewInvalid("standardFeeKobo", "The fee can't be negative.")
	case z.StandardDaysMin < 0 || z.StandardDaysMax < z.StandardDaysMin || z.StandardDaysMax > 30:
		return apperr.NewInvalid("standardDays", "Days must run from the shorter to the longer, up to 30.")
	case (z.ExpressFeeKobo == nil) != (z.ExpressDays == nil):
		return apperr.NewInvalid("express", "Set both an express fee and express days, or neither.")
	case z.ExpressFeeKobo != nil && (*z.ExpressFeeKobo < 0 || *z.ExpressDays < 0 || *z.ExpressDays > 14):
		return apperr.NewInvalid("express", "Express fee can't be negative; express days must be 0 to 14.")
	case z.FreeOverKobo != nil && *z.FreeOverKobo <= 0:
		return apperr.NewInvalid("freeOverKobo", "Leave free delivery blank, or set an amount above ₦0.")
	}
	p := sqlc.UpdateZoneParams{
		ID: z.ID, Name: z.Name, StandardFeeKobo: z.StandardFeeKobo,
		StandardDaysMin: int32(z.StandardDaysMin), StandardDaysMax: int32(z.StandardDaysMax),
	}
	if z.ExpressFeeKobo != nil {
		p.ExpressFeeKobo = pgtype.Int8{Int64: *z.ExpressFeeKobo, Valid: true}
		p.ExpressDays = pgtype.Int4{Int32: int32(*z.ExpressDays), Valid: true}
	}
	if z.FreeOverKobo != nil {
		p.FreeOverKobo = pgtype.Int8{Int64: *z.FreeOverKobo, Valid: true}
	}
	return s.q.UpdateZone(ctx, p)
}

func (s *Service) SetStateZone(ctx context.Context, stateID, zoneID int64) error {
	return s.q.SetStateZone(ctx, sqlc.SetStateZoneParams{ID: stateID, ZoneID: zoneID})
}

// SaveLga adds (id 0) or edits an LGA.
func (s *Service) SaveLga(ctx context.Context, id, stateID int64, l Lga) (int64, error) {
	l.Name = strings.TrimSpace(l.Name)
	if l.Name == "" {
		return 0, apperr.NewInvalid("name", "Give the LGA a name.")
	}
	if id == 0 {
		return s.q.InsertLga(ctx, sqlc.InsertLgaParams{StateID: stateID, Name: l.Name, Delivered: l.Delivered, Express: l.Express})
	}
	return id, s.q.UpdateLga(ctx, sqlc.UpdateLgaParams{ID: id, Name: l.Name, Delivered: l.Delivered, Express: l.Express})
}

func (s *Service) DeleteLga(ctx context.Context, id int64) error { return s.q.DeleteLga(ctx, id) }

func (s *Service) UpdateSettings(ctx context.Context, set Settings) error {
	t, err := time.Parse("15:04", set.ExpressCutoff)
	if err != nil {
		return apperr.NewInvalid("expressCutoff", "Enter the cut-off as a time, e.g. 12:00.")
	}
	days := []int32{}
	for _, d := range set.DispatchDays {
		if d < 1 || d > 7 {
			return apperr.NewInvalid("dispatchDays", "Dispatch days must be Monday to Sunday.")
		}
		days = append(days, int32(d))
	}
	if len(days) == 0 {
		return apperr.NewInvalid("dispatchDays", "Choose at least one dispatch day.")
	}
	us := int64(t.Hour()*3600+t.Minute()*60) * 1_000_000
	return s.q.UpdateDeliverySettings(ctx, sqlc.UpdateDeliverySettingsParams{
		ExpressCutoff: pgtype.Time{Microseconds: us, Valid: true}, DispatchDays: days,
	})
}

func (s *Service) AddHoliday(ctx context.Context, h Holiday) error {
	day, err := time.Parse(time.DateOnly, h.Day)
	if err != nil {
		return apperr.NewInvalid("day", "Choose a date.")
	}
	if strings.TrimSpace(h.Name) == "" {
		return apperr.NewInvalid("name", "Name the holiday, e.g. Christmas Day.")
	}
	return s.q.InsertHoliday(ctx, sqlc.InsertHolidayParams{Day: pgtype.Date{Time: day, Valid: true}, Name: strings.TrimSpace(h.Name)})
}

func (s *Service) DeleteHoliday(ctx context.Context, day string) error {
	d, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return apperr.ErrNotFound
	}
	return s.q.DeleteHoliday(ctx, pgtype.Date{Time: d, Valid: true})
}
