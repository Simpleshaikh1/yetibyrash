import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { formatNaira } from "@yetibyrash/ui";
import { api, statusLabels, type BoardOrder, type OrderStatus } from "../api";
import { ErrorBanner, Loading, PageHeader } from "../components/ui";
import { useAction, useApi } from "../useApi";

const columns: OrderStatus[] = ["paid", "packed", "shipped", "out_for_delivery", "delivered"];
const nextOf: Partial<Record<OrderStatus, OrderStatus>> = { paid: "packed", packed: "shipped", shipped: "out_for_delivery", out_for_delivery: "delivered" };

export function age(iso: string): string {
  const mins = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 60000));
  if (mins < 60) return `${mins}m`;
  const h = Math.round(mins / 60);
  return h < 48 ? `${h}h` : `${Math.round(h / 24)}d`;
}

// Orders board: one column per step. "Move to …" advances an order; shipping opens the order
// to pick a courier. Delivered shows the last 7 days.
export function OrdersPage() {
  const [params, setParams] = useSearchParams();
  const list = useApi(`orders:${params}`, (s) => api.orders(params, s));
  const { busy, error, run } = useAction();
  const navigate = useNavigate();

  const move = async (o: BoardOrder) => {
    const to = nextOf[o.status];
    if (!to) return;
    if (to === "shipped") {
      navigate(`/orders/${o.id}`); // needs a courier
      return;
    }
    const ok = await run(() => api.advanceOrder(o.id, { to }).then(() => true));
    if (ok) list.reload();
  };

  return (
    <>
      <PageHeader title="Orders" />
      <div className="row wrap">
        <label htmlFor="ord-search" className="visually-hidden">
          Search orders
        </label>
        <input
          id="ord-search"
          className="w-280"
          placeholder="Order number, name or phone"
          defaultValue={params.get("search") ?? ""}
          onChange={(e) => setParams(e.target.value ? { search: e.target.value } : {}, { replace: true })}
        />
        <span className="small muted">Only paid orders appear here. "Move to …" updates the customer's tracker and emails them.</span>
      </div>
      <ErrorBanner error={error ?? list.error} />
      {!list.data && !list.error && <Loading />}
      {list.data && (
        <div className="board">
          {columns.map((col) => {
            const cards = list.data!.filter((o) => o.status === col);
            return (
              <section key={col} className="board-col" aria-label={statusLabels[col]}>
                <div className="board-head">
                  <h2>{statusLabels[col]}</h2>
                  <span className="count-pill">{cards.length}</span>
                </div>
                {cards.length === 0 && <p className="small muted">None</p>}
                {cards.map((o) => (
                  <article key={o.id} className={o.late ? "order-card late" : "order-card"}>
                    <Link to={`/orders/${o.id}`} className="order-card-link">
                      <span className="row-between-admin">
                        <b>{o.number}</b>
                        <span className="small muted">{age(o.status === "paid" ? o.paidAt : o.statusAt)}</span>
                      </span>
                      <span>
                        {o.name} · {o.area}
                      </span>
                      <span className="small muted">
                        {o.items} {o.items === 1 ? "item" : "items"} · <b className="num">{formatNaira(o.totalKobo)}</b>
                        {o.speed === "express" && <span className="pill warn-pill">Express</span>}
                      </span>
                      {o.late && <span className="small warning-text">Paid over 24 hours ago</span>}
                    </Link>
                    {nextOf[o.status] && (
                      <button className="btn small secondary" disabled={busy} onClick={() => move(o)}>
                        Move to {statusLabels[nextOf[o.status]!].replace("Paid, to pack", "Packed")}
                      </button>
                    )}
                  </article>
                ))}
              </section>
            );
          })}
        </div>
      )}
    </>
  );
}
