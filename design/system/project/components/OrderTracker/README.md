# OrderTracker

The bead-string delivery tracker: five beads on a thread, Paid → Packed → Shipped → Out for delivery → Delivered.

- Done beads: `accent-rose-gold` fill, 18px. The current step: 24px with a 5px `surface-soft` halo. Steps to come: `surface-card` fill with a 2px muted outline.
- The thread between done beads is `accent-rose-gold`; ahead it is `surface-petal`.
- Each step has a label (`body`, bold for the current step) and a detail line (`small`): time, place or courier.
- A dark summary card above it gives the plain-language status: "On its way · Expected Thu 26 Nov".
- Step names and the messages sent at each step are edited in Admin › Settings › Customer messages.

The consumer provides the current step and the detail text for each step.
