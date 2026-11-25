# ProductCard

A product in a grid: square photo, badge, save button, name, price and rating.

- `surface-card` fill, `radius-card`, no border or shadow; photo is a square on `surface-soft`.
- Badge top left (Badge component), save heart top right (44px tap area).
- Name in `body` (14–15px), price in `price`, then one small line: rating and a short note ("★ 4.8 (32) · 4 colours").
- On sale: sale price in `price`, the old price struck through in `ink-muted`.
- Quick add: a full-width espresso pill along the bottom of the photo. Products with sizes open the option sheet; others go straight to the bag.

The consumer provides the photo, name, price(s), tier, rating and link.
