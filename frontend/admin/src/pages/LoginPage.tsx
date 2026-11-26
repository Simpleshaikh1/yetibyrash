import { useState, type FormEvent } from "react";
import { api } from "../api";
import { useAuth } from "../auth";
import { useAction } from "../useApi";

export function LoginPage() {
  const { signIn, ended } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const { busy, error, run } = useAction();

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const me = await run(() => api.login(email, password));
    if (me) signIn(me);
  };

  return (
    <div className="login">
      <form className="login-card" onSubmit={submit}>
        <span className="wordmark dark">Yetibyrash</span>
        <h1>Staff sign-in</h1>
        {ended && (
          <p className="banner warn" role="status">
            You were signed out because the admin wasn't used for a while. Please sign in again.
          </p>
        )}
        <div className="field">
          <label htmlFor="login-email">Email</label>
          <input id="login-email" type="email" autoComplete="username" required value={email} onChange={(e) => setEmail(e.target.value)} />
        </div>
        <div className="field">
          <label htmlFor="login-password">Password</label>
          <input id="login-password" type="password" autoComplete="current-password" required value={password} onChange={(e) => setPassword(e.target.value)} />
        </div>
        {error && (
          <p className="field-error" role="alert">
            {error.message}
          </p>
        )}
        <button className="btn primary" type="submit" disabled={busy}>
          {busy ? "Signing in…" : "Sign in"}
        </button>
        <p className="hint">Forgot your password? Ask the shop owner to set a new one on the Staff screen.</p>
      </form>
    </div>
  );
}
