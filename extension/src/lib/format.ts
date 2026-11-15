// Date labels ported from web/src/lib/format.ts; keep them in step.

// Which calendar day an instant falls on is decided in the profile timezone, like the server's
// filters and digests. Times of day are shown in the browser's.
let zone: string | undefined;
const zoneFormats = new Map<string, Intl.DateTimeFormat>();

/** Sets the profile timezone; undefined falls back to the browser's. */
export function setZone(tz: string | undefined) {
  zone = tz;
}

/** Calendar date (YYYY-MM-DD) of an instant in the profile timezone. */
export function dateOf(t: Date | string | number): string {
  const d = new Date(t);
  if (!zone) return isoDate(d);
  let f = zoneFormats.get(zone);
  if (!f) {
    f = new Intl.DateTimeFormat('en-US', { timeZone: zone, year: 'numeric', month: 'numeric', day: 'numeric' });
    zoneFormats.set(zone, f);
  }
  const p: Record<string, number> = {};
  for (const { type, value } of f.formatToParts(d)) p[type] = Number(value);
  return isoDate(new Date(p.year, p.month - 1, p.day));
}

/** Today in the profile timezone, as a calendar-day Date for isoDate and addDays. */
export function todayDate(now: number = Date.now()): Date {
  return parseDate(dateOf(now));
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

/** Time of day on a 24h clock, plus the day when it isn't today. */
export function timeLabel(iso: string, today = todayDate()): string {
  const d = new Date(iso);
  const time = d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' });
  return dateOf(d) === isoDate(today) ? time : `${dueLabel(dateOf(d), today)} ${time}`;
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

export function dateTimeLabel(iso: string): string {
  const d = new Date(iso);
  const day = dueLabel(dateOf(d));
  return `${day} ${d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' })}`;
}
