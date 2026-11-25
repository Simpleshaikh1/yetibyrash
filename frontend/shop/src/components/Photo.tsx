import type { ReactNode } from "react";
import { photoUrl } from "@yetibyrash/ui";

// Square photo on the soft blush ground (design system: every product photo sits on surface-soft).
// Products without a photo yet show their initial instead.
export function Photo({ name, imageKey, large, children }: { name: string; imageKey?: string; large?: boolean; children?: ReactNode }) {
  return (
    <span className={large ? "photo photo-large" : "photo"}>
      {imageKey ? (
        <img
          src={photoUrl(imageKey, large ? 1600 : 800)}
          srcSet={large ? `${photoUrl(imageKey, 800)} 800w, ${photoUrl(imageKey, 1600)} 1600w` : `${photoUrl(imageKey, 400)} 400w, ${photoUrl(imageKey, 800)} 800w`}
          sizes={large ? "(max-width: 560px) 100vw, 560px" : "(max-width: 560px) 50vw, 280px"}
          alt={name}
          loading={large ? "eager" : "lazy"}
        />
      ) : (
        <span className="photo-initial" role="img" aria-label={`${name}, photo coming soon`}>
          {name.charAt(0)}
        </span>
      )}
      {children}
    </span>
  );
}
