import { useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { api, type Category, type Tier } from "../api";
import { Icon } from "../components/Icon";
import { ProductGrid } from "../components/ProductCard";
import { ErrorMessage, Loading } from "../components/Status";
import { TierSwitch } from "../components/TierSwitch";
import { useApi } from "../useApi";

const sorts = [
  { value: "newest", label: "Newest" },
  { value: "price_asc", label: "Price: low to high" },
  { value: "price_desc", label: "Price: high to low" },
];

// Finds the category for the URL and the top category whose tabs to show.
function locate(cats: Category[], slug?: string): { current?: Category; top?: Category } {
  for (const top of cats) {
    if (top.slug === slug) return { current: top, top };
    const child = top.children.find((c) => c.slug === slug);
    if (child) return { current: child, top };
  }
  return {};
}

// Shop grid for all products or one category. Every filter lives in the URL,
// so the back button and shared links keep the shopper's choices.
export function ShopPage() {
  const { category } = useParams();
  const [params, setParams] = useSearchParams();
  const [showFilters, setShowFilters] = useState(false);

  const cats = useApi("categories", (s) => api.categories(s));
  const query = new URLSearchParams(params);
  if (category) query.set("category", category);
  const list = useApi(`products:${query.toString()}`, (s) => api.products(query, s));

  const { current, top } = locate(cats.data ?? [], category);
  const title = current?.name ?? (category ? "Shop" : "All products");

  // Changing a filter goes back to page 1.
  const setParam = (key: string, value: string) => {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value);
    else next.delete(key);
    next.delete("page");
    setParams(next, { replace: true });
  };

  const active: { key: string; label: string }[] = [];
  const bandId = params.get("band");
  const band = list.data?.filters.priceBands.find((b) => String(b.id) === bandId);
  if (band) active.push({ key: "band", label: band.label });
  if (params.get("colour")) active.push({ key: "colour", label: params.get("colour")! });
  if (params.get("size")) active.push({ key: "size", label: `Size ${params.get("size")}` });

  const page = Number(params.get("page") ?? "1");
  const totalPages = list.data ? Math.max(1, Math.ceil(list.data.total / list.data.pageSize)) : 1;

  return (
    <div className="stack">
      <div className="row-start">
        <Link to={top && top !== current ? `/shop/${top.slug}` : "/"} className="icon-btn" aria-label="Back">
          <Icon name="back" />
        </Link>
        <h1 className="page-title">{title}</h1>
      </div>

      {top && (
        <div className="chips" role="list">
          <Link to={`/shop/${top.slug}?${params.toString()}`} className={current === top ? "chip on" : "chip"}>
            All
          </Link>
          {top.children.map((c) => (
            <Link key={c.id} to={`/shop/${c.slug}?${params.toString()}`} className={current === c ? "chip on" : "chip"}>
              {c.name}
            </Link>
          ))}
        </div>
      )}
      {!category && cats.data && (
        <div className="chips" role="list">
          {cats.data.map((c) => (
            <Link key={c.id} to={`/shop/${c.slug}?${params.toString()}`} className="chip">
              {c.name}
            </Link>
          ))}
        </div>
      )}

      <TierSwitch withAll value={(params.get("tier") as Tier | null) ?? ""} onChange={(t) => setParam("tier", t)} />

      <div className="row-between">
        <span className="muted">{list.data ? `${list.data.total} ${list.data.total === 1 ? "item" : "items"}` : " "}</span>
        <div className="row-start">
          <button className="btn outline small" onClick={() => setShowFilters((v) => !v)} aria-expanded={showFilters}>
            <Icon name="filter" size={16} /> Filter{active.length > 0 && ` · ${active.length}`}
          </button>
          <label className="visually-hidden" htmlFor="sort">
            Sort
          </label>
          <select id="sort" className="select" value={params.get("sort") ?? "newest"} onChange={(e) => setParam("sort", e.target.value === "newest" ? "" : e.target.value)}>
            {sorts.map((s) => (
              <option key={s.value} value={s.value}>
                {s.label}
              </option>
            ))}
          </select>
        </div>
      </div>

      {showFilters && list.data && (
        <section className="filters" aria-label="Filters">
          <div className="stack-xs">
            <span className="label">Price</span>
            <div className="chips wrap">
              {list.data.filters.priceBands.map((b) => (
                <button key={b.id} className={bandId === String(b.id) ? "chip on" : "chip"} onClick={() => setParam("band", bandId === String(b.id) ? "" : String(b.id))}>
                  {b.label}
                </button>
              ))}
            </div>
          </div>
          {list.data.filters.colours.length > 0 && (
            <div className="stack-xs">
              <span className="label">Colour</span>
              <div className="chips wrap">
                {list.data.filters.colours.map((c) => (
                  <button key={c.name} className={params.get("colour") === c.name ? "chip on" : "chip"} onClick={() => setParam("colour", params.get("colour") === c.name ? "" : c.name)}>
                    <span className="dot" style={{ background: c.hex }} aria-hidden="true" />
                    {c.name}
                  </button>
                ))}
              </div>
            </div>
          )}
          {list.data.filters.sizes.length > 0 && (
            <div className="stack-xs">
              <span className="label">Size</span>
              <div className="chips wrap">
                {list.data.filters.sizes.map((s) => (
                  <button key={s} className={params.get("size") === s ? "chip on" : "chip"} onClick={() => setParam("size", params.get("size") === s ? "" : s)}>
                    {s}
                  </button>
                ))}
              </div>
            </div>
          )}
        </section>
      )}

      {active.length > 0 && (
        <div className="chips wrap">
          {active.map((a) => (
            <button key={a.key} className="chip dark" onClick={() => setParam(a.key, "")} aria-label={`Remove filter ${a.label}`}>
              {a.label} <Icon name="close" size={14} />
            </button>
          ))}
        </div>
      )}

      {list.error && <ErrorMessage error={list.error} />}
      {list.loading && !list.data && <Loading />}
      {list.data && list.data.items.length === 0 && (
        <p className="status-msg">Nothing matches these filters. Try removing one.</p>
      )}
      {list.data && list.data.items.length > 0 && <ProductGrid items={list.data.items} />}

      {totalPages > 1 && (
        <div className="row-between">
          <button className="btn outline small" disabled={page <= 1} onClick={() => setParams(withPage(params, page - 1))}>
            Previous
          </button>
          <span className="muted">
            Page {page} of {totalPages}
          </span>
          <button className="btn outline small" disabled={page >= totalPages} onClick={() => setParams(withPage(params, page + 1))}>
            Next
          </button>
        </div>
      )}
    </div>
  );
}

function withPage(params: URLSearchParams, page: number) {
  const next = new URLSearchParams(params);
  if (page > 1) next.set("page", String(page));
  else next.delete("page");
  return next;
}
