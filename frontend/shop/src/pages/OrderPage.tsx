import { useEffect, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { formatNaira } from "@yetibyrash/ui";
import { api, ApiError, shortDate, type OrderView } from "../api";
import { useBag } from "../bag";
import { Photo } from "../components/Photo";
import { ErrorMessage, Loading } from "../components/Status";

const paidStatuses = new Set(["paid", "packed", "shipped", "out_for_delivery", "delivered"]);

// Where Paystack sends the shopper after paying: /order/YB-20418?reference=…
// The payment can take a few seconds to confirm, so a pending order is checked again.
export function OrderPage() {
  const { number = "" } = useParams();
  const [params] = useSearchParams();
  const reference = params.get("reference") ?? params.get("trxref") ?? "";
  const { refresh } = useBag();
  const [order, setOrder] = useState<OrderView | null>(null);
  const [error, setError] = useState<ApiError | null>(null);
  const [gaveUp, setGaveUp] = useState(false);

  useEffect(() => {
    let live = true;
    let tries = 0;
    const load = async () => {
      try {
        const o = await api.order(number, reference);
        if (!live) return;
        setOrder(o);
        if (paidStatuses.has(o.status)) {
          void refresh(); // the bag was emptied when the order was paid
          return;
        }
        if (o.status === "pending_payment" && tries++ < 20) {
          setTimeout(load, 3000);
        } else if (o.status === "pending_payment") {
          setGaveUp(true);
        }
      } catch (e) {
        if (live) setError(e instanceof ApiError ? e : new ApiError(0, "unknown", "Couldn't load your order."));
      }
    };
    void load();
    return () => {
      live = false;
    };
  }, [number, reference, refresh]);

  if (error) {
    return (
      <div className="stack">
        <ErrorMessage error={error} />
        <Link to="/" className="btn outline">
          Go to the homepage
        </Link>
      </div>
    );
  }
  if (!order) return <Loading label="Checking your payment…" />;

  const paid = paidStatuses.has(order.status);

  return (
    <div className="stack">
      <div className="confirm-head">
        {paid ? (
          <>
            <span className="eyebrow accent">Payment received</span>
            <h1 className="page-title">Thank you, {order.name.split(" ")[0]}</h1>
            <p className="muted">
              We've got your order. Updates go to {order.phone} and {order.email}.
            </p>
          </>
        ) : order.status === "pending_payment" ? (
          <>
            <span className="eyebrow">Waiting for payment</span>
            <h1 className="page-title">Confirming your payment…</h1>
            <p className="muted">
              {gaveUp
                ? "We haven't heard from Paystack yet. If you paid, this page will update; you can also check back in a few minutes."
                : "This usually takes a few seconds. Please keep this page open."}
            </p>
          </>
        ) : (
          <>
            <span className="eyebrow">Not paid</span>
            <h1 className="page-title">Payment didn't go through</h1>
            <p className="muted">Nothing was charged. Your items are back on sale; add them to your bag again to try once more.</p>
            <Link to="/bag" className="btn primary">
              Go to your bag
            </Link>
          </>
        )}
      </div>

      <section className="info-box">
        <div className="row-between">
          <span>Order number</span>
          <b>{order.number}</b>
        </div>
        <div className="row-between">
          <span>{paid ? "Paid" : "Total"}</span>
          <b className="price">{formatNaira(order.totalKobo)}</b>
        </div>
        <div className="row-between">
          <span>Delivery to</span>
          <span className="right">{order.address}</span>
        </div>
        {paid && (
          <div className="row-between">
            <span>Expected</span>
            <span>{order.expectedFrom === order.expectedTo ? shortDate(order.expectedFrom) : `${shortDate(order.expectedFrom)} – ${shortDate(order.expectedTo)}`}</span>
          </div>
        )}
      </section>

      <section className="stack-sm">
        {order.items.map((it, i) => (
          <div key={i} className="bag-line">
            <span className="bag-photo">
              <Photo name={it.name} imageKey={it.image || undefined} />
            </span>
            <div className="bag-info">
              <span className="bag-name">{it.name}</span>
              {it.option && <span className="small muted">{it.option}</span>}
              <span className="small">{it.gift ? "Free gift" : `${it.qty} × ${formatNaira(it.unitKobo)}`}</span>
            </div>
          </div>
        ))}
        {order.discountKobo > 0 && (
          <div className="row-between small muted">
            <span>Discount{order.discountCode && ` (${order.discountCode})`}</span>
            <span>−{formatNaira(order.discountKobo)}</span>
          </div>
        )}
        <div className="row-between small muted">
          <span>Delivery</span>
          <span>{order.deliveryKobo === 0 ? "Free" : formatNaira(order.deliveryKobo)}</span>
        </div>
      </section>

      {paid && (
        <>
          <Link to={`/track?number=${encodeURIComponent(order.number)}`} className="btn primary">
            Track this order
          </Link>
          <Link to="/shop" className="btn outline">
            Continue shopping
          </Link>
        </>
      )}
    </div>
  );
}
