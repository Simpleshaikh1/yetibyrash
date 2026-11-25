import type { Tier } from "../api";

type Option = { value: Tier | ""; label: string };

// The Everyday / Premium switch (design system › TierSwitch).
// The shop page adds an "All" option; the homepage uses just the two tiers.
export function TierSwitch({ value, onChange, withAll }: { value: Tier | ""; onChange: (t: Tier | "") => void; withAll?: boolean }) {
  const options: Option[] = [
    ...(withAll ? [{ value: "" as const, label: "All" }] : []),
    { value: "everyday", label: "Everyday" },
    { value: "premium", label: "Premium" },
  ];
  return (
    <div className="tierswitch" role="tablist" aria-label="Price range" style={{ gridTemplateColumns: `repeat(${options.length}, minmax(0, 1fr))` }}>
      {options.map((o) => (
        <button key={o.label} role="tab" aria-selected={value === o.value} className={value === o.value ? "on" : ""} onClick={() => onChange(o.value)}>
          {o.label}
        </button>
      ))}
    </div>
  );
}
