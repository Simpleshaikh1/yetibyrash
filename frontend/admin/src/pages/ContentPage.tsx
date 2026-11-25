import { useState } from "react";
import { bannerUrl, Markdown } from "@yetibyrash/ui";
import { contentApi, type Announcement, type Banner, type ContentAdmin, type HomeSection, type SitePage } from "../api";
import { ErrorBanner, Field, Loading, PageHeader, Panel, SavedBanner } from "../components/ui";
import { useAction, useApi } from "../useApi";

type Tab = "homepage" | "banners" | "bar" | "pages";
const tabs: { key: Tab; label: string }[] = [
  { key: "homepage", label: "Homepage sections" },
  { key: "banners", label: "Hero banners" },
  { key: "bar", label: "Announcement bar" },
  { key: "pages", label: "Pages" },
];

// Design: Admin CMS › Homepage and content.
export function ContentPage() {
  const data = useApi("content", (s) => contentApi.get(s));
  const [tab, setTab] = useState<Tab>("homepage");
  return (
    <>
      <PageHeader title="Homepage and content" />
      <div className="tabs" role="tablist">
        {tabs.map((t) => (
          <button key={t.key} role="tab" aria-selected={tab === t.key} className={tab === t.key ? "on" : ""} onClick={() => setTab(t.key)}>
            {t.label}
          </button>
        ))}
      </div>
      <ErrorBanner error={data.error} />
      {!data.data && !data.error && <Loading />}
      {data.data && tab === "homepage" && <Sections key={JSON.stringify(data.data.sections)} sections={data.data.sections} reload={data.reload} />}
      {data.data && tab === "banners" && <Banners data={data.data} reload={data.reload} />}
      {data.data && tab === "bar" && <Bar data={data.data} reload={data.reload} />}
      {data.data && tab === "pages" && <Pages pages={data.data.pages} reload={data.reload} />}
    </>
  );
}

function move<T>(list: T[], i: number, by: -1 | 1): T[] {
  const j = i + by;
  if (j < 0 || j >= list.length) return list;
  const out = [...list];
  [out[i], out[j]] = [out[j], out[i]];
  return out;
}

function Mover({ index, count, onMove, label }: { index: number; count: number; onMove: (by: -1 | 1) => void; label: string }) {
  return (
    <span className="mover">
      <button type="button" className="btn small" disabled={index === 0} onClick={() => onMove(-1)} aria-label={`Move ${label} up`}>
        ↑
      </button>
      <button type="button" className="btn small" disabled={index === count - 1} onClick={() => onMove(1)} aria-label={`Move ${label} down`}>
        ↓
      </button>
    </span>
  );
}

// ---------- Homepage sections ----------

const sectionHelp: Record<string, string> = {
  hero: "Shows the live banner for the tier the shopper picked (see Hero banners).",
  categories: 'Categories marked "On homepage" in Catalogue setup.',
  new_in: "The newest products in the shopper's tier.",
  best_sellers: "Most sold in the last 30 days. Hidden on the shop until something sells.",
  gift_budget: "Links to the price bands set in Catalogue setup.",
  trust: "Short reasons to trust the shop, up to 4 lines.",
};

function Sections({ sections, reload }: { sections: HomeSection[]; reload: () => void }) {
  const [list, setList] = useState(sections);
  const [saved, setSaved] = useState(false);
  const { busy, error, run } = useAction();
  const update = (i: number, patch: Partial<HomeSection>) => {
    setSaved(false);
    setList((l) => l.map((s, j) => (j === i ? { ...s, ...patch } : s)));
  };
  const save = async () => {
    if (await run(() => contentApi.saveSections(list).then(() => true))) {
      setSaved(true);
      reload();
    }
  };
  return (
    <Panel
      title="Homepage sections"
      note="Reorder with the arrows, switch a section off to hide it, and rename it as the shop shows it."
      actions={
        <button className="btn primary" disabled={busy} onClick={save}>
          {busy ? "Saving…" : "Save homepage"}
        </button>
      }
    >
      <SavedBanner show={saved}>Homepage saved. The shop shows the new order now.</SavedBanner>
      <ErrorBanner error={error} />
      {list.map((s, i) => (
        <div key={s.key} className="section-row">
          <Mover index={i} count={list.length} label={s.title} onMove={(by) => { setSaved(false); setList((l) => move(l, i, by)); }} />
          <div className="stack-xs grow">
            <div className="row">
              <label htmlFor={`sec-${s.key}`} className="visually-hidden">
                Title
              </label>
              <input id={`sec-${s.key}`} value={s.title} onChange={(e) => update(i, { title: e.target.value })} />
              <label className="check" htmlFor={`sec-${s.key}-on`}>
                <input id={`sec-${s.key}-on`} type="checkbox" checked={s.visible} onChange={(e) => update(i, { visible: e.target.checked })} />
                Show
              </label>
              {(s.key === "new_in" || s.key === "best_sellers") && (
                <label className="check" htmlFor={`sec-${s.key}-n`}>
                  Products
                  <select id={`sec-${s.key}-n`} value={s.itemCount} onChange={(e) => update(i, { itemCount: Number(e.target.value) })}>
                    {[2, 4, 6, 8, 10, 12].map((n) => (
                      <option key={n}>{n}</option>
                    ))}
                  </select>
                </label>
              )}
            </div>
            <span className="small muted">{sectionHelp[s.key]}</span>
            {s.key === "trust" && (
              <textarea
                aria-label="Trust strip lines, one per line"
                rows={4}
                value={s.lines.join("\n")}
                onChange={(e) => update(i, { lines: e.target.value.split("\n") })}
              />
            )}
          </div>
        </div>
      ))}
    </Panel>
  );
}

// ---------- Hero banners ----------

const emptyBanner: Banner = {
  id: 0, eyebrow: "", headline: "", buttonText: "Shop now", buttonLink: "/shop", image: "",
  tier: "", startsOn: "", endsOn: "", active: true, live: false,
};

function when(startsOn: string, endsOn: string): string {
  if (!startsOn && !endsOn) return "From now, no end date";
  if (!endsOn) return `From ${startsOn}`;
  return `${startsOn || "Now"} – ${endsOn}`;
}

function Banners({ data, reload }: { data: ContentAdmin; reload: () => void }) {
  const [editing, setEditing] = useState<Banner | null>(null);
  const [session, setSession] = useState(0); // a new editor each time a banner is picked
  const pick = (b: Banner) => {
    setEditing(b);
    setSession((n) => n + 1);
  };
  const { error, run } = useAction();
  const reorder = (i: number, by: -1 | 1) => {
    const ids = move(data.banners, i, by).map((b) => b.id);
    void run(() => contentApi.orderBanners(ids)).then(reload);
  };
  return (
    <div className="split">
      <div className="stack-sm">
        <div className="list-panel">
          {data.banners.map((b, i) => (
            <div key={b.id} className={editing?.id === b.id ? "list-item on banner-item" : "list-item banner-item"}>
              <Mover index={i} count={data.banners.length} label={b.headline} onMove={(by) => reorder(i, by)} />
              <button className="banner-pick" onClick={() => pick(b)}>
                {b.image ? <img src={bannerUrl(b.image, "phone")} alt="" className="banner-thumb" /> : <span className="banner-thumb empty" />}
                <span className="stack-xs">
                  <b>{b.headline}</b>
                  <span className="small muted">
                    {b.tier ? (b.tier === "premium" ? "Premium" : "Everyday") : "Both tiers"} · {when(b.startsOn, b.endsOn)}
                  </span>
                  <span className={b.live ? "pill ok" : "pill neutral"}>{b.live ? "Live now" : b.active ? "Not showing" : "Off"}</span>
                </span>
              </button>
            </div>
          ))}
          <button className="list-item add" onClick={() => pick(emptyBanner)}>
            + New banner
          </button>
        </div>
        <ErrorBanner error={error} />
        <p className="small muted">The homepage shows the first live banner for each tier, top to bottom.</p>
      </div>
      {editing && (
        <BannerEditor
          key={session}
          banner={editing}
          onSaved={(id) => {
            reload();
            setEditing((e) => (e ? { ...e, id } : e));
          }}
          onDeleted={() => {
            setEditing(null);
            reload();
          }}
        />
      )}
    </div>
  );
}

function BannerEditor({ banner, onSaved, onDeleted }: { banner: Banner; onSaved: (id: number) => void; onDeleted: () => void }) {
  const [b, setB] = useState(banner);
  const [saved, setSaved] = useState("");
  const [warnings, setWarnings] = useState<string[]>([]);
  const { busy, error, run } = useAction();
  const set = (patch: Partial<Banner>) => {
    setSaved("");
    setB((x) => ({ ...x, ...patch }));
  };
  const fieldError = (f: string) => (error?.field === f ? error.message : undefined);
  const save = async () => {
    const r = await run(() => contentApi.saveBanner(b));
    if (r) {
      setSaved(b.id ? "Banner saved." : "Banner added. Now add its photo.");
      setB((x) => ({ ...x, id: r.id }));
      onSaved(r.id);
    }
  };
  const upload = async (file: File) => {
    const r = await run(() => contentApi.bannerImage(b.id, file));
    if (r) {
      setB((x) => ({ ...x, image: r.image }));
      setWarnings(r.warnings);
      setSaved("Photo saved.");
      onSaved(b.id);
    }
  };
  const remove = async () => {
    if (!window.confirm(`Delete the banner "${b.headline}"? This can't be undone.`)) return;
    if ((await run(() => contentApi.deleteBanner(b.id).then(() => true))) === true) onDeleted();
  };
  return (
    <Panel
      title={b.id ? "Edit banner" : "New banner"}
      actions={
        <div className="row">
          {b.id !== 0 && (
            <button className="btn" disabled={busy} onClick={remove}>
              Delete
            </button>
          )}
          <button className="btn primary" disabled={busy} onClick={save}>
            {busy ? "Saving…" : b.id ? "Save banner" : "Add banner"}
          </button>
        </div>
      }
    >
      <SavedBanner show={!!saved}>{saved}</SavedBanner>
      <ErrorBanner error={error && !error.field ? error : null} />
      <div className="grid-2">
        <Field label="Small label" htmlFor="bn-eyebrow" hint='e.g. "New in · Lounge sets"' error={fieldError("eyebrow")}>
          <input id="bn-eyebrow" value={b.eyebrow} onChange={(e) => set({ eyebrow: e.target.value })} />
        </Field>
        <Field label="Headline" htmlFor="bn-headline" error={fieldError("headline")}>
          <input id="bn-headline" value={b.headline} onChange={(e) => set({ headline: e.target.value })} />
        </Field>
        <Field label="Button text" htmlFor="bn-btn" error={fieldError("buttonText")}>
          <input id="bn-btn" value={b.buttonText} onChange={(e) => set({ buttonText: e.target.value })} />
        </Field>
        <Field label="Button goes to" htmlFor="bn-link" hint="A shop page, e.g. /shop/bags or /p/gold-huggie-hoops" error={fieldError("buttonLink")}>
          <input id="bn-link" value={b.buttonLink} onChange={(e) => set({ buttonLink: e.target.value })} />
        </Field>
        <Field label="Show for" htmlFor="bn-tier" error={fieldError("tier")}>
          <select id="bn-tier" value={b.tier} onChange={(e) => set({ tier: e.target.value as Banner["tier"] })}>
            <option value="">Both tiers</option>
            <option value="everyday">Everyday</option>
            <option value="premium">Premium</option>
          </select>
        </Field>
        <label className="check" htmlFor="bn-active">
          <input id="bn-active" type="checkbox" checked={b.active} onChange={(e) => set({ active: e.target.checked })} />
          Banner is on
        </label>
        <Field label="First day" htmlFor="bn-start" hint="Empty = from now" error={fieldError("startsOn")}>
          <input id="bn-start" type="date" value={b.startsOn} onChange={(e) => set({ startsOn: e.target.value })} />
        </Field>
        <Field label="Last day" htmlFor="bn-end" hint="Empty = no end date" error={fieldError("endsOn")}>
          <input id="bn-end" type="date" value={b.endsOn} onChange={(e) => set({ endsOn: e.target.value })} />
        </Field>
      </div>

      <div className="stack-sm">
        <h3>Photo</h3>
        {b.id === 0 ? (
          <p className="small muted">Add the banner first, then its photo.</p>
        ) : (
          <>
            <p className="small muted">One photo makes both crops: tall 4:5 for phones and a wide 16:7 strip for laptops. Use a photo at least 2000 px wide with the product near the middle.</p>
            {b.image && (
              <div className="banner-previews">
                <figure>
                  <img src={bannerUrl(b.image, "phone")} alt="Phone crop" />
                  <figcaption className="small muted">Phone 4:5</figcaption>
                </figure>
                <figure className="wide">
                  <img src={bannerUrl(b.image, "wide")} alt="Desktop crop" />
                  <figcaption className="small muted">Desktop 16:7</figcaption>
                </figure>
              </div>
            )}
            {warnings.includes("small") && <p className="warn-text">This photo is small and may look soft on a laptop.</p>}
            {warnings.includes("dark") && <p className="warn-text">This photo is quite dark; the headline may be hard to read.</p>}
            <label className="btn" htmlFor="bn-photo">
              {b.image ? "Replace photo" : "+ Add photo"}
            </label>
            <input
              id="bn-photo"
              type="file"
              accept="image/jpeg,image/png,image/webp"
              className="visually-hidden"
              onChange={(e) => {
                const f = e.target.files?.[0];
                e.target.value = "";
                if (f) void upload(f);
              }}
            />
          </>
        )}
      </div>
    </Panel>
  );
}

// ---------- Announcement bar ----------

const emptyAnnouncement: Announcement = { id: 0, text: "", link: "", startsOn: "", endsOn: "", active: true, live: false };

function Bar({ data, reload }: { data: ContentAdmin; reload: () => void }) {
  const { error, run } = useAction();
  const reorder = (i: number, by: -1 | 1) => {
    const ids = move(data.announcements, i, by).map((a) => a.id);
    void run(() => contentApi.orderAnnouncements(ids)).then(reload);
  };
  return (
    <Panel title="Announcement bar" note="Short messages that take turns at the top of every shop page. Keep each under 70 characters so it fits on a phone.">
      <ErrorBanner error={error} />
      {data.saleMessages.map((m) => (
        <div key={m} className="ann-row sale">
          <span className="pill ok">From a live sale</span> {m}
          <span className="small muted">Edit it under Promotions.</span>
        </div>
      ))}
      {data.announcements.map((a, i) => (
        <div key={a.id} className="ann-row">
          <Mover index={i} count={data.announcements.length} label={a.text} onMove={(by) => reorder(i, by)} />
          <AnnouncementForm key={JSON.stringify(a)} a={a} reload={reload} />
        </div>
      ))}
      <div className="ann-row">
        <AnnouncementForm key={`new-${data.announcements.length}`} a={emptyAnnouncement} reload={reload} />
      </div>
    </Panel>
  );
}

function AnnouncementForm({ a: initial, reload }: { a: Announcement; reload: () => void }) {
  const [a, setA] = useState(initial);
  const { busy, error, run } = useAction();
  const changed = JSON.stringify(a) !== JSON.stringify(initial);
  const id = `ann-${initial.id || "new"}`;
  const save = async () => {
    if (await run(() => contentApi.saveAnnouncement(a))) reload();
  };
  const remove = async () => {
    if (!window.confirm(`Delete "${a.text}"?`)) return;
    if ((await run(() => contentApi.deleteAnnouncement(a.id).then(() => true))) === true) reload();
  };
  return (
    <div className="stack-xs grow">
      <div className="row wrap">
        <label htmlFor={`${id}-text`} className="visually-hidden">
          Message
        </label>
        <input id={`${id}-text`} className="grow" placeholder={initial.id ? "" : "Add a message, e.g. Free delivery in Lagos over ₦40,000"} value={a.text} maxLength={70} onChange={(e) => setA({ ...a, text: e.target.value })} />
        <label htmlFor={`${id}-link`} className="visually-hidden">
          Link
        </label>
        <input id={`${id}-link`} placeholder="Link (optional), e.g. /shop" value={a.link} onChange={(e) => setA({ ...a, link: e.target.value })} />
        {initial.id !== 0 && (
          <span className={a.live ? "pill ok" : "pill neutral"}>{initial.live ? "Showing" : initial.active ? "Scheduled or ended" : "Off"}</span>
        )}
      </div>
      {(initial.id !== 0 || a.text) && (
        <div className="row wrap">
          <label className="check" htmlFor={`${id}-on`}>
            <input id={`${id}-on`} type="checkbox" checked={a.active} onChange={(e) => setA({ ...a, active: e.target.checked })} />
            On
          </label>
          <label className="check" htmlFor={`${id}-start`}>
            From
            <input id={`${id}-start`} type="date" value={a.startsOn} onChange={(e) => setA({ ...a, startsOn: e.target.value })} />
          </label>
          <label className="check" htmlFor={`${id}-end`}>
            Until
            <input id={`${id}-end`} type="date" value={a.endsOn} onChange={(e) => setA({ ...a, endsOn: e.target.value })} />
          </label>
          {changed && (
            <button className="btn small primary" disabled={busy} onClick={save}>
              {initial.id ? "Save" : "Add message"}
            </button>
          )}
          {initial.id !== 0 && (
            <button className="btn small" disabled={busy} onClick={remove}>
              Delete
            </button>
          )}
        </div>
      )}
      {error && <span className="field-error">{error.message}</span>}
    </div>
  );
}

// ---------- Pages ----------

const emptyPage: SitePage = { id: 0, slug: "", title: "", body: "", published: false, updatedAt: "" };

function Pages({ pages, reload }: { pages: SitePage[]; reload: () => void }) {
  const [selected, setSelected] = useState<number>(pages[0]?.id ?? 0);
  const [session, setSession] = useState(0); // a new editor each time a page is picked
  const pick = (id: number) => {
    setSelected(id);
    setSession((n) => n + 1);
  };
  const page = pages.find((p) => p.id === selected) ?? emptyPage;
  return (
    <div className="split">
      <div className="list-panel">
        {pages.map((p) => (
          <button key={p.id} className={p.id === selected ? "list-item on" : "list-item"} onClick={() => pick(p.id)}>
            <b>{p.title}</b>
            <span className="small muted">{p.published ? `/page/${p.slug}` : "Draft"}</span>
          </button>
        ))}
        <button className={selected === 0 ? "list-item add on" : "list-item add"} onClick={() => pick(0)}>
          + New page
        </button>
      </div>
      <PageEditor
        key={session}
        page={page}
        onSaved={(id) => {
          setSelected(id);
          reload();
        }}
        onDeleted={() => {
          pick(pages[0]?.id ?? 0);
          reload();
        }}
      />
    </div>
  );
}

function PageEditor({ page, onSaved, onDeleted }: { page: SitePage; onSaved: (id: number) => void; onDeleted: () => void }) {
  const [p, setP] = useState(page);
  const [saved, setSaved] = useState(false);
  const { busy, error, run } = useAction();
  const set = (patch: Partial<SitePage>) => {
    setSaved(false);
    setP((x) => ({ ...x, ...patch }));
  };
  const fieldError = (f: string) => (error?.field === f ? error.message : undefined);
  const save = async () => {
    const r = await run(() => contentApi.savePage(p));
    if (r) {
      setSaved(true);
      setP((x) => ({ ...x, id: r.id }));
      onSaved(r.id);
    }
  };
  const remove = async () => {
    if (!window.confirm(`Delete the page "${p.title}"? This can't be undone.`)) return;
    if ((await run(() => contentApi.deletePage(p.id).then(() => true))) === true) onDeleted();
  };
  return (
    <div className="stack">
      <Panel
        title={p.id ? p.title || "Page" : "New page"}
        actions={
          <div className="row">
            {p.id !== 0 && (
              <button className="btn" disabled={busy} onClick={remove}>
                Delete
              </button>
            )}
            <button className="btn primary" disabled={busy} onClick={save}>
              {busy ? "Saving…" : "Save page"}
            </button>
          </div>
        }
      >
        <SavedBanner show={saved}>{p.published ? "Page saved and live in the shop's footer." : "Draft saved. Tick Published when it's ready."}</SavedBanner>
        <ErrorBanner error={error && !error.field ? error : null} />
        <div className="grid-2">
          <Field label="Title" htmlFor="pg-title" error={fieldError("title")}>
            <input id="pg-title" value={p.title} onChange={(e) => set({ title: e.target.value })} />
          </Field>
          <Field label="Web address" htmlFor="pg-slug" hint={`Shows at /page/${p.slug || "…"}`} error={fieldError("slug")}>
            <input id="pg-slug" value={p.slug} onChange={(e) => set({ slug: e.target.value })} />
          </Field>
        </div>
        <label className="check" htmlFor="pg-pub">
          <input id="pg-pub" type="checkbox" checked={p.published} onChange={(e) => set({ published: e.target.checked })} />
          Published (shown in the shop's footer)
        </label>
        <Field label="Page text" htmlFor="pg-body" hint="## for a heading, - for a list, **bold**, [link text](/shop)" error={fieldError("body")}>
          <textarea id="pg-body" rows={16} value={p.body} onChange={(e) => set({ body: e.target.value })} />
        </Field>
      </Panel>
      {p.body.trim() && (
        <Panel title="Preview">
          <Markdown source={p.body} className="page-preview" />
        </Panel>
      )}
    </div>
  );
}
