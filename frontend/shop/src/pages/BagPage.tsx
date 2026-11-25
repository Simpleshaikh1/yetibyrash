import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { formatNaira } from "@yetibyrash/ui";
import { api, ApiError, type Bag, type BagLine } from "../api";
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
              {l.compareAtKobo && (
                <span className="small sale-note">
                  {l.saleBadge || "On sale"} · was <s>{formatNaira(l.compareAtKobo * l.qty)}</s>
                </span>
              )}
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

      {bag.gift && <GiftNote gift={bag.gift} />}

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
        {bag.discountKobo > 0 && (
          <div className="row-between discount">
            <span>Discount ({bag.code?.code})</span>
            <b className="price">−{formatNaira(bag.discountKobo)}</b>
          </div>
        )}
        <div className="row-between muted">
          <span>Delivery</span>
          <span>{bag.code?.freeDelivery && !bag.code.problem ? "Free with your code" : "Worked out at checkout"}</span>
        </div>
      </div>

      <CodeBox bag={bag} onBag={setBag} />

      {bag.canCheckout ? (
        <Link to="/checkout" className="btn primary">
          Checkout · {formatNaira(bag.subtotalKobo - bag.discountKobo)}
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

// "₦X more for your free gift", or the gift that's going in.
function GiftNote({ gift }: { gift: NonNullable<Bag["gift"]> }) {
  const name = gift.option ? `${gift.name} (${gift.option})` : gift.name;
  if (gift.remainingKobo > 0) {
    const pct = Math.min(100, Math.round(((gift.thresholdKobo - gift.remainingKobo) / gift.thresholdKobo) * 100));
    return (
      <div className="gift-note">
        <p>
          Add <b>{formatNaira(gift.remainingKobo)}</b> more to get <Link to={`/p/${gift.slug}`}>{name}</Link> free.
        </p>
        <span className="gift-bar" aria-hidden="true">
          <span style={{ width: `${pct}%` }} />
        </span>
      </div>
    );
  }
  return (
    <div className="gift-note in">
      <p>
        <b>Free gift added:</b> {name}. It comes with your order.
      </p>
    </div>
  );
}

// Type a discount code; the bag keeps it and says if it doesn't apply yet.
function CodeBox({ bag, onBag }: { bag: Bag; onBag: (b: Bag) => void }) {
  const [code, setCode] = useState("");
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const run = async (fn: () => Promise<Bag>) => {
    setBusy(true);
    setError("");
    try {
      onBag(await fn());
      setCode("");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Couldn't apply the code.");
    } finally {
      setBusy(false);
    }
  };
  const apply = (e: FormEvent) => {
    e.preventDefault();
    if (code.trim()) void run(() => api.applyCode(code));
  };

  if (bag.code) {
    return (
      <div className={bag.code.problem ? "code-box problem" : "code-box"}>
        <div className="row-between">
          <span>
            <b>{bag.code.code}</b> · {bag.code.summary}
          </span>
          <button className="link-btn small" disabled={busy} onClick={() => run(() => api.removeCode())}>
            Remove
          </button>
        </div>
        {bag.code.problem && <span className="small urgent">{bag.code.problem}</span>}
      </div>
    );
  }
  if (!open) {
    return (
      <button className="link-btn" onClick={() => setOpen(true)}>
        Have a discount code?
      </button>
    );
  }
  return (
    <form className="code-form" onSubmit={apply} noValidate>
      <label htmlFor="bag-code" className="visually-hidden">
        Discount code
      </label>
      <input id="bag-code" autoFocus placeholder="Discount code" autoCapitalize="characters" autoComplete="off" value={code} onChange={(e) => setCode(e.target.value)} />
      <button className="btn outline small" type="submit" disabled={busy || !code.trim()}>
        Apply
      </button>
      {error && (
        <span className="field-error" role="alert">
          {error}
        </span>
      )}
    </form>
  );
}
