import { Link } from "react-router-dom";
import { bannerUrl, formatNaira } from "@yetibyrash/ui";
import { api, type Home, type HomeSection, type PriceBand, type Tier } from "../api";
import { ProductGrid } from "../components/ProductCard";
import { ErrorMessage, Loading } from "../components/Status";
import { TierSwitch } from "../components/TierSwitch";
import { useTier } from "../tier";
import { useApi } from "../useApi";

// The homepage draws the sections the admin set, in their order (Admin › Homepage and content).
export function HomePage() {
  const { tier, setTier } = useTier();
  const { data, error, loading } = useApi(`home:${tier}`, (s) => api.home(tier, s));

  return (
    <div className="stack">
      <TierSwitch value={tier} onChange={(t) => setTier(t || "everyday")} />
      {error && <ErrorMessage error={error} />}
      {loading && !data && <Loading />}
      {data?.sections.map((s) => <Section key={s.key} section={s} home={data} tier={tier} />)}
    </div>
  );
}

function Section({ section, home, tier }: { section: HomeSection; home: Home; tier: Tier }) {
  switch (section.key) {
    case "hero":
      return <Hero home={home} tier={tier} />;
    case "categories":
      if (home.categories.length === 0) return null;
      return (
        <section className="stack-sm">
          <h2>{section.title}</h2>
          <div className="cat-row">
            {home.categories.map((c) => (
              <Link key={c.id} to={`/shop/${c.slug}`} className="cat">
                <span className="cat-dot" aria-hidden="true">
                  {c.name.charAt(0)}
                </span>
                {c.name}
              </Link>
            ))}
          </div>
        </section>
      );
    case "new_in":
      return (
        <section className="stack-sm">
          <div className="row-between">
            <h2>{section.title}</h2>
            <Link to={`/shop?tier=${tier}`}>See all</Link>
          </div>
          {home.newIn.length === 0 ? <p className="muted">Nothing new in this range yet.</p> : <ProductGrid items={home.newIn} />}
        </section>
      );
    case "best_sellers":
      if (home.bestSellers.length === 0) return null; // nothing sold yet
      return (
        <section className="stack-sm">
          <h2>{section.title}</h2>
          <ProductGrid items={home.bestSellers} />
        </section>
      );
    case "gift_budget":
      if (home.budgets.length === 0) return null;
      return (
        <section className="stack-sm">
          <h2>{section.title}</h2>
          <div className="budget-row">
            {home.budgets.map((b) => (
              <Link key={b.id} to={`/shop?tier=${tier}&band=${b.id}`} className="budget">
                {budgetLabel(b)}
              </Link>
            ))}
          </div>
        </section>
      );
    case "reviews":
      if (home.reviews.length === 0) return null;
      return (
        <section className="stack-sm">
          <h2>{section.title}</h2>
          <div className="review-strip">
            {home.reviews.map((r) => (
              <figure key={r.id} className="review-quote">
                <span className="stars small" aria-label="5 out of 5 stars">
                  ★★★★★
                </span>
                <blockquote>{r.text}</blockquote>
                <figcaption className="small muted">
                  {r.author}
                  {r.area && ` · ${r.area}`} · <Link to={`/p/${r.productSlug}`}>{r.product}</Link>
                </figcaption>
              </figure>
            ))}
          </div>
        </section>
      );
    case "trust":
      if (section.lines.length === 0) return null;
      return (
        <section className="trust" aria-label={section.title} style={{ gridTemplateColumns: `repeat(${Math.min(section.lines.length, 4)}, minmax(0, 1fr))` }}>
          {section.lines.map((l) => (
            <span key={l}>{l}</span>
          ))}
        </section>
      );
  }
  return null;
}

function budgetLabel(b: PriceBand): string {
  if (b.label) return b.label;
  if (b.maxKobo === null) return `${formatNaira(b.minKobo)} and up`;
  return `Under ${formatNaira(b.maxKobo)}`;
}

// The live banner for this tier, or the wordmark hero when there isn't one.
function Hero({ home, tier }: { home: Home; tier: Tier }) {
  const b = home.hero;
  if (!b) {
    return (
      <section className="hero">
        <span className="hero-y" aria-hidden="true">
          Y
        </span>
        <span className="eyebrow">{tier === "premium" ? "Premium" : "Everyday"} · New in</span>
        <h1>{home.tagline}</h1>
        <Link to={`/shop?tier=${tier}`} className="btn primary small">
          Shop the edit
        </Link>
      </section>
    );
  }
  return (
    <section className={b.image ? "hero has-photo" : "hero"}>
      {b.image ? (
        <picture className="hero-photo">
          <source media="(min-width: 720px)" srcSet={bannerUrl(b.image, "wide")} />
          <img src={bannerUrl(b.image, "phone")} alt="" />
        </picture>
      ) : (
        <span className="hero-y" aria-hidden="true">
          Y
        </span>
      )}
      {b.eyebrow && <span className="eyebrow">{b.eyebrow}</span>}
      <h1>{b.headline}</h1>
      <Link to={b.buttonLink} className="btn primary small">
        {b.buttonText}
      </Link>
    </section>
  );
}
