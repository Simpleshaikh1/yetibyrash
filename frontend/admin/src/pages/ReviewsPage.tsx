import { useState } from "react";
import { Link } from "react-router-dom";
import { photoUrl } from "@yetibyrash/ui";
import { api, type AdminReview, type ReviewStatus } from "../api";
import { ErrorBanner, Loading, PageHeader, Panel } from "../components/ui";
import { useAction, useApi } from "../useApi";

const tabs: { key: ReviewStatus; label: string }[] = [
  { key: "waiting", label: "Waiting" },
  { key: "published", label: "Published" },
  { key: "hidden", label: "Hidden" },
];

// Design: Admin CMS › Reviews. Only buyers can review; nothing shows until approved.
export function ReviewsPage() {
  const [status, setStatus] = useState<ReviewStatus>("waiting");
  const data = useApi(`reviews:${status}`, (s) => api.reviews(status, s));
  const d = data.data;
  return (
    <>
      <PageHeader title="Reviews" />
      {d && (
        <p className="muted">
          {d.published > 0 ? `★ ${d.average.toFixed(1)} average · ${d.published} published. ` : ""}Only buyers can review. Nothing shows on the shop until you approve it.
        </p>
      )}
      <div className="tabs" role="tablist">
        {tabs.map((t) => (
          <button key={t.key} role="tab" aria-selected={status === t.key} className={status === t.key ? "on" : ""} onClick={() => setStatus(t.key)}>
            {t.label}
            {d && <span className="count-pill">{d[t.key]}</span>}
          </button>
        ))}
      </div>
      <ErrorBanner error={data.error} />
      {!d && !data.error && <Loading />}
      <div className="split reviews-split">
        <div className="stack">
          {d?.reviews.length === 0 && <p className="muted">{status === "waiting" ? "No reviews waiting. New ones show here." : "None here."}</p>}
          {d?.reviews.map((r) => <ReviewCard key={r.id} r={r} reload={data.reload} />)}
        </div>
        <Panel title="Asking for reviews">
          <p className="small">
            Three days after an order is marked <b>Delivered</b>, the customer gets an email and a text asking for a review, with a private link. Signed-in customers can also review from their account.
          </p>
          <p className="small">Buyers rate each item, can write a few words and add up to 3 photos.</p>
          <p className="small">
            Change the wording on <Link to="/messages">Customer messages</Link> › Review request.
          </p>
          <p className="small muted">Published reviews show as stars on product cards, in full on the product page, and 5-star ones on the homepage strip.</p>
        </Panel>
      </div>
    </>
  );
}

function ReviewCard({ r, reload }: { r: AdminReview; reload: () => void }) {
  const [reply, setReply] = useState(r.reply);
  const [replying, setReplying] = useState(false);
  const { busy, error, run } = useAction();
  const setStatus = (s: ReviewStatus) => run(() => api.setReviewStatus(r.id, s).then(() => true)).then((ok) => ok && reload());
  const saveReply = () =>
    run(() => api.replyToReview(r.id, reply).then(() => true)).then((ok) => {
      if (ok) {
        setReplying(false);
        reload();
      }
    });
  return (
    <article className="panel review-card">
      <div className="row-between-admin">
        <span className="stars" aria-label={`${r.rating} out of 5 stars`}>
          {"★".repeat(r.rating)}
          <span className="stars-off">{"★".repeat(5 - r.rating)}</span>
        </span>
        <span className="small muted">{new Date(r.createdAt).toLocaleDateString("en-GB", { day: "numeric", month: "short" })}</span>
      </div>
      <b>{r.product}</b>
      {r.text ? <p className="review-text">{r.text}</p> : <p className="small muted">No words, just stars.</p>}
      {r.photos.length > 0 && (
        <div className="row wrap">
          {r.photos.map((k) => (
            <a key={k} href={photoUrl(k, 1600)} target="_blank" rel="noreferrer">
              <img src={photoUrl(k, 400)} alt={`Photo from ${r.author}`} className="review-photo" />
            </a>
          ))}
        </div>
      )}
      <span className="small muted">
        {r.author}
        {r.area && ` · ${r.area}`}
        {r.option && ` · ${r.option}`} · order {r.orderNumber}
      </span>
      {r.reply && !replying && (
        <p className="small review-reply">
          <b>Your reply:</b> {r.reply}
        </p>
      )}
      {replying && (
        <div className="stack-xs">
          <label htmlFor={`reply-${r.id}`} className="small">
            Public reply (shows under the review)
          </label>
          <textarea id={`reply-${r.id}`} rows={3} value={reply} onChange={(e) => setReply(e.target.value)} />
          <div className="row">
            <button className="btn small primary" disabled={busy} onClick={saveReply}>
              Save reply
            </button>
            <button className="btn small" onClick={() => { setReplying(false); setReply(r.reply); }}>
              Cancel
            </button>
          </div>
        </div>
      )}
      <ErrorBanner error={error} />
      <div className="row wrap">
        {r.status !== "published" && (
          <button className="btn small primary" disabled={busy} onClick={() => setStatus("published")}>
            Approve
          </button>
        )}
        {r.status !== "hidden" && (
          <button className="btn small" disabled={busy} onClick={() => setStatus("hidden")}>
            Hide
          </button>
        )}
        {r.status === "hidden" && (
          <button className="btn small" disabled={busy} onClick={() => setStatus("waiting")}>
            Back to waiting
          </button>
        )}
        {!replying && (
          <button className="btn small" onClick={() => setReplying(true)}>
            {r.reply ? "Edit reply" : "Reply publicly"}
          </button>
        )}
      </div>
    </article>
  );
}
