import { useFocusEffect } from 'expo-router';
import * as Linking from 'expo-linking';
import * as WebBrowser from 'expo-web-browser';
import React, { createContext, useCallback, useContext, useRef, useState } from 'react';
import { api, Connection, setLanguage, setToken, User } from './api';

type Session = {
  user: User | null;
  login: (psuId: string) => Promise<void>;
  logout: () => void;
  setUser: (u: User) => void;
};

const Ctx = createContext<Session>({ user: null, login: async () => {}, logout: () => {}, setUser: () => {} });

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [user, setUserState] = useState<User | null>(null);
  const setUser = (u: User) => {
    setLanguage(u.language);
    setUserState(u);
  };
  const login = async (psuId: string) => {
    const res = await api<{ token: string; user: User }>('POST', '/auth/demo-login', { psuId });
    setToken(res.token);
    // The app's UI is in English; switch the user's server-side texts to match on first login.
    if (res.user.language !== 'en') {
      setUser(await api<User>('PATCH', '/me', { language: 'en' }));
    } else {
      setUser(res.user);
    }
  };
  const logout = () => {
    setToken(null);
    setUserState(null);
  };
  return <Ctx.Provider value={{ user, login, logout, setUser }}>{children}</Ctx.Provider>;
}

export const useSession = () => useContext(Ctx);

/** Loads a GET endpoint, reloads when the screen comes back into focus. */
export function useApi<T>(path: string | null) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const pathRef = useRef(path);
  pathRef.current = path;

  const load = useCallback(async () => {
    if (!pathRef.current) return;
    try {
      const d = await api<T>('GET', pathRef.current);
      setData(d);
      setError(null);
    } catch (e: any) {
      setError(e?.message ?? 'Something went wrong');
    } finally {
      setLoading(false);
    }
  }, []);

  useFocusEffect(
    useCallback(() => {
      load();
    }, [load, path]),
  );

  /** Pull to refresh: sync with the banks, then reload. */
  const refresh = useCallback(async () => {
    setRefreshing(true);
    try {
      await api('POST', '/sync');
    } catch {
      // still reload what we have
    }
    await load();
    setRefreshing(false);
  }, [load]);

  return { data, error, loading, refreshing, reload: load, refresh, setData };
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

/**
 * Links a bank: asks the backend for a consent, opens the bank's approval page,
 * then polls the connection until the approval lands. Returns the final status.
 */
export async function connectBank(bankCode: string, renewId?: string): Promise<string> {
  const redirectUri = Linking.createURL('connected');
  let connectionId = renewId;
  let authUrl: string;
  if (renewId) {
    ({ authUrl } = await api<{ authUrl: string }>('POST', `/connections/${renewId}/renew`, { redirectUri }));
  } else {
    const res = await api<{ connectionId: string; authUrl: string }>('POST', '/connections', { bankCode, redirectUri });
    connectionId = res.connectionId;
    authUrl = res.authUrl;
  }
  await WebBrowser.openAuthSessionAsync(authUrl, redirectUri);
  // Whatever the browser reports, the bank's answer is what counts: poll it.
  let status = 'pending';
  for (let i = 0; i < 8; i++) {
    const c = await api<Connection>('GET', `/connections/${connectionId}`);
    status = c.status;
    if (status !== 'pending' || (renewId && i > 1)) break;
    await sleep(1000);
  }
  return status;
}
