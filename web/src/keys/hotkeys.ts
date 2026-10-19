import { useEffect, useRef, useSyncExternalStore } from 'react';
import { comboOf, Matcher, type Binding } from './matcher';

export type { Binding } from './matcher';

// Scopes registered by mounted components; later scopes win on conflicts.
const scopes = new Map<symbol, () => Binding[]>();
const listeners = new Set<() => void>();
let snapshot: Binding[] = [];
const matcher = new Matcher();

function changed() {
  snapshot = [...scopes.values()].flatMap((f) => f());
  listeners.forEach((l) => l());
}

function isEditable(el: Element | null): boolean {
  if (!el || !(el instanceof HTMLElement)) return false;
  if (el.isContentEditable) return true;
  if (el instanceof HTMLTextAreaElement || el instanceof HTMLSelectElement) return true;
  if (el instanceof HTMLInputElement) return !['checkbox', 'radio', 'button', 'submit'].includes(el.type);
  return false;
}

function onKeyDown(e: KeyboardEvent) {
  if (e.defaultPrevented || e.isComposing) return;
  const combo = comboOf(e);
  if (!combo) return;
  const target = e.target instanceof Element ? e.target : null;
  const typing = isEditable(target);
  const inDialog = !!target?.closest('[role="dialog"], [role="presentation"]');
  // Most recently registered scopes take precedence.
  const all = [...scopes.values()].reverse().flatMap((f) => f());
  const usable = all.filter((b) => (!typing || b.allowInInput) && (!inDialog || b.allowInDialog));
  const hit = matcher.feed(usable, combo, Date.now());
  if (hit === 'pending') {
    e.preventDefault();
    return;
  }
  if (hit) {
    e.preventDefault();
    hit.run(e);
  }
}

let installed = false;
function install() {
  if (installed || typeof window === 'undefined') return;
  installed = true;
  window.addEventListener('keydown', onKeyDown);
}

/** Registers bindings while the component is mounted and enabled. */
export function useHotkeys(bindings: Binding[], enabled = true) {
  const ref = useRef(bindings);
  useEffect(() => {
    ref.current = bindings;
  });
  const keysSig = bindings.map((b) => b.keys + b.description).join('|');
  useEffect(() => {
    if (!enabled) return;
    install();
    const id = Symbol();
    // Bindings call through the ref so handlers always see fresh state.
    scopes.set(id, () =>
      ref.current.map((b, i) => ({ ...b, run: (e: KeyboardEvent) => ref.current[i]?.run(e) })),
    );
    changed();
    return () => {
      scopes.delete(id);
      changed();
    };
  }, [enabled, keysSig]);
}

/** All currently registered bindings, for the help dialog. */
export function useRegisteredBindings(): Binding[] {
  return useSyncExternalStore(
    (l) => {
      listeners.add(l);
      return () => listeners.delete(l);
    },
    () => snapshot,
  );
}
