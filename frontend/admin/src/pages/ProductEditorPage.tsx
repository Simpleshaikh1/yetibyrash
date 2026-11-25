import { useMemo, useRef, useState, type FormEvent } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import { formatNaira, koboToInput, parseNaira, photoUrl } from "@yetibyrash/ui";
import { api, ApiError, type AdminProduct, type Setup, type SetupCategory, type Status, type Tier, type VariantInput } from "../api";
import { ErrorBanner, Field, Loading, PageHeader, Panel, SavedBanner } from "../components/ui";
import { useAction, useApi } from "../useApi";

const BADGES = ["New", "Bestseller", "Handmade"];

type Row = { sku: string; price: string; was: string; stock: string; active: boolean };

type Draft = {
  name: string;
  description: string;
  categoryId: number;
  tier: Tier;
  status: Status;
  materialLine: string;
  badges: string[];
  giftBoxAllowed: boolean;
  preorderAllowed: boolean;
  details: Record<number, string>;
  colourIds: number[];
  sizeSetId: number | null;
  sizeIds: number[];
  rows: Record<string, Row>;
  basePrice: string;
};

const comboKey = (colourId: number | null, sizeId: number | null) => `${colourId ?? "-"}/${sizeId ?? "-"}`;

function findCategory(setup: Setup, id: number): { cat?: SetupCategory; parent?: SetupCategory } {
  for (const top of setup.categories) {
    if (top.id === id) return { cat: top };
    const child = top.children.find((c) => c.id === id);
    if (child) return { cat: child, parent: top };
  }
  return {};
}

function defaultSizeSet(setup: Setup, categoryId: number): number | null {
  const { cat, parent } = findCategory(setup, categoryId);
  return cat?.defaultSizeSetId ?? parent?.defaultSizeSetId ?? null;
}

function skuPrefix(name: string): string {
  const words = name.toUpperCase().replace(/[^A-Z0-9 ]/g, "").split(/\s+/).filter(Boolean);
  return words.slice(0, 3).map((w) => w.slice(0, words.length === 1 ? 3 : 1)).join("") || "YB";
}

function emptyDraft(): Draft {
  return {
    name: "", description: "", categoryId: 0, tier: "everyday", status: "draft", materialLine: "",
    badges: [], giftBoxAllowed: true, preorderAllowed: false, details: {},
    colourIds: [], sizeSetId: null, sizeIds: [], rows: {}, basePrice: "",
  };
}

function draftFromProduct(p: AdminProduct, setup: Setup): Draft {
  const active = p.variants.filter((v) => v.active);
  const colourIds = [...new Set(active.map((v) => v.colourId).filter((x): x is number => x !== null))];
  const usedSizes = new Set(active.map((v) => v.sizeId).filter((x): x is number => x !== null));
  const anySize = p.variants.find((v) => v.sizeId !== null)?.sizeId;
  const set = setup.sizeSets.find((s) => s.sizes.some((sz) => sz.id === anySize));
  const sizeSetId = set?.id ?? defaultSizeSet(setup, p.categoryId);
  const sizeIds = (setup.sizeSets.find((s) => s.id === sizeSetId)?.sizes ?? []).map((s) => s.id).filter((id) => usedSizes.has(id));
  const rows: Record<string, Row> = {};
  for (const v of p.variants) {
    rows[comboKey(v.colourId, v.sizeId)] = {
      sku: v.sku, price: koboToInput(v.priceKobo), was: koboToInput(v.compareAtKobo), stock: String(v.stock), active: v.active,
    };
  }
  return {
    name: p.name, description: p.description, categoryId: p.categoryId, tier: p.tier, status: p.status,
    materialLine: p.materialLine, badges: p.badges, giftBoxAllowed: p.giftBoxAllowed, preorderAllowed: p.preorderAllowed,
    details: Object.fromEntries(p.details.map((d) => [d.fieldId, d.value])),
    colourIds, sizeSetId, sizeIds, rows, basePrice: koboToInput(active[0]?.priceKobo),
  };
}

export function ProductEditorPage() {
  const { id } = useParams();
  const productId = id === undefined ? 0 : Number(id);
  const setup = useApi("setup", (s) => api.setup(s));
  const product = useApi(`product:${productId}`, (s) => (productId ? api.product(productId, s) : Promise.resolve(null)));

  if (setup.error || product.error) return <ErrorBanner error={setup.error ?? product.error} />;
  if (!setup.data || (productId && !product.data)) return <Loading />;
  return (
    <Editor
      key={product.data?.id ?? "new"}
      setup={setup.data}
      product={product.data}
      reload={product.reload}
    />
  );
}

function Editor({ setup, product, reload }: { setup: Setup; product: AdminProduct | null; reload: () => void }) {
  const navigate = useNavigate();
  const location = useLocation();
  const [draft, setDraft] = useState<Draft>(() => (product ? draftFromProduct(product, setup) : emptyDraft()));
  const [saved, setSaved] = useState(Boolean((location.state as { saved?: boolean } | null)?.saved));
  const { busy, error, setError, run } = useAction();

  const set = <K extends keyof Draft>(key: K, value: Draft[K]) => {
    setSaved(false);
    setDraft((d) => ({ ...d, [key]: value }));
  };

  const { cat, parent } = findCategory(setup, draft.categoryId);
  const fields = (parent ?? cat)?.fields ?? [];
  const sizeSet = setup.sizeSets.find((s) => s.id === draft.sizeSetId) ?? null;
  const colourName = (cid: number | null) => setup.colours.find((c) => c.id === cid)?.name ?? "";
  const sizeLabel = (sid: number | null) => sizeSet?.sizes.find((s) => s.id === sid)?.label ?? "";

  // Every colour × size combination the product is sold in.
  const combos = useMemo(() => {
    const cs = draft.colourIds.length ? draft.colourIds : [null];
    const ss = draft.sizeIds.length ? draft.sizeIds : [null];
    return cs.flatMap((c) => ss.map((s) => ({ colourId: c, sizeId: s, key: comboKey(c, s) })));
  }, [draft.colourIds, draft.sizeIds]);

  const rowFor = (c: number | null, s: number | null): Row => {
    const existing = draft.rows[comboKey(c, s)];
    if (existing) return existing;
    const parts = [skuPrefix(draft.name)];
    if (c !== null) parts.push(colourName(c).toUpperCase().replace(/[^A-Z0-9]/g, "").slice(0, 3));
    if (s !== null) parts.push(sizeLabel(s).toUpperCase().replace(/[^A-Z0-9]/g, ""));
    return { sku: parts.join("-"), price: draft.basePrice, was: "", stock: "0", active: true };
  };

  const setRow = (key: string, c: number | null, s: number | null, patch: Partial<Row>) => {
    setSaved(false);
    setDraft((d) => ({ ...d, rows: { ...d.rows, [key]: { ...rowFor(c, s), ...patch } } }));
  };

  const changeCategory = (categoryId: number) => {
    const sizeSetId = defaultSizeSet(setup, categoryId);
    const sizes = setup.sizeSets.find((s) => s.id === sizeSetId)?.sizes ?? [];
    setSaved(false);
    setDraft((d) => ({
      ...d,
      categoryId,
      sizeSetId: d.sizeSetId === sizeSetId ? d.sizeSetId : sizeSetId,
      sizeIds: d.sizeSetId === sizeSetId ? d.sizeIds : sizes.map((s) => s.id),
    }));
  };

  const changeSizeSet = (sizeSetId: number | null) => {
    const sizes = setup.sizeSets.find((s) => s.id === sizeSetId)?.sizes ?? [];
    set("sizeSetId", sizeSetId);
    set("sizeIds", sizes.map((s) => s.id));
  };

  const toggle = (list: number[], id: number, order?: number[]) => {
    const next = list.includes(id) ? list.filter((x) => x !== id) : [...list, id];
    return order ? order.filter((x) => next.includes(x)) : next;
  };

  const applyBasePrice = () => {
    const rows = { ...draft.rows };
    for (const c of combos) rows[c.key] = { ...rowFor(c.colourId, c.sizeId), price: draft.basePrice };
    set("rows", rows);
  };

  const save = async (e: FormEvent) => {
    e.preventDefault();
    const variants: VariantInput[] = [];
    for (const [i, c] of combos.entries()) {
      const r = rowFor(c.colourId, c.sizeId);
      const label = [colourName(c.colourId), sizeLabel(c.sizeId)].filter(Boolean).join(" / ") || "the option";
      const price = parseNaira(r.price);
      if (price === null) {
        setError(new ApiError(400, "invalid", `Enter a price for ${label} (row ${i + 1}).`));
        return;
      }
      const was = r.was.trim() ? parseNaira(r.was) : null;
      if (r.was.trim() && was === null) {
        setError(new ApiError(400, "invalid", `The "was" price for ${label} isn't a number.`));
        return;
      }
      const stock = Number(r.stock);
      if (!Number.isInteger(stock) || stock < 0) {
        setError(new ApiError(400, "invalid", `Stock for ${label} must be a whole number.`));
        return;
      }
      variants.push({ colourId: c.colourId, sizeId: c.sizeId, sku: r.sku, priceKobo: price, compareAtKobo: was, stock, active: r.active });
    }
    const body = {
      name: draft.name, description: draft.description, categoryId: draft.categoryId, tier: draft.tier, status: draft.status,
      materialLine: draft.materialLine, badges: draft.badges, giftBoxAllowed: draft.giftBoxAllowed, preorderAllowed: draft.preorderAllowed,
      details: fields.map((f) => ({ fieldId: f.id, value: draft.details[f.id] ?? "" })),
      variants,
    };
    if (product) {
      const ok = await run(() => api.updateProduct(product.id, body).then(() => true));
      if (ok) {
        setSaved(true);
        reload();
      }
    } else {
      const res = await run(() => api.createProduct(body));
      if (res) navigate(`/products/${res.id}`, { replace: true, state: { saved: true } });
    }
  };

  const fieldError = (name: string) => (error?.field === name ? error.message : undefined);
  const shopLink = product && product.status === "visible" ? `http://localhost:5173/p/${product.slug}` : null;

  return (
    <form onSubmit={save} className="stack">
      <PageHeader
        title={product ? product.name : "Add a product"}
        back={<Link to="/products">‹ Products</Link>}
        actions={
          <>
            {shopLink && (
              <a className="btn secondary" href={shopLink} target="_blank" rel="noreferrer">
                View on shop
              </a>
            )}
            <button className="btn primary" type="submit" disabled={busy}>
              {busy ? "Saving…" : product ? "Save product" : "Create product"}
            </button>
          </>
        }
      />
      <SavedBanner show={saved}>{product ? "Product saved." : ""}</SavedBanner>
      {error && !error.field?.startsWith("name") && <ErrorBanner error={error} />}

      <div className="editor-grid">
        <div className="stack">
          <Panel title="1. What is it?">
            <Field label="Product name" htmlFor="p-name" error={fieldError("name")}>
              <input id="p-name" value={draft.name} onChange={(e) => set("name", e.target.value)} maxLength={120} />
            </Field>
            <Field label="Category" htmlFor="p-cat" error={fieldError("categoryId")} hint="Decides the sizes and the detail fields below.">
              <select id="p-cat" value={draft.categoryId || ""} onChange={(e) => changeCategory(Number(e.target.value))}>
                <option value="">Choose a category</option>
                {setup.categories.map((top) =>
                  top.children.length === 0 ? (
                    <option key={top.id} value={top.id}>
                      {top.name}
                    </option>
                  ) : (
                    <optgroup key={top.id} label={top.name}>
                      {top.children.map((c) => (
                        <option key={c.id} value={c.id}>
                          {c.name}
                        </option>
                      ))}
                    </optgroup>
                  ),
                )}
              </select>
            </Field>
            <Field label="Description" htmlFor="p-desc">
              <textarea id="p-desc" rows={3} value={draft.description} onChange={(e) => set("description", e.target.value)} />
            </Field>
            <Field label="Line under the name" htmlFor="p-material" error={fieldError("materialLine")} hint='e.g. "18k gold plated" or "Cotton-blend rib"'>
              <input id="p-material" value={draft.materialLine} onChange={(e) => set("materialLine", e.target.value)} maxLength={80} />
            </Field>
          </Panel>

          {product ? (
            <Photos product={product} reload={reload} />
          ) : (
            <Panel title="2. Photos" note="Create the product first, then add photos here.">
              <span />
            </Panel>
          )}

          <Panel title="3. Colours and sizes" note="Tap to turn colours and sizes on or off. Each combination becomes an option with its own price and stock.">
            <div className="stack-xs">
              <span className="label">Colours</span>
              <div className="chips">
                {setup.colours.map((c) => (
                  <button
                    type="button"
                    key={c.id}
                    className={draft.colourIds.includes(c.id) ? "chip on" : "chip"}
                    aria-pressed={draft.colourIds.includes(c.id)}
                    onClick={() => set("colourIds", toggle(draft.colourIds, c.id))}
                  >
                    <span className="dot" style={{ background: c.hex }} aria-hidden="true" />
                    {c.name}
                  </button>
                ))}
              </div>
            </div>
            <Field label="Size set" htmlFor="p-sizeset">
              <select id="p-sizeset" value={draft.sizeSetId ?? ""} onChange={(e) => changeSizeSet(e.target.value ? Number(e.target.value) : null)}>
                <option value="">No sizes</option>
                {setup.sizeSets.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                    {s.sizes.length === 0 ? " (no sizes)" : ""}
                  </option>
                ))}
              </select>
            </Field>
            {sizeSet && sizeSet.sizes.length > 0 && (
              <div className="chips">
                {sizeSet.sizes.map((s) => (
                  <button
                    type="button"
                    key={s.id}
                    className={draft.sizeIds.includes(s.id) ? "chip on" : "chip"}
                    aria-pressed={draft.sizeIds.includes(s.id)}
                    onClick={() => set("sizeIds", toggle(draft.sizeIds, s.id, sizeSet.sizes.map((x) => x.id)))}
                  >
                    {s.label}
                  </button>
                ))}
              </div>
            )}

            <div className="row">
              <Field label="Price for all options (₦)" htmlFor="p-base">
                <input id="p-base" inputMode="decimal" className="w-140" value={draft.basePrice} onChange={(e) => set("basePrice", e.target.value)} />
              </Field>
              <button type="button" className="btn secondary align-end" onClick={applyBasePrice} disabled={parseNaira(draft.basePrice) === null}>
                Set all prices
              </button>
            </div>

            <div className="table-wrap">
              <table className="table compact">
                <thead>
                  <tr>
                    <th>Option</th>
                    <th>SKU</th>
                    <th>Price ₦</th>
                    <th>Was ₦</th>
                    <th>Stock</th>
                    <th>On sale</th>
                  </tr>
                </thead>
                <tbody>
                  {combos.map((c, i) => {
                    const r = rowFor(c.colourId, c.sizeId);
                    const label = [colourName(c.colourId), sizeLabel(c.sizeId)].filter(Boolean).join(" / ") || "One option";
                    const rowErr = error?.field?.startsWith(`variants.${i}`);
                    return (
                      <tr key={c.key} className={rowErr ? "row-error" : ""}>
                        <td>{label}</td>
                        <td>
                          <input aria-label={`SKU for ${label}`} className="w-140" value={r.sku} onChange={(e) => setRow(c.key, c.colourId, c.sizeId, { sku: e.target.value.toUpperCase() })} />
                        </td>
                        <td>
                          <input aria-label={`Price for ${label}`} inputMode="decimal" className="w-100" value={r.price} onChange={(e) => setRow(c.key, c.colourId, c.sizeId, { price: e.target.value })} />
                        </td>
                        <td>
                          <input aria-label={`Was price for ${label}`} inputMode="decimal" className="w-100" placeholder="—" value={r.was} onChange={(e) => setRow(c.key, c.colourId, c.sizeId, { was: e.target.value })} />
                        </td>
                        <td>
                          <input
                            aria-label={`Stock for ${label}`}
                            inputMode="numeric"
                            className={`w-70 ${r.stock === "0" ? "input-danger" : Number(r.stock) <= 3 ? "input-warning" : ""}`}
                            value={r.stock}
                            onChange={(e) => setRow(c.key, c.colourId, c.sizeId, { stock: e.target.value })}
                          />
                        </td>
                        <td>
                          <input type="checkbox" aria-label={`${label} on sale`} checked={r.active} onChange={(e) => setRow(c.key, c.colourId, c.sizeId, { active: e.target.checked })} />
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
            <p className="small muted">
              {combos.length} {combos.length === 1 ? "option" : "options"}. Options you remove are switched off, not deleted, so past orders still show them.
            </p>
          </Panel>

          {fields.length > 0 && (
            <Panel title={`4. ${(parent ?? cat)?.name} details`} note={'Shown on the product page under "Details". Leave a field blank to hide it.'}>
              <div className="grid-2">
                {fields.map((f) => (
                  <Field key={f.id} label={f.label} htmlFor={`d-${f.id}`}>
                    <input id={`d-${f.id}`} value={draft.details[f.id] ?? ""} onChange={(e) => set("details", { ...draft.details, [f.id]: e.target.value })} />
                  </Field>
                ))}
              </div>
            </Panel>
          )}
        </div>

        <div className="stack">
          <Panel title="On the shop">
            <Field label="Status" htmlFor="p-status" error={fieldError("status")}>
              <select id="p-status" value={draft.status} onChange={(e) => set("status", e.target.value as Status)}>
                <option value="draft">Draft (only staff see it)</option>
                <option value="visible">Visible to shoppers</option>
                <option value="hidden">Hidden</option>
              </select>
            </Field>
          </Panel>
          <Panel title="Tier" note="Premium adds the dark label and the gift-box option on the product page.">
            <div className="segmented" role="radiogroup" aria-label="Tier">
              {(["everyday", "premium"] as Tier[]).map((t) => (
                <button type="button" role="radio" aria-checked={draft.tier === t} key={t} className={draft.tier === t ? "on" : ""} onClick={() => set("tier", t)}>
                  {t === "everyday" ? "Everyday" : "Premium"}
                </button>
              ))}
            </div>
          </Panel>
          <Panel title="Badges">
            <div className="chips">
              {BADGES.map((b) => (
                <button type="button" key={b} className={draft.badges.includes(b) ? "chip on" : "chip"} aria-pressed={draft.badges.includes(b)} onClick={() => set("badges", draft.badges.includes(b) ? draft.badges.filter((x) => x !== b) : [...draft.badges, b])}>
                  {b}
                </button>
              ))}
            </div>
          </Panel>
          <Panel title="Selling options">
            <label className="check" htmlFor="p-gift">
              <input id="p-gift" type="checkbox" checked={draft.giftBoxAllowed} onChange={(e) => set("giftBoxAllowed", e.target.checked)} />
              Offer gift box
            </label>
            <label className="check" htmlFor="p-preorder">
              <input id="p-preorder" type="checkbox" checked={draft.preorderAllowed} onChange={(e) => set("preorderAllowed", e.target.checked)} />
              Allow pre-order when sold out
            </label>
          </Panel>
          {cat && (
            <Panel title="Returns">
              <p className="small">{cat.returnRule || parent?.returnRule || "No return rule set for this category."}</p>
              <p className="small muted">Set per category in Catalogue setup.</p>
            </Panel>
          )}
          {product && combos.length > 0 && (
            <Panel title="Price range">
              <p>
                {(() => {
                  const prices = combos.map((c) => parseNaira(rowFor(c.colourId, c.sizeId).price)).filter((x): x is number => x !== null);
                  if (!prices.length) return "—";
                  const lo = Math.min(...prices);
                  const hi = Math.max(...prices);
                  return lo === hi ? formatNaira(lo) : `${formatNaira(lo)} – ${formatNaira(hi)}`;
                })()}
              </p>
            </Panel>
          )}
        </div>
      </div>
    </form>
  );
}

const warningText: Record<string, string> = {
  small: "Small: under 1200 px, zoom will look soft.",
  dark: "Too dark: retake or brighten.",
};

function Photos({ product, reload }: { product: AdminProduct; reload: () => void }) {
  const input = useRef<HTMLInputElement>(null);
  const [kind, setKind] = useState("studio");
  const [progress, setProgress] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<number | null>(null);
  const { busy, error, run } = useAction();

  const upload = async (files: FileList | null) => {
    if (!files?.length) return;
    const list = [...files];
    for (const [i, f] of list.entries()) {
      setProgress(`Uploading ${i + 1} of ${list.length}: ${f.name}`);
      const ok = await run(() => api.uploadImage(product.id, f, kind));
      if (!ok) break;
    }
    setProgress(null);
    if (input.current) input.current.value = "";
    reload();
  };

  return (
    <Panel title="2. Photos" note="Photos are cropped to a square. The first photo shows in the shop grid. Phone shots under 1200 px or too dark get a warning.">
      <ErrorBanner error={error} />
      <div className="photos">
        {product.images.map((im, i) => (
          <figure key={im.id} className={im.warnings.length ? "photo warn" : "photo"}>
            <img src={photoUrl(im.key, 400)} alt={`Photo ${i + 1}`} />
            <figcaption>
              <span className="tag">{i === 0 ? "Main · " : ""}{im.kind === "real_life" ? "Real life" : "Studio"}</span>
              {im.warnings.map((w) => (
                <span key={w} className="warn-text">
                  {warningText[w] ?? w}
                </span>
              ))}
              <span className="row">
                {i > 0 && (
                  <button type="button" className="link-btn" disabled={busy} onClick={() => run(() => api.makeImageMain(product.id, im.id)).then(reload)}>
                    Make main
                  </button>
                )}
                {confirmDelete === im.id ? (
                  <>
                    <button type="button" className="link-btn danger-text" disabled={busy} onClick={() => run(() => api.deleteImage(product.id, im.id)).then(reload)}>
                      Delete photo
                    </button>
                    <button type="button" className="link-btn" onClick={() => setConfirmDelete(null)}>
                      Keep
                    </button>
                  </>
                ) : (
                  <button type="button" className="link-btn" onClick={() => setConfirmDelete(im.id)}>
                    Remove
                  </button>
                )}
              </span>
            </figcaption>
          </figure>
        ))}
        <label className="photo add" htmlFor="photo-input">
          <input id="photo-input" ref={input} type="file" accept="image/jpeg,image/png,image/webp" multiple className="visually-hidden" onChange={(e) => upload(e.target.files)} disabled={busy} />
          <span>{progress ?? "+ Add photos"}</span>
        </label>
      </div>
      <div className="row">
        <label htmlFor="photo-kind" className="label">
          New photos are
        </label>
        <select id="photo-kind" value={kind} onChange={(e) => setKind(e.target.value)}>
          <option value="studio">Studio shots</option>
          <option value="real_life">Real-life shots</option>
        </select>
      </div>
    </Panel>
  );
}
