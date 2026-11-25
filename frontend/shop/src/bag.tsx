import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { api, type Bag } from "./api";

type BagState = {
  bag: Bag | null;
  /** Number of items, for the bag icon. */
  count: number;
  setBag: (b: Bag) => void;
  refresh: () => Promise<void>;
};

const BagContext = createContext<BagState>({ bag: null, count: 0, setBag: () => {}, refresh: async () => {} });

// Keeps the shopper's bag in one place so the header, bottom nav and bag page agree.
export function BagProvider({ children }: { children: ReactNode }) {
  const [bag, setBag] = useState<Bag | null>(null);
  const refresh = useCallback(async () => {
    try {
      setBag(await api.bag());
    } catch {
      // offline: keep what we had
    }
  }, []);
  useEffect(() => {
    void refresh();
  }, [refresh]);
  const count = bag?.lines.reduce((n, l) => n + l.qty, 0) ?? 0;
  return <BagContext.Provider value={{ bag, count, setBag, refresh }}>{children}</BagContext.Provider>;
}

export const useBag = () => useContext(BagContext);
