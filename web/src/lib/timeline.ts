import type { TimeEntry, WorkingDay } from '../api/types';
import { addDays } from './format';

/** The part of a time entry that falls on one day of the week. */
export interface Segment {
  entry: TimeEntry;
  day: number; // 0 = Monday
  start: number; // minutes since local midnight
  end: number;
}

/**
 * Splits entries into per-day segments for the week starting at weekStart
 * (local midnight of a Monday). Running entries end at now.
 */
export function segments(entries: TimeEntry[], weekStart: Date, now: number): Segment[] {
  const out: Segment[] = [];
  for (const e of entries) {
    const from = new Date(e.started_at).getTime();
    const to = e.stopped_at ? new Date(e.stopped_at).getTime() : now;
    for (let day = 0; day < 7; day++) {
      const d0 = addDays(weekStart, day).getTime();
      const d1 = addDays(weekStart, day + 1).getTime();
      const s = Math.max(from, d0);
      const t = Math.min(to, d1);
      if (s < t) out.push({ entry: e, day, start: (s - d0) / 60000, end: (t - d0) / 60000 });
    }
  }
  return out;
}

/**
 * The hours shown on the axis: whole hours covering the segments and the
 * working hours, or 8–18 when there is neither.
 */
export function hourRange(segs: Segment[], hours: WorkingDay[]): [number, number] {
  const starts = [...segs.map((s) => s.start), ...hours.map((h) => h.start_min)];
  const ends = [...segs.map((s) => s.end), ...hours.map((h) => h.end_min)];
  if (starts.length === 0) return [8, 18];
  return [Math.floor(Math.min(...starts) / 60), Math.ceil(Math.max(...ends) / 60)];
}
