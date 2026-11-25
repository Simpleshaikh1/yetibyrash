import type { ReactNode } from "react";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import type { Permission } from "./api";
import { AuthProvider, useAuth } from "./auth";
import { Layout } from "./components/Layout";
import { PageHeader, Panel } from "./components/ui";
import { CataloguePage } from "./pages/CataloguePage";
import { DeliveryPage } from "./pages/DeliveryPage";
import { LoginPage } from "./pages/LoginPage";
import { ComingSoonPage, OverviewPage } from "./pages/OverviewPage";
import { ProductEditorPage } from "./pages/ProductEditorPage";
import { ProductsPage } from "./pages/ProductsPage";
import { StaffPage } from "./pages/StaffPage";

export function App() {
  return (
    <AuthProvider>
      <BrowserRouter basename="/admin">
        <Gate />
      </BrowserRouter>
    </AuthProvider>
  );
}

// Shows the sign-in page until a staff member is signed in.
function Gate() {
  const { me, checking } = useAuth();
  if (checking) return null;
  if (!me) return <LoginPage />;
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<OverviewPage />} />
        <Route path="products" element={<Allow perm="products"><ProductsPage /></Allow>} />
        <Route path="products/new" element={<Allow perm="products"><ProductEditorPage /></Allow>} />
        <Route path="products/:id" element={<Allow perm="products"><ProductEditorPage /></Allow>} />
        <Route path="catalogue" element={<Allow perm="catalogue"><CataloguePage /></Allow>} />
        <Route path="staff" element={<Allow perm="staff"><StaffPage /></Allow>} />
        <Route path="orders" element={<ComingSoonPage title="Orders" milestone="M4" />} />
        <Route path="content" element={<ComingSoonPage title="Homepage and content" milestone="M5" />} />
        <Route path="promotions" element={<ComingSoonPage title="Promotions" milestone="M5" />} />
        <Route path="reviews" element={<ComingSoonPage title="Reviews" milestone="M6" />} />
        <Route path="delivery" element={<Allow perm="catalogue"><DeliveryPage /></Allow>} />
        <Route path="*" element={<ComingSoonPage title="Page not found" milestone="—" />} />
      </Route>
    </Routes>
  );
}

// Hides a screen from roles that can't use it (the API refuses them too).
function Allow({ perm, children }: { perm: Permission; children: ReactNode }) {
  const { can } = useAuth();
  if (can(perm)) return <>{children}</>;
  return (
    <>
      <PageHeader title="Not available" />
      <Panel note="Your role can't open this screen. Ask the shop owner if you need access.">
        <span />
      </Panel>
    </>
  );
}
