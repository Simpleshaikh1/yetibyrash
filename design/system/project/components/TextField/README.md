# TextField

A labelled input for checkout, order tracking and admin forms.

- Label above the field in `small` / `ink-secondary`; never a placeholder instead of a label.
- Shop: 46px tall, white fill, 1px `line-shop` border, `radius-md`.
- Admin: 42px tall, white fill, 1px `line-field` border, `radius-field`.
- Error: 2px `danger` border and a message under the field that says how to fix it.
- Nigerian address fields go State, LGA, street, landmark (optional).

The consumer provides the label, value, type (tel, email) and any error message.
