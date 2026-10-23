import type { ApiResponse, CachedTags, Request } from './lib/messages';
import { getSettings, originPattern } from './lib/settings';

chrome.runtime.onInstalled.addListener(async ({ reason }) => {
  if (reason === 'install' && !(await getSettings())) await chrome.runtime.openOptionsPage();
});

// A different server or token invalidates the cached tags and timer dot.
chrome.storage.onChanged.addListener((_, area) => {
  if (area !== 'local') return;
  void chrome.storage.session.remove('tags');
  void setTimerDot(false).then(refreshTimerDot);
});

// The toolbar icon shows a red dot while a timer runs. Timer calls made here update it
// at once; the alarm catches timers started or stopped elsewhere.
const iconSizes = [16, 32];
async function setTimerDot(on: boolean) {
  if (!on) {
    await chrome.action.setIcon({ path: Object.fromEntries(iconSizes.map((n) => [n, `icons/icon-${n}.png`])) });
    return;
  }
  const imageData: Record<number, ImageData> = {};
  for (const n of iconSizes) {
    const bmp = await createImageBitmap(await (await fetch(chrome.runtime.getURL(`icons/icon-${n}.png`))).blob());
    const ctx = new OffscreenCanvas(n, n).getContext('2d')!;
    ctx.drawImage(bmp, 0, 0, n, n);
    const r = n * 0.22;
    ctx.beginPath();
    ctx.arc(n - r, r, r, 0, 2 * Math.PI);
    ctx.fillStyle = '#e5484d';
    ctx.fill();
    imageData[n] = ctx.getImageData(0, 0, n, n);
  }
  await chrome.action.setIcon({ imageData });
}

// callApi updates the dot from the response.
const refreshTimerDot = () => callApi('GET', '/timers/active', undefined);

// The worker restarts often; recreating the alarm each time would keep postponing it.
void chrome.alarms.get('timers').then(async (a) => {
  if (!a) await chrome.alarms.create('timers', { periodInMinutes: 1 });
});
chrome.alarms.onAlarm.addListener((a) => {
  if (a.name === 'timers') void refreshTimerDot();
});
chrome.runtime.onStartup.addListener(() => void refreshTimerDot());

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
  if (method === 'GET' && path === '/timers/active') await setTimerDot((parsed as unknown[]).length > 0);
  else if (method === 'POST' && /^\/(timers|tasks\/\d+\/timer)\//.test(path)) void refreshTimerDot();
  return { ok: true, status: res.status, body: parsed };
}

async function handle(msg: Request): Promise<unknown> {
  switch (msg.type) {
    case 'api':
      return callApi(msg.method, msg.path, msg.body);
    case 'options':
      return chrome.runtime.openOptionsPage();
    case 'open-task': {
      const s = await getSettings();
      if (s) await chrome.tabs.create({ url: `${s.baseUrl}/tasks?task=${msg.id}` });
      return;
    }
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
