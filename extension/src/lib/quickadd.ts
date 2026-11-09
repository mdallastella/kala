// Pure helpers ported from the web app; keep them in step with:
//   currentToken, suggestions, completeToken  web/src/lib/quickadd.ts
//   tagColor      web/src/lib/tags.ts
//   searchQuery   web/src/components/CommandPalette.tsx

export type TokenPrefix = '#' | '@' | '!';

interface NamespaceLike {
  name: string;
  values: { value: string }[];
}

/** The token being typed at the end of the input, if it is a tag, project or priority token. */
export function currentToken(text: string): { prefix: TokenPrefix; partial: string } | null {
  const m = text.match(/(?:^|\s)([#@!])(\S*)$/);
  return m ? { prefix: m[1] as TokenPrefix, partial: m[2].toLowerCase() } : null;
}

/**
 * Completions for a token: # offers every tag and every defined namespace value (as the tag
 * picker does), @ the projects and ! the priority values.
 */
export function suggestions(token: { prefix: TokenPrefix; partial: string } | null, tags: string[], namespaces: NamespaceLike[]): string[] {
  if (!token) return [];
  let pool: string[];
  if (token.prefix === '@') pool = tags.filter((n) => n.startsWith('project:')).map((n) => n.slice(8));
  else if (token.prefix === '!') pool = namespaces.find((n) => n.name === 'priority')?.values.map((v) => v.value) ?? [];
  else pool = [...new Set([...tags, ...namespaces.flatMap((n) => n.values.map((v) => `${n.name}:${v.value}`))])].sort();
  return pool.filter((n) => n.includes(token.partial) && n !== token.partial).slice(0, 8);
}

/** Replaces the token being typed with value. */
export function completeToken(text: string, value: string): string {
  return text.replace(/([#@!])(\S*)$/, `$1${value} `);
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
