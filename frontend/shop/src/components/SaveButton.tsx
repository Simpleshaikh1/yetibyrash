import { useAccount } from "../account";
import { Icon } from "./Icon";

// The heart: saves a product to the account (guests are asked to sign in).
export function SaveButton({ productId, name, className = "save-btn" }: { productId: number; name: string; className?: string }) {
  const { saved, toggleSave } = useAccount();
  const on = saved.has(productId);
  return (
    <button
      type="button"
      className={on ? `${className} on` : className}
      aria-pressed={on}
      aria-label={on ? `Remove ${name} from saved items` : `Save ${name}`}
      onClick={(e) => {
        e.preventDefault();
        e.stopPropagation();
        void toggleSave(productId);
      }}
    >
      <Icon name="heart" size={20} />
    </button>
  );
}

// Stars for a rating, e.g. ★★★★☆ with a label for screen readers.
export function Stars({ value, size = "small" }: { value: number; size?: "small" | "large" }) {
  const full = Math.round(value);
  return (
    <span className={`stars ${size}`} role="img" aria-label={`${value.toFixed(1)} out of 5 stars`}>
      {"★★★★★".slice(0, full)}
      <span className="stars-off">{"★★★★★".slice(full)}</span>
    </span>
  );
}
