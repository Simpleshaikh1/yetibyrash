import { Link, NavLink, Outlet } from "react-router-dom";
import { useBag } from "../bag";
import { Icon, type IconName } from "./Icon";

const nav: { to: string; label: string; icon: IconName; end?: boolean }[] = [
  { to: "/", label: "Home", icon: "home", end: true },
  { to: "/shop", label: "Shop", icon: "grid" },
  { to: "/saved", label: "Saved", icon: "heart" },
  { to: "/bag", label: "Bag", icon: "bag" },
  { to: "/track", label: "Track", icon: "track" },
];

// Every shop screen: header with the wordmark and bag, page content, bottom navigation.
export function Layout() {
  const { count } = useBag();
  const bagLabel = count ? `Bag, ${count} ${count === 1 ? "item" : "items"}` : "Bag";
  return (
    <div className="app">
      <header className="topbar">
        <Link to="/shop" className="icon-btn" aria-label="Browse categories">
          <Icon name="menu" />
        </Link>
        <Link to="/" className="wordmark">
          Yetibyrash
        </Link>
        <Link to="/bag" className="icon-btn bag-icon" aria-label={bagLabel}>
          <Icon name="bag" />
          {count > 0 && (
            <span className="count" aria-hidden="true">
              {count}
            </span>
          )}
        </Link>
      </header>
      <main className="page">
        <Outlet />
      </main>
      <nav className="bottomnav" aria-label="Main">
        {nav.map((n) => (
          <NavLink key={n.to} to={n.to} end={n.end} aria-label={n.to === "/bag" ? bagLabel : undefined}>
            <Icon name={n.icon} />
            {n.label}
          </NavLink>
        ))}
      </nav>
    </div>
  );
}
