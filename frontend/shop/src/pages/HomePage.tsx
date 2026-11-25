import { Link } from "react-router-dom";
import { api } from "../api";
import { ProductGrid } from "../components/ProductCard";
import { ErrorMessage, Loading } from "../components/Status";
import { TierSwitch } from "../components/TierSwitch";
import { useTier } from "../tier";
import { useApi } from "../useApi";

export function HomePage() {
  const { tier, setTier } = useTier();
  const { data, error, loading } = useApi(`home:${tier}`, (s) => api.home(tier, s));

  return (
    <div className="stack">
      <TierSwitch value={tier} onChange={(t) => setTier(t || "everyday")} />

      <section className="hero">
        <span className="hero-y" aria-hidden="true">
          Y
        </span>
        <span className="eyebrow">{tier === "premium" ? "Premium" : "Everyday"} · New in</span>
        <h1>{data?.tagline ?? "Elegance meets affordability"}</h1>
        <Link to={`/shop?tier=${tier}`} className="btn primary small">
          Shop the edit
        </Link>
      </section>

      {error && <ErrorMessage error={error} />}
      {loading && !data && <Loading />}

      {data && (
        <>
          <section className="stack-sm">
            <h2>Shop by category</h2>
            <div className="cat-row">
              {data.categories.map((c) => (
                <Link key={c.id} to={`/shop/${c.slug}`} className="cat">
                  <span className="cat-dot" aria-hidden="true">
                    {c.name.charAt(0)}
                  </span>
                  {c.name}
                </Link>
              ))}
            </div>
          </section>

          <section className="stack-sm">
            <div className="row-between">
              <h2>New in</h2>
              <Link to={`/shop?tier=${tier}`}>See all</Link>
            </div>
            {data.newIn.length === 0 ? <p className="muted">Nothing new in this range yet.</p> : <ProductGrid items={data.newIn} />}
          </section>
        </>
      )}

      <section className="trust">
        <span>Secure payment with Paystack</span>
        <span>Discreet packaging</span>
        <span>Delivery to all 36 states</span>
      </section>
    </div>
  );
}
