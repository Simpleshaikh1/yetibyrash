import { useState } from "react";
import { useParams } from "react-router-dom";
import { photoUrl } from "@yetibyrash/ui";
import { api, ApiError, type ReviewItem } from "../api";
import { Photo } from "../components/Photo";
import { ErrorMessage, Loading } from "../components/Status";
import { useApi } from "../useApi";

// Opened from the "how did you like it?" message: rate each item, add words and photos.
export function ReviewPage() {
  const { token = "" } = useParams();
  const req = useApi(`review:${token}`, (s) => api.reviewRequest(token, s));
  if (req.error) return <ErrorMessage error={req.error} />;
  if (!req.data) return <Loading />;
  const r = req.data;
  const done = r.items.every((i) => i.review);
  return (
    <div className="stack narrow-page">
      <div className="stack-xs">
        <span className="eyebrow">Order {r.orderNumber}</span>
        <h1 className="page-title">{r.firstName ? `How are you finding them, ${r.firstName}?` : "How are you finding them?"}</h1>
        <p className="muted">Your review helps other shoppers. We read every one before it goes on the shop.</p>
      </div>
      {r.items.map((it) => (
        <ItemForm key={it.orderItemId} token={token} item={it} authorName={r.authorName} reload={req.reload} />
      ))}
      {done && <p className="nudge">Thank you! Your reviews will show on the shop once we've checked them.</p>}
    </div>
  );
}

function ItemForm({ token, item, authorName, reload }: { token: string; item: ReviewItem; authorName: string; reload: () => void }) {
  const [rating, setRating] = useState(0);
  const [text, setText] = useState("");
  const [name, setName] = useState(authorName);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const id = `rv-${item.orderItemId}`;

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      await api.submitReview(token, { orderItemId: item.orderItemId, rating, text, name });
      reload();
    } catch (e) {
      setError(e instanceof ApiError ? e : null);
    } finally {
      setBusy(false);
    }
  };
  const addPhoto = async (file: File) => {
    if (!item.review) return;
    setBusy(true);
    setError(null);
    try {
      await api.reviewPhoto(token, item.review.id, file);
      reload();
    } catch (e) {
      setError(e instanceof ApiError ? e : null);
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="form-card review-item">
      <div className="row-start">
        <span className="review-thumb">
          <Photo name={item.name} imageKey={item.image || undefined} />
        </span>
        <div className="stack-xs">
          <b>{item.name}</b>
          {item.option && <span className="small muted">{item.option}</span>}
        </div>
      </div>

      {item.review ? (
        <div className="stack-sm">
          <p className="small">
            <span className="stars small">{"★".repeat(item.review.rating)}</span> Thank you for your review!{" "}
            {item.review.status === "waiting" ? "It shows on the shop once we've checked it." : ""}
          </p>
          {item.review.status === "waiting" && (
            <div className="row-start wrap">
              {item.review.photos.map((p) => (
                <img key={p} src={photoUrl(p, 400)} alt="" className="review-photo" />
              ))}
              {item.review.photos.length < 3 && (
                <>
                  <label className="btn outline small" htmlFor={`${id}-photo`}>
                    {busy ? "Adding…" : "+ Add a photo"}
                  </label>
                  <input
                    id={`${id}-photo`}
                    type="file"
                    accept="image/jpeg,image/png,image/webp"
                    className="visually-hidden"
                    onChange={(e) => {
                      const f = e.target.files?.[0];
                      e.target.value = "";
                      if (f) void addPhoto(f);
                    }}
                  />
                </>
              )}
            </div>
          )}
        </div>
      ) : (
        <div className="stack-sm">
          <div className="star-picker" role="radiogroup" aria-label="Your rating">
            {[1, 2, 3, 4, 5].map((n) => (
              <button key={n} type="button" role="radio" aria-checked={rating === n} aria-label={`${n} ${n === 1 ? "star" : "stars"}`} className={n <= rating ? "on" : ""} onClick={() => setRating(n)}>
                ★
              </button>
            ))}
          </div>
          <div className="field">
            <label htmlFor={`${id}-text`}>Your review (optional)</label>
            <textarea id={`${id}-text`} rows={4} maxLength={1000} placeholder="How's the fit, the feel, the quality?" value={text} onChange={(e) => setText(e.target.value)} />
          </div>
          <div className="field">
            <label htmlFor={`${id}-name`}>Show my name as</label>
            <input id={`${id}-name`} value={name} maxLength={40} onChange={(e) => setName(e.target.value)} />
          </div>
          {error && (
            <span className="field-error" role="alert">
              {error.message}
            </span>
          )}
          <button className="btn primary" disabled={busy || rating === 0} onClick={submit}>
            {busy ? "Sending…" : "Send review"}
          </button>
          <span className="small muted">You can add photos after sending.</span>
        </div>
      )}
      {item.review && error && (
        <span className="field-error" role="alert">
          {error.message}
        </span>
      )}
    </section>
  );
}
