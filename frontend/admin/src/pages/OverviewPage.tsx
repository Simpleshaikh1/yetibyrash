import { Link } from "react-router-dom";
import { formatNaira } from "@yetibyrash/ui";
import { api } from "../api";
import { useAuth } from "../auth";
import { ErrorBanner, Loading, PageHeader, Panel } from "../components/ui";
import { useApi } from "../useApi";
import { age } from "./OrdersPage";

export function OverviewPage() {
  const { me, can } = useAuth();
  const ov = useApi("overview", (s) => api.overview(s));
  const today = new Date().toLocaleDateString("en-GB", { weekday: "long", day: "numeric", month: "long" });
  const hour = new Date().getHours();
  const greeting = hour < 12 ? "Good morning" : hour < 17 ? "Good afternoon" : "Good evening";

  return (
    <>
      <PageHeader
        title={`${greeting}, ${me?.staff.name.split(" ")[0] ?? ""}`}
        back={<span className="small muted">{today}</span>}
        actions={
          <>
            {can("products") && (
              <Link to="/products/new" className="btn primary">
                + Add product
              </Link>
            )}
            {can("orders") && (
              <Link to="/orders" className="btn secondary">
                Open orders board
              </Link>
            )}
          </>
        }
      />
      <ErrorBanner error={ov.error} />
      {!ov.data && !ov.error && <Loading />}
      {ov.data && (
        <>
          <div className="tiles">
            <div className="tile">
              <span className="muted">Sales today</span>
              <b className="num">{formatNaira(ov.data.salesTodayKobo)}</b>
              <span className="small success-text">
                {ov.data.ordersToday} paid {ov.data.ordersToday === 1 ? "order" : "orders"}
              </span>
            </div>
            <div className="tile">
              <span className="muted">Waiting to pack</span>
              <b>{ov.data.toPack}</b>
              <span className={ov.data.lateToPack.length ? "small warning-text" : "small muted"}>
                {ov.data.lateToPack.length ? `${ov.data.lateToPack.length} waiting over 24 hours` : `${ov.data.packed} packed, not shipped`}
              </span>
            </div>
            <div className="tile">
              <span className="muted">On the way</span>
              <b>{ov.data.onTheWay}</b>
              <span className="small muted">{ov.data.outForDelivery} out for delivery today</span>
            </div>
            <div className="tile">
              <span className="muted">Low stock</span>
              <b>{ov.data.lowStock.length}</b>
              <span className={ov.data.soldOut ? "small danger-text" : "small muted"}>{ov.data.soldOut} sold out</span>
            </div>
          </div>

          <div className="editor-grid">
            <Panel title="Needs your attention">
              {ov.data.lateToPack.length + ov.data.problemPayments.length + ov.data.lowStock.length + ov.data.failedEmails === 0 && (
                <p className="muted">Nothing needs attention right now.</p>
              )}
              {ov.data.lateToPack.map((o) => (
                <Link key={o.id} to={`/orders/${o.id}`} className="attention warn">
                  <span>
                    <b>{o.number}</b> ({o.name}) paid {age(o.paidAt)} ago, still not packed
                  </span>
                  <span>Open ›</span>
                </Link>
              ))}
              {ov.data.problemPayments.map((p) => (
                <div key={p.reference} className="attention danger">
                  <span>
                    {p.outcome === "no_order" ? (
                      <>
                        <b className="num">{formatNaira(p.amountKobo)}</b> was paid (ref {p.reference}) but there's no matching order. Check Paystack.
                      </>
                    ) : (
                      <>
                        Payment {p.reference} for <b className="num">{formatNaira(p.amountKobo)}</b> didn't match its order total.
                      </>
                    )}
                  </span>
                  {p.orderId && <Link to={`/orders/${p.orderId}`}>Open ›</Link>}
                </div>
              ))}
              {ov.data.failedEmails > 0 && (
                <div className="attention danger">
                  <span>
                    {ov.data.failedEmails} customer {ov.data.failedEmails === 1 ? "email" : "emails"} couldn't be sent after several tries. Check the email settings.
                  </span>
                </div>
              )}
              {ov.data.lowStock.map((l, i) => (
                <Link key={i} to={`/products/${l.productId}`} className={l.stock === 0 ? "attention danger" : "attention warn"}>
                  <span>
                    <b>{l.name}</b>
                    {l.option && ` · ${l.option}`} {l.stock === 0 ? "is sold out" : `has ${l.stock} left`}
                  </span>
                  <span>Restock ›</span>
                </Link>
              ))}
            </Panel>
            <Panel title="Sales, last 7 days" actions={<span className="small muted num">{formatNaira(ov.data.last7Days.reduce((s, d) => s + d.totalKobo, 0))} total</span>}>
              <SalesChart days={ov.data.last7Days} />
            </Panel>
          </div>
        </>
      )}
    </>
  );
}

function SalesChart({ days }: { days: { day: string; totalKobo: number }[] }) {
  const max = Math.max(1, ...days.map((d) => d.totalKobo));
  return (
    <div className="chart" role="img" aria-label={days.map((d) => `${d.day}: ${formatNaira(d.totalKobo)}`).join(", ")}>
      {days.map((d, i) => (
        <div key={d.day} className="bar-col">
          <span className="small muted num">{d.totalKobo ? formatNaira(d.totalKobo).replace(/,\d{3}$/, "k") : ""}</span>
          <span className={i === days.length - 1 ? "bar today" : "bar"} style={{ height: `${Math.max(2, (d.totalKobo / max) * 140)}px` }} />
          <span className="small muted">{new Date(d.day + "T00:00:00").toLocaleDateString("en-GB", { weekday: "short" })}</span>
        </div>
      ))}
    </div>
  );
}

export function ComingSoonPage({ title, milestone }: { title: string; milestone: string }) {
  return (
    <>
      <PageHeader title={title} />
      <Panel note={`This screen arrives in milestone ${milestone}.`}>
        <span />
      </Panel>
    </>
  );
}
