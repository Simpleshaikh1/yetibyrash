// Types and calls for the Go API's /api/admin/v1 endpoints.
// Money is always in kobo. The session lives in an HttpOnly cookie the browser sends automatically.

export type Role = "owner" | "manager" | "packer" | "content_editor";
export type Permission = "products" | "catalogue" | "orders" | "staff";
export type Member = { id: number; email: string; name: string; role: Role };
export type Me = { staff: Member; permissions: Permission[] };

export type Tier = "everyday" | "premium";
export type Status = "draft" | "visible" | "hidden";

export type ProductRow = {
  id: number;
  slug: string;
  name: string;
  tier: Tier;
  status: Status;
  category: string;
  minPriceKobo: number;
  maxPriceKobo: number;
  totalStock: number;
  optionCount: number;
  soldOutCount: number;
  image: string;
  updatedAt: string;
};

export type Field = { id: number; label: string };
export type DetailInput = { fieldId: number; value: string };
export type VariantInput = {
  colourId: number | null;
  sizeId: number | null;
  sku: string;
  priceKobo: number;
  compareAtKobo: number | null;
  stock: number;
  active: boolean;
};
export type AdminImage = { id: number; colourId: number | null; key: string; kind: string; width: number; warnings: string[] };

export type ProductInput = {
  name: string;
  description: string;
  categoryId: number;
  tier: Tier;
  status: Status;
  materialLine: string;
  badges: string[];
  giftBoxAllowed: boolean;
  preorderAllowed: boolean;
  details: DetailInput[];
  variants: VariantInput[];
};

export type AdminProduct = ProductInput & {
  id: number;
  slug: string;
  updatedAt: string;
  fields: Field[];
  variants: (VariantInput & { id: number })[];
  images: AdminImage[];
};

export type SetupSize = { id: number; label: string; measurement: string; usedBy: number };
export type SizeSet = { id: number; name: string; measureLabel: string; sizes: SetupSize[] };
export type Colour = { id: number; name: string; hex: string; usedBy: number };
export type SetupCategory = {
  id: number;
  parentId: number | null;
  name: string;
  slug: string;
  defaultSizeSetId: number | null;
  returnRule: string;
  showOnHome: boolean;
  position: number;
  fields: Field[];
  children: SetupCategory[];
};
export type Setup = { categories: SetupCategory[]; sizeSets: SizeSet[]; colours: Colour[] };

export type StaffAccount = Member & { active: boolean; lastLoginAt: string | null; createdAt: string };

export type Zone = {
  id: number;
  name: string;
  standardFeeKobo: number;
  standardDaysMin: number;
  standardDaysMax: number;
  expressFeeKobo: number | null;
  expressDays: number | null;
  freeOverKobo: number | null;
};
export type DeliveryLga = { id: number; name: string; express: boolean; delivered: boolean };
export type DeliveryState = { id: number; name: string; zoneId: number; zoneName: string; lgas: DeliveryLga[] };
export type OrderStatus = "pending_payment" | "paid" | "packed" | "shipped" | "out_for_delivery" | "delivered" | "abandoned" | "cancelled" | "refunded";

export const statusLabels: Record<OrderStatus, string> = {
  pending_payment: "Waiting for payment",
  paid: "Paid, to pack",
  packed: "Packed",
  shipped: "Shipped",
  out_for_delivery: "Out for delivery",
  delivered: "Delivered",
  abandoned: "Not paid",
  cancelled: "Cancelled",
  refunded: "Refunded",
};

export type BoardOrder = {
  id: number;
  number: string;
  status: OrderStatus;
  statusAt: string;
  paidAt: string;
  name: string;
  phone: string;
  area: string;
  speed: string;
  totalKobo: number;
  items: number;
  late: boolean;
};

export type AdminOrder = {
  id: number;
  number: string;
  status: OrderStatus;
  reference: string;
  name: string;
  phone: string;
  email: string;
  address: string;
  zoneName: string;
  speed: string;
  subtotalKobo: number;
  deliveryKobo: number;
  totalKobo: number;
  expectedFrom: string;
  expectedTo: string;
  paidAt: string | null;
  statusAt: string;
  createdAt: string;
  courierId: number | null;
  courierName: string;
  trackingNumber: string;
  next: OrderStatus | "";
  canCancel: boolean;
  items: { name: string; slug: string; option: string; image: string; unitKobo: number; qty: number }[];
  adminEvents: { status: OrderStatus; note: string; public: boolean; by: string; at: string }[];
  payments: { reference: string; amountKobo: number; channel: string; outcome: string; at: string }[];
};

export type Courier = { id: number; name: string; trackingUrl: string; active: boolean };

export type Overview = {
  salesTodayKobo: number;
  ordersToday: number;
  toPack: number;
  packed: number;
  onTheWay: number;
  outForDelivery: number;
  lateToPack: { id: number; number: string; name: string; paidAt: string }[];
  lowStock: { productId: number; name: string; option: string; stock: number }[];
  soldOut: number;
  problemPayments: { reference: string; orderId: number | null; amountKobo: number; outcome: string; at: string }[];
  failedEmails: number;
  last7Days: { day: string; totalKobo: number }[];
};

export type MessageTemplate = { event: OrderStatus; subject: string; body: string; active: boolean };

export type DeliveryAdmin = {
  zones: Zone[];
  states: DeliveryState[];
  settings: { expressCutoff: string; dispatchDays: number[] };
  holidays: { day: string; name: string }[];
};

/** An error from the API, with the message to show and (for form errors) the field it's about. */
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

/** Called when the session has ended, so the app can show the sign-in page. */
let onSignedOut: () => void = () => {};
export function setSignedOutHandler(fn: () => void) {
  onSignedOut = fn;
}

async function request<T>(method: string, path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  let res: Response;
  const isForm = body instanceof FormData;
  try {
    res = await fetch(`/api/admin/v1${path}`, {
      method,
      signal,
      credentials: "same-origin",
      headers: body && !isForm ? { "Content-Type": "application/json" } : undefined,
      body: body === undefined ? undefined : isForm ? body : JSON.stringify(body),
    });
  } catch (e) {
    if (signal?.aborted) throw e;
    throw new ApiError(0, "offline", "Can't reach the server. Check your connection and that the API is running.");
  }
  if (res.status === 204) return undefined as T;
  const data = await res.json().catch(() => null);
  if (!res.ok) {
    if (res.status === 401 && path !== "/auth/login") onSignedOut();
    throw new ApiError(res.status, data?.error?.code ?? "unknown", data?.error?.message ?? `Request failed (${res.status}).`, data?.error?.field);
  }
  return data as T;
}

export const api = {
  login: (email: string, password: string) => request<Me>("POST", "/auth/login", { email, password }),
  logout: () => request<void>("POST", "/auth/logout"),
  me: (signal?: AbortSignal) => request<Me>("GET", "/auth/me", undefined, signal),

  setup: (signal?: AbortSignal) => request<Setup>("GET", "/setup", undefined, signal),

  products: (params: URLSearchParams, signal?: AbortSignal) =>
    request<{ products: ProductRow[] }>("GET", `/products?${params}`, undefined, signal).then((r) => r.products),
  product: (id: number, signal?: AbortSignal) => request<AdminProduct>("GET", `/products/${id}`, undefined, signal),
  createProduct: (p: ProductInput) => request<{ id: number }>("POST", "/products", p),
  updateProduct: (id: number, p: ProductInput) => request<void>("PUT", `/products/${id}`, p),
  uploadImage: (id: number, file: File, kind: string) => {
    const form = new FormData();
    form.append("photo", file);
    form.append("kind", kind);
    return request<AdminImage>("POST", `/products/${id}/images`, form);
  },
  deleteImage: (id: number, imageId: number) => request<void>("DELETE", `/products/${id}/images/${imageId}`),
  makeImageMain: (id: number, imageId: number) => request<void>("POST", `/products/${id}/images/${imageId}/main`),

  saveSizeSet: (id: number, body: { name: string; measureLabel: string; sizes: { id: number; label: string; measurement: string }[] }) =>
    request<{ id: number }>(id ? "PUT" : "POST", id ? `/size-sets/${id}` : "/size-sets", body),
  saveColour: (id: number, body: { name: string; hex: string }) =>
    request<{ id: number }>(id ? "PUT" : "POST", id ? `/colours/${id}` : "/colours", body),
  deleteColour: (id: number) => request<void>("DELETE", `/colours/${id}`),
  saveCategory: (
    id: number,
    body: { parentId?: number | null; name: string; defaultSizeSetId: number | null; returnRule: string; showOnHome: boolean; position: number },
  ) => request<{ id: number }>(id ? "PUT" : "POST", id ? `/categories/${id}` : "/categories", body),
  saveCategoryFields: (id: number, fields: { id: number; label: string }[]) => request<void>("PUT", `/categories/${id}/fields`, { fields }),

  delivery: (signal?: AbortSignal) => request<DeliveryAdmin>("GET", "/delivery", undefined, signal),
  updateZone: (z: Zone) => request<void>("PUT", `/delivery/zones/${z.id}`, z),
  setStateZone: (stateId: number, zoneId: number) => request<void>("PUT", `/delivery/states/${stateId}`, { zoneId }),
  addLga: (stateId: number, l: Omit<DeliveryLga, "id">) => request<{ id: number }>("POST", `/delivery/states/${stateId}/lgas`, l),
  updateLga: (l: DeliveryLga) => request<void>("PUT", `/delivery/lgas/${l.id}`, l),
  deleteLga: (id: number) => request<void>("DELETE", `/delivery/lgas/${id}`),
  updateDeliverySettings: (s: { expressCutoff: string; dispatchDays: number[] }) => request<void>("PUT", "/delivery/settings", s),
  addHoliday: (h: { day: string; name: string }) => request<void>("POST", "/delivery/holidays", h),
  deleteHoliday: (day: string) => request<void>("DELETE", `/delivery/holidays/${day}`),

  overview: (signal?: AbortSignal) => request<Overview>("GET", "/overview", undefined, signal),
  orders: (params: URLSearchParams, signal?: AbortSignal) =>
    request<{ orders: BoardOrder[] }>("GET", `/orders?${params}`, undefined, signal).then((r) => r.orders),
  order: (id: number, signal?: AbortSignal) => request<AdminOrder>("GET", `/orders/${id}`, undefined, signal),
  advanceOrder: (id: number, body: { to: string; courierId?: number; trackingNumber?: string; note?: string }) =>
    request<void>("POST", `/orders/${id}/advance`, body),
  cancelOrder: (id: number, reason: string) => request<void>("POST", `/orders/${id}/cancel`, { reason }),
  couriers: (signal?: AbortSignal) => request<{ couriers: Courier[] }>("GET", "/couriers", undefined, signal).then((r) => r.couriers),
  saveCourier: (c: Courier) => request<{ id: number }>(c.id ? "PUT" : "POST", c.id ? `/couriers/${c.id}` : "/couriers", c),
  messages: (signal?: AbortSignal) =>
    request<{ templates: MessageTemplate[]; placeholders: { key: string; example: string }[] }>("GET", "/messages", undefined, signal),
  saveMessage: (t: MessageTemplate) => request<void>("PUT", `/messages/${t.event}`, t),
  previewMessage: (subject: string, body: string) => request<{ subject: string; body: string }>("POST", "/messages/preview", { subject, body }),

  staff: (signal?: AbortSignal) => request<{ staff: StaffAccount[] }>("GET", "/staff", undefined, signal).then((r) => r.staff),
  createStaff: (body: { email: string; name: string; role: Role; password: string }) => request<{ id: number }>("POST", "/staff", body),
  updateStaff: (id: number, body: { name: string; role: Role; active: boolean }) => request<void>("PUT", `/staff/${id}`, body),
  setStaffPassword: (id: number, password: string) => request<void>("PUT", `/staff/${id}/password`, { password }),
};

export const roleLabels: Record<Role, string> = {
  owner: "Owner",
  manager: "Manager",
  packer: "Packer",
  content_editor: "Content editor",
};
