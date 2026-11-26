// Types and calls for the Go API's /api/v1 catalogue endpoints.
// Money is always in kobo; format it with formatNaira from @yetibyrash/ui.

export type Tier = "everyday" | "premium";

export type Category = {
  id: number;
  name: string;
  slug: string;
  showOnHome: boolean;
  children: Category[];
};

export type ProductCard = {
  id: number;
  slug: string;
  name: string;
  tier: Tier;
  materialLine: string;
  categoryName: string;
  badges: string[];
  priceKobo: number;
  maxPriceKobo: number;
  compareAtKobo: number | null;
  saleBadge: string; // a running sale's badge, "" for none
  rating: Rating;
  colourCount: number;
  inStock: boolean;
  preorder: boolean;
  image: string; // main photo key, "" if none
};

export type Colour = { name: string; hex: string };
export type PriceBand = { id: number; label: string; minKobo: number; maxKobo: number | null };

export type ProductList = {
  items: ProductCard[];
  total: number;
  page: number;
  pageSize: number;
  filters: { colours: Colour[]; sizes: string[]; priceBands: PriceBand[] };
};

export type Variant = {
  id: number;
  sku: string;
  colour: string;
  size: string;
  priceKobo: number;
  compareAtKobo: number | null;
  stock: number;
};

export type ProductDetail = {
  id: number;
  slug: string;
  name: string;
  description: string;
  tier: Tier;
  materialLine: string;
  badges: string[];
  saleBadge: string;
  giftBoxAllowed: boolean;
  preorderAllowed: boolean;
  category: { name: string; slug: string };
  parent: { name: string; slug: string } | null;
  returnRule: string;
  colours: Colour[];
  sizeSet: { name: string; measureLabel: string; sizes: { label: string; measurement: string }[] } | null;
  variants: Variant[];
  details: { label: string; value: string }[];
  images: { id: number; key: string; kind: string }[];
  pairings: ProductCard[];
  lowStockThreshold: number;
  rating: Rating;
  reviews: Review[];
};

/** Published reviews in short; average is 0 when there are none. */
export type Rating = { average: number; count: number };
export type Review = {
  id: number;
  rating: number;
  text: string;
  photos: string[];
  reply: string;
  author: string;
  area: string;
  option: string;
  publishedAt: string;
};
export type HomeReview = { id: number; text: string; author: string; area: string; product: string; productSlug: string };

// ---------- Accounts (M6) ----------

export type Customer = { id: number; name: string; phone: string; email: string; marketingOptIn: boolean };
export type Me = { customer: Customer | null; saved: number[] };
export type OrderSummary = { number: string; status: OrderView["status"]; totalKobo: number; items: number; image: string; createdAt: string };
export type Address = {
  id: number;
  stateId: number;
  stateName: string;
  lgaId: number;
  lgaText: string;
  lgaName: string;
  street: string;
  landmark: string;
};

// ---------- Reviews (M6) ----------

export type ReviewItem = {
  orderItemId: number;
  name: string;
  slug: string;
  option: string;
  image: string;
  review: { id: number; rating: number; status: string; photos: string[] } | null;
};
export type ReviewRequest = { orderNumber: string; firstName: string; authorName: string; items: ReviewItem[] };

export type HomeSection = { key: "hero" | "categories" | "new_in" | "best_sellers" | "gift_budget" | "trust" | "reviews"; title: string; lines: string[] };
export type HeroBanner = { eyebrow: string; headline: string; buttonText: string; buttonLink: string; image: string };
/** The homepage: `sections` lists the visible blocks in the admin's order. */
export type Home = {
  tagline: string;
  sections: HomeSection[];
  hero: HeroBanner | null;
  categories: Category[];
  newIn: ProductCard[];
  bestSellers: ProductCard[];
  budgets: PriceBand[];
  reviews: HomeReview[];
};

/** Announcement bar and footer links, shown on every page. */
export type Site = { announcements: { text: string; link: string }[]; pages: { slug: string; title: string }[] };
export type SitePage = { slug: string; title: string; body: string; updatedAt: string };

export type BagLine = {
  variantId: number;
  productId: number;
  slug: string;
  name: string;
  option: string;
  sku: string;
  image: string;
  unitKobo: number;
  compareAtKobo: number | null;
  qty: number;
  stock: number;
  lineKobo: number;
  saleBadge: string;
  problem: string; // "" when fine
};
/** The discount code on the bag. `problem` says why it gives nothing yet. */
export type BagCode = { code: string; discountKobo: number; freeDelivery: boolean; summary: string; problem: string };
/** The free-gift offer; remainingKobo 0 means the gift is in. */
export type BagGift = { slug: string; name: string; option: string; image: string; thresholdKobo: number; remainingKobo: number };
export type Bag = {
  lines: BagLine[];
  count: number;
  subtotalKobo: number;
  canCheckout: boolean;
  code: BagCode | null;
  discountKobo: number;
  gift: BagGift | null;
};

export type Lga = { id: number; name: string; express: boolean };
export type State = { id: number; name: string; zoneName: string; lgas: Lga[] };

export type DeliveryOption = { speed: "standard" | "express"; feeKobo: number; free: boolean; from: string; to: string; label: string };
export type Quote = {
  subtotalKobo: number;
  options: DeliveryOption[];
  zoneName: string;
  code: string;
  discountKobo: number; // off the items
  freeDelivery: boolean; // the code makes delivery free
  codeProblem: string;
  gift: string; // free gift going in, "" for none
};

export type DeliverySummary = {
  cutoffAt: string | null;
  lagos: DeliveryOption | null;
  zones: { name: string; feeKobo: number; days: string; freeOverKobo: number | null; states: string[] }[];
};

export type CheckoutInput = {
  name: string;
  phone: string;
  email: string;
  stateId: number;
  lgaId: number;
  lgaText: string;
  street: string;
  landmark: string;
  speed: "standard" | "express";
  /** Signed-in shoppers: keep this address on the account. */
  saveAddress?: boolean;
};

export type OrderView = {
  number: string;
  status: "pending_payment" | "paid" | "packed" | "shipped" | "out_for_delivery" | "delivered" | "abandoned" | "cancelled" | "refunded";
  name: string;
  phone: string;
  email: string;
  address: string;
  speed: string;
  subtotalKobo: number;
  deliveryKobo: number;
  discountKobo: number;
  discountCode: string;
  totalKobo: number;
  expectedFrom: string;
  expectedTo: string;
  paidAt: string | null;
  items: { name: string; slug: string; option: string; image: string; unitKobo: number; qty: number; gift: boolean }[];
  events: { status: string; note: string; at: string }[];
};

export type Tracking = {
  number: string;
  status: OrderView["status"];
  headline: string;
  area: string;
  items: number;
  speed: string;
  expectedFrom: string;
  expectedTo: string;
  courier: string;
  trackingNumber: string;
  trackingLink: string;
  steps: { status: string; label: string; at: string | null; done: boolean; current: boolean }[];
  notes: { text: string; at: string }[];
};

/** Error with the API's message, ready to show to the shopper. */
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public field?: string,
  ) {
    super(message);
  }
}

async function send<T>(method: string, path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      signal,
      credentials: "same-origin",
      headers: body === undefined ? undefined : { "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch (e) {
    if (signal?.aborted) throw e;
    throw new ApiError(0, "offline", "We couldn't reach the shop. Check your connection and try again.");
  }
  if (!res.ok) {
    const data = await res.json().catch(() => null);
    throw new ApiError(res.status, data?.error?.code ?? "unknown", data?.error?.message ?? `Request failed (${res.status}).`, data?.error?.field);
  }
  if (res.status === 204) return undefined as T; // no content
  return (await res.json()) as T;
}

const getJSON = <T,>(path: string, signal?: AbortSignal) => send<T>("GET", path, undefined, signal);

export const api = {
  home: (tier: Tier, signal?: AbortSignal) => getJSON<Home>(`/api/v1/home?tier=${tier}`, signal),
  categories: (signal?: AbortSignal) =>
    getJSON<{ categories: Category[] }>("/api/v1/categories", signal).then((r) => r.categories),
  products: (params: URLSearchParams, signal?: AbortSignal) =>
    getJSON<ProductList>(`/api/v1/products?${params.toString()}`, signal),
  product: (slug: string, signal?: AbortSignal) =>
    getJSON<ProductDetail>(`/api/v1/products/${encodeURIComponent(slug)}`, signal),

  bag: (signal?: AbortSignal) => getJSON<Bag>("/api/v1/cart", signal),
  addToBag: (variantId: number, qty = 1) => send<Bag>("POST", "/api/v1/cart/items", { variantId, qty }),
  setQty: (variantId: number, qty: number) => send<Bag>("PUT", `/api/v1/cart/items/${variantId}`, { qty }),
  applyCode: (code: string) => send<Bag>("POST", "/api/v1/cart/code", { code }),
  removeCode: () => send<Bag>("DELETE", "/api/v1/cart/code"),

  // Accounts
  me: (signal?: AbortSignal) => getJSON<Me>("/api/v1/me", signal),
  sendCode: (target: string) => send<{ value: string; isPhone: boolean }>("POST", "/api/v1/auth/code", { target }),
  verifyCode: (target: string, code: string) => send<{ customer: Customer }>("POST", "/api/v1/auth/verify", { target, code }),
  signOut: () => send<void>("POST", "/api/v1/auth/logout"),
  updateMe: (name: string, marketingOptIn: boolean) => send<void>("PUT", "/api/v1/me", { name, marketingOptIn }),
  deleteMe: () => send<void>("DELETE", "/api/v1/me"),
  myOrders: (signal?: AbortSignal) => getJSON<{ orders: OrderSummary[] }>("/api/v1/me/orders", signal).then((r) => r.orders),
  reviewLink: (number: string) => send<{ token: string }>("POST", `/api/v1/me/orders/${encodeURIComponent(number)}/review`),
  addresses: (signal?: AbortSignal) => getJSON<{ addresses: Address[] }>("/api/v1/me/addresses", signal).then((r) => r.addresses),
  deleteAddress: (id: number) => send<void>("DELETE", `/api/v1/me/addresses/${id}`),
  saved: (signal?: AbortSignal) => getJSON<{ items: ProductCard[] }>("/api/v1/me/saved", signal).then((r) => r.items),
  save: (productId: number) => send<void>("PUT", `/api/v1/me/saved/${productId}`),
  unsave: (productId: number) => send<void>("DELETE", `/api/v1/me/saved/${productId}`),

  // Reviews
  reviewRequest: (token: string, signal?: AbortSignal) => getJSON<ReviewRequest>(`/api/v1/reviews/${encodeURIComponent(token)}`, signal),
  submitReview: (token: string, body: { orderItemId: number; rating: number; text: string; name: string }) =>
    send<{ id: number }>("POST", `/api/v1/reviews/${encodeURIComponent(token)}`, body),
  reviewPhoto: async (token: string, reviewId: number, file: File) => {
    const form = new FormData();
    form.append("photo", file);
    const res = await fetch(`/api/v1/reviews/${encodeURIComponent(token)}/${reviewId}/photos`, { method: "POST", body: form, credentials: "same-origin" });
    if (!res.ok) {
      const data = await res.json().catch(() => null);
      throw new ApiError(res.status, data?.error?.code ?? "unknown", data?.error?.message ?? "Couldn't add the photo.");
    }
    return (await res.json()) as { photo: string };
  },

  site: (signal?: AbortSignal) => getJSON<Site>("/api/v1/site", signal),
  page: (slug: string, signal?: AbortSignal) => getJSON<SitePage>(`/api/v1/pages/${encodeURIComponent(slug)}`, signal),

  locations: (signal?: AbortSignal) => getJSON<{ states: State[] }>("/api/v1/locations", signal).then((r) => r.states),
  delivery: (signal?: AbortSignal) => getJSON<DeliverySummary>("/api/v1/delivery", signal),
  quote: (stateId: number, lgaId: number, lgaText: string) => send<Quote>("POST", "/api/v1/checkout/quote", { stateId, lgaId, lgaText }),
  checkout: (input: CheckoutInput) => send<{ orderNumber: string; paymentUrl: string }>("POST", "/api/v1/checkout", input),
  order: (number: string, reference: string, signal?: AbortSignal) =>
    getJSON<OrderView>(`/api/v1/orders/${encodeURIComponent(number)}?reference=${encodeURIComponent(reference)}`, signal),
  track: (number: string, phone: string) => send<Tracking>("POST", "/api/v1/track", { number, phone }),
};

/** "2026-11-26" → "Thu 26 Nov" */
export function shortDate(iso: string): string {
  const [y, m, d] = iso.split("-").map(Number);
  return new Date(y, m - 1, d).toLocaleDateString("en-GB", { weekday: "short", day: "numeric", month: "short" }).replace(",", "");
}
