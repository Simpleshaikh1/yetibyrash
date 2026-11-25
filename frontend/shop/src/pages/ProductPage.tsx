import { useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { formatNaira } from "@yetibyrash/ui";
import { api, type ProductDetail, type Variant } from "../api";
import { Badge } from "../components/Badge";
import { Icon } from "../components/Icon";
import { Photo } from "../components/Photo";
import { ProductGrid } from "../components/ProductCard";
import { ErrorMessage, Loading } from "../components/Status";
import { useApi } from "../useApi";

export function ProductPage() {
  const { slug = "" } = useParams();
  const { data, error, loading } = useApi(`product:${slug}`, (s) => api.product(slug, s));

  if (error) {
    return (
      <div className="stack">
        <ErrorMessage error={error} />
        <Link to="/shop" className="btn outline">
          Back to the shop
        </Link>
      </div>
    );
  }
  if (loading || !data) return <Loading />;
  // key resets the colour and size choice when moving to another product.
  return <Product key={data.slug} p={data} />;
}

function Product({ p }: { p: ProductDetail }) {
  const hasColours = p.colours.length > 0;
  const hasSizes = p.sizeSet !== null;

  // Start on the first colour that has stock.
  const firstColour = p.colours.find((c) => p.variants.some((v) => v.colour === c.name && v.stock > 0))?.name ?? p.colours[0]?.name ?? "";
  const [colour, setColour] = useState(firstColour);
  const [size, setSize] = useState("");

  const variant: Variant | undefined = useMemo(
    () => p.variants.find((v) => v.colour === (hasColours ? colour : "") && (hasSizes ? v.size === size : true)),
    [p.variants, colour, size, hasColours, hasSizes],
  );
  const stockFor = (sizeLabel: string) => p.variants.find((v) => v.colour === (hasColours ? colour : "") && v.size === sizeLabel)?.stock;

  const prices = p.variants.map((v) => v.priceKobo);
  const minPrice = Math.min(...prices);
  const price = variant?.priceKobo ?? minPrice;
  const compareAt = variant?.compareAtKobo ?? p.variants.find((v) => v.compareAtKobo)?.compareAtKobo ?? null;

  const needsSize = hasSizes && !size;
  const soldOut = variant !== undefined && variant.stock === 0;
  const lowStock = variant !== undefined && variant.stock > 0 && variant.stock <= p.lowStockThreshold;

  let buttonLabel = `Add to bag · ${formatNaira(price)}`;
  if (needsSize) buttonLabel = "Choose a size";
  else if (soldOut) buttonLabel = p.preorderAllowed ? `Pre-order · ${formatNaira(price)}` : "Sold out";

  return (
    <div className="stack product">
      <Link to={`/shop/${p.category.slug}`} className="icon-btn" aria-label={`Back to ${p.category.name}`}>
        <Icon name="back" />
      </Link>

      <Photo name={p.name} large>
        <Badge product={{ ...p, compareAtKobo: compareAt }} />
      </Photo>

      <div className="stack-xs">
        <span className="small muted">
          {p.parent ? `${p.parent.name} · ` : ""}
          {p.category.name}
        </span>
        <h1 className="product-name">{p.name}</h1>
        {p.materialLine && <span className="muted">{p.materialLine}</span>}
        <div className="row-start baseline">
          <span className="price-large">{formatNaira(price)}</span>
          {compareAt && <s className="muted">{formatNaira(compareAt)}</s>}
          {lowStock && <span className="urgent">Only {variant!.stock} left, ships from Lagos</span>}
        </div>
      </div>

      {hasColours && (
        <div className="stack-xs">
          <span className="label">
            Colour: <b>{colour}</b>
          </span>
          <div className="swatches">
            {p.colours.map((c) => (
              <button
                key={c.name}
                className={c.name === colour ? "swatch on" : "swatch"}
                style={{ background: c.hex }}
                aria-label={c.name}
                aria-pressed={c.name === colour}
                onClick={() => setColour(c.name)}
              />
            ))}
          </div>
        </div>
      )}

      {p.sizeSet && (
        <div className="stack-xs">
          <span className="label">Size ({p.sizeSet.name.replace(/ sizes?/i, "").replace(/[()]/g, "")})</span>
          <div className="sizes" style={{ gridTemplateColumns: `repeat(${Math.min(p.sizeSet.sizes.length, 5)}, minmax(0, 1fr))` }}>
            {p.sizeSet.sizes.map((s) => {
              const stock = stockFor(s.label);
              const unavailable = stock === undefined || (stock === 0 && !p.preorderAllowed);
              return (
                <button
                  key={s.label}
                  className={[s.label === size ? "on" : "", stock === 0 ? "out" : ""].join(" ")}
                  disabled={unavailable}
                  aria-pressed={s.label === size}
                  onClick={() => setSize(s.label)}
                >
                  {s.label}
                </button>
              );
            })}
          </div>
          {size && <span className="small muted">{p.sizeSet.sizes.find((s) => s.label === size)?.measurement}</span>}
        </div>
      )}

      <div className="info-box">
        <span>
          <b>Payment:</b> Paystack, card, transfer or USSD
        </span>
        <span>
          <b>Returns:</b> {p.returnRule}
        </span>
        <span>
          <b>Packaging:</b> Yetibyrash pouch, discreet outer bag
        </span>
      </div>

      {p.description && <p>{p.description}</p>}

      {p.details.length > 0 && (
        <dl className="details">
          {p.details.map((d) => (
            <div key={d.label}>
              <dt>{d.label}</dt>
              <dd>{d.value}</dd>
            </div>
          ))}
        </dl>
      )}

      {p.pairings.length > 0 && (
        <section className="stack-sm">
          <h2>Complete the look</h2>
          <ProductGrid items={p.pairings} />
        </section>
      )}

      <div className="buybar">
        <button className="btn primary" disabled title="The bag arrives in the next update (M3)">
          {buttonLabel}
        </button>
        <span className="small muted center">The bag and checkout arrive in milestone M3.</span>
      </div>
    </div>
  );
}
