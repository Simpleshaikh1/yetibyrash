import { Link } from "react-router-dom";

// Placeholder for screens built in later milestones.
export function ComingSoon({ title, milestone }: { title: string; milestone: string }) {
  return (
    <div className="stack">
      <h1 className="page-title">{title}</h1>
      <p className="status-msg">This screen arrives in milestone {milestone}.</p>
      <Link to="/shop" className="btn outline">
        Keep shopping
      </Link>
    </div>
  );
}

export function NotFound() {
  return (
    <div className="stack">
      <h1 className="page-title">Page not found</h1>
      <p className="status-msg">There's nothing at this address.</p>
      <Link to="/" className="btn outline">
        Go to the homepage
      </Link>
    </div>
  );
}
