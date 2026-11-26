import { afterEach, describe, expect, it } from 'vitest';
import type { Tag } from '../api/types';
import { ago, clock, dateOf, dueLabel, duration, fullDateTime, isoDate, isOverdue, monthGrid, parseClock, setZone, startOfDay, timeLabel, todayDate } from './format';
import { buildTree, chipTags, projectFromQuery, tagColor, tagQuery, withNamespaceValues } from './tags';
import { completeToken, currentToken, suggestions } from './quickadd';
import { hourRange, segments } from './timeline';

const tag = (name: string, open = 1, color: string | null = null): Tag => ({ id: 0, name, color, pinned: false, position: 0, open, total: open });

describe('tags', () => {
  it('builds a tree with virtual parents and subtree counts', () => {
    const tree = buildTree([tag('project:kala:api', 2), tag('project:kala', 1), tag('project:infra', 3), tag('urgent')]);
    const project = tree.find((n) => n.name === 'project')!;
    expect(project.tag).toBeUndefined();
    expect(project.open).toBe(6);
    const kala = project.children.find((n) => n.label === 'kala')!;
    expect(kala.open).toBe(3);
    expect(kala.children[0].name).toBe('project:kala:api');
  });

  it('adds unused namespace values and orders by value list', () => {
    const [root] = buildTree([tag('status:closed', 1), tag('status:zzz', 1), tag('status:open', 2)]);
    const node = withNamespaceValues(root, { name: 'status', values: [{ value: 'open' }, { value: 'pending' }, { value: 'closed' }] });
    expect(node.children.map((c) => c.label)).toEqual(['open', 'pending', 'closed', 'zzz']);
    expect(node.children[1].tag).toBeUndefined();
    expect(node.children[1].open).toBe(0);
    expect(root.children).toHaveLength(3);
    expect(withNamespaceValues(undefined, { name: 'client', values: [{ value: 'fimi' }] }).children[0].name).toBe('client:fimi');
  });

  it('inherits colors from ancestors', () => {
    const map = new Map([['project:kala', tag('project:kala', 1, '#ff0000')]]);
    expect(tagColor('project:kala:api', map)).toBe('#ff0000');
    expect(tagColor('other', map)).toMatch(/^#/);
  });

  it('hides status and priority chips', () => {
    expect(chipTags(['status:open', 'priority:high', 'project:x', 'urgent'])).toEqual(['project:x', 'urgent']);
  });

  it('finds the project of a filter', () => {
    expect(projectFromQuery('project:kala -status:closed,cancelled')).toBe('kala');
    expect(projectFromQuery('-status:closed,cancelled project:kala:api')).toBe('kala:api');
    expect(projectFromQuery('project:a,b')).toBeNull();
    expect(projectFromQuery('-project: -status:closed,cancelled')).toBeNull();
  });

  it('builds tag links with the open query except for namespaces with terminal values', () => {
    const nss = [
      { id: 1, name: 'status', exclusive: true, values: [{ value: 'open', terminal: false }, { value: 'closed', terminal: true }] },
      { id: 2, name: 'project', exclusive: false, values: [] },
    ];
    const open = '-status:closed';
    expect(tagQuery('project:kala', nss, open)).toBe('project:kala -status:closed');
    expect(tagQuery('urgent', nss, open)).toBe('#urgent -status:closed');
    expect(tagQuery('status:closed', nss, open)).toBe('status:closed');
    expect(tagQuery('status:closed:won', nss, open)).toBe('status:closed:won');
    expect(tagQuery('status', nss, open)).toBe('#status');
    expect(tagQuery('project:kala', nss, '')).toBe('project:kala');
    expect(tagQuery('project:kala', undefined, undefined)).toBe('project:kala');
  });
});

describe('format', () => {
  it('labels relative due dates', () => {
    const today = new Date(2026, 9, 21);
    expect(dueLabel('2026-10-21', today)).toBe('Today');
    expect(dueLabel('2026-10-22', today)).toBe('Tomorrow');
    expect(dueLabel('2026-10-20', today)).toBe('Yesterday');
    expect(isoDate(today)).toBe('2026-10-21');
  });
  it('formats durations', () => {
    expect(duration(45)).toBe('45s');
    expect(duration(125)).toBe('2m');
    expect(duration(3900)).toBe('1h 05m');
    expect(clock(3725)).toBe('1:02:05');
  });
  it('formats times on a 24h clock', () => {
    const iso = new Date(2026, 9, 21, 14, 5).toISOString();
    expect(timeLabel(iso)).toBe('14:05');
    expect(fullDateTime(iso)).toContain('14:05');
  });
});

describe('profile timezone', () => {
  afterEach(() => setZone(undefined));

  it('decides which day an instant falls on', () => {
    setZone('Asia/Tokyo');
    expect(dateOf('2026-10-21T14:59:59Z')).toBe('2026-10-21');
    expect(dateOf('2026-10-21T15:00:00Z')).toBe('2026-10-22');
    setZone('America/Los_Angeles');
    expect(dateOf('2026-10-21T06:59:59Z')).toBe('2026-10-20');
    const lateEvening = new Date('2026-10-22T05:00:00Z').getTime(); // still the 21st in Los Angeles
    expect(isoDate(todayDate(lateEvening))).toBe('2026-10-21');
    expect(dueLabel('2026-10-21', todayDate(lateEvening))).toBe('Today');
    expect(isOverdue('2026-10-21', todayDate(lateEvening))).toBe(false);
    expect(isOverdue('2026-10-20', todayDate(lateEvening))).toBe(true);
  });

  it('finds the instant a day begins, across DST changes', () => {
    setZone('Asia/Tokyo');
    expect(startOfDay('2026-10-22').toISOString()).toBe('2026-10-21T15:00:00.000Z');
    setZone('Europe/Rome');
    expect(startOfDay('2026-03-29').toISOString()).toBe('2026-03-28T23:00:00.000Z'); // clocks go forward that day
    expect(startOfDay('2026-03-30').toISOString()).toBe('2026-03-29T22:00:00.000Z');
    expect(startOfDay(new Date(2026, 9, 25)).toISOString()).toBe('2026-10-24T22:00:00.000Z'); // and back
    expect(startOfDay('2026-10-26').toISOString()).toBe('2026-10-25T23:00:00.000Z');
  });

  it('splits the week timeline at its midnights', () => {
    setZone('Asia/Tokyo');
    const entry = { id: 1, task_id: 1, task_title: 'T', task_tags: [], started_at: '2026-10-19T14:00:00Z', stopped_at: '2026-10-19T16:30:00Z', note: '', seconds: 0 };
    const segs = segments([entry], new Date(2026, 9, 19), 0); // 23:00 Monday to 01:30 Tuesday in Tokyo
    expect(segs.map((s) => [s.day, s.start, s.end])).toEqual([
      [0, 1380, 1440],
      [1, 0, 90],
    ]);
  });
});

describe('parseClock', () => {
  it('reads 24h clocks', () => {
    expect(parseClock('09:30')).toBe(570);
    expect(parseClock('9:05')).toBe(545);
    expect(parseClock('23:59')).toBe(1439);
    expect(parseClock('24:00')).toBeNull();
    expect(parseClock('9:5')).toBeNull();
    expect(parseClock('9:00 PM')).toBeNull();
  });
});

describe('monthGrid', () => {
  it('covers the month with whole Monday-first weeks', () => {
    const days = monthGrid(new Date(2026, 9, 21)); // October 2026 starts on a Thursday
    expect(days.length).toBe(35);
    expect(isoDate(days[0])).toBe('2026-09-28');
    expect(isoDate(days[days.length - 1])).toBe('2026-11-01');
  });

  it('needs no padding when the month starts on Monday and ends on Sunday', () => {
    const days = monthGrid(new Date(2027, 1, 10)); // February 2027
    expect(days.length).toBe(28);
    expect(isoDate(days[0])).toBe('2027-02-01');
  });
});

describe('timeline', () => {
  const week = new Date(2026, 9, 19); // Monday
  const at = (day: number, h: number, m = 0) => new Date(2026, 9, 19 + day, h, m).toISOString();
  const entry = (id: number, from: string, to: string | null) => ({ id, task_id: 1, task_title: 'T', task_tags: [], started_at: from, stopped_at: to, note: '', seconds: 0 });

  it('splits entries at midnight and ends running ones now', () => {
    const now = new Date(2026, 9, 21, 10, 30).getTime();
    const segs = segments([entry(1, at(0, 23), at(1, 1)), entry(2, at(2, 9), null)], week, now);
    expect(segs.map((s) => [s.entry.id, s.day, s.start, s.end])).toEqual([
      [1, 0, 1380, 1440],
      [1, 1, 0, 60],
      [2, 2, 540, 630],
    ]);
  });

  it('covers segments and working hours in whole hours', () => {
    const segs = segments([entry(1, at(0, 7, 45), at(0, 9))], week, 0);
    expect(hourRange(segs, [{ weekday: 1, start_min: 540, end_min: 1050 }])).toEqual([7, 18]);
    expect(hourRange([], [])).toEqual([8, 18]);
  });
});

describe('ago', () => {
  it('rounds down to the largest unit', () => {
    const now = new Date('2026-10-21T12:00:00Z').getTime();
    expect(ago('2026-10-21T11:59:30Z', now)).toBe('just now');
    expect(ago('2026-10-21T11:15:00Z', now)).toBe('45m ago');
    expect(ago('2026-10-21T09:00:00Z', now)).toBe('3h ago');
    expect(ago('2026-10-18T12:00:00Z', now)).toBe('3d ago');
  });
});

describe('quickadd', () => {
  it('completes tags, namespace values, projects and priorities', () => {
    const nss = [
      { name: 'priority', values: [{ value: 'high' }, { value: 'low' }] },
      { name: 'status', values: [{ value: 'open' }, { value: 'delegated' }] },
    ];
    const tags = ['project:kala', 'status:open', 'home'];
    expect(currentToken('mail a#b')).toBeNull();
    expect(suggestions(currentToken('x #del'), tags, nss)).toEqual(['status:delegated']);
    expect(suggestions(currentToken('x @k'), tags, nss)).toEqual(['kala']);
    expect(suggestions(currentToken('x !'), tags, nss)).toEqual(['high', 'low']);
    expect(completeToken('x !hi', 'high')).toBe('x !high ');
  });
});
