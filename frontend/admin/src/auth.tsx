import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from "react";
import { api, setSignedOutHandler, type Me, type Permission } from "./api";

type Auth = {
  me: Me | null;
  checking: boolean;
  /** True when a sign-in ended while the admin was open (e.g. 8 hours without use). */
  ended: boolean;
  signIn: (me: Me) => void;
  signOut: () => Promise<void>;
  can: (p: Permission) => boolean;
};

const AuthContext = createContext<Auth | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [me, setMe] = useState<Me | null>(null);
  const [checking, setChecking] = useState(true);
  const [ended, setEnded] = useState(false);
  const signedIn = useRef(false);
  signedIn.current = me !== null;

  useEffect(() => {
    // Any 401 from the API means the session ended: show the sign-in page, and say why
    // if someone was signed in (rather than just opening the admin).
    setSignedOutHandler(() => {
      if (signedIn.current) setEnded(true);
      setMe(null);
    });
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
    ended,
    signIn: (m) => {
      setEnded(false);
      setMe(m);
    },
    signOut: async () => {
      await api.logout().catch(() => {});
      signedIn.current = false; // signing out on purpose: no "you were signed out" note
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
