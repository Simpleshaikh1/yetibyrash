import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { formatNaira, photoUrl } from "@yetibyrash/ui";
import { api } from "../api";
import { ErrorBanner, Loading, PageHeader, StatusPill } from "../components/ui";
import { useApi } from "../useApi";

export function ProductsPage() {
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const setup = useApi("setup", (s) => api.setup(s));
  const list = useApi(`products:${params}`, (s) => api.products(params, s));

  const set = (key: string, value: string) => {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value);
    else next.delete(key);
    setParams(next, { replace: true });
  };

  return (
    <>
      <PageHeader
        title="Products"
        actions={
          <Link to="/products/new" className="btn primary">
            + Add product
          </Link>
        }
      />

      <div className="row wrap">
        <label className="visually-hidden" htmlFor="search">
          Search products
        </label>
        <input id="search" className="w-280" placeholder="Search by name or SKU" defaultValue={params.get("search") ?? ""} onChange={(e) => set("search", e.target.value)} />
        <label className="visually-hidden" htmlFor="category">
          Category
        </label>
        <select id="category" value={params.get("category") ?? ""} onChange={(e) => set("category", e.target.value)}>
          <option value="">All categories</option>
          {setup.data?.categories.map((c) => (
            <optgroup key={c.id} label={c.name}>
              <option value={c.id}>All {c.name.toLowerCase()}</option>
              {c.children.map((ch) => (
                <option key={ch.id} value={ch.id}>
                  {ch.name}
                </option>
              ))}
            </optgroup>
          ))}
        </select>
        <label className="visually-hidden" htmlFor="status">
          Status
        </label>
        <select id="status" value={params.get("status") ?? ""} onChange={(e) => set("status", e.target.value)}>
          <option value="">Any status</option>
          <option value="visible">Visible</option>
          <option value="draft">Draft</option>
          <option value="hidden">Hidden</option>
        </select>
      </div>

      <ErrorBanner error={list.error} />
      {list.loading && !list.data && <Loading />}
      {list.data && list.data.length === 0 && <p className="muted">No products match. Clear the search or filters, or add a product.</p>}
      {list.data && list.data.length > 0 && (
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th aria-label="Photo" />
                <th>Product</th>
                <th>Category</th>
                <th>Tier</th>
                <th>Price</th>
                <th>Stock</th>
                <th>On shop</th>
              </tr>
            </thead>
            <tbody>
              {list.data.map((p) => (
                <tr key={p.id} className="clickable" onClick={() => navigate(`/products/${p.id}`)}>
                  <td>
                    {p.image ? <img className="thumb" src={photoUrl(p.image, 400)} alt="" /> : <span className="thumb empty" aria-hidden="true" />}
                  </td>
                  <td>
                    <Link to={`/products/${p.id}`} onClick={(e) => e.stopPropagation()}>
                      <b>{p.name}</b>
                    </Link>
                    <div className="small muted">
                      {p.optionCount} {p.optionCount === 1 ? "option" : "options"}
                    </div>
                  </td>
                  <td className="muted">{p.category}</td>
                  <td>
                    <span className={`pill ${p.tier === "premium" ? "premium" : "everyday"}`}>{p.tier === "premium" ? "Premium" : "Everyday"}</span>
                  </td>
                  <td className="num">
                    {p.optionCount === 0 ? "—" : p.minPriceKobo === p.maxPriceKobo ? formatNaira(p.minPriceKobo) : `${formatNaira(p.minPriceKobo)} – ${formatNaira(p.maxPriceKobo)}`}
                  </td>
                  <td className={p.totalStock === 0 ? "num danger-text" : p.soldOutCount > 0 ? "num warning-text" : "num"}>
                    {p.totalStock === 0 ? "Sold out" : p.totalStock}
                    {p.totalStock > 0 && p.soldOutCount > 0 && <span className="small"> · {p.soldOutCount} sold out</span>}
                  </td>
                  <td>
                    <StatusPill status={p.status} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
