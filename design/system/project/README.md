# Yetibyrash

Elegance meets affordability. Yetibyrash sells jewelry, watches, bags, women's clothing and intimates to shoppers across Nigeria, in two price tiers: **Everyday** and **Premium**. This system is the "Blush & Espresso" direction: a warm blush ground, espresso brown ink and deep rose accents, with a fashion-magazine serif for headings.

It was built from the Yetibyrash design canvas (phone shop screens and the admin dashboard). Every value here is the value used there.

## Voice

- Plain and warm, like a friend who works in the shop. Short sentences.
- Talk about what the shopper gets: "Get it in Lagos by Thu 26 Nov", not "Estimated delivery window".
- Nigerian conventions, always:
  - Money: `₦14,500` (naira sign, comma thousands, no decimals). Ranges: `₦10k–₦30k` only in filters and chips.
  - Phones: `0803 000 0000`.
  - Dates: `Thu 26 Nov`. Times: `12 noon`, `4:12 pm`.
  - Addresses: State, then LGA, then street, then a landmark.
  - "Bank transfer" and "USSD" are named next to "card" wherever payment is mentioned.
- Buttons say exactly what happens: "Add to bag · ₦14,500", "Pay ₦29,000 with Paystack", "Save: mark as Shipped".
- Errors say what to fix: "Enter an 11-digit phone number, e.g. 0803 000 0000."
- No emoji in the interface.

## Colour

| Role | Token | Rule |
| --- | --- | --- |
| Shop page | `ground-shop` | Every shopper screen. |
| Cards and bars | `surface-card` | Product cards, bag items, sticky bottom bars. |
| Photos | `surface-soft` | Every product photo sits on this square, so studio shots and phone shots match. |
| Hero | `surface-petal` | The homepage hero and the product gallery. |
| Text | `ink`, `ink-secondary`, `ink-muted` | Three levels only. |
| Primary action | `ink` fill, `ink-on-dark` text | One per screen. |
| Links and urgency | `accent-rose` | Links, "Only 6 left", sale badges. |
| Premium | `ink` ground, `accent-rose-light` text | Premium labels and dark feature blocks. |
| Decoration | `accent-rose-gold` | Tracker beads and charts. **Never behind text** (white on it is only 3.8:1). |

The admin dashboard uses `ground-admin`, `surface-white` panels, `line-admin` borders and `ink-admin` text, with the espresso `ink` sidebar. States (success, warning, danger) appear only in the admin and in order status, never as decoration.

There is one theme (light). The shop is designed for daylight phone use; a dark mode is not planned.

## Type

- **Bodoni Moda** (`--font-display`) is for headings only: the hero, product names, section titles and the placeholder wordmark. Never below 18px, never for buttons, prices or forms.
- **Jost** (`--font-body`) is everything else, and the whole admin dashboard.
- Prices always use tabular figures (`font-variant-numeric: tabular-nums`).
- Small capital labels (`eyebrow`) get 0.16em letter-spacing.
- Load both from Google Fonts: `family=Bodoni+Moda:ital,wght@0,500;0,700;1,500&family=Jost:wght@400;500;600`.

## Layout

- Phone first, designed at 390px wide with a 16px side gutter (`space-4`).
- Bottom navigation with five items: Home, Shop, Saved, Bag, Track (Account replaces Track when signed in).
- Every tap target is at least 44px (`touch-min`). Primary buttons are 52px tall pills.
- Product grids are two columns on phones, 12px gaps.
- Rounded everything on the shop (`radius-card`, `radius-pill`); tighter corners in the admin (`radius-panel`, `radius-field`).
- Borders, not shadows. The one shadow (`shadow-raised`) marks the selected half of a switch.

## Photography

- Square crops on `surface-soft`. Studio shots lead; phone shots appear as "real life" extras.
- Bags show a phone in at least one photo for scale.
- Clothing shows the model's size and height in the product details.

## Iconography

Line icons, 22px in a 24px box, 1.6px stroke, round joins, coloured with the text colour they sit beside. Icon-only buttons are 44px circles on `surface-card` with an `aria-label`. No filled or two-tone icons, no emoji.

## Logo

Not supplied yet. Until it arrives, the name is set in the `wordmark` style: Bodoni Moda, capitals, 0.2em tracking, in `ink`.

## Components

| Component | Use it for |
| --- | --- |
| Button | Every action: primary pill, outline pill, Paystack pay, admin button. |
| TextField | Checkout, tracking lookup and admin forms. |
| ProductCard | Shop grids, New in, You may also like. |
| TierSwitch | The Everyday / Premium switch on Home and in the admin. |
| OptionPicker | Colour swatches and size buttons on the product page. |
| OrderTracker | The bead-string delivery tracker. |
| Badge | Tier, sale, New and stock labels on products. |
| StatusPill | Admin states: Live, Scheduled, Hidden, Sold out, Waiting. |

These are reference previews in HTML and CSS built on the tokens. The React components for the real site will be built from them.

## Accessibility

- Text meets 4.5:1 on its ground; check `ink-muted` before using it on `surface-petal`.
- Selected states change more than colour: a thicker border, a fill or a check.
- Every form control has a visible label above it.
