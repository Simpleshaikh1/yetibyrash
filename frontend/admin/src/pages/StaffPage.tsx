import { useState, type FormEvent } from "react";
import { api, roleLabels, type Role, type StaffAccount } from "../api";
import { useAuth } from "../auth";
import { ErrorBanner, Field, Loading, PageHeader, Panel, SavedBanner } from "../components/ui";
import { useAction, useApi } from "../useApi";

const roles: Role[] = ["owner", "manager", "packer", "content_editor"];
const roleHelp: Record<Role, string> = {
  owner: "Everything, including staff and payments.",
  manager: "Orders, products, catalogue setup, content, delivery.",
  packer: "Orders only: pack, ship, update tracking.",
  content_editor: "Products, photos, homepage and pages.",
};

function formatWhen(iso: string | null) {
  if (!iso) return "Never signed in";
  return "Last signed in " + new Date(iso).toLocaleString("en-NG", { day: "numeric", month: "short", hour: "numeric", minute: "2-digit" });
}

export function StaffPage() {
  const list = useApi("staff", (s) => api.staff(s));
  return (
    <>
      <PageHeader title="Staff and roles" />
      <div className="editor-grid">
        <div className="stack">
          <ErrorBanner error={list.error} />
          {!list.data && !list.error && <Loading />}
          {list.data?.map((s) => (
            <StaffCard key={s.id} account={s} reload={list.reload} />
          ))}
        </div>
        <div className="stack">
          <AddStaff reload={list.reload} />
          <Panel title="What each role can do">
            <dl className="roles">
              {roles.map((r) => (
                <div key={r}>
                  <dt>{roleLabels[r]}</dt>
                  <dd>{roleHelp[r]}</dd>
                </div>
              ))}
            </dl>
            <p className="small muted">Every change is logged with who made it and when.</p>
          </Panel>
        </div>
      </div>
    </>
  );
}

function StaffCard({ account, reload }: { account: StaffAccount; reload: () => void }) {
  const { me } = useAuth();
  const [name, setName] = useState(account.name);
  const [role, setRole] = useState<Role>(account.role);
  const [active, setActive] = useState(account.active);
  const [password, setPassword] = useState("");
  const [saved, setSaved] = useState<string | null>(null);
  const { busy, error, run } = useAction();
  const changed = name !== account.name || role !== account.role || active !== account.active;
  const id = `st-${account.id}`;
  const isMe = me?.staff.id === account.id;

  return (
    <Panel title={`${account.name}${isMe ? " (you)" : ""}`} note={`${account.email} · ${formatWhen(account.lastLoginAt)}`}>
      <SavedBanner show={saved !== null}>{saved}</SavedBanner>
      <ErrorBanner error={error} />
      <div className="grid-3">
        <Field label="Name" htmlFor={`${id}-name`}>
          <input id={`${id}-name`} value={name} onChange={(e) => setName(e.target.value)} />
        </Field>
        <Field label="Role" htmlFor={`${id}-role`}>
          <select id={`${id}-role`} value={role} onChange={(e) => setRole(e.target.value as Role)}>
            {roles.map((r) => (
              <option key={r} value={r}>
                {roleLabels[r]}
              </option>
            ))}
          </select>
        </Field>
        <label className="check align-end" htmlFor={`${id}-active`}>
          <input id={`${id}-active`} type="checkbox" checked={active} onChange={(e) => setActive(e.target.checked)} />
          Can sign in
        </label>
      </div>
      {changed && (
        <button className="btn small primary" disabled={busy} onClick={() => run(() => api.updateStaff(account.id, { name, role, active }).then(() => true)).then((ok) => { if (ok) { setSaved("Saved."); reload(); } })}>
          Save changes
        </button>
      )}
      <form
        className="row"
        onSubmit={async (e: FormEvent) => {
          e.preventDefault();
          const ok = await run(() => api.setStaffPassword(account.id, password).then(() => true));
          if (ok) {
            setPassword("");
            setSaved(isMe ? "Password changed. Sign in again with the new password." : "Password changed. They're signed out everywhere.");
          }
        }}
      >
        <label htmlFor={`${id}-pw`} className="visually-hidden">
          New password for {account.name}
        </label>
        <input id={`${id}-pw`} type="password" autoComplete="new-password" placeholder="New password (10+ characters)" value={password} onChange={(e) => setPassword(e.target.value)} />
        <button className="btn small secondary" disabled={busy || password.length < 10}>
          Set password
        </button>
      </form>
    </Panel>
  );
}

function AddStaff({ reload }: { reload: () => void }) {
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [role, setRole] = useState<Role>("packer");
  const [password, setPassword] = useState("");
  const [added, setAdded] = useState(false);
  const { busy, error, run } = useAction();

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const res = await run(() => api.createStaff({ email, name, role, password }));
    if (res) {
      setEmail("");
      setName("");
      setPassword("");
      setAdded(true);
      reload();
    }
  };

  return (
    <Panel title="Add staff" note="Give them their email and this first password; they can't change it themselves yet, so set a new one here when needed.">
      <form className="stack-sm" onSubmit={submit}>
        <SavedBanner show={added}>Staff account added.</SavedBanner>
        <ErrorBanner error={error} />
        <Field label="Email" htmlFor="new-email">
          <input id="new-email" type="email" required value={email} onChange={(e) => { setAdded(false); setEmail(e.target.value); }} />
        </Field>
        <Field label="Name" htmlFor="new-name">
          <input id="new-name" required value={name} onChange={(e) => setName(e.target.value)} />
        </Field>
        <Field label="Role" htmlFor="new-role" hint={roleHelp[role]}>
          <select id="new-role" value={role} onChange={(e) => setRole(e.target.value as Role)}>
            {roles.map((r) => (
              <option key={r} value={r}>
                {roleLabels[r]}
              </option>
            ))}
          </select>
        </Field>
        <Field label="First password" htmlFor="new-password" hint="At least 10 characters.">
          <input id="new-password" type="password" autoComplete="new-password" required minLength={10} value={password} onChange={(e) => setPassword(e.target.value)} />
        </Field>
        <button className="btn primary" disabled={busy}>
          {busy ? "Adding…" : "Add staff"}
        </button>
      </form>
    </Panel>
  );
}
