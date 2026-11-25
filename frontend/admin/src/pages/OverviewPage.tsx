import { Link } from "react-router-dom";
import { useAuth } from "../auth";
import { PageHeader, Panel } from "../components/ui";

// The full Overview (sales, orders waiting, low stock) arrives with orders in M4.
export function OverviewPage() {
  const { me, can } = useAuth();
  return (
    <>
      <PageHeader title={`Welcome, ${me?.staff.name.split(" ")[0] ?? ""}`} />
      <Panel title="What you can do now">
        <ul className="link-list">
          {can("products") && (
            <>
              <li>
                <Link to="/products">Products</Link>: edit names, prices, options, stock and photos.
              </li>
              <li>
                <Link to="/products/new">Add a product</Link>
              </li>
            </>
          )}
          {can("catalogue") && (
            <li>
              <Link to="/catalogue">Catalogue setup</Link>: categories, size sets and colours.
            </li>
          )}
          {can("staff") && (
            <li>
              <Link to="/staff">Staff</Link>: add staff and choose what they can do.
            </li>
          )}
        </ul>
      </Panel>
      <Panel title="Coming next" note="Sales, orders waiting to pack and low-stock alerts appear here once checkout and orders are built (M3 and M4)." >
        <span />
      </Panel>
    </>
  );
}

export function ComingSoonPage({ title, milestone }: { title: string; milestone: string }) {
  return (
    <>
      <PageHeader title={title} />
      <Panel note={`This screen arrives in milestone ${milestone}.`}>
        <span />
      </Panel>
    </>
  );
}
