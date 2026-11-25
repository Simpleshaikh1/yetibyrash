import { BrowserRouter, Route, Routes } from "react-router-dom";
import { BagProvider } from "./bag";
import { Layout } from "./components/Layout";
import { BagPage } from "./pages/BagPage";
import { CheckoutPage } from "./pages/CheckoutPage";
import { ComingSoon, NotFound } from "./pages/ComingSoon";
import { HomePage } from "./pages/HomePage";
import { OrderPage } from "./pages/OrderPage";
import { ProductPage } from "./pages/ProductPage";
import { ShopPage } from "./pages/ShopPage";
import { TrackPage } from "./pages/TrackPage";
import { TierProvider } from "./tier";

export function App() {
  return (
    <TierProvider>
      <BagProvider>
        <BrowserRouter>
          <Routes>
            <Route element={<Layout />}>
              <Route index element={<HomePage />} />
              <Route path="shop" element={<ShopPage />} />
              <Route path="shop/:category" element={<ShopPage />} />
              <Route path="p/:slug" element={<ProductPage />} />
              <Route path="bag" element={<BagPage />} />
              <Route path="checkout" element={<CheckoutPage />} />
              <Route path="order/:number" element={<OrderPage />} />
              <Route path="saved" element={<ComingSoon title="Saved items" milestone="M6" />} />
              <Route path="track" element={<TrackPage />} />
              <Route path="*" element={<NotFound />} />
            </Route>
          </Routes>
        </BrowserRouter>
      </BagProvider>
    </TierProvider>
  );
}
