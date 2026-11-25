import { useEffect, useState } from "react";
import { Link, NavLink, Outlet } from "react-router-dom";
import { api, type Site } from "../api";
import { useBag } from "../bag";
import { useApi } from "../useApi";
import { Icon, type IconName } from "./Icon";

const nav: { to: string; label: string; icon: IconName; end?: boolean }[] = [
  { to: "/", label: "Home", icon: "home", end: true },
  { to: "/shop", label: "Shop", icon: "grid" },
  { to: "/saved", label: "Saved", icon: "heart" },
  { to: "/bag", label: "Bag", icon: "bag" },
  { to: "/track", label: "Track", icon: "track" },
];

// Every shop screen: announcement bar, header with the wordmark and bag, page content,
// footer links, and the bottom navigation.
export function Layout() {
  const { count } = useBag();
  const site = useApi("site", (s) => api.site(s));
  const bagLabel = count ? `Bag, ${count} ${count === 1 ? "item" : "items"}` : "Bag";
  return (
    <div className="app">
      {site.data && site.data.announcements.length > 0 && <AnnouncementBar items={site.data.announcements} />}
      <header className="topbar">
        <Link to="/shop" className="icon-btn menu-btn" aria-label="Browse categories">
          <Icon name="menu" />
        </Link>
        <Link to="/" className="wordmark">
          Yetibyrash
        </Link>
        {/* Laptops and wider: the main links sit in the header instead of the bottom bar. */}
        <nav className="topnav" aria-label="Main">
          {nav
            .filter((n) => n.to !== "/bag")
            .map((n) => (
              <NavLink key={n.to} to={n.to} end={n.end}>
                {n.label}
              </NavLink>
            ))}
        </nav>
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
        {site.data && <Footer pages={site.data.pages} />}
      </main>
      <nav className="bottomnav" aria-label="Main (phone)">
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

// One message at a time, changing every 5 seconds (paused while the pointer is over it).
function AnnouncementBar({ items }: { items: Site["announcements"] }) {
  const [i, setI] = useState(0);
  const [paused, setPaused] = useState(false);
  useEffect(() => {
    if (items.length < 2 || paused) return;
    const t = setInterval(() => setI((n) => (n + 1) % items.length), 5000);
    return () => clearInterval(t);
  }, [items.length, paused]);
  const a = items[i % items.length];
  return (
    <div className="announce" onMouseEnter={() => setPaused(true)} onMouseLeave={() => setPaused(false)}>
      {a.link ? (
        <Link key={i} to={a.link}>
          {a.text}
        </Link>
      ) : (
        <span key={i}>{a.text}</span>
      )}
    </div>
  );
}

function Footer({ pages }: { pages: Site["pages"] }) {
  return (
    <footer className="footer">
      <span className="wordmark small-mark">Yetibyrash</span>
      <span className="small muted">Elegance meets affordability</span>
      {pages.length > 0 && (
        <nav aria-label="About the shop" className="footer-links">
          {pages.map((p) => (
            <Link key={p.slug} to={`/page/${p.slug}`}>
              {p.title}
            </Link>
          ))}
        </nav>
      )}
    </footer>
  );
}
