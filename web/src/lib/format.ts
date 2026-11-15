// Which calendar day an instant falls on is decided in the profile timezone, like the server's
// filters and digests (due:today, the briefing). Times of day are shown in the browser's.
let zone: string | undefined;
const zoneFormats = new Map<string, Intl.DateTimeFormat>();

/** Sets the profile timezone; undefined falls back to the browser's. */
export function setZone(tz: string | undefined) {
  zone = tz;
}

/** The wall clock of an instant in the profile timezone, expressed as a UTC timestamp. */
function wallClock(ms: number): number {
  if (!zone) {
    const d = new Date(ms);
    return Date.UTC(d.getFullYear(), d.getMonth(), d.getDate(), d.getHours(), d.getMinutes(), d.getSeconds());
  }
  let f = zoneFormats.get(zone);
  if (!f) {
    f = new Intl.DateTimeFormat('en-US', { timeZone: zone, hourCycle: 'h23', year: 'numeric', month: 'numeric', day: 'numeric', hour: 'numeric', minute: 'numeric', second: 'numeric' });
    zoneFormats.set(zone, f);
  }
  const p: Record<string, number> = {};
  for (const { type, value } of f.formatToParts(ms)) p[type] = Number(value);
  return Date.UTC(p.year, p.month - 1, p.day, p.hour, p.minute, p.second);
}

/** Calendar date (YYYY-MM-DD) of an instant in the profile timezone. */
export function dateOf(t: Date | string | number): string {
  return new Date(wallClock(new Date(t).getTime())).toISOString().slice(0, 10);
}

/** Today in the profile timezone, as a calendar-day Date for isoDate, addDays and startOfWeek. */
export function todayDate(now: number = Date.now()): Date {
  return parseDate(dateOf(now));
}

/** The instant a calendar day begins in the profile timezone. */
export function startOfDay(day: Date | string): Date {
  const [y, m, d] = (typeof day === 'string' ? day : isoDate(day)).split('-').map(Number);
  if (!zone) return new Date(y, m - 1, d);
  const wall = Date.UTC(y, m - 1, d);
  // The offset depends on the instant, so settle on it in two steps (it changes on DST days).
  let t = wall;
  for (let i = 0; i < 2; i++) t = wall - (wallClock(t) - t);
  return new Date(t);
}

/** Calendar date of a calendar-day Date as YYYY-MM-DD. */
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
export function dueLabel(s: string, today = todayDate()): string {
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

export function isOverdue(s: string | null, today = todayDate()): boolean {
  return s != null && s < isoDate(today);
}

export function dateTimeLabel(iso: string): string {
  const d = new Date(iso);
  const day = dueLabel(dateOf(d));
  return `${day} ${d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' })}`;
}

/** Short relative time of a past instant: "just now", "5m ago", "3h ago", "2d ago". */
export function ago(iso: string, now = Date.now()): string {
  const min = Math.floor((now - new Date(iso).getTime()) / 60000);
  if (min < 1) return 'just now';
  if (min < 60) return `${min}m ago`;
  if (min < 24 * 60) return `${Math.floor(min / 60)}h ago`;
  return `${Math.floor(min / (24 * 60))}d ago`;
}

export function timeLabel(iso: string): string {
  return new Date(iso).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' });
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

/** Minutes since midnight from a 24h H:MM or HH:MM clock, or null when invalid. */
export function parseClock(v: string): number | null {
  const m = v.trim().match(/^([01]?\d|2[0-3]):([0-5]\d)$/);
  return m ? Number(m[1]) * 60 + Number(m[2]) : null;
}

/** Full local date and time, 24h. */
export function fullDateTime(iso: string): string {
  return new Date(iso).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short', hourCycle: 'h23' });
}
