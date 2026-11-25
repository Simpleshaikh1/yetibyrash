import { useCallback, useEffect, useState } from "react";
import { ApiError } from "./api";

type State<T> = { data: T | null; error: ApiError | null; loading: boolean; reload: () => void };

/** Loads data whenever `key` changes; `reload()` fetches it again (e.g. after saving). */
export function useApi<T>(key: string, load: (signal: AbortSignal) => Promise<T>): State<T> {
  const [nonce, setNonce] = useState(0);
  const [state, setState] = useState<Omit<State<T>, "reload">>({ data: null, error: null, loading: true });

  useEffect(() => {
    const ctrl = new AbortController();
    setState((s) => ({ ...s, error: null, loading: true }));
    load(ctrl.signal)
      .then((data) => setState({ data, error: null, loading: false }))
      .catch((e: unknown) => {
        if (ctrl.signal.aborted) return;
        setState({ data: null, error: e instanceof ApiError ? e : new ApiError(0, "unknown", "Something went wrong."), loading: false });
      });
    return () => ctrl.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, nonce]);

  const reload = useCallback(() => setNonce((n) => n + 1), []);
  return { ...state, reload };
}

/** Wraps a save action: tracks busy state and the error to show. */
export function useAction() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const run = async <T,>(fn: () => Promise<T>): Promise<T | undefined> => {
    setBusy(true);
    setError(null);
    try {
      return await fn();
    } catch (e) {
      setError(e instanceof ApiError ? e : new ApiError(0, "unknown", "Something went wrong."));
      return undefined;
    } finally {
      setBusy(false);
    }
  };
  return { busy, error, setError, run };
}
