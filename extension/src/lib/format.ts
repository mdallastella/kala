// Date labels ported from web/src/lib/format.ts.

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

/** Time of day on a 24h clock, plus the day when it isn't today. */
export function timeLabel(iso: string, today = new Date()): string {
  const d = new Date(iso);
  const time = d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' });
  return isoDate(d) === isoDate(today) ? time : `${dueLabel(isoDate(d), today)} ${time}`;
}

export function dateTimeLabel(iso: string): string {
  const d = new Date(iso);
  const day = dueLabel(isoDate(d));
  return `${day} ${d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' })}`;
}
