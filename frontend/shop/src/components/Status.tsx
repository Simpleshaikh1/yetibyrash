import type { ApiError } from "../api";

export function Loading({ label = "Loading…" }: { label?: string }) {
  return (
    <p className="status-msg" aria-live="polite">
      {label}
    </p>
  );
}

export function ErrorMessage({ error, onRetry }: { error: ApiError; onRetry?: () => void }) {
  return (
    <div className="status-msg error" role="alert">
      <p>{error.message}</p>
      {onRetry && (
        <button className="btn outline" onClick={onRetry}>
          Try again
        </button>
      )}
    </div>
  );
}
