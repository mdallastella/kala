import { describe, expect, it } from 'vitest';
import type { Tag } from '../api/types';
import { clock, dueLabel, duration, isoDate, monthGrid } from './format';
import { buildTree, chipTags, projectFromQuery, tagColor } from './tags';

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

  it('inherits colors from ancestors', () => {
    const map = new Map([['project:kala', tag('project:kala', 1, '#ff0000')]]);
    expect(tagColor('project:kala:api', map)).toBe('#ff0000');
    expect(tagColor('other', map)).toMatch(/^#/);
  });

  it('hides status and priority chips', () => {
    expect(chipTags(['status:open', 'priority:high', 'project:x', 'urgent'])).toEqual(['project:x', 'urgent']);
  });

  it('finds the project of a filter', () => {
    expect(projectFromQuery('project:kala is:open')).toBe('kala');
    expect(projectFromQuery('is:open project:kala:api')).toBe('kala:api');
    expect(projectFromQuery('project:a,b')).toBeNull();
    expect(projectFromQuery('-project: is:open')).toBeNull();
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
