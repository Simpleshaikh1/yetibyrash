import { useEffect, useState } from "react";
import { api, statusLabels, type MessageTemplate } from "../api";
import { ErrorBanner, Field, Loading, PageHeader, Panel, SavedBanner } from "../components/ui";
import { useAction, useApi } from "../useApi";

const when: Record<string, string> = {
  paid: "Sent when Paystack confirms the payment",
  packed: "Sent when an order is moved to Packed",
  shipped: "Sent when an order is moved to Shipped",
  out_for_delivery: "Sent when an order is moved to Out for delivery",
  delivered: "Sent when an order is moved to Delivered",
  cancelled: "Sent when an order is cancelled",
};

export function MessagesPage() {
  const data = useApi("messages", (s) => api.messages(s));
  const [selected, setSelected] = useState("paid");
  const tpl = data.data?.templates.find((t) => t.event === selected);

  return (
    <>
      <PageHeader title="Customer messages" />
      <p className="muted">The emails customers get at each step. WhatsApp and SMS use the same wording once they're switched on.</p>
      <ErrorBanner error={data.error} />
      {!data.data && !data.error && <Loading />}
      {data.data && (
        <div className="split">
          <div className="list-panel">
            {data.data.templates.map((t) => (
              <button key={t.event} className={t.event === selected ? "list-item on" : "list-item"} onClick={() => setSelected(t.event)}>
                <b>{statusLabels[t.event]}</b>
                <span className="small muted">{t.active ? when[t.event] : "Switched off"}</span>
              </button>
            ))}
          </div>
          {tpl && <Editor key={tpl.event} tpl={tpl} placeholders={data.data.placeholders} reload={data.reload} />}
        </div>
      )}
    </>
  );
}

function Editor({ tpl, placeholders, reload }: { tpl: MessageTemplate; placeholders: { key: string; example: string }[]; reload: () => void }) {
  const [subject, setSubject] = useState(tpl.subject);
  const [body, setBody] = useState(tpl.body);
  const [active, setActive] = useState(tpl.active);
  const [saved, setSaved] = useState(false);
  const [preview, setPreview] = useState<{ subject: string; body: string } | null>(null);
  const { busy, error, run } = useAction();

  // Live preview with example values.
  useEffect(() => {
    const t = setTimeout(() => {
      api.previewMessage(subject, body).then(setPreview).catch(() => setPreview(null));
    }, 300);
    return () => clearTimeout(t);
  }, [subject, body]);

  const save = async () => {
    const ok = await run(() => api.saveMessage({ event: tpl.event, subject, body, active }).then(() => true));
    if (ok) {
      setSaved(true);
      reload();
    }
  };

  return (
    <div className="stack">
      <Panel title={statusLabels[tpl.event]} note={when[tpl.event]} actions={<button className="btn primary" disabled={busy} onClick={save}>{busy ? "Saving…" : "Save message"}</button>}>
        <SavedBanner show={saved}>Message saved.</SavedBanner>
        <ErrorBanner error={error} />
        <label className="check" htmlFor="msg-active">
          <input id="msg-active" type="checkbox" checked={active} onChange={(e) => { setSaved(false); setActive(e.target.checked); }} />
          Send this message
        </label>
        <Field label="Email subject" htmlFor="msg-subject" error={error?.field === "subject" ? error.message : undefined}>
          <input id="msg-subject" value={subject} onChange={(e) => { setSaved(false); setSubject(e.target.value); }} />
        </Field>
        <Field label="Message" htmlFor="msg-body" error={error?.field === "body" ? error.message : undefined}>
          <textarea id="msg-body" rows={12} value={body} onChange={(e) => { setSaved(false); setBody(e.target.value); }} />
        </Field>
        <div className="chips">
          <span className="small muted">Insert:</span>
          {placeholders.map((p) => (
            <button type="button" key={p.key} className="chip small-chip" onClick={() => setBody((b) => b + `{${p.key}}`)}>
              {`{${p.key}}`}
            </button>
          ))}
        </div>
      </Panel>
      {preview && (
        <Panel title="Preview" note="With example details. Empty lines from unused placeholders are removed.">
          <div className="email-preview">
            <b>{preview.subject}</b>
            <pre>{preview.body}</pre>
          </div>
        </Panel>
      )}
    </div>
  );
}
