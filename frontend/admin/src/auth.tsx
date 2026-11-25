import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { api, setSignedOutHandler, type Me, type Permission } from "./api";

type Auth = {
  me: Me | null;
  checking: boolean;
  signIn: (me: Me) => void;
  signOut: () => Promise<void>;
  can: (p: Permission) => boolean;
};

const AuthContext = createContext<Auth | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [me, setMe] = useState<Me | null>(null);
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    // Any 401 from the API means the session ended: show the sign-in page.
    setSignedOutHandler(() => setMe(null));
    const ctrl = new AbortController();
    api
      .me(ctrl.signal)
      .then(setMe)
      .catch(() => setMe(null))
      .finally(() => setChecking(false));
    return () => ctrl.abort();
  }, []);

  const value: Auth = {
    me,
    checking,
    signIn: setMe,
    signOut: async () => {
      await api.logout().catch(() => {});
      setMe(null);
    },
    can: (p) => me?.permissions.includes(p) ?? false,
  };
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): Auth {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth outside AuthProvider");
  return ctx;
}
