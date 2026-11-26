import { describe, expect, it } from 'vitest';
import { dateOf, dueLabel, isoDate, setZone, timeLabel, todayDate } from './format';
import { completeToken, currentToken, searchQuery, suggestions, tagColor } from './quickadd';
import { normalizeBaseUrl, originPattern } from './settings';

describe('quickadd', () => {
  it('finds the tag token being typed', () => {
    expect(currentToken('buy milk #ho')).toEqual({ prefix: '#', partial: 'ho' });
    expect(currentToken('@Kala')).toEqual({ prefix: '@', partial: 'kala' });
    expect(currentToken('buy milk')).toBeNull();
    expect(currentToken('mail a#b')).toBeNull();
    expect(currentToken('fix !h')).toEqual({ prefix: '!', partial: 'h' });
  });

  it('completes tags, namespace values, projects and priorities', () => {
    const nss = [
      { name: 'priority', values: [{ value: 'high' }, { value: 'low' }] },
      { name: 'status', values: [{ value: 'open' }, { value: 'delegated' }] },
    ];
    const tags = ['project:kala', 'status:open', 'home'];
    expect(suggestions(currentToken('x #del'), tags, nss)).toEqual(['status:delegated']);
    expect(suggestions(currentToken('x @k'), tags, nss)).toEqual(['kala']);
    expect(suggestions(currentToken('x !'), tags, nss)).toEqual(['high', 'low']);
    expect(completeToken('x !hi', 'high')).toBe('x !high ');
  });
  it('builds the search query from the parsed title', () => {
    expect(searchQuery(' write "report" ', '-status:closed,cancelled')).toBe('"write report" -status:closed,cancelled');
    expect(searchQuery('write', '')).toBe('"write"');
    expect(searchQuery('a', '-status:closed')).toBeNull();
  });
  it('colours tags from the nearest ancestor', () => {
    const colors = new Map([['project', '#123456']]);
    expect(tagColor('project:kala:api', colors)).toBe('#123456');
    expect(tagColor('urgent', colors)).toBe(tagColor('urgent', new Map()));
  });
});

describe('settings', () => {
  it('normalises the server URL', () => {
    expect(normalizeBaseUrl(' https://kala.example.com/ ')).toBe('https://kala.example.com');
    expect(normalizeBaseUrl('http://localhost:8080/kala/')).toBe('http://localhost:8080/kala');
    expect(() => normalizeBaseUrl('ftp://x')).toThrow();
    expect(() => normalizeBaseUrl('nope')).toThrow();
    expect(originPattern('http://localhost:8080/kala')).toBe('http://localhost:8080/*');
  });
});

describe('format', () => {
  it('labels relative due dates', () => {
    const today = new Date(2026, 9, 21);
    expect(dueLabel('2026-10-21', today)).toBe('Today');
    expect(dueLabel('2026-10-22', today)).toBe('Tomorrow');
    expect(isoDate(today)).toBe('2026-10-21');
  });
  it('takes the day from the profile timezone', () => {
    setZone('Asia/Tokyo');
    try {
      expect(dateOf('2026-10-21T14:59:59Z')).toBe('2026-10-21');
      expect(dateOf('2026-10-21T15:00:00Z')).toBe('2026-10-22');
      const now = new Date('2026-10-21T16:00:00Z').getTime(); // already the 22nd in Tokyo
      expect(isoDate(todayDate(now))).toBe('2026-10-22');
      expect(dueLabel('2026-10-22', todayDate(now))).toBe('Today');
      expect(timeLabel('2026-10-21T15:30:00Z', todayDate(now))).not.toContain('Yesterday');
    } finally {
      setZone(undefined);
    }
  });
});
