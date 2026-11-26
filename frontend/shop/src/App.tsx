import { BrowserRouter, Route, Routes } from "react-router-dom";
import { AccountProvider } from "./account";
import { BagProvider } from "./bag";
import { Layout } from "./components/Layout";
import { AccountPage } from "./pages/AccountPage";
import { BagPage } from "./pages/BagPage";
import { CheckoutPage } from "./pages/CheckoutPage";
import { NotFound } from "./pages/ComingSoon";
import { HomePage } from "./pages/HomePage";
import { InfoPage } from "./pages/InfoPage";
import { OrderPage } from "./pages/OrderPage";
import { ProductPage } from "./pages/ProductPage";
import { ReviewPage } from "./pages/ReviewPage";
import { SavedPage } from "./pages/SavedPage";
import { ShopPage } from "./pages/ShopPage";
import { TrackPage } from "./pages/TrackPage";
import { TierProvider } from "./tier";

export function App() {
  return (
    <TierProvider>
      <BagProvider>
        <BrowserRouter>
          <AccountProvider>
            <Routes>
              <Route element={<Layout />}>
                <Route index element={<HomePage />} />
                <Route path="shop" element={<ShopPage />} />
                <Route path="shop/:category" element={<ShopPage />} />
                <Route path="p/:slug" element={<ProductPage />} />
                <Route path="bag" element={<BagPage />} />
                <Route path="checkout" element={<CheckoutPage />} />
                <Route path="order/:number" element={<OrderPage />} />
                <Route path="saved" element={<SavedPage />} />
                <Route path="account" element={<AccountPage />} />
                <Route path="review/:token" element={<ReviewPage />} />
                <Route path="track" element={<TrackPage />} />
                <Route path="page/:slug" element={<InfoPage />} />
                <Route path="*" element={<NotFound />} />
              </Route>
            </Routes>
          </AccountProvider>
        </BrowserRouter>
      </BagProvider>
    </TierProvider>
  );
}
