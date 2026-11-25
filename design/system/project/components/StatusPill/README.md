# StatusPill

An admin label for the state of a banner, product, review, payment or order.

- 12.5px text at 600 weight, 4px × 10px padding, `radius-pill`.
- **Live / Visible / Paid / Published**: `success-tint` with `success` text.
- **Scheduled / Waiting / Late**: `warning-tint` with `warning` text. Late order cards also get a 2px `warning-edge` outline.
- **Hidden / Ended / Draft**: `line-admin` fill with `ink-secondary` text.
- **Sold out / No order saved**: `danger-tint` with `danger` text.
- Admin only. Shoppers see order states in the OrderTracker instead.

The consumer provides the state.
