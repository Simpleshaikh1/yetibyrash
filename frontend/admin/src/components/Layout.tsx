import { NavLink, Outlet } from "react-router-dom";
import { roleLabels, type Permission } from "../api";
import { useAuth } from "../auth";

const nav: { to: string; label: string; perm?: Permission; soon?: string }[] = [
  { to: "/", label: "Overview" },
  { to: "/orders", label: "Orders", perm: "orders", soon: "M4" },
  { to: "/products", label: "Products", perm: "products" },
  { to: "/catalogue", label: "Catalogue setup", perm: "catalogue" },
  { to: "/content", label: "Homepage and content", perm: "catalogue", soon: "M5" },
  { to: "/promotions", label: "Promotions", perm: "catalogue", soon: "M5" },
  { to: "/reviews", label: "Reviews", perm: "products", soon: "M6" },
  { to: "/delivery", label: "Delivery", perm: "catalogue" },
  { to: "/staff", label: "Staff", perm: "staff" },
];

// Espresso sidebar + content area (design: Admin CMS screens).
export function Layout() {
  const { me, can, signOut } = useAuth();
  return (
    <div className="layout">
      <aside className="sidebar">
        <span className="wordmark">Yetibyrash</span>
        <nav aria-label="Admin">
          {nav
            .filter((n) => !n.perm || can(n.perm))
            .map((n) => (
              <NavLink key={n.to} to={n.to} end={n.to === "/"}>
                {n.label}
                {n.soon && <span className="soon">{n.soon}</span>}
              </NavLink>
            ))}
        </nav>
        <div className="signed-in">
          <span>
            {me?.staff.name} · {me && roleLabels[me.staff.role]}
          </span>
          <button className="link-btn" onClick={signOut}>
            Sign out
          </button>
        </div>
      </aside>
      <main className="content">
        <Outlet />
      </main>
    </div>
  );
}
