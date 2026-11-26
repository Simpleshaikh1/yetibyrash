import { useState, type FormEvent } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { formatNaira, photoUrl } from "@yetibyrash/ui";
import { useAccount } from "../account";
import { api, ApiError, type OrderSummary } from "../api";
import { ErrorMessage, Loading } from "../components/Status";
import { useApi } from "../useApi";

const statusText: Record<string, string> = {
  paid: "Paid",
  packed: "Packed",
  shipped: "Shipped",
  out_for_delivery: "Out for delivery",
  delivered: "Delivered",
  cancelled: "Cancelled",
  refunded: "Refunded",
};

// Design: Account. Sign-in uses a one-time code, so there's no password to forget.
export function AccountPage() {
  const { customer } = useAccount();
  if (customer === undefined) return <Loading />;
  return customer ? <Account /> : <SignIn />;
}

function SignIn() {
  const { refresh } = useAccount();
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const [target, setTarget] = useState("");
  const [sentTo, setSentTo] = useState<{ value: string; isPhone: boolean } | null>(null);
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);

  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    setError(null);
    try {
      await fn();
    } catch (e) {
      setError(e instanceof ApiError ? e : new ApiError(0, "unknown", "Something went wrong. Please try again."));
    } finally {
      setBusy(false);
    }
  };
  const send = (e: FormEvent) => {
    e.preventDefault();
    void run(async () => {
      setSentTo(await api.sendCode(target));
      setCode("");
    });
  };
  const verify = (e: FormEvent) => {
    e.preventDefault();
    void run(async () => {
      await api.verifyCode(sentTo!.value, code);
      await refresh();
      const next = params.get("next");
      if (next && next.startsWith("/") && !next.startsWith("//")) navigate(next);
    });
  };

  return (
    <div className="stack narrow-page sign-in">
      <div className="stack-sm">
        <span className="eyebrow">Account</span>
        <h1 className="page-title">Sign in</h1>
        <p className="muted">We'll send a one-time code to your phone or email. No password to remember, and you don't need an account to shop.</p>
      </div>

      {!sentTo ? (
        <form className="stack-sm" onSubmit={send} noValidate>
          <div className={error?.field === "target" ? "field has-error" : "field"}>
            <label htmlFor="si-target">Phone number or email</label>
            <input id="si-target" autoComplete="username" placeholder="0803 000 0000 or you@example.com" value={target} onChange={(e) => setTarget(e.target.value)} />
          </div>
          {error && (
            <span className="field-error" role="alert">
              {error.message}
            </span>
          )}
          <button className="btn primary" type="submit" disabled={busy || !target.trim()}>
            {busy ? "Sending…" : "Send me a code"}
          </button>
        </form>
      ) : (
        <form className="stack-sm" onSubmit={verify} noValidate>
          <p>
            We sent a 6-digit code to <b>{sentTo.value}</b>. It works for 10 minutes.
          </p>
          <div className={error?.field === "code" ? "field has-error" : "field"}>
            <label htmlFor="si-code">Code</label>
            <input
              id="si-code"
              className="code-input"
              inputMode="numeric"
              autoComplete="one-time-code"
              maxLength={6}
              autoFocus
              value={code}
              onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
            />
          </div>
          {error && (
            <span className="field-error" role="alert">
              {error.message}
            </span>
          )}
          <button className="btn primary" type="submit" disabled={busy || code.length !== 6}>
            {busy ? "Checking…" : "Sign in"}
          </button>
          <div className="row-between small">
            <button type="button" className="link-btn" onClick={() => setSentTo(null)}>
              Use a different number or email
            </button>
            <button type="button" className="link-btn" disabled={busy} onClick={() => run(async () => void (await api.sendCode(sentTo.value)))}>
              Send a new code
            </button>
          </div>
        </form>
      )}
    </div>
  );
}

function Account() {
  const { customer, signOut, saved, refresh } = useAccount();
  const c = customer!;
  const orders = useApi("my-orders", (s) => api.myOrders(s));
  const addresses = useApi("my-addresses", (s) => api.addresses(s));
  const site = useApi("site", (s) => api.site(s));
  const navigate = useNavigate();
  const [name, setName] = useState(c.name);
  const [optIn, setOptIn] = useState(c.marketingOptIn);
  const [profileMsg, setProfileMsg] = useState("");
  const [error, setError] = useState<ApiError | null>(null);

  const saveProfile = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    try {
      await api.updateMe(name, optIn);
      await refresh();
      setProfileMsg("Saved.");
    } catch (err) {
      setError(err instanceof ApiError ? err : null);
    }
  };
  const review = async (o: OrderSummary) => {
    try {
      const { token } = await api.reviewLink(o.number);
      navigate(`/review/${token}`);
    } catch (err) {
      setError(err instanceof ApiError ? err : null);
    }
  };
  const deleteAccount = async () => {
    // Orders on their way aren't affected: say so, so nobody worries they'll lose them.
    const onTheWay = (orders.data ?? []).filter((o) => ["paid", "packed", "shipped", "out_for_delivery"].includes(o.status));
    const lines = ["Delete your account?", "", "Your saved addresses and saved items will be removed. This can't be undone."];
    if (onTheWay.length > 0) {
      const which = onTheWay.map((o) => o.number).join(", ");
      lines.push(
        "",
        `You have ${onTheWay.length === 1 ? "1 order" : `${onTheWay.length} orders`} on the way (${which}). ${onTheWay.length === 1 ? "It" : "They"}'ll still be delivered, and you'll still get updates. You can track ${onTheWay.length === 1 ? "it" : "them"} any time with the order number and your phone number on the Track page.`,
      );
    }
    lines.push("", "If you sign in again later with the same phone or email, your past orders will show up again.");
    if (!window.confirm(lines.join("\n"))) return;
    await api.deleteMe().catch(() => {});
    await signOut();
    navigate("/");
  };

  return (
    <div className="stack narrow-page account">
      <div className="stack-xs">
        <h1 className="page-title">Hi{c.name ? `, ${c.name.split(" ")[0]}` : ""}</h1>
        <span className="muted">Signed in with {c.phone || c.email}</span>
      </div>
      {error && <ErrorMessage error={error} />}

      <section className="stack-sm">
        <h2>My orders</h2>
        {orders.error && <ErrorMessage error={orders.error} />}
        {orders.data?.length === 0 && (
          <p className="muted">
            No orders yet. <Link to="/shop">Start shopping</Link>
          </p>
        )}
        {orders.data?.map((o) => (
          <article key={o.number} className="order-row">
            <span className="order-thumb">{o.image ? <img src={photoUrl(o.image, 400)} alt="" /> : null}</span>
            <div className="stack-xs grow">
              <b>{o.number}</b>
              <span className="small muted">
                {o.items} {o.items === 1 ? "item" : "items"} · {formatNaira(o.totalKobo)}
              </span>
              <div className="row-start small">
                <Link to={`/track?number=${encodeURIComponent(o.number)}`}>Track</Link>
                {o.status === "delivered" && (
                  <button className="link-btn small" onClick={() => review(o)}>
                    Write a review
                  </button>
                )}
              </div>
            </div>
            <span className={`status-pill ${o.status}`}>{statusText[o.status] ?? o.status}</span>
          </article>
        ))}
      </section>

      <section className="stack-sm">
        <div className="row-between">
          <h2>Saved items</h2>
          <Link to="/saved">
            {saved.size} {saved.size === 1 ? "item" : "items"} ›
          </Link>
        </div>
      </section>

      <section className="stack-sm">
        <h2>Saved addresses</h2>
        {addresses.data?.length === 0 && <p className="small muted">Addresses you use at checkout are kept here.</p>}
        {addresses.data?.map((a) => (
          <div key={a.id} className="address-row">
            <span>
              {a.street}
              {a.landmark && `, ${a.landmark}`}
              <br />
              <span className="small muted">
                {a.lgaName}, {a.stateName}
              </span>
            </span>
            <button className="link-btn small" onClick={() => api.deleteAddress(a.id).then(addresses.reload)} aria-label={`Remove ${a.street}`}>
              Remove
            </button>
          </div>
        ))}
      </section>

      <section className="form-card">
        <h2 className="form-h">Profile and notifications</h2>
        <form className="stack-sm" onSubmit={saveProfile}>
          <div className="field">
            <label htmlFor="ac-name">Your name</label>
            <input id="ac-name" autoComplete="name" value={name} onChange={(e) => { setName(e.target.value); setProfileMsg(""); }} />
          </div>
          <label className="check-row" htmlFor="ac-optin">
            <input id="ac-optin" type="checkbox" checked={optIn} onChange={(e) => { setOptIn(e.target.checked); setProfileMsg(""); }} />
            Tell me about new pieces and sales
          </label>
          <div className="row-start">
            <button className="btn outline small" type="submit">
              Save
            </button>
            {profileMsg && <span className="small muted">{profileMsg}</span>}
          </div>
        </form>
      </section>

      {site.data && site.data.pages.length > 0 && (
        <section className="stack-sm">
          <h2>Help and returns</h2>
          <div className="row-start wrap">
            {site.data.pages.map((p) => (
              <Link key={p.slug} to={`/page/${p.slug}`}>
                {p.title}
              </Link>
            ))}
          </div>
        </section>
      )}

      <p className="small muted">Sign-in uses a one-time code sent to your phone or email, so there's no password to forget.</p>
      <button className="btn outline" onClick={() => signOut().then(() => navigate("/"))}>
        Sign out
      </button>
      <button className="link-btn small center" onClick={deleteAccount}>
        Delete my account
      </button>
    </div>
  );
}
