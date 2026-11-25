package orders

import (
	"context"

	"yetibyrash/backend/internal/db/sqlc"
)

// giveBackOffers returns the discount-code use and free gift an order took at checkout,
// when it goes unpaid or is cancelled. (Its items, gift included, are restocked separately.)
func giveBackOffers(ctx context.Context, q *sqlc.Queries, orderID int64) error {
	o, err := q.GetOrderOffers(ctx, orderID)
	if err != nil {
		return err
	}
	if o.DiscountCode != "" {
		if err := q.ReturnCode(ctx, o.DiscountCode); err != nil {
			return err
		}
	}
	if o.GiftOfferID.Valid {
		return q.ReturnGift(ctx, o.GiftOfferID.Int64)
	}
	return nil
}

// retakeOffers is the reverse, for an abandoned order whose payment arrived late.
func retakeOffers(ctx context.Context, q *sqlc.Queries, orderID int64) error {
	o, err := q.GetOrderOffers(ctx, orderID)
	if err != nil {
		return err
	}
	if o.DiscountCode != "" {
		if err := q.RetakeCode(ctx, o.DiscountCode); err != nil {
			return err
		}
	}
	if o.GiftOfferID.Valid {
		return q.GiveGift(ctx, o.GiftOfferID.Int64)
	}
	return nil
}
