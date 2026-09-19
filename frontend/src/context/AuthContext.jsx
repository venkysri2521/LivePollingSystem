import { createContext, useContext, useEffect, useMemo, useState } from "react";
import { api, tokenStore } from "../api/client.js";

const AuthContext = createContext(null);

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [ready, setReady] = useState(false);

  // A stored token is only trusted once the server confirms it. That keeps an
  // expired or revoked token from rendering a signed-in shell.
  useEffect(() => {
    if (!tokenStore.get()) {
      setReady(true);
      return;
    }
    api
      .me()
      .then((data) => setUser(data.user))
      .catch(() => tokenStore.clear())
      .finally(() => setReady(true));
  }, []);

  const value = useMemo(
    () => ({
      user,
      ready,
      async signIn(credentials) {
        const data = await api.login(credentials);
        tokenStore.set(data.token);
        setUser(data.user);
      },
      async register(details) {
        const data = await api.signup(details);
        tokenStore.set(data.token);
        setUser(data.user);
      },
      signOut() {
        tokenStore.clear();
        setUser(null);
      },
    }),
    [user, ready]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside AuthProvider");
  return ctx;
}
