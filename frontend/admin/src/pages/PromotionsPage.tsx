import { useState } from "react";
import { formatNaira, koboToInput, parseNaira } from "@yetibyrash/ui";
import { promotionsApi, type DiscountCode, type GiftOffer, type PromotionsAdmin, type Sale } from "../api";
import { ErrorBanner, Field, Loading, PageHeader, Panel } from "../components/ui";
import { useAction, useApi } from "../useApi";

// Design: Admin CMS › Promotions.
export function PromotionsPage() {
  const data = useApi("promotions", (s) => promotionsApi.get(s));
  return (
    <>
      <PageHeader title="Promotions" />
      <p className="muted">Sales, free gifts and discount codes. Prices, badges and the banner change on the shop automatically and switch back when the sale ends.</p>
      <ErrorBanner error={data.error} />
      {!data.data && !data.error && <Loading />}
      {data.data && (
        <div className="stack">
          <Sales data={data.data} reload={data.reload} />
          <Gifts data={data.data} reload={data.reload} />
          <Codes data={data.data} reload={data.reload} />
        </div>
      )}
    </>
  );
}

const today = () => new Date().toLocaleDateString("en-CA", { timeZone: "Africa/Lagos" });

function salePrice(kind: string, value: number, kobo: number): number {
  if (kind === "percent") return value >= 1 && value <= 90 ? Math.floor((kobo * (100 - value)) / 100 / 100) * 100 : kobo;
  return kobo - value >= 10000 ? kobo - value : kobo;
}

function offText(kind: string, value: number): string {
  return kind === "percent" ? `${value}% off` : `${formatNaira(value)} off`;
}

// ---------- Sales ----------

const stateLabel: Record<Sale["state"], string> = { live: "Live", scheduled: "Scheduled", ended: "Ended", off: "Off" };

function Sales({ data, reload }: { data: PromotionsAdmin; reload: () => void }) {
  const [editing, setEditing] = useState<Sale | null>(null);
  const [session, setSession] = useState(0);
  const pick = (s: Sale | null) => {
    setEditing(s);
    setSession((n) => n + 1);
  };
  const blank: Sale = {
    id: 0, name: "", kind: "percent", value: 20, appliesTo: "all", tier: "", categoryId: 0, categoryName: "",
    startsOn: today(), endsOn: "", badge: "", announcement: "", excludeOnSale: true, active: true,
    state: "scheduled", endsInDays: 0, orders: 0, salesKobo: 0,
  };
  return (
    <Panel
      title="Sales"
      actions={
        <button className="btn primary" onClick={() => pick(blank)}>
          + New sale
        </button>
      }
    >
      {data.sales.length === 0 && !editing && <p className="muted">No sales yet.</p>}
      {data.sales.map((s) => (
        <button key={s.id} className={editing?.id === s.id ? "promo-row on" : "promo-row"} onClick={() => pick(s)}>
          <span className="stack-xs">
            <b>{s.name}</b>
            <span className="small muted">
              {offText(s.kind, s.value)} · {s.appliesTo === "all" ? "Everything" : s.appliesTo === "tier" ? (s.tier === "premium" ? "Premium only" : "Everyday only") : s.categoryName} · {s.startsOn} – {s.endsOn}
            </span>
          </span>
          <span className="stack-xs right">
            <span className={s.state === "live" ? "pill ok" : "pill neutral"}>
              {stateLabel[s.state]}
              {s.state === "live" && ` · ends in ${s.endsInDays} ${s.endsInDays === 1 ? "day" : "days"}`}
            </span>
            {s.orders > 0 && (
              <span className="small muted">
                {s.orders} {s.orders === 1 ? "order" : "orders"} · {formatNaira(s.salesKobo)}
              </span>
            )}
          </span>
        </button>
      ))}
      {editing && (
        <SaleEditor
          key={session}
          sale={editing}
          data={data}
          onDone={() => {
            pick(null);
            reload();
          }}
        />
      )}
    </Panel>
  );
}

function SaleEditor({ sale, data, onDone }: { sale: Sale; data: PromotionsAdmin; onDone: () => void }) {
  const [s, setS] = useState(sale);
  const [amount, setAmount] = useState(sale.kind === "fixed" ? koboToInput(sale.value) : String(sale.value));
  const { busy, error, run } = useAction();
  const set = (patch: Partial<Sale>) => setS((x) => ({ ...x, ...patch }));
  const fieldError = (f: string) => (error?.field === f ? error.message : undefined);
  const value = s.kind === "fixed" ? (parseNaira(amount) ?? 0) : Number(amount) || 0;
  const example = 1450000;
  const save = async () => {
    if (await run(() => promotionsApi.saveSale({ ...s, value }))) onDone();
  };
  const remove = async () => {
    if (!window.confirm(`Delete the sale "${s.name}"?`)) return;
    if ((await run(() => promotionsApi.deleteSale(s.id).then(() => true))) === true) onDone();
  };
  return (
    <div className="promo-editor stack">
      <ErrorBanner error={error && !error.field ? error : null} />
      <div className="grid-2">
        <Field label="Name" htmlFor="sale-name" hint="Staff see this; shoppers see the badge" error={fieldError("name")}>
          <input id="sale-name" value={s.name} placeholder="Black Friday week" onChange={(e) => set({ name: e.target.value })} />
        </Field>
        <div className="field">
          <span className="label">Discount</span>
          <div className="row">
            <select aria-label="Kind of discount" value={s.kind} onChange={(e) => { set({ kind: e.target.value as Sale["kind"] }); setAmount(""); }}>
              <option value="percent">% off</option>
              <option value="fixed">₦ off each item</option>
            </select>
            <input aria-label={s.kind === "percent" ? "Percent off" : "Naira off"} inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value)} />
          </div>
          {fieldError("value") && <span className="field-error">{fieldError("value")}</span>}
        </div>
        <Field label="Applies to" htmlFor="sale-scope" error={fieldError("appliesTo") ?? fieldError("tier") ?? fieldError("categoryId")}>
          <div className="row">
            <select id="sale-scope" value={s.appliesTo} onChange={(e) => set({ appliesTo: e.target.value as Sale["appliesTo"] })}>
              <option value="all">Everything</option>
              <option value="tier">One tier</option>
              <option value="category">One category</option>
            </select>
            {s.appliesTo === "tier" && (
              <select aria-label="Tier" value={s.tier} onChange={(e) => set({ tier: e.target.value as Sale["tier"] })}>
                <option value="">Choose…</option>
                <option value="everyday">Everyday only</option>
                <option value="premium">Premium only</option>
              </select>
            )}
            {s.appliesTo === "category" && (
              <select aria-label="Category" value={s.categoryId} onChange={(e) => set({ categoryId: Number(e.target.value) })}>
                <option value={0}>Choose…</option>
                {data.categories.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>
            )}
          </div>
        </Field>
        <div className="row">
          <Field label="First day" htmlFor="sale-start" error={fieldError("startsOn")}>
            <input id="sale-start" type="date" value={s.startsOn} onChange={(e) => set({ startsOn: e.target.value })} />
          </Field>
          <Field label="Last day" htmlFor="sale-end" error={fieldError("endsOn")}>
            <input id="sale-end" type="date" value={s.endsOn} onChange={(e) => set({ endsOn: e.target.value })} />
          </Field>
        </div>
        <Field label="Badge on products" htmlFor="sale-badge" hint="Optional, e.g. 20% OFF · BLACK FRIDAY" error={fieldError("badge")}>
          <input id="sale-badge" value={s.badge} maxLength={30} onChange={(e) => set({ badge: e.target.value })} />
        </Field>
        <Field label="Announcement bar" htmlFor="sale-ann" hint="Optional; shows at the top of the shop during the sale" error={fieldError("announcement")}>
          <input id="sale-ann" value={s.announcement} maxLength={70} onChange={(e) => set({ announcement: e.target.value })} />
        </Field>
      </div>
      <label className="check" htmlFor="sale-exclude">
        <input id="sale-exclude" type="checkbox" checked={s.excludeOnSale} onChange={(e) => set({ excludeOnSale: e.target.checked })} />
        Leave out products that are already on sale
      </label>
      <label className="check" htmlFor="sale-active">
        <input id="sale-active" type="checkbox" checked={s.active} onChange={(e) => set({ active: e.target.checked })} />
        Sale is on (untick to stop it early)
      </label>

      <div className="stack-xs">
        <span className="small muted">How a product card looks during the sale</span>
        <div className="card-preview">
          <span className="card-preview-photo">{s.badge && <span className="card-preview-badge">{s.badge}</span>}</span>
          <b>Ribbed lounge joggers</b>
          <span>
            <b>{formatNaira(salePrice(s.kind, value, example))}</b> <s className="muted">{formatNaira(example)}</s>
          </span>
        </div>
      </div>

      <div className="row">
        <button className="btn primary" disabled={busy} onClick={save}>
          {busy ? "Saving…" : s.id ? "Save sale" : "Create sale"}
        </button>
        <button className="btn" onClick={onDone}>
          Close
        </button>
        {s.id !== 0 && s.orders === 0 && (
          <button className="btn" disabled={busy} onClick={remove}>
            Delete
          </button>
        )}
      </div>
    </div>
  );
}

// ---------- Free gift ----------

function Gifts({ data, reload }: { data: PromotionsAdmin; reload: () => void }) {
  const blank: GiftOffer = {
    id: 0, thresholdKobo: 5000000, sku: "", productName: "", option: "", stock: 0, quantity: 50, given: 0,
    startsOn: "", endsOn: "", active: true, live: false,
  };
  const [adding, setAdding] = useState(false);
  return (
    <Panel
      title="Free gift with purchase"
      note={'The bag shows "₦X more for your free gift". The newest live offer applies; it stops when the gifts or the item\'s stock run out.'}
      actions={
        !adding && (
          <button className="btn" onClick={() => setAdding(true)}>
            + New gift offer
          </button>
        )
      }
    >
      {data.gifts.map((g) => (
        <GiftForm key={JSON.stringify(g)} gift={g} reload={reload} />
      ))}
      {adding && (
        <GiftForm
          gift={blank}
          reload={() => {
            setAdding(false);
            reload();
          }}
        />
      )}
      {data.gifts.length === 0 && !adding && <p className="muted">No gift offers yet.</p>}
    </Panel>
  );
}

function GiftForm({ gift, reload }: { gift: GiftOffer; reload: () => void }) {
  const [g, setG] = useState(gift);
  const [spend, setSpend] = useState(koboToInput(gift.thresholdKobo));
  const { busy, error, run } = useAction();
  const set = (patch: Partial<GiftOffer>) => setG((x) => ({ ...x, ...patch }));
  const fieldError = (f: string) => (error?.field === f ? error.message : undefined);
  const id = `gift-${gift.id || "new"}`;
  const save = async () => {
    if (await run(() => promotionsApi.saveGift({ ...g, thresholdKobo: parseNaira(spend) ?? 0 }))) reload();
  };
  return (
    <div className="promo-editor stack-sm">
      {gift.id !== 0 && (
        <div className="row-between-admin">
          <b>
            {gift.productName}
            {gift.option && ` (${gift.option})`}
          </b>
          <span className={gift.live ? "pill ok" : "pill neutral"}>{gift.live ? "Live" : gift.active ? "Not running" : "Off"}</span>
        </div>
      )}
      <div className="row wrap">
        <Field label="Spend of" htmlFor={`${id}-spend`} error={fieldError("thresholdKobo")}>
          <input id={`${id}-spend`} inputMode="decimal" value={spend} onChange={(e) => setSpend(e.target.value)} />
        </Field>
        <Field label="Gift (SKU of the option)" htmlFor={`${id}-sku`} hint="e.g. ER-HUG-GOL" error={fieldError("sku")}>
          <input id={`${id}-sku`} value={g.sku} onChange={(e) => set({ sku: e.target.value.toUpperCase() })} />
        </Field>
        <Field label="Gifts to give" htmlFor={`${id}-qty`} error={fieldError("quantity")}>
          <input id={`${id}-qty`} type="number" min={1} value={g.quantity} onChange={(e) => set({ quantity: Number(e.target.value) })} />
        </Field>
        <Field label="First day" htmlFor={`${id}-start`} hint="Empty = from now" error={fieldError("startsOn")}>
          <input id={`${id}-start`} type="date" value={g.startsOn} onChange={(e) => set({ startsOn: e.target.value })} />
        </Field>
        <Field label="Last day" htmlFor={`${id}-end`} hint="Empty = until they run out" error={fieldError("endsOn")}>
          <input id={`${id}-end`} type="date" value={g.endsOn} onChange={(e) => set({ endsOn: e.target.value })} />
        </Field>
      </div>
      <div className="row wrap">
        <label className="check" htmlFor={`${id}-on`}>
          <input id={`${id}-on`} type="checkbox" checked={g.active} onChange={(e) => set({ active: e.target.checked })} />
          Offer is on
        </label>
        {gift.id !== 0 && (
          <span className="small muted">
            {gift.given} of {gift.quantity} gifts given · {gift.stock} in stock
          </span>
        )}
        <button className="btn small primary" disabled={busy} onClick={save}>
          {gift.id ? "Save" : "Add gift offer"}
        </button>
      </div>
      <ErrorBanner error={error && !error.field ? error : null} />
    </div>
  );
}

// ---------- Discount codes ----------

function Codes({ data, reload }: { data: PromotionsAdmin; reload: () => void }) {
  const blank: DiscountCode = {
    id: 0, code: "", kind: "percent", value: 10, minOrderKobo: 0, maxUses: null, used: 0, firstOrderOnly: false,
    zoneId: 0, zoneName: "", startsOn: "", endsOn: "", active: true, summary: "",
  };
  const [editing, setEditing] = useState<DiscountCode | null>(null);
  const [session, setSession] = useState(0);
  const pick = (c: DiscountCode | null) => {
    setEditing(c);
    setSession((n) => n + 1);
  };
  return (
    <Panel
      title="Discount codes"
      actions={
        <button className="btn" onClick={() => pick(blank)}>
          + New code
        </button>
      }
    >
      {data.codes.length > 0 && (
        <table className="table">
          <thead>
            <tr>
              <th>Code</th>
              <th>What it gives</th>
              <th>Used</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {data.codes.map((c) => (
              <tr key={c.id} className="clickable" onClick={() => pick(c)}>
                <td>
                  <b>{c.code}</b>
                </td>
                <td>
                  {c.summary}
                  {c.kind === "free_delivery" && c.zoneName && ` (${c.zoneName})`}
                  {c.minOrderKobo > 0 && ` on orders over ${formatNaira(c.minOrderKobo)}`}
                  {c.firstOrderOnly && " · first order only"}
                </td>
                <td>
                  {c.used}
                  {c.maxUses != null && ` / ${c.maxUses}`}
                </td>
                <td>
                  <span className={c.active ? "pill ok" : "pill neutral"}>{c.active ? "On" : "Off"}</span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {data.codes.length === 0 && !editing && <p className="muted">No codes yet.</p>}
      {editing && (
        <CodeEditor
          key={session}
          code={editing}
          data={data}
          onDone={() => {
            pick(null);
            reload();
          }}
        />
      )}
    </Panel>
  );
}

function CodeEditor({ code, data, onDone }: { code: DiscountCode; data: PromotionsAdmin; onDone: () => void }) {
  const [c, setC] = useState(code);
  const [amount, setAmount] = useState(code.kind === "fixed" ? koboToInput(code.value) : code.kind === "percent" ? String(code.value) : "");
  const [minSpend, setMinSpend] = useState(code.minOrderKobo ? koboToInput(code.minOrderKobo) : "");
  const { busy, error, run } = useAction();
  const set = (patch: Partial<DiscountCode>) => setC((x) => ({ ...x, ...patch }));
  const fieldError = (f: string) => (error?.field === f ? error.message : undefined);
  const save = async () => {
    const value = c.kind === "fixed" ? (parseNaira(amount) ?? 0) : c.kind === "percent" ? Number(amount) || 0 : 0;
    const minOrderKobo = minSpend.trim() ? (parseNaira(minSpend) ?? -1) : 0;
    if (await run(() => promotionsApi.saveCode({ ...c, value, minOrderKobo }))) onDone();
  };
  return (
    <div className="promo-editor stack">
      <ErrorBanner error={error && !error.field ? error : null} />
      <div className="grid-2">
        <Field label="Code" htmlFor="code-code" hint="Letters and numbers; shoppers type it in the bag" error={fieldError("code")}>
          <input id="code-code" value={c.code} onChange={(e) => set({ code: e.target.value.toUpperCase() })} />
        </Field>
        <div className="field">
          <span className="label">What it gives</span>
          <div className="row">
            <select aria-label="Kind of discount" value={c.kind} onChange={(e) => { set({ kind: e.target.value as DiscountCode["kind"] }); setAmount(""); }}>
              <option value="percent">% off the items</option>
              <option value="fixed">₦ off the order</option>
              <option value="free_delivery">Free delivery</option>
            </select>
            {c.kind !== "free_delivery" && (
              <input aria-label={c.kind === "percent" ? "Percent off" : "Naira off"} inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value)} />
            )}
            {c.kind === "free_delivery" && (
              <select aria-label="Delivery area" value={c.zoneId} onChange={(e) => set({ zoneId: Number(e.target.value) })}>
                <option value={0}>Anywhere</option>
                {data.zones.map((z) => (
                  <option key={z.id} value={z.id}>
                    {z.name} only
                  </option>
                ))}
              </select>
            )}
          </div>
          {fieldError("value") && <span className="field-error">{fieldError("value")}</span>}
        </div>
        <Field label="Minimum spend (₦)" htmlFor="code-min" hint="Empty = any order" error={fieldError("minOrderKobo")}>
          <input id="code-min" inputMode="decimal" value={minSpend} onChange={(e) => setMinSpend(e.target.value)} />
        </Field>
        <Field label="Uses allowed" htmlFor="code-max" hint={`Empty = no limit. Used ${c.used} so far.`} error={fieldError("maxUses")}>
          <input id="code-max" type="number" min={1} value={c.maxUses ?? ""} onChange={(e) => set({ maxUses: e.target.value ? Number(e.target.value) : null })} />
        </Field>
        <Field label="First day" htmlFor="code-start" hint="Empty = from now" error={fieldError("startsOn")}>
          <input id="code-start" type="date" value={c.startsOn} onChange={(e) => set({ startsOn: e.target.value })} />
        </Field>
        <Field label="Last day" htmlFor="code-end" hint="Empty = no end date" error={fieldError("endsOn")}>
          <input id="code-end" type="date" value={c.endsOn} onChange={(e) => set({ endsOn: e.target.value })} />
        </Field>
      </div>
      <label className="check" htmlFor="code-first">
        <input id="code-first" type="checkbox" checked={c.firstOrderOnly} onChange={(e) => set({ firstOrderOnly: e.target.checked })} />
        First order only (checked against the phone number and email at checkout)
      </label>
      <label className="check" htmlFor="code-on">
        <input id="code-on" type="checkbox" checked={c.active} onChange={(e) => set({ active: e.target.checked })} />
        Code is on
      </label>
      <div className="row">
        <button className="btn primary" disabled={busy} onClick={save}>
          {busy ? "Saving…" : c.id ? "Save code" : "Create code"}
        </button>
        <button className="btn" onClick={onDone}>
          Close
        </button>
      </div>
    </div>
  );
}
