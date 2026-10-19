import type { Money } from './api';

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const MONTHS_LONG = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];

function groupThousands(n: number): string {
  return n.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
}

/** 342080 → "3 420.80" */
export function amount(minor: number): string {
  const abs = Math.abs(minor);
  return `${groupThousands(Math.floor(abs / 100))}.${String(abs % 100).padStart(2, '0')}`;
}

export function symbol(currency: string): string {
  return currency === 'USD' ? '$' : currency === 'EUR' ? '€' : '₼';
}

/** Money with sign and currency: "-14.90 ₼", "+1 800.00 ₼", "$765.00". */
export function money(m: Money | null | undefined, opts: { sign?: boolean } = {}): string {
  if (!m) return '—';
  const sign = m.amountMinor < 0 ? '-' : opts.sign && m.amountMinor > 0 ? '+' : '';
  if (m.currency === 'USD' || m.currency === 'EUR') return `${sign}${symbol(m.currency)}${amount(m.amountMinor)}`;
  return `${sign}${amount(m.amountMinor)} ₼`;
}

function parse(d: string): Date {
  const [y, m, day] = d.slice(0, 10).split('-').map(Number);
  return new Date(y, m - 1, day);
}

/** "2026-10-19" → "19 Oct" */
export function shortDate(d: string): string {
  const x = parse(d);
  return `${x.getDate()} ${MONTHS[x.getMonth()]}`;
}

/** "2026-10-19" → "19 Oct 2026" */
export function longDate(d: string): string {
  const x = parse(d);
  return `${x.getDate()} ${MONTHS[x.getMonth()]} ${x.getFullYear()}`;
}

export function monthName(d: string): string {
  return MONTHS_LONG[parse(d).getMonth()];
}

export function todayISO(): string {
  const n = new Date();
  return `${n.getFullYear()}-${String(n.getMonth() + 1).padStart(2, '0')}-${String(n.getDate()).padStart(2, '0')}`;
}

export function daysFromToday(d: string): number {
  return Math.round((parse(d).getTime() - parse(todayISO()).getTime()) / 86400000);
}

/** "Today · 19 Oct", "18 Oct" */
export function dayHeader(d: string): string {
  return daysFromToday(d) === 0 ? `Today · ${shortDate(d)}` : shortDate(d);
}

/** "Updated 2 h ago" from an ISO timestamp. */
export function updatedAgo(ts: string | null | undefined): string {
  if (!ts) return 'Not synced yet';
  const mins = Math.max(0, Math.round((Date.now() - new Date(ts).getTime()) / 60000));
  if (mins < 1) return 'Updated just now';
  if (mins < 60) return `Updated ${mins} min ago`;
  const h = Math.round(mins / 60);
  if (h < 24) return `Updated ${h} h ago`;
  return `Updated ${Math.round(h / 24)} d ago`;
}

export function timeLabel(ts: string): string {
  const d = new Date(ts);
  const days = Math.round((parse(todayISO()).getTime() - new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()) / 86400000);
  const hm = `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
  if (days === 0) return `Today, ${hm}`;
  if (days === 1) return 'Yesterday';
  return `${d.getDate()} ${MONTHS[d.getMonth()]}`;
}

export function periodLabel(p: string): string {
  return p === 'week' ? 'Weekly' : p === 'year' ? 'Yearly' : 'Monthly';
}

/** "4169********4812" → "•• 4812" */
export function last4(pan: string | null): string {
  return pan ? `•• ${pan.slice(-4)}` : '';
}

/** "AZ00MABB00000000000000000001" → "AZ00 MABB ... 0001" */
export function shortIban(iban: string): string {
  return `${iban.slice(0, 4)} ${iban.slice(4, 8)} ... ${iban.slice(-4)}`;
}
