import { useState } from "react";
import { useSearchParams } from "react-router-dom";
import { api, type Colour, type Setup, type SetupCategory, type SizeSet } from "../api";
import { ErrorBanner, Field, Loading, PageHeader, Panel, SavedBanner } from "../components/ui";
import { useAction, useApi } from "../useApi";

const tabs = [
  { id: "categories", label: "Categories" },
  { id: "sizes", label: "Size sets" },
  { id: "colours", label: "Colours" },
] as const;

export function CataloguePage() {
  const [params, setParams] = useSearchParams();
  const tab = params.get("tab") ?? "sizes";
  const setup = useApi("setup", (s) => api.setup(s));

  return (
    <>
      <PageHeader title="Catalogue setup" />
      <p className="muted">The building blocks every product uses. The shop's menus, filters, size buttons and colour circles all read from here.</p>
      <div className="tabs" role="tablist">
        {tabs.map((t) => (
          <button key={t.id} role="tab" aria-selected={tab === t.id} className={tab === t.id ? "on" : ""} onClick={() => setParams({ tab: t.id }, { replace: true })}>
            {t.label}
          </button>
        ))}
      </div>
      <ErrorBanner error={setup.error} />
      {!setup.data && !setup.error && <Loading />}
      {setup.data && tab === "sizes" && <SizeSets setup={setup.data} reload={setup.reload} />}
      {setup.data && tab === "colours" && <Colours colours={setup.data.colours} reload={setup.reload} />}
      {setup.data && tab === "categories" && <Categories setup={setup.data} reload={setup.reload} />}
    </>
  );
}

// ---------- Size sets ----------

type SizeRow = { id: number; label: string; measurement: string; usedBy: number };

function SizeSets({ setup, reload }: { setup: Setup; reload: () => void }) {
  const [selected, setSelected] = useState<number>(setup.sizeSets[0]?.id ?? 0);
  const current = setup.sizeSets.find((s) => s.id === selected);

  return (
    <div className="split">
      <div className="list-panel">
        {setup.sizeSets.map((s) => (
          <button key={s.id} className={s.id === selected ? "list-item on" : "list-item"} onClick={() => setSelected(s.id)}>
            <b>{s.name}</b>
            <span className="small muted">{s.sizes.map((z) => z.label).join(" · ") || "No sizes"}</span>
          </button>
        ))}
        <button className="list-item add" onClick={() => setSelected(-1)}>
          + New size set
        </button>
      </div>
      <SizeSetEditor key={selected} set={selected === -1 ? null : current ?? null} onSaved={(id) => { setSelected(id); reload(); }} />
    </div>
  );
}

function SizeSetEditor({ set, onSaved }: { set: SizeSet | null; onSaved: (id: number) => void }) {
  const [name, setName] = useState(set?.name ?? "");
  const [measure, setMeasure] = useState(set?.measureLabel ?? "");
  const [rows, setRows] = useState<SizeRow[]>(set?.sizes ?? [{ id: 0, label: "", measurement: "", usedBy: 0 }]);
  const [saved, setSaved] = useState(false);
  const { busy, error, run } = useAction();

  const update = (i: number, patch: Partial<SizeRow>) => {
    setSaved(false);
    setRows((rs) => rs.map((r, j) => (j === i ? { ...r, ...patch } : r)));
  };
  const move = (i: number, dir: -1 | 1) => {
    setRows((rs) => {
      const next = [...rs];
      [next[i], next[i + dir]] = [next[i + dir], next[i]];
      return next;
    });
  };

  const save = async () => {
    const res = await run(() => api.saveSizeSet(set?.id ?? 0, { name, measureLabel: measure, sizes: rows.map(({ id, label, measurement }) => ({ id, label, measurement })) }));
    if (res) {
      setSaved(true);
      onSaved(res.id);
    }
  };

  return (
    <Panel title={set ? set.name : "New size set"} actions={<button className="btn primary" onClick={save} disabled={busy}>{busy ? "Saving…" : "Save size set"}</button>}>
      <SavedBanner show={saved} />
      <ErrorBanner error={error} />
      <div className="grid-2">
        <Field label="Name" htmlFor="ss-name">
          <input id="ss-name" value={name} onChange={(e) => setName(e.target.value)} />
        </Field>
        <Field label="Measurement column heading" htmlFor="ss-measure" hint="e.g. UK size · bust · waist · hips">
          <input id="ss-measure" value={measure} onChange={(e) => setMeasure(e.target.value)} />
        </Field>
      </div>
      <div className="table-wrap">
        <table className="table compact">
          <thead>
            <tr>
              <th>Size button shows</th>
              <th>{measure || "Measurement"}</th>
              <th>Order</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {rows.map((r, i) => (
              <tr key={i}>
                <td>
                  <input aria-label={`Size ${i + 1} label`} className="w-100" value={r.label} onChange={(e) => update(i, { label: e.target.value })} />
                </td>
                <td>
                  <input aria-label={`Size ${i + 1} measurement`} value={r.measurement} onChange={(e) => update(i, { measurement: e.target.value })} />
                </td>
                <td className="nowrap">
                  <button type="button" className="icon-btn" aria-label="Move up" disabled={i === 0} onClick={() => move(i, -1)}>
                    ↑
                  </button>
                  <button type="button" className="icon-btn" aria-label="Move down" disabled={i === rows.length - 1} onClick={() => move(i, 1)}>
                    ↓
                  </button>
                </td>
                <td>
                  {r.usedBy > 0 ? (
                    <span className="small muted">Used by {r.usedBy} {r.usedBy === 1 ? "option" : "options"}</span>
                  ) : (
                    <button type="button" className="link-btn danger-text" onClick={() => setRows((rs) => rs.filter((_, j) => j !== i))}>
                      Remove
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <button type="button" className="link-btn" onClick={() => setRows((rs) => [...rs, { id: 0, label: "", measurement: "", usedBy: 0 }])}>
        + Add size
      </button>
      {rows.length > 0 && (
        <div className="stack-xs">
          <span className="label">How it looks on the product page</span>
          <div className="chips">
            {rows.filter((r) => r.label).map((r, i) => (
              <span key={i} className="size-preview">
                {r.label}
              </span>
            ))}
          </div>
        </div>
      )}
    </Panel>
  );
}

// ---------- Colours ----------

function Colours({ colours, reload }: { colours: Colour[]; reload: () => void }) {
  return (
    <div className="colour-grid">
      {colours.map((c) => (
        <ColourCard key={c.id} colour={c} reload={reload} />
      ))}
      <ColourCard colour={null} reload={reload} />
    </div>
  );
}

function ColourCard({ colour, reload }: { colour: Colour | null; reload: () => void }) {
  const [name, setName] = useState(colour?.name ?? "");
  const [hex, setHex] = useState(colour?.hex ?? "#B76E79");
  const [confirm, setConfirm] = useState(false);
  const { busy, error, run } = useAction();
  const changed = !colour || name !== colour.name || hex.toUpperCase() !== colour.hex.toUpperCase();
  const idBase = `col-${colour?.id ?? "new"}`;

  const save = async () => {
    const res = await run(() => api.saveColour(colour?.id ?? 0, { name, hex }));
    if (res) {
      if (!colour) setName("");
      reload();
    }
  };

  return (
    <div className={colour ? "colour-card" : "colour-card new"}>
      <div className="row">
        <label htmlFor={`${idBase}-hex`} className="visually-hidden">
          Swatch colour
        </label>
        <input id={`${idBase}-hex`} type="color" className="swatch-input" value={hex} onChange={(e) => setHex(e.target.value)} />
        <label htmlFor={`${idBase}-name`} className="visually-hidden">
          Colour name
        </label>
        <input id={`${idBase}-name`} placeholder={colour ? "" : "New colour name"} value={name} onChange={(e) => setName(e.target.value)} />
      </div>
      <span className="small muted">{colour ? `${hex.toUpperCase()} · ${colour.usedBy} ${colour.usedBy === 1 ? "product" : "products"}` : "Add a colour"}</span>
      {error && <span className="field-error">{error.message}</span>}
      <div className="row">
        {changed && (
          <button className="btn small primary" onClick={save} disabled={busy || !name.trim()}>
            {colour ? "Save" : "Add colour"}
          </button>
        )}
        {colour && colour.usedBy === 0 && !confirm && (
          <button className="link-btn danger-text" onClick={() => setConfirm(true)}>
            Delete
          </button>
        )}
        {confirm && colour && (
          <>
            <button className="link-btn danger-text" disabled={busy} onClick={() => run(() => api.deleteColour(colour.id)).then(reload)}>
              Delete {colour.name}
            </button>
            <button className="link-btn" onClick={() => setConfirm(false)}>
              Keep
            </button>
          </>
        )}
      </div>
    </div>
  );
}

// ---------- Categories ----------

function Categories({ setup, reload }: { setup: Setup; reload: () => void }) {
  return (
    <div className="stack">
      {setup.categories.map((top) => (
        <Panel key={top.id} title={top.name}>
          <CategoryRow cat={top} setup={setup} reload={reload} top />
          {top.children.length > 0 && <h3>Sub-categories</h3>}
          {top.children.map((c) => (
            <CategoryRow key={c.id} cat={c} setup={setup} reload={reload} />
          ))}
          <NewSubCategory parent={top} reload={reload} />
          <FieldsEditor cat={top} reload={reload} />
        </Panel>
      ))}
      <Panel title="Add a top category">
        <NewSubCategory parent={null} reload={reload} />
      </Panel>
    </div>
  );
}

function CategoryRow({ cat, setup, reload, top }: { cat: SetupCategory; setup: Setup; reload: () => void; top?: boolean }) {
  const [name, setName] = useState(cat.name);
  const [sizeSet, setSizeSet] = useState<number | null>(cat.defaultSizeSetId);
  const [rule, setRule] = useState(cat.returnRule);
  const [home, setHome] = useState(cat.showOnHome);
  const { busy, error, run } = useAction();
  const changed = name !== cat.name || sizeSet !== cat.defaultSizeSetId || rule !== cat.returnRule || home !== cat.showOnHome;
  const id = `cat-${cat.id}`;

  return (
    <div className="cat-row">
      <Field label="Name" htmlFor={`${id}-name`}>
        <input id={`${id}-name`} value={name} onChange={(e) => setName(e.target.value)} />
      </Field>
      <Field label="Default size set" htmlFor={`${id}-sizes`}>
        <select id={`${id}-sizes`} value={sizeSet ?? ""} onChange={(e) => setSizeSet(e.target.value ? Number(e.target.value) : null)}>
          <option value="">{top ? "None" : "Same as parent"}</option>
          {setup.sizeSets.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </select>
      </Field>
      <Field label="Returns" htmlFor={`${id}-rule`}>
        <input id={`${id}-rule`} value={rule} onChange={(e) => setRule(e.target.value)} placeholder={top ? "" : "Same as parent"} />
      </Field>
      {top && (
        <label className="check align-end" htmlFor={`${id}-home`}>
          <input id={`${id}-home`} type="checkbox" checked={home} onChange={(e) => setHome(e.target.checked)} />
          On homepage
        </label>
      )}
      <div className="align-end">
        {changed && (
          <button className="btn small primary" disabled={busy} onClick={() => run(() => api.saveCategory(cat.id, { name, defaultSizeSetId: sizeSet, returnRule: rule, showOnHome: home, position: cat.position })).then((r) => r && reload())}>
            Save
          </button>
        )}
      </div>
      {error && <span className="field-error full">{error.message}</span>}
    </div>
  );
}

function NewSubCategory({ parent, reload }: { parent: SetupCategory | null; reload: () => void }) {
  const [name, setName] = useState("");
  const { busy, error, run } = useAction();
  const id = `new-cat-${parent?.id ?? "top"}`;
  const add = async () => {
    const res = await run(() => api.saveCategory(0, { parentId: parent?.id ?? null, name, defaultSizeSetId: null, returnRule: "", showOnHome: false, position: 0 }));
    if (res) {
      setName("");
      reload();
    }
  };
  return (
    <div className="row">
      <label htmlFor={id} className="visually-hidden">
        {parent ? `New sub-category of ${parent.name}` : "New top category"}
      </label>
      <input id={id} placeholder={parent ? `New sub-category of ${parent.name}` : "New top category name"} value={name} onChange={(e) => setName(e.target.value)} />
      <button className="btn small secondary" disabled={busy || !name.trim()} onClick={add}>
        + Add
      </button>
      {error && <span className="field-error">{error.message}</span>}
    </div>
  );
}

function FieldsEditor({ cat, reload }: { cat: SetupCategory; reload: () => void }) {
  const [fields, setFields] = useState(cat.fields.map((f) => ({ id: f.id, label: f.label })));
  const [saved, setSaved] = useState(false);
  const { busy, error, run } = useAction();
  const changed = JSON.stringify(fields) !== JSON.stringify(cat.fields.map((f) => ({ id: f.id, label: f.label })));
  const removing = cat.fields.filter((f) => !fields.some((x) => x.id === f.id));

  return (
    <div className="stack-xs fields-editor">
      <h3>Product details for {cat.name.toLowerCase()}</h3>
      <p className="small muted">Fields staff fill in on every {cat.name.toLowerCase()} product, shown under "Details" on the product page.</p>
      {fields.map((f, i) => (
        <div className="row" key={f.id || `new-${i}`}>
          <label htmlFor={`fld-${cat.id}-${i}`} className="visually-hidden">
            Field {i + 1}
          </label>
          <input id={`fld-${cat.id}-${i}`} value={f.label} onChange={(e) => { setSaved(false); setFields((fs) => fs.map((x, j) => (j === i ? { ...x, label: e.target.value } : x))); }} />
          <button type="button" className="link-btn danger-text" onClick={() => setFields((fs) => fs.filter((_, j) => j !== i))}>
            Remove
          </button>
        </div>
      ))}
      <div className="row">
        <button type="button" className="link-btn" onClick={() => setFields((fs) => [...fs, { id: 0, label: "" }])}>
          + Add field
        </button>
        {changed && (
          <button className="btn small primary" disabled={busy} onClick={() => run(() => api.saveCategoryFields(cat.id, fields).then(() => true)).then((ok) => { if (ok) { setSaved(true); reload(); } })}>
            Save fields
          </button>
        )}
      </div>
      {removing.length > 0 && changed && (
        <p className="small warning-text">
          Saving removes {removing.map((f) => f.label).join(", ")} and what products had filled in for {removing.length === 1 ? "it" : "them"}.
        </p>
      )}
      {saved && !changed && <span className="small success-text">Fields saved.</span>}
      {error && <span className="field-error">{error.message}</span>}
    </div>
  );
}
