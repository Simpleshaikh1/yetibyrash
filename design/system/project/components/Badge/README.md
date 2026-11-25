# Badge

A short capital label on a product photo or next to a product name.

- Shape: `radius-pill`, 10px capitals at 600 weight, 0.1em tracking, 4px × 8px padding.
- **Everyday**: `surface-card` fill, `ink` text.
- **Premium**: `ink` fill, `accent-rose-light` text.
- **Sale**: `accent-rose` fill, white text. Text comes from the active promotion ("20% OFF · BLACK FRIDAY").
- **New** and **Pre-order**: outline in `ink`, transparent fill.
- Stock urgency ("Only 6 left") is not a badge: it is `accent-rose` text next to the price.
- One badge per photo. Sale wins over tier.

The consumer provides the kind and, for sales, the text.
