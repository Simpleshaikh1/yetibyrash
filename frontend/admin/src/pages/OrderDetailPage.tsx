import { useState, type FormEvent } from "react";
import { Link, useParams } from "react-router-dom";
import { formatNaira, photoUrl } from "@yetibyrash/ui";
import { api, statusLabels, type AdminOrder } from "../api";
import { useAuth } from "../auth";
import { ErrorBanner, Field, Loading, PageHeader, Panel, SavedBanner } from "../components/ui";
import { useAction, useApi } from "../useApi";

function when(iso: string) {
  return new Date(iso).toLocaleString("en-NG", { weekday: "short", day: "numeric", month: "short", hour: "numeric", minute: "2-digit" });
}

function day(iso: string) {
  return new Date(iso + "T00:00:00").toLocaleDateString("en-GB", { weekday: "short", day: "numeric", month: "short" });
}

export function OrderDetailPage() {
  const id = Number(useParams().id);
  const order = useApi(`order:${id}`, (s) => api.order(id, s));
  if (order.error) return <ErrorBanner error={order.error} />;
  if (!order.data) return <Loading />;
  return <Detail key={order.data.id} o={order.data} reload={order.reload} />;
}

function Detail({ o, reload }: { o: AdminOrder; reload: () => void }) {
  const { me } = useAuth();
  const couriers = useApi("couriers", (s) => api.couriers(s));
  const [courierId, setCourierId] = useState<number>(o.courierId ?? 0);
  const [tracking, setTracking] = useState(o.trackingNumber);
  const [note, setNote] = useState("");
  const [saved, setSaved] = useState<string | null>(null);
  const [cancelling, setCancelling] = useState(false);
  const [reason, setReason] = useState("");
  const { busy, error, run } = useAction();
  const canCancelRole = me?.staff.role === "owner" || me?.staff.role === "manager";

  const advance = async (e: FormEvent) => {
    e.preventDefault();
    if (!o.next) return;
    const ok = await run(() =>
      api.advanceOrder(o.id, { to: o.next, courierId: courierId || undefined, trackingNumber: tracking, note }).then(() => true),
    );
    if (ok) {
      setSaved(`Moved to ${statusLabels[o.next as keyof typeof statusLabels]}. The customer will get an email.`);
      setNote("");
      reload();
    }
  };

  const cancel = async () => {
    const ok = await run(() => api.cancelOrder(o.id, reason).then(() => true));
    if (ok) {
      setSaved("Order cancelled and stock returned. Refund the payment in Paystack.");
      reload();
    }
  };

  return (
    <div className="stack">
      <PageHeader
        title={`Order ${o.number}`}
        back={<Link to="/orders">‹ Orders</Link>}
        actions={<span className={`pill ${o.status === "cancelled" ? "neutral" : "ok"}`}>{statusLabels[o.status]}</span>}
      />
      <SavedBanner show={saved !== null}>{saved}</SavedBanner>
      <ErrorBanner error={error} />

      <div className="editor-grid">
        <div className="stack">
          <Panel title="Customer and delivery">
            <div className="grid-2">
              <div className="stack-xs">
                <b>{o.name}</b>
                <span>{o.phone}</span>
                <span>{o.email}</span>
              </div>
              <div className="stack-xs">
                <span>{o.address}</span>
                <span className="small muted">
                  {o.speed === "express" ? "Express" : "Standard"} · {o.zoneName} · expected {o.expectedFrom === o.expectedTo ? day(o.expectedFrom) : `${day(o.expectedFrom)} – ${day(o.expectedTo)}`}
                </span>
              </div>
            </div>
          </Panel>

          <Panel title="Items">
            {o.items.map((it, i) => (
              <div key={i} className="item-row">
                {it.image ? <img className="thumb" src={photoUrl(it.image, 400)} alt="" /> : <span className="thumb empty" aria-hidden="true" />}
                <span className="grow">
                  <b>{it.name}</b>
                  {it.option && <span className="small muted"> · {it.option}</span>}
                </span>
                <span className="num">
                  {it.qty} × {formatNaira(it.unitKobo)}
                </span>
              </div>
            ))}
            <div className="totals">
              <span>Items</span>
              <span className="num">{formatNaira(o.subtotalKobo)}</span>
              <span>Delivery</span>
              <span className="num">{o.deliveryKobo === 0 ? "Free" : formatNaira(o.deliveryKobo)}</span>
              <b>Total</b>
              <b className="num">{formatNaira(o.totalKobo)}</b>
            </div>
          </Panel>

          <Panel title="History">
            <ol className="timeline">
              {o.adminEvents.map((e, i) => (
                <li key={i}>
                  <b>{statusLabels[e.status]}</b> <span className="small muted">· {when(e.at)}{e.by && ` · ${e.by}`}</span>
                  {e.note && (
                    <div className="small">
                      {e.note} {!e.public && <span className="pill neutral">Staff only</span>}
                    </div>
                  )}
                </li>
              ))}
            </ol>
          </Panel>
        </div>

        <div className="stack">
          {o.next && (
            <Panel title={`Move to ${statusLabels[o.next]}`}>
              <form className="stack-sm" onSubmit={advance}>
                {o.next === "shipped" && (
                  <>
                    <Field label="Courier" htmlFor="od-courier" error={error?.field === "courierId" ? error.message : undefined}>
                      <select id="od-courier" value={courierId || ""} onChange={(e) => setCourierId(Number(e.target.value))}>
                        <option value="">Choose a courier</option>
                        {couriers.data?.filter((c) => c.active).map((c) => (
                          <option key={c.id} value={c.id}>
                            {c.name}
                          </option>
                        ))}
                      </select>
                    </Field>
                    <Field label="Tracking number (optional)" htmlFor="od-tracking">
                      <input id="od-tracking" value={tracking} onChange={(e) => setTracking(e.target.value)} />
                    </Field>
                  </>
                )}
                <Field label="Note to the customer (optional)" htmlFor="od-note" hint="Shown on their tracker and in the email.">
                  <textarea id="od-note" rows={2} value={note} onChange={(e) => setNote(e.target.value)} />
                </Field>
                <button className="btn primary" disabled={busy}>
                  {busy ? "Saving…" : `Mark as ${statusLabels[o.next].toLowerCase()}`}
                </button>
                <span className="small muted">{o.name.split(" ")[0]} gets an email.</span>
              </form>
            </Panel>
          )}

          {o.courierName && (
            <Panel title="Shipping">
              <span>{o.courierName}</span>
              {o.trackingNumber && <span className="small muted">Tracking number {o.trackingNumber}</span>}
            </Panel>
          )}

          <Panel title="Payment">
            {o.payments.length === 0 && <span className="small muted">No payment recorded.</span>}
            {o.payments.map((p) => (
              <div key={p.reference} className="stack-xs small">
                <span>
                  <b className="num">{formatNaira(p.amountKobo)}</b> · {p.channel || "Paystack"} · {when(p.at)}
                </span>
                <span className="muted">Ref {p.reference}</span>
                {p.outcome !== "success" && <span className="warning-text">{p.outcome === "amount_mismatch" ? "Amount didn't match the order" : p.outcome}</span>}
              </div>
            ))}
          </Panel>

          {o.canCancel && canCancelRole && (
            <Panel title="Cancel order">
              {!cancelling ? (
                <button className="btn small secondary" onClick={() => setCancelling(true)}>
                  Cancel this order…
                </button>
              ) : (
                <div className="stack-sm">
                  <Field label="Reason (the customer sees this)" htmlFor="od-reason">
                    <textarea id="od-reason" rows={2} value={reason} onChange={(e) => setReason(e.target.value)} />
                  </Field>
                  <span className="small muted">Stock goes back on sale. Refund {formatNaira(o.totalKobo)} in Paystack yourself.</span>
                  <div className="row">
                    <button className="btn small danger" disabled={busy || !reason.trim()} onClick={cancel}>
                      Cancel order {o.number}
                    </button>
                    <button className="link-btn" onClick={() => setCancelling(false)}>
                      Keep it
                    </button>
                  </div>
                </div>
              )}
            </Panel>
          )}
        </div>
      </div>
    </div>
  );
}
