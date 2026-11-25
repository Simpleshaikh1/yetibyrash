import type { ReactNode } from "react";
import type { ApiError, Status } from "../api";

export function PageHeader({ title, back, actions }: { title: string; back?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="page-header">
      <div className="stack-xs">
        {back}
        <h1>{title}</h1>
      </div>
      {actions && <div className="row">{actions}</div>}
    </div>
  );
}

export function Panel({ title, note, children, actions }: { title?: string; note?: ReactNode; children: ReactNode; actions?: ReactNode }) {
  return (
    <section className="panel">
      {(title || actions) && (
        <div className="panel-head">
          {title && <h2>{title}</h2>}
          {actions}
        </div>
      )}
      {note && <p className="note">{note}</p>}
      {children}
    </section>
  );
}

/** A labelled form control. `error` shows under it in red. */
export function Field({ label, htmlFor, error, hint, children }: { label: string; htmlFor: string; error?: string; hint?: string; children: ReactNode }) {
  return (
    <div className="field">
      <label htmlFor={htmlFor}>{label}</label>
      {children}
      {hint && !error && <span className="hint">{hint}</span>}
      {error && (
        <span className="field-error" role="alert">
          {error}
        </span>
      )}
    </div>
  );
}

export function ErrorBanner({ error }: { error: ApiError | null }) {
  if (!error) return null;
  return (
    <div className="banner danger" role="alert">
      {error.message}
    </div>
  );
}

export function SavedBanner({ show, children = "Saved." }: { show: boolean; children?: ReactNode }) {
  if (!show) return null;
  return (
    <div className="banner success" role="status">
      {children}
    </div>
  );
}

const statusLabels: Record<Status, string> = { visible: "Visible", draft: "Draft", hidden: "Hidden" };

export function StatusPill({ status }: { status: Status }) {
  return <span className={`pill ${status === "visible" ? "ok" : "neutral"}`}>{statusLabels[status]}</span>;
}

export function Loading() {
  return <p className="muted">Loading…</p>;
}
