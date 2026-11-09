// Quick-add completion helpers. extension/src/lib/quickadd.ts has a copy; keep them in step.

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
