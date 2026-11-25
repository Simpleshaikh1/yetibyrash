// Shared code for the shop and admin apps.

/** Formats an amount in kobo (how the API sends money) as naira, e.g. 1450000 → "₦14,500". */
export function formatNaira(kobo: number): string {
  const naira = kobo / 100;
  return "₦" + naira.toLocaleString("en-NG", { maximumFractionDigits: naira % 1 === 0 ? 0 : 2 });
}

/** Turns what someone typed as naira ("14,500", "₦14500.50") into kobo. Returns null if it isn't a number. */
export function parseNaira(input: string): number | null {
  const cleaned = input.replace(/[₦,\s]/g, "");
  if (cleaned === "" || !/^\d+(\.\d{1,2})?$/.test(cleaned)) return null;
  return Math.round(Number(cleaned) * 100);
}

/** Kobo as a plain naira number for an input box: 1450000 → "14500". */
export function koboToInput(kobo: number | null | undefined): string {
  if (kobo == null) return "";
  return kobo % 100 === 0 ? String(kobo / 100) : (kobo / 100).toFixed(2);
}

/** URL of a stored product photo at one of the saved widths. */
export function photoUrl(key: string, width: 400 | 800 | 1600): string {
  return `/media/${key}-${width}.jpg`;
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
