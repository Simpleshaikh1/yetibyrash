import { useState, type FormEvent } from "react";
import { formatNaira, koboToInput, parseNaira } from "@yetibyrash/ui";
import { api, ApiError, type DeliveryAdmin, type DeliveryState, type Zone } from "../api";
import { ErrorBanner, Field, Loading, PageHeader, Panel, SavedBanner } from "../components/ui";
import { useAction, useApi } from "../useApi";

const weekdays = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

export function DeliveryPage() {
  const data = useApi("delivery", (s) => api.delivery(s));
  return (
    <>
      <PageHeader title="Delivery" />
      <p className="muted">Checkout works out the fee from the shopper's state and LGA. The product page and bag show the same numbers.</p>
      <ErrorBanner error={data.error} />
      {!data.data && !data.error && <Loading />}
      {data.data && (
        <div className="stack">
          <Panel title="Zones and fees" note="Standard delivery is free when the bag reaches the 'free over' amount. Express shows only in LGAs ticked for express below.">
            <div className="table-wrap">
              <table className="table compact">
                <thead>
                  <tr>
                    <th>Zone</th>
                    <th>Standard ₦</th>
                    <th>Days</th>
                    <th>Express ₦</th>
                    <th>Express days</th>
                    <th>Free over ₦</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {data.data.zones.map((z) => (
                    <ZoneRow key={z.id} zone={z} reload={data.reload} />
                  ))}
                </tbody>
              </table>
            </div>
            <p className="small muted">Express days: 0 = same day if ordered before the cut-off, 1 = next working day. Leave express blank where it isn't offered.</p>
          </Panel>
          <div className="editor-grid">
            <States data={data.data} reload={data.reload} />
            <div className="stack">
              <Rules data={data.data} reload={data.reload} />
              <Holidays data={data.data} reload={data.reload} />
            </div>
          </div>
        </div>
      )}
    </>
  );
}

function ZoneRow({ zone, reload }: { zone: Zone; reload: () => void }) {
  const [v, setV] = useState({
    name: zone.name,
    fee: koboToInput(zone.standardFeeKobo),
    min: String(zone.standardDaysMin),
    max: String(zone.standardDaysMax),
    exFee: koboToInput(zone.expressFeeKobo),
    exDays: zone.expressDays === null ? "" : String(zone.expressDays),
    free: koboToInput(zone.freeOverKobo),
  });
  const [saved, setSaved] = useState(false);
  const { busy, error, setError, run } = useAction();
  const set = (k: keyof typeof v, val: string) => {
    setSaved(false);
    setV((x) => ({ ...x, [k]: val }));
  };

  const save = async () => {
    const fee = parseNaira(v.fee);
    const exFee = v.exFee.trim() ? parseNaira(v.exFee) : null;
    const free = v.free.trim() ? parseNaira(v.free) : null;
    if (fee === null || (v.exFee.trim() && exFee === null) || (v.free.trim() && free === null)) {
      setError(new ApiError(400, "invalid", "Amounts must be numbers in naira, e.g. 2500."));
      return;
    }
    const ok = await run(() =>
      api
        .updateZone({
          id: zone.id, name: v.name, standardFeeKobo: fee, standardDaysMin: Number(v.min), standardDaysMax: Number(v.max),
          expressFeeKobo: exFee, expressDays: v.exDays.trim() === "" ? null : Number(v.exDays), freeOverKobo: free,
        })
        .then(() => true),
    );
    if (ok) {
      setSaved(true);
      reload();
    }
  };

  const id = `z-${zone.id}`;
  return (
    <>
      <tr>
        <td>
          <input aria-label="Zone name" id={`${id}-name`} value={v.name} onChange={(e) => set("name", e.target.value)} />
        </td>
        <td>
          <input aria-label={`${zone.name} standard fee`} className="w-100" inputMode="decimal" value={v.fee} onChange={(e) => set("fee", e.target.value)} />
        </td>
        <td className="nowrap">
          <input aria-label={`${zone.name} fewest days`} className="w-70" inputMode="numeric" value={v.min} onChange={(e) => set("min", e.target.value)} /> –{" "}
          <input aria-label={`${zone.name} most days`} className="w-70" inputMode="numeric" value={v.max} onChange={(e) => set("max", e.target.value)} />
        </td>
        <td>
          <input aria-label={`${zone.name} express fee`} className="w-100" inputMode="decimal" placeholder="—" value={v.exFee} onChange={(e) => set("exFee", e.target.value)} />
        </td>
        <td>
          <input aria-label={`${zone.name} express days`} className="w-70" inputMode="numeric" placeholder="—" value={v.exDays} onChange={(e) => set("exDays", e.target.value)} />
        </td>
        <td>
          <input aria-label={`${zone.name} free over`} className="w-100" inputMode="decimal" placeholder="Never" value={v.free} onChange={(e) => set("free", e.target.value)} />
        </td>
        <td>
          <button className="btn small primary" disabled={busy} onClick={save}>
            Save
          </button>
          {saved && <span className="small success-text"> Saved</span>}
        </td>
      </tr>
      {error && (
        <tr>
          <td colSpan={7} className="field-error">
            {error.message}
          </td>
        </tr>
      )}
    </>
  );
}

function States({ data, reload }: { data: DeliveryAdmin; reload: () => void }) {
  const [selected, setSelected] = useState<number>(data.states.find((s) => s.name === "Lagos")?.id ?? data.states[0]?.id ?? 0);
  const state = data.states.find((s) => s.id === selected);
  return (
    <Panel title="States and LGAs" note="Pick a state to change its zone or its LGAs. States without listed LGAs ask the shopper to type their LGA or town.">
      <Field label="State" htmlFor="dl-state">
        <select id="dl-state" value={selected} onChange={(e) => setSelected(Number(e.target.value))}>
          {data.states.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name} · {s.zoneName} · {s.lgas.length ? `${s.lgas.length} LGAs` : "LGA typed"}
            </option>
          ))}
        </select>
      </Field>
      {state && <StateEditor key={state.id} state={state} zones={data.zones} reload={reload} />}
    </Panel>
  );
}

function StateEditor({ state, zones, reload }: { state: DeliveryState; zones: Zone[]; reload: () => void }) {
  const [newLga, setNewLga] = useState("");
  const { busy, error, run } = useAction();

  return (
    <div className="stack-sm">
      <ErrorBanner error={error} />
      <Field label="Zone" htmlFor="dl-zone">
        <select id="dl-zone" value={state.zoneId} disabled={busy} onChange={(e) => run(() => api.setStateZone(state.id, Number(e.target.value)).then(() => true)).then((ok) => ok && reload())}>
          {zones.map((z) => (
            <option key={z.id} value={z.id}>
              {z.name}
            </option>
          ))}
        </select>
      </Field>
      {state.lgas.length > 0 && (
        <div className="table-wrap">
          <table className="table compact">
            <thead>
              <tr>
                <th>LGA</th>
                <th>We deliver</th>
                <th>Express</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {state.lgas.map((l) => (
                <tr key={l.id}>
                  <td>{l.name}</td>
                  <td>
                    <input type="checkbox" aria-label={`Deliver to ${l.name}`} checked={l.delivered} disabled={busy} onChange={(e) => run(() => api.updateLga({ ...l, delivered: e.target.checked }).then(() => true)).then((ok) => ok && reload())} />
                  </td>
                  <td>
                    <input type="checkbox" aria-label={`Express in ${l.name}`} checked={l.express} disabled={busy} onChange={(e) => run(() => api.updateLga({ ...l, express: e.target.checked }).then(() => true)).then((ok) => ok && reload())} />
                  </td>
                  <td>
                    <button className="link-btn danger-text" disabled={busy} onClick={() => run(() => api.deleteLga(l.id).then(() => true)).then((ok) => ok && reload())}>
                      Remove
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <form
        className="row"
        onSubmit={async (e: FormEvent) => {
          e.preventDefault();
          const ok = await run(() => api.addLga(state.id, { name: newLga, delivered: true, express: false }).then(() => true));
          if (ok) {
            setNewLga("");
            reload();
          }
        }}
      >
        <label htmlFor="dl-newlga" className="visually-hidden">
          New LGA in {state.name}
        </label>
        <input id="dl-newlga" placeholder={`Add an LGA in ${state.name}`} value={newLga} onChange={(e) => setNewLga(e.target.value)} />
        <button className="btn small secondary" disabled={busy || !newLga.trim()}>
          + Add LGA
        </button>
      </form>
    </div>
  );
}

function Rules({ data, reload }: { data: DeliveryAdmin; reload: () => void }) {
  const [cutoff, setCutoff] = useState(data.settings.expressCutoff);
  const [days, setDays] = useState<number[]>(data.settings.dispatchDays);
  const [saved, setSaved] = useState(false);
  const { busy, error, run } = useAction();
  const toggle = (d: number) => {
    setSaved(false);
    setDays((ds) => (ds.includes(d) ? ds.filter((x) => x !== d) : [...ds, d].sort()));
  };
  return (
    <Panel title="Rules">
      <SavedBanner show={saved} />
      <ErrorBanner error={error} />
      <Field label="Same-day cut-off" htmlFor="dl-cutoff" hint="Orders paid before this time leave the same day.">
        <input id="dl-cutoff" type="time" value={cutoff} onChange={(e) => { setSaved(false); setCutoff(e.target.value); }} />
      </Field>
      <div className="stack-xs">
        <span className="label">Days you dispatch</span>
        <div className="chips">
          {weekdays.map((w, i) => (
            <button type="button" key={w} className={days.includes(i + 1) ? "chip on" : "chip"} aria-pressed={days.includes(i + 1)} onClick={() => toggle(i + 1)}>
              {w}
            </button>
          ))}
        </div>
      </div>
      <button className="btn small primary" disabled={busy} onClick={() => run(() => api.updateDeliverySettings({ expressCutoff: cutoff, dispatchDays: days }).then(() => true)).then((ok) => { if (ok) { setSaved(true); reload(); } })}>
        Save rules
      </button>
    </Panel>
  );
}

function Holidays({ data, reload }: { data: DeliveryAdmin; reload: () => void }) {
  const [day, setDay] = useState("");
  const [name, setName] = useState("");
  const { busy, error, run } = useAction();
  return (
    <Panel title="Public holidays" note="No parcels leave on these days; delivery dates skip them.">
      <ErrorBanner error={error} />
      {data.holidays.length === 0 && <p className="small muted">No upcoming holidays added.</p>}
      {data.holidays.map((h) => (
        <div key={h.day} className="row-between-admin">
          <span>
            {new Date(h.day + "T00:00:00").toLocaleDateString("en-GB", { weekday: "short", day: "numeric", month: "short", year: "numeric" })} · {h.name}
          </span>
          <button className="link-btn danger-text" disabled={busy} onClick={() => run(() => api.deleteHoliday(h.day).then(() => true)).then((ok) => ok && reload())}>
            Remove
          </button>
        </div>
      ))}
      <form
        className="row wrap"
        onSubmit={async (e: FormEvent) => {
          e.preventDefault();
          const ok = await run(() => api.addHoliday({ day, name }).then(() => true));
          if (ok) {
            setDay("");
            setName("");
            reload();
          }
        }}
      >
        <label htmlFor="hol-day" className="visually-hidden">
          Holiday date
        </label>
        <input id="hol-day" type="date" value={day} onChange={(e) => setDay(e.target.value)} />
        <label htmlFor="hol-name" className="visually-hidden">
          Holiday name
        </label>
        <input id="hol-name" placeholder="e.g. Christmas Day" value={name} onChange={(e) => setName(e.target.value)} />
        <button className="btn small secondary" disabled={busy || !day || !name.trim()}>
          + Add
        </button>
      </form>
      <p className="small muted">Fees shown to shoppers: {data.zones.map((z) => `${z.name} ${formatNaira(z.standardFeeKobo)}`).join(" · ")}</p>
    </Panel>
  );
}
