export interface Settings {
  baseUrl: string;
  token: string;
}

export async function getSettings(): Promise<Settings | null> {
  const s = (await chrome.storage.local.get(['baseUrl', 'token'])) as Partial<Settings>;
  return s.baseUrl && s.token ? { baseUrl: s.baseUrl, token: s.token } : null;
}

/** Normalises a server URL to its origin plus path, without a trailing slash. Throws on invalid input. */
export function normalizeBaseUrl(raw: string): string {
  const u = new URL(raw.trim());
  if (u.protocol !== 'http:' && u.protocol !== 'https:') throw new Error('The URL must start with http:// or https://');
  return (u.origin + u.pathname).replace(/\/+$/, '');
}

/** Match pattern granting host access to the server. */
export function originPattern(baseUrl: string): string {
  return `${new URL(baseUrl).origin}/*`;
}
