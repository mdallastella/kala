/**
 * Key matching for single keys ("x", "?", "mod+k", "delete") and two-key
 * sequences ("g t"). Printable keys are matched by the character they
 * produce, so "Q" means shift+q and "?" works on any layout.
 */

export interface Binding {
  /** Space-separated sequence, e.g. "g t", "mod+k", "shift+enter". */
  keys: string;
  description: string;
  group: string;
  run: (e: KeyboardEvent) => void;
  /** Fire even while typing in an input. */
  allowInInput?: boolean;
  /** Fire while a dialog has focus. */
  allowInDialog?: boolean;
  /** Hide from the help dialog. */
  hidden?: boolean;
}

const named: Record<string, string> = {
  ' ': 'space',
  ArrowDown: 'down',
  ArrowUp: 'up',
  ArrowLeft: 'left',
  ArrowRight: 'right',
  Escape: 'escape',
  Enter: 'enter',
  Delete: 'delete',
  Backspace: 'backspace',
  Home: 'home',
  End: 'end',
  Tab: 'tab',
};

export interface KeyLike {
  key: string;
  ctrlKey: boolean;
  metaKey: boolean;
  altKey: boolean;
  shiftKey: boolean;
}

/** Normalised combo of a key event, or null for lone modifiers. */
export function comboOf(e: KeyLike): string | null {
  if (['Shift', 'Control', 'Meta', 'Alt', 'CapsLock'].includes(e.key)) return null;
  let key = named[e.key] ?? e.key;
  const printable = key.length === 1;
  if (printable && (e.ctrlKey || e.metaKey || e.altKey)) key = key.toLowerCase();
  let mods = '';
  if (e.ctrlKey || e.metaKey) mods += 'mod+';
  if (e.altKey) mods += 'alt+';
  if (e.shiftKey && !printable) mods += 'shift+';
  return mods + key;
}

export const SEQUENCE_TIMEOUT = 1000;

export class Matcher {
  private pending: string | null = null;
  private at = 0;

  /**
   * Feeds a combo. Returns the matched binding, "pending" when the combo
   * starts a sequence, or null.
   */
  feed(bindings: Binding[], combo: string, now: number): Binding | 'pending' | null {
    if (this.pending && now - this.at <= SEQUENCE_TIMEOUT) {
      const seq = `${this.pending} ${combo}`;
      this.pending = null;
      const hit = bindings.find((b) => b.keys === seq);
      if (hit) return hit;
    }
    this.pending = null;
    if (bindings.some((b) => b.keys.startsWith(combo + ' '))) {
      this.pending = combo;
      this.at = now;
      return 'pending';
    }
    return bindings.find((b) => b.keys === combo) ?? null;
  }

  reset() {
    this.pending = null;
  }
}

/** Human-readable key label for help texts. */
export function keyLabel(keys: string): string {
  const mac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform);
  return keys
    .split(' ')
    .map((k) =>
      k
        .replace('mod+', mac ? '⌘' : 'Ctrl+')
        .replace('alt+', mac ? '⌥' : 'Alt+')
        .replace('shift+', '⇧')
        .replace('escape', 'Esc')
        .replace('enter', '↵')
        .replace('delete', 'Del')
        .replace('space', 'Space')
        .replace('down', '↓')
        .replace('up', '↑'),
    )
    .join(' then ');
}
