# TierSwitch

The two-way Everyday / Premium switch that filters the whole shop.

- Track: `surface-soft`, `radius-pill`, 4px inner padding, two equal halves.
- Selected half: `surface-card` fill, `ink` text at 600 weight, `shadow-raised`. Unselected: `ink-secondary` text.
- Sits under the search field on Home; the same control (with `radius-field` corners) sets a product's tier in the admin.
- Switching changes the hero banner and the products shown; it never changes prices.

The consumer provides the selected tier and the change handler.
