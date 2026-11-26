import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import { api, type Customer } from "./api";

type AccountState = {
  /** The signed-in customer, null for a guest, undefined while checking. */
  customer: Customer | null | undefined;
  saved: Set<number>;
  refresh: () => Promise<void>;
  signOut: () => Promise<void>;
  /** Saves or unsaves a product; guests are sent to sign in first. */
  toggleSave: (productId: number) => Promise<void>;
};

const AccountContext = createContext<AccountState>({
  customer: undefined,
  saved: new Set(),
  refresh: async () => {},
  signOut: async () => {},
  toggleSave: async () => {},
});

// Accounts are optional: guests shop and check out as before.
export function AccountProvider({ children }: { children: ReactNode }) {
  const [customer, setCustomer] = useState<Customer | null | undefined>(undefined);
  const [saved, setSaved] = useState<Set<number>>(new Set());
  const navigate = useNavigate();

  const refresh = useCallback(async () => {
    try {
      const me = await api.me();
      setCustomer(me.customer);
      setSaved(new Set(me.saved));
    } catch {
      setCustomer((c) => (c === undefined ? null : c)); // offline: treat as a guest
    }
  }, []);
  useEffect(() => {
    void refresh();
  }, [refresh]);

  const signOut = useCallback(async () => {
    await api.signOut().catch(() => {});
    setCustomer(null);
    setSaved(new Set());
  }, []);

  const toggleSave = useCallback(
    async (productId: number) => {
      if (!customer) {
        navigate(`/account?next=${encodeURIComponent(window.location.pathname)}`);
        return;
      }
      const isSaved = saved.has(productId);
      setSaved((s) => {
        const n = new Set(s);
        if (isSaved) n.delete(productId);
        else n.add(productId);
        return n;
      });
      try {
        await (isSaved ? api.unsave(productId) : api.save(productId));
      } catch {
        void refresh(); // put it back the way the server has it
      }
    },
    [customer, saved, navigate, refresh],
  );

  return <AccountContext.Provider value={{ customer, saved, refresh, signOut, toggleSave }}>{children}</AccountContext.Provider>;
}

export const useAccount = () => useContext(AccountContext);
