import type { ApiResponse, CachedTags, Request } from './lib/messages';
import { getSettings, originPattern } from './lib/settings';

chrome.runtime.onInstalled.addListener(async ({ reason }) => {
  if (reason === 'install' && !(await getSettings())) await chrome.runtime.openOptionsPage();
});

// A different server or token invalidates the cached tags.
chrome.storage.onChanged.addListener((_, area) => {
  if (area === 'local') void chrome.storage.session.remove('tags');
});

async function callApi(method: string, path: string, body: unknown): Promise<ApiResponse> {
  const s = await getSettings();
  if (!s) return { ok: false, status: 0, error: { code: 'unconfigured', message: 'kala is not set up yet' } };
  if (!(await chrome.permissions.contains({ origins: [originPattern(s.baseUrl)] })))
    return { ok: false, status: 0, error: { code: 'permission', message: `No permission to reach ${s.baseUrl} — open Extension settings` } };

  const headers: Record<string, string> = { Authorization: `Bearer ${s.token}` };
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  let res: Response;
  try {
    res = await fetch(`${s.baseUrl}/api/v1${path}`, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
  } catch {
    return { ok: false, status: 0, error: { code: 'network', message: `Cannot reach ${s.baseUrl}` } };
  }
  let parsed: unknown;
  try {
    const text = await res.text();
    parsed = text ? JSON.parse(text) : undefined;
  } catch {
    parsed = undefined;
  }
  if (!res.ok) {
    const error = (parsed as { error?: ApiResponse['error'] } | undefined)?.error ?? { code: 'http', message: res.statusText || `HTTP ${res.status}` };
    return { ok: false, status: res.status, error };
  }
  if (method === 'GET' && path === '/tags') await chrome.storage.session.set({ tags: parsed });
  return { ok: true, status: res.status, body: parsed };
}

async function handle(msg: Request): Promise<unknown> {
  switch (msg.type) {
    case 'api':
      return callApi(msg.method, msg.path, msg.body);
    case 'options':
      return chrome.runtime.openOptionsPage();
    case 'cached-tags': {
      const { tags } = await chrome.storage.session.get('tags');
      return (tags ?? null) as CachedTags;
    }
  }
}

chrome.runtime.onMessage.addListener((msg: Request, _sender, reply) => {
  handle(msg).then(reply, (e: unknown) => reply({ ok: false, status: 0, error: { code: 'internal', message: String(e) } }));
  return true; // reply asynchronously
});
