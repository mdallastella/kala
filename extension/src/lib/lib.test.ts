import { describe, expect, it } from 'vitest';
import { dueLabel, isoDate } from './format';
import { currentToken, searchQuery, tagColor } from './quickadd';
import { normalizeBaseUrl, originPattern } from './settings';

describe('quickadd', () => {
  it('finds the tag token being typed', () => {
    expect(currentToken('buy milk #ho')).toEqual({ prefix: '#', partial: 'ho' });
    expect(currentToken('@Kala')).toEqual({ prefix: '@', partial: 'kala' });
    expect(currentToken('buy milk')).toBeNull();
    expect(currentToken('mail a#b')).toBeNull();
  });
  it('builds the search query from the parsed title', () => {
    expect(searchQuery(' write "report" ')).toBe('"write report" is:open');
    expect(searchQuery('a')).toBeNull();
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
});
