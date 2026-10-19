import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import { onSession, post, refreshSession, setSession } from '../api/client';
import type { Session, User } from '../api/types';

interface AuthState {
  user: User | null;
  ready: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

const Ctx = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    const off = onSession((s) => setUser(s?.user ?? null));
    void refreshSession().finally(() => setReady(true));
    return off;
  }, []);

  const value: AuthState = {
    user,
    ready,
    async login(email, password) {
      setSession(await post<Session>('/auth/login', { email, password }));
    },
    async register(email, password) {
      const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
      setSession(await post<Session>('/auth/register', { email, password, timezone }));
    },
    async logout() {
      try {
        await post('/auth/logout');
      } finally {
        setSession(null);
      }
    },
  };
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useAuth(): AuthState {
  const v = useContext(Ctx);
  if (!v) throw new Error('useAuth outside AuthProvider');
  return v;
}
