import { useEffect, useState } from "react";
import { fetchHealth, formatNaira, type Health } from "@yetibyrash/ui";

// M0 start screen: proves the shop loads the brand fonts and tokens and can reach the Go API.
// Replaced by the real Home screen in M1.
export function App() {
  const [health, setHealth] = useState<Health | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const ctrl = new AbortController();
    fetchHealth(ctrl.signal)
      .then(setHealth)
      .catch((e: unknown) => {
        if (!ctrl.signal.aborted) setError(e instanceof Error ? e.message : String(e));
      });
    return () => ctrl.abort();
  }, []);

  return (
    <main className="start">
      <span className="wordmark">Yetibyrash</span>
      <section className="hero">
        <span className="eyebrow">Shop · coming soon</span>
        <h1>Elegance meets affordability</h1>
      </section>
      <p className="status">
        {error && <>API not reachable: {error}. Is the <b>api</b> run configuration running?</>}
        {!error && !health && "Checking the API…"}
        {health && (
          <>
            API <b>{health.status}</b> · database <b>{health.database}</b> · version {health.version}
            <br />
            Money check: 1450000 kobo shows as <b>{formatNaira(1450000)}</b>
          </>
        )}
      </p>
    </main>
  );
}
