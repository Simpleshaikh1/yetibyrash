import type { ReactNode } from "react";

// Square photo ground (design system: every product photo sits on surface-soft).
// Real photos arrive with photo upload in M2; until then it shows the product's initial.
export function Photo({ name, large, children }: { name: string; large?: boolean; children?: ReactNode }) {
  return (
    <span className={large ? "photo photo-large" : "photo"} role="img" aria-label={`${name}, photo coming soon`}>
      <span className="photo-initial" aria-hidden="true">
        {name.charAt(0)}
      </span>
      {children}
    </span>
  );
}
