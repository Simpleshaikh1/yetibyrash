import { useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { formatNaira, photoUrl } from "@yetibyrash/ui";
import { api, ApiError, shortDate, type ProductDetail, type Variant } from "../api";
import { useBag } from "../bag";
import { Badge } from "../components/Badge";
import { Icon } from "../components/Icon";
import { Photo } from "../components/Photo";
import { ProductGrid } from "../components/ProductCard";
import { SaveButton, Stars } from "../components/SaveButton";
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
  const [photoIndex, setPhotoIndex] = useState(0);
  const [adding, setAdding] = useState(false);
  const [addError, setAddError] = useState<string | null>(null);
  const { setBag } = useBag();
  const navigate = useNavigate();

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

  // Pre-orders aren't taken yet, so a sold-out option can't go in the bag.
  let buttonLabel = `Add to bag · ${formatNaira(price)}`;
  if (needsSize) buttonLabel = "Choose a size";
  else if (soldOut || !variant) buttonLabel = "Sold out";

  const addToBag = async () => {
    if (!variant) return;
    setAdding(true);
    setAddError(null);
    try {
      setBag(await api.addToBag(variant.id, 1));
      navigate("/bag");
    } catch (e) {
      setAddError(e instanceof ApiError ? e.message : "Couldn't add to your bag. Please try again.");
    } finally {
      setAdding(false);
    }
  };

  return (
    <div className="stack product">
      <Link to={`/shop/${p.category.slug}`} className="icon-btn" aria-label={`Back to ${p.category.name}`}>
        <Icon name="back" />
      </Link>

      <div className="product-layout">
        <div className="product-media stack-sm">
          <Photo name={p.name} imageKey={p.images[photoIndex]?.key} large>
            <Badge product={{ ...p, compareAtKobo: compareAt }} />
          </Photo>
          {p.images.length > 1 && (
            <div className="thumbs" role="list" aria-label="More photos">
              {p.images.map((im, i) => (
                <button key={im.id} className={i === photoIndex ? "thumb on" : "thumb"} aria-label={`Photo ${i + 1} of ${p.images.length}`} aria-pressed={i === photoIndex} onClick={() => setPhotoIndex(i)}>
                  <img src={photoUrl(im.key, 400)} alt="" loading="lazy" />
                </button>
              ))}
            </div>
          )}
        </div>

        <div className="product-info stack">
          <div className="stack-xs">
            <span className="small muted">
              {p.parent ? `${p.parent.name} · ` : ""}
              {p.category.name}
            </span>
            <div className="row-between title-row">
              <h1 className="product-name">{p.name}</h1>
              <SaveButton productId={p.id} name={p.name} className="save-btn inline" />
            </div>
            {p.rating.count > 0 && (
              <a href="#reviews" className="rating-line">
                <Stars value={p.rating.average} /> {p.rating.average.toFixed(1)} · {p.rating.count} {p.rating.count === 1 ? "review" : "reviews"}
              </a>
            )}
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
                  const unavailable = stock === undefined || stock === 0;
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
            <DeliveryLine />
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

          <div className="buybar">
            {addError && (
              <span className="urgent small center" role="alert">
                {addError}
              </span>
            )}
            <button className="btn primary" disabled={needsSize || soldOut || !variant || adding} onClick={addToBag}>
              {adding ? "Adding…" : buttonLabel}
            </button>
          </div>
        </div>
      </div>

      {p.reviews.length > 0 && (
        <section className="stack-sm" id="reviews">
          <h2>
            Reviews <span className="muted small">· {p.rating.average.toFixed(1)} from {p.rating.count}</span>
          </h2>
          <div className="reviews">
            {p.reviews.map((r) => (
              <article key={r.id} className="review">
                <Stars value={r.rating} />
                {r.text && <p>{r.text}</p>}
                {r.photos.length > 0 && (
                  <div className="row-start wrap">
                    {r.photos.map((k) => (
                      <img key={k} src={photoUrl(k, 400)} alt={`Photo from ${r.author}`} className="review-photo" loading="lazy" />
                    ))}
                  </div>
                )}
                <span className="small muted">
                  {r.author}
                  {r.area && ` · ${r.area}`}
                  {r.option && ` · ${r.option}`}
                </span>
                {r.reply && (
                  <p className="review-reply small">
                    <b>Yetibyrash:</b> {r.reply}
                  </p>
                )}
              </article>
            ))}
          </div>
        </section>
      )}

      {p.pairings.length > 0 && (
        <section className="stack-sm">
          <h2>Complete the look</h2>
          <ProductGrid items={p.pairings} />
        </section>
      )}
    </div>
  );
}

// "Order within 3h 20m, get it in Lagos by Thu 26 Nov", from the real delivery rules.
function DeliveryLine() {
  const { data } = useApi("delivery", (s) => api.delivery(s));
  if (!data?.lagos) return null;
  const lagos = `get it in Lagos ${data.lagos.from === data.lagos.to ? "on" : "by"} ${shortDate(data.lagos.to)}`;
  let lead = "Order now";
  if (data.cutoffAt) {
    const mins = Math.max(0, Math.round((new Date(data.cutoffAt).getTime() - Date.now()) / 60000));
    lead = mins >= 60 ? `Order within ${Math.floor(mins / 60)}h ${mins % 60}m` : `Order within ${mins}m`;
  }
  const others = data.zones.filter((z) => z.name !== "Lagos").map((z) => z.days);
  const range = others.length ? `${Math.min(...others.map((d) => Number(d.split("–")[0])))}–${Math.max(...others.map((d) => Number(d.split("–").pop())))}` : "";
  return (
    <>
      <span style={{ color: "var(--ink)" }}>
        <b>{lead}</b>, {lagos}
      </span>
      {range && (
        <span>
          <b>Other states:</b> {range} working days
        </span>
      )}
    </>
  );
}
