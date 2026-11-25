import { useState } from "react";
import { Link } from "react-router-dom";
import { formatNaira } from "@yetibyrash/ui";
import { api, ApiError, type BagLine } from "../api";
import { useBag } from "../bag";
import { Photo } from "../components/Photo";
import { ErrorMessage, Loading } from "../components/Status";
import { useApi } from "../useApi";

export function BagPage() {
  const { bag, setBag } = useBag();
  const delivery = useApi("delivery", (s) => api.delivery(s));
  const [busy, setBusy] = useState<number | null>(null);
  const [error, setError] = useState<ApiError | null>(null);

  const change = async (l: BagLine, qty: number) => {
    setBusy(l.variantId);
    setError(null);
    try {
      setBag(await api.setQty(l.variantId, qty));
    } catch (e) {
      setError(e instanceof ApiError ? e : new ApiError(0, "unknown", "Couldn't update your bag."));
    } finally {
      setBusy(null);
    }
  };

  if (!bag) return <Loading />;

  if (bag.lines.length === 0) {
    return (
      <div className="stack">
        <h1 className="page-title">Your bag</h1>
        <p className="status-msg">Your bag is empty.</p>
        <Link to="/shop" className="btn primary">
          Start shopping
        </Link>
      </div>
    );
  }

  const freeOver = delivery.data?.zones.find((z) => z.name === "Lagos")?.freeOverKobo ?? null;
  const toFree = freeOver !== null ? freeOver - bag.subtotalKobo : null;

  return (
    <div className="stack">
      <div className="row-between">
        <h1 className="page-title">Your bag</h1>
        <span className="muted">
          {bag.count} {bag.count === 1 ? "item" : "items"}
        </span>
      </div>

      {error && <ErrorMessage error={error} />}

      <div className="stack-sm">
        {bag.lines.map((l) => (
          <article key={l.variantId} className={l.problem ? "bag-line problem" : "bag-line"}>
            <Link to={`/p/${l.slug}`} className="bag-photo" aria-label={l.name}>
              <Photo name={l.name} imageKey={l.image || undefined} />
            </Link>
            <div className="bag-info">
              <Link to={`/p/${l.slug}`} className="bag-name">
                {l.name}
              </Link>
              {l.option && <span className="small muted">{l.option}</span>}
              {l.problem && <span className="urgent small">{l.problem}</span>}
              <div className="row-between">
                <div className="stepper" aria-label={`Quantity of ${l.name}`}>
                  <button aria-label="One fewer" disabled={busy === l.variantId} onClick={() => change(l, l.qty - 1)}>
                    −
                  </button>
                  <span aria-live="polite">{l.qty}</span>
                  <button aria-label="One more" disabled={busy === l.variantId || l.qty >= l.stock} onClick={() => change(l, l.qty + 1)}>
                    +
                  </button>
                </div>
                <b className="price">{formatNaira(l.lineKobo)}</b>
              </div>
              {l.problem && l.stock > 0 && l.qty > l.stock && (
                <button className="link-btn small" onClick={() => change(l, l.stock)}>
                  Change to {l.stock}
                </button>
              )}
              {l.problem && (l.stock === 0 || l.problem === "No longer available") && (
                <button className="link-btn small" onClick={() => change(l, 0)}>
                  Remove from bag
                </button>
              )}
            </div>
          </article>
        ))}
      </div>

      {toFree !== null && toFree > 0 && (
        <p className="nudge">
          Add <b>{formatNaira(toFree)}</b> more for free delivery in Lagos.
        </p>
      )}
      {toFree !== null && toFree <= 0 && <p className="nudge">You get free standard delivery in Lagos.</p>}

      <div className="stack-xs">
        <div className="row-between">
          <span>Subtotal</span>
          <b className="price">{formatNaira(bag.subtotalKobo)}</b>
        </div>
        <div className="row-between muted">
          <span>Delivery</span>
          <span>Worked out at checkout</span>
        </div>
      </div>

      {bag.canCheckout ? (
        <Link to="/checkout" className="btn primary">
          Checkout · {formatNaira(bag.subtotalKobo)}
        </Link>
      ) : (
        <button className="btn primary" disabled>
          Fix the items above to check out
        </button>
      )}
      <Link to="/shop" className="center">
        Keep shopping
      </Link>
    </div>
  );
}
