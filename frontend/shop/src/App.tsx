import { BrowserRouter, Route, Routes } from "react-router-dom";
import { Layout } from "./components/Layout";
import { ComingSoon, NotFound } from "./pages/ComingSoon";
import { HomePage } from "./pages/HomePage";
import { ProductPage } from "./pages/ProductPage";
import { ShopPage } from "./pages/ShopPage";
import { TierProvider } from "./tier";

export function App() {
  return (
    <TierProvider>
      <BrowserRouter>
        <Routes>
          <Route element={<Layout />}>
            <Route index element={<HomePage />} />
            <Route path="shop" element={<ShopPage />} />
            <Route path="shop/:category" element={<ShopPage />} />
            <Route path="p/:slug" element={<ProductPage />} />
            <Route path="bag" element={<ComingSoon title="Your bag" milestone="M3" />} />
            <Route path="saved" element={<ComingSoon title="Saved items" milestone="M6" />} />
            <Route path="track" element={<ComingSoon title="Track order" milestone="M4" />} />
            <Route path="*" element={<NotFound />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </TierProvider>
  );
}
