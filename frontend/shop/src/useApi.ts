import { useEffect, useState } from "react";
import { ApiError } from "./api";

type State<T> = { data: T | null; error: ApiError | null; loading: boolean };

/**
 * Runs an API call whenever `key` changes and returns its result.
 * Earlier calls are cancelled, so a fast tap on another filter never shows stale results.
 */
export function useApi<T>(key: string, load: (signal: AbortSignal) => Promise<T>): State<T> {
  const [state, setState] = useState<State<T>>({ data: null, error: null, loading: true });

  useEffect(() => {
    const ctrl = new AbortController();
    setState((s) => ({ data: s.data, error: null, loading: true }));
    load(ctrl.signal)
      .then((data) => setState({ data, error: null, loading: false }))
      .catch((e: unknown) => {
        if (ctrl.signal.aborted) return;
        const err = e instanceof ApiError ? e : new ApiError(0, "unknown", "Something went wrong. Please try again.");
        setState({ data: null, error: err, loading: false });
      });
    return () => ctrl.abort();
    // `load` is recreated every render; `key` says when the request really changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  return state;
}
