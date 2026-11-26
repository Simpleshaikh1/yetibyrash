import { Link } from "react-router-dom";
import { useAccount } from "../account";
import { api } from "../api";
import { ProductGrid } from "../components/ProductCard";
import { ErrorMessage, Loading } from "../components/Status";
import { useApi } from "../useApi";

export function SavedPage() {
  const { customer, saved } = useAccount();
  if (customer === undefined) return <Loading />;
  if (!customer) {
    return (
      <div className="stack narrow-page">
        <h1 className="page-title">Saved items</h1>
        <p className="muted">Sign in to keep the pieces you love in one place, on any phone or laptop.</p>
        <Link to="/account?next=/saved" className="btn primary">
          Sign in
        </Link>
      </div>
    );
  }
  return <SavedList key={[...saved].join(",")} />;
}

function SavedList() {
  const { data, error } = useApi("saved", (s) => api.saved(s));
  return (
    <div className="stack">
      <h1 className="page-title">Saved items</h1>
      {error && <ErrorMessage error={error} />}
      {!data && !error && <Loading />}
      {data?.length === 0 && (
        <p className="muted">
          Nothing saved yet. Tap the heart on a product to keep it here. <Link to="/shop">Browse the shop</Link>
        </p>
      )}
      {data && data.length > 0 && <ProductGrid items={data} />}
    </div>
  );
}
