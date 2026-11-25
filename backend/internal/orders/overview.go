package orders

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"yetibyrash/backend/internal/db/sqlc"
	"yetibyrash/backend/internal/delivery"
	"yetibyrash/backend/internal/jobs"
)

type DaySales struct {
	Day       string `json:"day"` // "2026-11-25"
	TotalKobo int64  `json:"totalKobo"`
}

type LateOrder struct {
	ID     int64     `json:"id"`
	Number string    `json:"number"`
	Name   string    `json:"name"`
	PaidAt time.Time `json:"paidAt"`
}

type LowStockItem struct {
	ProductID int64  `json:"productId"`
	Name      string `json:"name"`
	Option    string `json:"option"`
	Stock     int    `json:"stock"`
}

type ProblemPayment struct {
	Reference  string    `json:"reference"`
	OrderID    *int64    `json:"orderId"`
	AmountKobo int64     `json:"amountKobo"`
	Outcome    string    `json:"outcome"`
	At         time.Time `json:"at"`
}

// Overview is the admin's first screen: today's numbers and what needs attention.
type Overview struct {
	SalesTodayKobo  int64            `json:"salesTodayKobo"`
	OrdersToday     int              `json:"ordersToday"`
	ToPack          int              `json:"toPack"`
	Packed          int              `json:"packed"`
	OnTheWay        int              `json:"onTheWay"`
	OutForDelivery  int              `json:"outForDelivery"`
	LateToPack      []LateOrder      `json:"lateToPack"`
	LowStock        []LowStockItem   `json:"lowStock"`
	SoldOut         int              `json:"soldOut"`
	ProblemPayments []ProblemPayment `json:"problemPayments"`
	FailedEmails    int              `json:"failedEmails"`
	Last7Days       []DaySales       `json:"last7Days"`
}

func (s *Service) Overview(ctx context.Context) (Overview, error) {
	var ov Overview
	now := time.Now().In(delivery.Lagos)
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, delivery.Lagos)

	today, err := s.q.SalesSince(ctx, pgtype.Timestamptz{Time: midnight, Valid: true})
	if err != nil {
		return ov, err
	}
	ov.SalesTodayKobo, ov.OrdersToday = today.TotalKobo, int(today.Orders)

	counts, err := s.q.CountOrdersByStatus(ctx)
	if err != nil {
		return ov, err
	}
	for _, c := range counts {
		switch c.Status {
		case sqlc.OrderStatusPaid:
			ov.ToPack = int(c.N)
		case sqlc.OrderStatusPacked:
			ov.Packed = int(c.N)
		case sqlc.OrderStatusShipped:
			ov.OnTheWay += int(c.N)
		case sqlc.OrderStatusOutForDelivery:
			ov.OnTheWay += int(c.N)
			ov.OutForDelivery = int(c.N)
		}
	}

	late, err := s.q.LateToPack(ctx)
	if err != nil {
		return ov, err
	}
	ov.LateToPack = []LateOrder{}
	for _, l := range late {
		ov.LateToPack = append(ov.LateToPack, LateOrder{ID: l.ID, Number: l.Number, Name: l.CustomerName, PaidAt: l.PaidAt.Time})
	}

	threshold := 6
	if raw, err := s.q.GetStoreSetting(ctx, "low_stock_threshold"); err == nil {
		_ = json.Unmarshal(raw.Value, &threshold)
	}
	low, err := s.q.LowStock(ctx, int32(threshold))
	if err != nil {
		return ov, err
	}
	ov.LowStock = []LowStockItem{}
	for _, l := range low {
		if l.Stock == 0 {
			ov.SoldOut++
		}
		if len(ov.LowStock) < 8 {
			ov.LowStock = append(ov.LowStock, LowStockItem{ProductID: l.ProductID, Name: l.Name, Option: l.Option, Stock: int(l.Stock)})
		}
	}

	pays, err := s.q.ProblemPayments(ctx)
	if err != nil {
		return ov, err
	}
	ov.ProblemPayments = []ProblemPayment{}
	for _, p := range pays {
		pp := ProblemPayment{Reference: p.Reference, AmountKobo: p.AmountKobo, Outcome: p.Outcome, At: p.ReceivedAt.Time}
		if p.OrderID.Valid {
			pp.OrderID = &p.OrderID.Int64
		}
		ov.ProblemPayments = append(ov.ProblemPayments, pp)
	}

	failed, err := s.q.CountFailedJobs(ctx, jobs.MaxAttempts)
	if err != nil {
		return ov, err
	}
	ov.FailedEmails = int(failed)

	days, err := s.q.DailySales(ctx)
	if err != nil {
		return ov, err
	}
	ov.Last7Days = []DaySales{}
	for _, d := range days {
		ov.Last7Days = append(ov.Last7Days, DaySales{Day: d.Day.Time.Format(time.DateOnly), TotalKobo: d.TotalKobo})
	}
	return ov, nil
}
