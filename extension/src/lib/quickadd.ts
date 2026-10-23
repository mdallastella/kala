// Pure helpers ported from the web app; keep them in step with:
//   currentToken  web/src/components/QuickAdd.tsx
//   tagColor      web/src/lib/tags.ts
//   searchQuery   web/src/components/CommandPalette.tsx

/** The token being typed at the end of the input, if it is a tag token. */
export function currentToken(text: string): { prefix: '#' | '@'; partial: string } | null {
  const m = text.match(/(?:^|\s)([#@])(\S*)$/);
  return m ? { prefix: m[1] as '#' | '@', partial: m[2].toLowerCase() } : null;
}

/** Filter query finding open tasks whose text contains title, or null when it is too short to search. */
export function searchQuery(title: string): string | null {
  const t = title.replace(/"/g, '').trim();
  return t.length < 2 ? null : `"${t}" is:open`;
}

const palette = ['#5b5bd6', '#0e7c86', '#c2410c', '#15803d', '#be185d', '#7c3aed', '#0369a1', '#a16207', '#b91c1c', '#4d7c0f'];

function hash(s: string): number {
  let h = 0;
  for (const c of s) h = (h * 31 + c.charCodeAt(0)) | 0;
  return Math.abs(h);
}

/** Colour of a tag: its own, the nearest ancestor's, or a stable fallback. */
export function tagColor(name: string, colors: Map<string, string | null>): string {
  for (let p = name; p; ) {
    const c = colors.get(p);
    if (c) return c;
    const i = p.lastIndexOf(':');
    p = i < 0 ? '' : p.slice(0, i);
  }
  return palette[hash(name) % palette.length];
}
