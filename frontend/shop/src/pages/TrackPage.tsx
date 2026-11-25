import { useState, type FormEvent } from "react";
import { useSearchParams } from "react-router-dom";
import { api, ApiError, shortDate, type Tracking } from "../api";

// Track an order with its number and the phone number used at checkout.
// The number can come from the link in our emails (?number=YB-20418); the phone never goes in the URL.
export function TrackPage() {
  const [params] = useSearchParams();
  const [number, setNumber] = useState(params.get("number") ?? "");
  const [phone, setPhone] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const [result, setResult] = useState<Tracking | null>(null);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      setResult(await api.track(number.trim(), phone.trim()));
    } catch (err) {
      setResult(null);
      setError(err instanceof ApiError ? err : new ApiError(0, "unknown", "Couldn't look up that order."));
    } finally {
      setBusy(false);
    }
  };

  if (result) return <TrackResult t={result} onBack={() => setResult(null)} />;

  return (
    <form className="stack track-form" onSubmit={submit} noValidate>
      <div className="stack-sm">
        <span className="eyebrow">Track order</span>
        <h1 className="page-title">Where's my order?</h1>
        <p className="muted">Enter your order number and the phone number you used at checkout.</p>
      </div>
      <div className={error?.field === "number" ? "field has-error" : "field"}>
        <label htmlFor="track-number">Order number</label>
        <input id="track-number" placeholder="YB-20418" autoCapitalize="characters" value={number} onChange={(e) => setNumber(e.target.value)} />
      </div>
      <div className={error?.field === "phone" ? "field has-error" : "field"}>
        <label htmlFor="track-phone">Phone number</label>
        <input id="track-phone" type="tel" inputMode="tel" autoComplete="tel" placeholder="0803 123 4567" value={phone} onChange={(e) => setPhone(e.target.value)} />
      </div>
      {error && (
        <span className="field-error" role="alert">
          {error.message}
        </span>
      )}
      <button className="btn primary" type="submit" disabled={busy || !number.trim() || !phone.trim()}>
        {busy ? "Looking…" : "Track order"}
      </button>
    </form>
  );
}

function when(iso: string): string {
  return new Date(iso).toLocaleString("en-GB", { weekday: "short", day: "numeric", month: "short", hour: "numeric", minute: "2-digit" }).replace(",", "");
}

function TrackResult({ t, onBack }: { t: Tracking; onBack: () => void }) {
  const expected = t.expectedFrom === t.expectedTo ? shortDate(t.expectedFrom) : `${shortDate(t.expectedFrom)} – ${shortDate(t.expectedTo)}`;
  const stopped = t.status === "cancelled" || t.status === "refunded";
  return (
    <div className="stack">
      <div className="stack-sm">
        <span className="eyebrow accent">{t.number}</span>
        <h1 className="page-title">{t.headline}</h1>
        <p className="muted">
          {t.items} {t.items === 1 ? "item" : "items"} to {t.area}
          {!stopped && t.status !== "delivered" && t.status !== "out_for_delivery" && <> · expected {expected}</>}
        </p>
      </div>

      {!stopped && (
        <ol className="bead-track" aria-label="Order progress">
          {t.steps.map((s) => (
            <li key={s.status} className={s.current ? "current" : s.done ? "done" : ""} aria-current={s.current ? "step" : undefined}>
              <span className="bead" aria-hidden="true" />
              <span className="bead-label">{s.label}</span>
              {s.at && <span className="small muted">{when(s.at)}</span>}
            </li>
          ))}
        </ol>
      )}

      {t.courier && (
        <section className="info-box">
          <div className="row-between">
            <span>Courier</span>
            <b>{t.courier}</b>
          </div>
          {t.trackingNumber && (
            <div className="row-between">
              <span>Tracking number</span>
              <span>{t.trackingNumber}</span>
            </div>
          )}
          {t.trackingLink && (
            <a className="btn outline" href={t.trackingLink} target="_blank" rel="noreferrer">
              Open the courier's tracking
            </a>
          )}
        </section>
      )}

      {t.notes.length > 0 && (
        <section className="stack-sm">
          <h2 className="form-h">Updates</h2>
          {t.notes.map((n, i) => (
            <div key={i} className="track-note">
              <span className="small muted">{when(n.at)}</span>
              <span>{n.text}</span>
            </div>
          ))}
        </section>
      )}

      <button className="btn outline" onClick={onBack}>
        Track another order
      </button>
    </div>
  );
}
