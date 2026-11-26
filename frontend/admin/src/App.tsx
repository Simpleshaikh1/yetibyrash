import type { ReactNode } from "react";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import type { Permission } from "./api";
import { AuthProvider, useAuth } from "./auth";
import { Layout } from "./components/Layout";
import { PageHeader, Panel } from "./components/ui";
import { CataloguePage } from "./pages/CataloguePage";
import { ContentPage } from "./pages/ContentPage";
import { DeliveryPage } from "./pages/DeliveryPage";
import { LoginPage } from "./pages/LoginPage";
import { MessagesPage } from "./pages/MessagesPage";
import { OrderDetailPage } from "./pages/OrderDetailPage";
import { OrdersPage } from "./pages/OrdersPage";
import { ComingSoonPage, OverviewPage } from "./pages/OverviewPage";
import { ProductEditorPage } from "./pages/ProductEditorPage";
import { ProductsPage } from "./pages/ProductsPage";
import { PromotionsPage } from "./pages/PromotionsPage";
import { ReviewsPage } from "./pages/ReviewsPage";
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
        <Route path="orders" element={<Allow perm="orders"><OrdersPage /></Allow>} />
        <Route path="orders/:id" element={<Allow perm="orders"><OrderDetailPage /></Allow>} />
        <Route path="messages" element={<Allow perm="catalogue"><MessagesPage /></Allow>} />
        <Route path="content" element={<Allow perm="content"><ContentPage /></Allow>} />
        <Route path="promotions" element={<Allow perm="promotions"><PromotionsPage /></Allow>} />
        <Route path="reviews" element={<Allow perm="products"><ReviewsPage /></Allow>} />
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
