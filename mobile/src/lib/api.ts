import Constants from 'expo-constants';

// Cibim backend (App API Contract v0.3).
export const API_URL: string =
  process.env.EXPO_PUBLIC_API_URL ??
  (Constants.expoConfig?.extra as { apiUrl?: string } | undefined)?.apiUrl ??
  'https://cibim.farahmobilya.az/api/v1';

let token: string | null = null;
let lang: 'az' | 'en' = 'en';

export function setToken(t: string | null) {
  token = t;
}
export function setLanguage(l: 'az' | 'en') {
  lang = l;
}
export function getLanguage() {
  return lang;
}

export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string) {
    super(message);
  }
}

export async function api<T = any>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(API_URL + path, {
    method,
    headers: {
      'Content-Type': 'application/json',
      'Accept-Language': lang,
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  const data = text ? JSON.parse(text) : undefined;
  if (!res.ok) {
    throw new ApiError(res.status, data?.error?.code ?? 'ERROR', data?.error?.message ?? `HTTP ${res.status}`);
  }
  return data as T;
}

// ---- Shared shapes ----

export type Money = { amountMinor: number; currency: string };
export type Bank = { code: string; name: string; logoUrl: string | null };
export type Category = { id: string; key: string; name: string; icon: string; kind: string };
export type Account = {
  id: string;
  bank: Bank;
  name: string;
  type: 'CARD' | 'CURRENT' | 'SAVINGS';
  maskedPan: string | null;
  iban: string;
  balance: Money;
  balanceAzn: Money;
  connectionId: string;
};
export type Transaction = {
  id: string;
  accountId: string;
  bank: Bank;
  date: string;
  status: 'booked' | 'pending';
  amount: Money;
  amountAzn: Money;
  merchant: { name: string; logoUrl: string | null };
  rawDescription: string;
  category: Category;
  isInternalTransfer: boolean;
  recurringId: string | null;
  loanId: string | null;
};
export type Alert = {
  id: string;
  type: string;
  severity: string;
  createdAt: string;
  read: boolean;
  params: Record<string, any>;
  fallbackText: { title: string; body: string };
  action: { screen: string; targetId: string | null };
};
export type Recurring = {
  id: string;
  type: 'income' | 'expense';
  name: string;
  merchant: { name: string };
  category: Category;
  cadence: string;
  typicalAmount: Money;
  lastDate: string | null;
  nextDate: string;
  status: 'active' | 'late' | 'changed';
  source: string;
  confirmed: boolean;
};
export type Connection = {
  id: string;
  bank: Bank;
  status: string;
  validUntil: string | null;
  daysLeft: number | null;
  lastSyncedAt: string | null;
  accountsCount: number;
};
export type Budget = {
  id: string;
  category: Category | null;
  period: 'week' | 'month' | 'year';
  limit: Money;
  spent: Money;
  alreadySpent: Money;
  pct: number;
  status: 'ok' | 'warning' | 'over';
  basis: 'user' | 'baseline';
  dataSince: string | null;
};
export type Home = {
  onboarding: boolean;
  dataUpdatedAt: string | null;
  totalBalance: Money | null;
  accounts: Account[];
  spending: { today: Money; thisWeek: Money; thisMonth: Money } | null;
  safeToSpend: { today: Money; nextIncome: { name: string; date: string; amount: Money } | null } | null;
  streak: { current: number; best: number; today: string } | null;
  alerts: Alert[];
  upcoming: { recurringId: string; name: string; type: string; date: string; amount: Money }[];
  recentTransactions: Transaction[];
};
export type User = { id: string; name: string; language: 'az' | 'en'; profile: { householder: boolean; schoolAgeChildren: boolean } };
