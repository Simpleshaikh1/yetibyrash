// Shared code for the shop and admin apps.

/** Formats an amount in kobo (how the API sends money) as naira, e.g. 1450000 → "₦14,500". */
export function formatNaira(kobo: number): string {
  const naira = kobo / 100;
  return "₦" + naira.toLocaleString("en-NG", { maximumFractionDigits: naira % 1 === 0 ? 0 : 2 });
}

export type Health = {
  status: "ok" | "degraded";
  database: "ok" | "unreachable";
  version: string;
};

/** Calls GET /api/health. Used by both apps' start screens to confirm the API is reachable. */
export async function fetchHealth(signal?: AbortSignal): Promise<Health> {
  const res = await fetch("/api/health", { signal });
  if (!res.ok && res.status !== 503) {
    throw new Error(`API returned ${res.status}`);
  }
  return (await res.json()) as Health;
}
