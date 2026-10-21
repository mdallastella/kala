/** Local calendar date as YYYY-MM-DD. */
export function isoDate(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}

export function addDays(d: Date, n: number): Date {
  const c = new Date(d);
  c.setDate(c.getDate() + n);
  return c;
}

/** Local midnight of the Monday on or before d. */
export function startOfWeek(d: Date): Date {
  const c = new Date(d.getFullYear(), d.getMonth(), d.getDate());
  c.setDate(c.getDate() - ((c.getDay() + 6) % 7));
  return c;
}

/** The days shown for month's month: whole Monday–Sunday weeks covering it. */
export function monthGrid(month: Date): Date[] {
  const first = new Date(month.getFullYear(), month.getMonth(), 1);
  const last = new Date(month.getFullYear(), month.getMonth() + 1, 0);
  const days: Date[] = [];
  for (let d = startOfWeek(first); d <= last || days.length % 7 !== 0; d = addDays(d, 1)) days.push(d);
  return days;
}

export function parseDate(s: string): Date {
  const [y, m, d] = s.split('-').map(Number);
  return new Date(y, m - 1, d);
}

/** Human label for a due date relative to today. */
export function dueLabel(s: string, today = new Date()): string {
  const t = isoDate(today);
  if (s === t) return 'Today';
  if (s === isoDate(addDays(today, 1))) return 'Tomorrow';
  if (s === isoDate(addDays(today, -1))) return 'Yesterday';
  const d = parseDate(s);
  const diff = Math.round((d.getTime() - parseDate(t).getTime()) / 86400000);
  const opts: Intl.DateTimeFormatOptions =
    diff > 0 && diff < 7 ? { weekday: 'long' } : { weekday: 'short', day: 'numeric', month: 'short' };
  if (d.getFullYear() !== today.getFullYear()) opts.year = 'numeric';
  return d.toLocaleDateString(undefined, opts);
}

export function isOverdue(s: string | null, today = new Date()): boolean {
  return s != null && s < isoDate(today);
}

export function dateTimeLabel(iso: string): string {
  const d = new Date(iso);
  const day = dueLabel(isoDate(d));
  return `${day} ${d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })}`;
}

export function timeLabel(iso: string): string {
  return new Date(iso).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
}

/** Formats seconds as 1h 05m / 12m / 45s. */
export function duration(secs: number): string {
  secs = Math.max(0, Math.floor(secs));
  const h = Math.floor(secs / 3600);
  const m = Math.floor((secs % 3600) / 60);
  if (h > 0) return `${h}h ${String(m).padStart(2, '0')}m`;
  if (m > 0) return `${m}m`;
  return `${secs}s`;
}

/** Formats seconds as H:MM:SS for live timers. */
export function clock(secs: number): string {
  secs = Math.max(0, Math.floor(secs));
  const h = Math.floor(secs / 3600);
  const m = Math.floor((secs % 3600) / 60);
  const s = secs % 60;
  return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
}

/** Value for <input type="datetime-local"> from an ISO instant. */
export function toLocalInput(iso: string | null): string {
  if (!iso) return '';
  const d = new Date(iso);
  return `${isoDate(d)}T${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

/** ISO instant from an <input type="datetime-local"> value. */
export function fromLocalInput(v: string): string | null {
  return v ? new Date(v).toISOString() : null;
}

export function minutesToClock(min: number): string {
  return `${String(Math.floor(min / 60)).padStart(2, '0')}:${String(min % 60).padStart(2, '0')}`;
}

export function clockToMinutes(v: string): number {
  const [h, m] = v.split(':').map(Number);
  return h * 60 + m;
}
