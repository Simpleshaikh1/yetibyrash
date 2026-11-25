import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { Link } from "react-router-dom";
import { formatNaira } from "@yetibyrash/ui";
import { api, ApiError, type CheckoutInput, type Quote } from "../api";
import { useBag } from "../bag";
import { ErrorMessage, Loading } from "../components/Status";
import { useApi } from "../useApi";

// Remembers contact and address details on this phone, so a returning shopper doesn't retype them.
const SAVED = "yetibyrash.checkout";

function loadSaved(): Partial<CheckoutInput> {
  try {
    return JSON.parse(localStorage.getItem(SAVED) ?? "{}");
  } catch {
    return {};
  }
}

export function CheckoutPage() {
  const { bag } = useBag();
  const states = useApi("locations", (s) => api.locations(s));
  const [form, setForm] = useState<CheckoutInput>(() => ({
    name: "", phone: "", email: "", stateId: 0, lgaId: 0, lgaText: "", street: "", landmark: "", speed: "standard",
    ...loadSaved(),
  }));
  const [quote, setQuote] = useState<Quote | null>(null);
  const [quoteError, setQuoteError] = useState<ApiError | null>(null);
  const [error, setError] = useState<ApiError | null>(null);
  const [busy, setBusy] = useState(false);

  const state = states.data?.find((s) => s.id === form.stateId);
  const needsLgaList = (state?.lgas.length ?? 0) > 0;
  const addressReady = !!state && (needsLgaList ? form.lgaId > 0 : form.lgaText.trim().length > 1);

  const set = <K extends keyof CheckoutInput>(k: K, v: CheckoutInput[K]) => setForm((f) => ({ ...f, [k]: v }));

  // Re-price whenever the address or the bag changes.
  useEffect(() => {
    if (!addressReady || !bag?.canCheckout) {
      setQuote(null);
      return;
    }
    let live = true;
    const t = setTimeout(() => {
      api
        .quote(form.stateId, needsLgaList ? form.lgaId : 0, needsLgaList ? "" : form.lgaText)
        .then((q) => {
          if (!live) return;
          setQuote(q);
          setQuoteError(null);
          if (!q.options.some((o) => o.speed === form.speed)) set("speed", "standard");
        })
        .catch((e) => live && setQuoteError(e instanceof ApiError ? e : null));
    }, 250);
    return () => {
      live = false;
      clearTimeout(t);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [form.stateId, form.lgaId, form.lgaText, addressReady, bag?.subtotalKobo, bag?.code?.code]);

  const option = quote?.options.find((o) => o.speed === form.speed) ?? quote?.options[0];
  const freeDelivery = !!quote?.freeDelivery && !quote.codeProblem;
  const fee = option ? (freeDelivery ? 0 : option.feeKobo) : 0;
  const discount = quote && !quote.codeProblem ? quote.discountKobo : 0;
  const total = quote && option ? quote.subtotalKobo - discount + fee : null;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const { name, phone, email, stateId, lgaId, lgaText, street, landmark } = form;
      try {
        localStorage.setItem(SAVED, JSON.stringify({ name, phone, email, stateId, lgaId, lgaText, street, landmark }));
      } catch {
        // private browsing: not remembered
      }
      const res = await api.checkout({ ...form, lgaId: needsLgaList ? form.lgaId : 0, lgaText: needsLgaList ? "" : form.lgaText });
      window.location.href = res.paymentUrl; // Paystack's page (or the test payment page in development)
    } catch (err) {
      setError(err instanceof ApiError ? err : new ApiError(0, "unknown", "Something went wrong. Please try again."));
      setBusy(false);
    }
  };

  if (!bag || states.loading) return <Loading />;
  if (bag.lines.length === 0) {
    return (
      <div className="stack">
        <h1 className="page-title">Checkout</h1>
        <p className="status-msg">Your bag is empty.</p>
        <Link to="/shop" className="btn primary">
          Start shopping
        </Link>
      </div>
    );
  }

  const fieldError = (f: string) => (error?.field === f ? error.message : undefined);

  return (
    <form className="stack checkout" onSubmit={submit} noValidate>
      <div className="row-between">
        <h1 className="page-title">Checkout</h1>
        <span className="small muted">No account needed</span>
      </div>

      <section className="form-card">
        <h2 className="form-h">1. Contact</h2>
        <Field id="co-name" label="Full name" error={fieldError("name")}>
          <input id="co-name" autoComplete="name" value={form.name} onChange={(e) => set("name", e.target.value)} />
        </Field>
        <Field id="co-phone" label="Phone (for delivery and WhatsApp updates)" error={fieldError("phone")}>
          <input id="co-phone" type="tel" inputMode="tel" autoComplete="tel" placeholder="0803 000 0000" value={form.phone} onChange={(e) => set("phone", e.target.value)} />
        </Field>
        <Field id="co-email" label="Email (for your receipt)" error={fieldError("email")}>
          <input id="co-email" type="email" autoComplete="email" value={form.email} onChange={(e) => set("email", e.target.value)} />
        </Field>
      </section>

      <section className="form-card">
        <h2 className="form-h">2. Delivery address</h2>
        <div className="two-col">
          <Field id="co-state" label="State" error={fieldError("stateId")}>
            <select id="co-state" value={form.stateId || ""} onChange={(e) => setForm((f) => ({ ...f, stateId: Number(e.target.value), lgaId: 0 }))}>
              <option value="">Choose</option>
              {states.data?.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </select>
          </Field>
          {needsLgaList ? (
            <Field id="co-lga" label="LGA" error={fieldError("lgaId")}>
              <select id="co-lga" value={form.lgaId || ""} onChange={(e) => set("lgaId", Number(e.target.value))}>
                <option value="">Choose</option>
                {state?.lgas.map((l) => (
                  <option key={l.id} value={l.id}>
                    {l.name}
                  </option>
                ))}
              </select>
            </Field>
          ) : (
            <Field id="co-lgatext" label="LGA or town" error={fieldError("lgaText")}>
              <input id="co-lgatext" value={form.lgaText} disabled={!state} onChange={(e) => set("lgaText", e.target.value)} />
            </Field>
          )}
        </div>
        <Field id="co-street" label="House number and street" error={fieldError("street")}>
          <input id="co-street" autoComplete="street-address" value={form.street} onChange={(e) => set("street", e.target.value)} />
        </Field>
        <Field id="co-landmark" label="Nearest landmark (optional)" error={fieldError("landmark")}>
          <input id="co-landmark" placeholder="e.g. opposite the filling station" value={form.landmark} onChange={(e) => set("landmark", e.target.value)} />
        </Field>
      </section>

      <section className="form-card">
        <h2 className="form-h">3. Delivery speed</h2>
        {!addressReady && <p className="small muted">Choose your state and LGA to see delivery options.</p>}
        {quoteError && <ErrorMessage error={quoteError} />}
        {quote?.options.map((o) => (
          <label key={o.speed} className={form.speed === o.speed ? "option on" : "option"} htmlFor={`speed-${o.speed}`}>
            <input type="radio" id={`speed-${o.speed}`} name="speed" checked={form.speed === o.speed} onChange={() => set("speed", o.speed)} />
            <span className="option-text">
              <b>{o.speed === "express" ? "Express" : "Standard"}</b>
              <span className="small muted">Arrives {o.label}</span>
            </span>
            <b className="price">{o.free || freeDelivery ? "Free" : formatNaira(o.feeKobo)}</b>
          </label>
        ))}
      </section>

      <section className="stack-xs summary">
        <div className="row-between">
          <span>Items ({bag.count})</span>
          <span className="price">{formatNaira(bag.subtotalKobo)}</span>
        </div>
        {discount > 0 && (
          <div className="row-between discount">
            <span>Discount ({quote?.code})</span>
            <span className="price">−{formatNaira(discount)}</span>
          </div>
        )}
        <div className="row-between">
          <span>Delivery{freeDelivery && ` (${quote?.code})`}</span>
          <span className="price">{option ? (option.free || freeDelivery ? "Free" : formatNaira(option.feeKobo)) : "—"}</span>
        </div>
        {quote?.gift && (
          <div className="row-between">
            <span>Free gift: {quote.gift}</span>
            <span className="price">₦0</span>
          </div>
        )}
        {quote?.codeProblem && (
          <p className="urgent small" role="alert">
            Code {quote.code}: {quote.codeProblem} <Link to="/bag">Change it in your bag.</Link>
          </p>
        )}
        <div className="row-between total">
          <span>Total</span>
          <span className="price">{total !== null ? formatNaira(total) : "—"}</span>
        </div>
      </section>

      {error && !error.field && <ErrorMessage error={error} />}
      {error?.field && (
        <p className="urgent small" role="alert">
          Check the highlighted field above.
        </p>
      )}

      <div className="paybar">
        <button className="btn primary" type="submit" disabled={busy || total === null || !bag.canCheckout}>
          {busy ? "Opening payment…" : total !== null ? `Pay ${formatNaira(total)} with Paystack` : "Pay with Paystack"}
        </button>
        <span className="small muted center">Card, bank transfer or USSD on Paystack's secure page. Your order is placed once the payment succeeds.</span>
      </div>
    </form>
  );
}

function Field({ id, label, error, children }: { id: string; label: string; error?: string; children: ReactNode }) {
  return (
    <div className={error ? "field has-error" : "field"}>
      <label htmlFor={id}>{label}</label>
      {children}
      {error && (
        <span className="field-error" role="alert">
          {error}
        </span>
      )}
    </div>
  );
}
