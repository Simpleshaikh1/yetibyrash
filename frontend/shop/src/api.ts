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
};

export type HomeSection = { key: "hero" | "categories" | "new_in" | "best_sellers" | "gift_budget" | "trust"; title: string; lines: string[] };
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
