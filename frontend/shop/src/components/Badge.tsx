import type { Tier } from "../api";

type BadgeSource = { tier: Tier; badges: string[]; compareAtKobo: number | null; saleBadge?: string; preorder?: boolean; inStock?: boolean };

// One badge per photo; a sale wins over the tier (design system › Badge).
export function Badge({ product }: { product: BadgeSource }) {
  if (product.compareAtKobo) return <span className="badge sale">{product.saleBadge || "Sale"}</span>;
  if (product.inStock === false && product.preorder) return <span className="badge outline">Pre-order</span>;
  if (product.badges.includes("New")) return <span className="badge outline">New</span>;
  if (product.tier === "premium") return <span className="badge premium">Premium</span>;
  return <span className="badge everyday">Everyday</span>;
}
