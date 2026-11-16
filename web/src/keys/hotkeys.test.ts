import { describe, expect, it } from 'vitest';
import { usableBindings } from './hotkeys';
import type { Binding } from './matcher';

const b = (keys: string, extra: Partial<Binding> = {}): Binding => ({ keys, description: keys, group: 't', run: () => {}, ...extra });
const all = [b('enter'), b('space'), b('down'), b('x'), b('mod+k', { allowInInput: true })];
const keys = (target: Element | null) => usableBindings(all, target).map((x) => x.keys);

function el(html: string): Element {
  document.body.innerHTML = html;
  return document.body.querySelector('[data-t]')!;
}

describe('usableBindings', () => {
  it('fires everything on the page body', () => {
    expect(keys(document.body)).toEqual(['enter', 'space', 'down', 'x', 'mod+k']);
  });

  it('leaves Enter, Space and arrows to a focused button, link or option', () => {
    for (const html of ['<button data-t>+</button>', '<a href="/" data-t>Home</a>', '<div role="option" data-t>a</div>', '<li role="button"><span data-t>b</span></li>'])
      expect(keys(el(html))).toEqual(['x', 'mod+k']);
  });

  it('lets hotkey-driven list rows opt back in', () => {
    expect(keys(el('<div role="button" data-hotkeys data-t>task</div>'))).toEqual(['enter', 'space', 'down', 'x', 'mod+k']);
  });

  it('only fires input-allowed bindings while typing', () => {
    expect(keys(el('<input data-t>'))).toEqual(['mod+k']);
  });
});
