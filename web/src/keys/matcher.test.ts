import { describe, expect, it } from 'vitest';
import { comboOf, Matcher, SEQUENCE_TIMEOUT, type Binding, type KeyLike } from './matcher';

const key = (k: string, mods: Partial<KeyLike> = {}): KeyLike => ({ key: k, ctrlKey: false, metaKey: false, altKey: false, shiftKey: false, ...mods });
const b = (keys: string): Binding => ({ keys, description: keys, group: 't', run: () => {} });

describe('comboOf', () => {
  it('uses the produced character for printable keys', () => {
    expect(comboOf(key('x'))).toBe('x');
    expect(comboOf(key('Q', { shiftKey: true }))).toBe('Q');
    expect(comboOf(key('?', { shiftKey: true }))).toBe('?');
  });
  it('normalises modifiers and named keys', () => {
    expect(comboOf(key('k', { ctrlKey: true }))).toBe('mod+k');
    expect(comboOf(key('K', { metaKey: true, shiftKey: true }))).toBe('mod+k');
    expect(comboOf(key('Enter', { shiftKey: true }))).toBe('shift+enter');
    expect(comboOf(key(' '))).toBe('space');
    expect(comboOf(key('ArrowDown'))).toBe('down');
    expect(comboOf(key('Shift', { shiftKey: true }))).toBeNull();
  });
});

describe('Matcher', () => {
  const bindings = [b('g t'), b('g u'), b('x'), b('t'), b('* a')];

  it('matches single keys', () => {
    expect(new Matcher().feed(bindings, 'x', 0)).toMatchObject({ keys: 'x' });
    expect(new Matcher().feed(bindings, 'z', 0)).toBeNull();
  });

  it('matches sequences within the timeout', () => {
    const m = new Matcher();
    expect(m.feed(bindings, 'g', 0)).toBe('pending');
    expect(m.feed(bindings, 't', 100)).toMatchObject({ keys: 'g t' });
    expect(m.feed(bindings, '*', 200)).toBe('pending');
    expect(m.feed(bindings, 'a', 300)).toMatchObject({ keys: '* a' });
  });

  it('falls back to the single key after a timeout or a miss', () => {
    const m = new Matcher();
    m.feed(bindings, 'g', 0);
    expect(m.feed(bindings, 't', SEQUENCE_TIMEOUT + 1)).toMatchObject({ keys: 't' });
    m.feed(bindings, 'g', 0);
    expect(m.feed(bindings, 'x', 10)).toMatchObject({ keys: 'x' });
  });
});
