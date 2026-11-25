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
  colourCount: number;
  inStock: boolean;
  preorder: boolean;
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

export type Home = { tagline: string; categories: Category[]; newIn: ProductCard[] };

/** Error with the API's message, ready to show to the shopper. */
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
  }
}

async function getJSON<T>(path: string, signal?: AbortSignal): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, { signal });
  } catch (e) {
    if (signal?.aborted) throw e;
    throw new ApiError(0, "offline", "We couldn't reach the shop. Check your connection and try again.");
  }
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    throw new ApiError(res.status, body?.error?.code ?? "unknown", body?.error?.message ?? `Request failed (${res.status}).`);
  }
  return (await res.json()) as T;
}

export const api = {
  home: (tier: Tier, signal?: AbortSignal) => getJSON<Home>(`/api/v1/home?tier=${tier}`, signal),
  categories: (signal?: AbortSignal) =>
    getJSON<{ categories: Category[] }>("/api/v1/categories", signal).then((r) => r.categories),
  products: (params: URLSearchParams, signal?: AbortSignal) =>
    getJSON<ProductList>(`/api/v1/products?${params.toString()}`, signal),
  product: (slug: string, signal?: AbortSignal) =>
    getJSON<ProductDetail>(`/api/v1/products/${encodeURIComponent(slug)}`, signal),
};
