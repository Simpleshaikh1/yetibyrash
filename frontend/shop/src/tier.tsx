import { createContext, useContext, useState, type ReactNode } from "react";
import type { Tier } from "./api";

// The Everyday / Premium choice on the homepage, remembered on this phone.
const KEY = "yetibyrash.tier";

function readTier(): Tier {
  try {
    return localStorage.getItem(KEY) === "premium" ? "premium" : "everyday";
  } catch {
    return "everyday";
  }
}

const TierContext = createContext<{ tier: Tier; setTier: (t: Tier) => void }>({
  tier: "everyday",
  setTier: () => {},
});

export function TierProvider({ children }: { children: ReactNode }) {
  const [tier, setTierState] = useState<Tier>(readTier);
  const setTier = (t: Tier) => {
    setTierState(t);
    try {
      localStorage.setItem(KEY, t);
    } catch {
      // private browsing: the choice just isn't remembered
    }
  };
  return <TierContext.Provider value={{ tier, setTier }}>{children}</TierContext.Provider>;
}

export const useTier = () => useContext(TierContext);
