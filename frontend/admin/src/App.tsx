import { useEffect, useState } from "react";
import { fetchHealth, type Health } from "@yetibyrash/ui";

// M0 start screen for the admin: shows whether the API and database are up.
// Replaced by sign-in and the Overview screen in M2.
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

  const apiOk = health?.status === "ok";

  return (
    <div className="layout">
      <aside className="sidebar">
        <span className="wordmark">Yetibyrash</span>
      </aside>
      <main className="content">
        <h1>System status</h1>
        <div className="panel">
          <span>API</span>
          {error && <span className="pill bad">Not reachable: {error}</span>}
          {!error && !health && <span>Checking…</span>}
          {health && <span className={apiOk ? "pill ok" : "pill bad"}>{health.status}</span>}
        </div>
        <div className="panel">
          <span>Database</span>
          {health && <span className={health.database === "ok" ? "pill ok" : "pill bad"}>{health.database}</span>}
        </div>
      </main>
    </div>
  );
}
