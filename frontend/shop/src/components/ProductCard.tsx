import { Link } from "react-router-dom";
import { formatNaira } from "@yetibyrash/ui";
import type { ProductCard as Card } from "../api";
import { Badge } from "./Badge";
import { Photo } from "./Photo";

// One product in a grid (design system › ProductCard).
export function ProductCard({ p }: { p: Card }) {
  const ranged = p.maxPriceKobo > p.priceKobo;
  const note = !p.inStock ? (p.preorder ? "Pre-order" : "Sold out") : p.colourCount > 1 ? `${p.colourCount} colours` : p.materialLine;

  return (
    <Link to={`/p/${p.slug}`} className="card">
      <Photo name={p.name}>
        <Badge product={p} />
      </Photo>
      <span className="card-meta">
        <span className="card-name">{p.name}</span>
        <span className="price">
          {ranged && "From "}
          {formatNaira(p.priceKobo)}
          {p.compareAtKobo && <s>{formatNaira(p.compareAtKobo)}</s>}
        </span>
        <span className="small muted-2">{note}</span>
      </span>
    </Link>
  );
}

export function ProductGrid({ items }: { items: Card[] }) {
  return (
    <div className="grid">
      {items.map((p) => (
        <ProductCard key={p.id} p={p} />
      ))}
    </div>
  );
}
