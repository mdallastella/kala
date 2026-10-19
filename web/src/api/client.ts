import type { ApiErrorBody, Session } from './types';

const BASE = '/api/v1';

export class ApiError extends Error {
  status: number;
  code: string;
  details?: unknown;

  constructor(status: number, body: ApiErrorBody) {
    super(body.message);
    this.status = status;
    this.code = body.code;
    this.details = body.details;
  }
}

// The access token lives in memory only; the refresh token is an HttpOnly
// cookie the browser sends to /api/v1/auth.
let accessToken: string | null = null;
let refreshing: Promise<Session | null> | null = null;
const sessionListeners = new Set<(s: Session | null) => void>();

export function onSession(fn: (s: Session | null) => void): () => void {
  sessionListeners.add(fn);
  return () => sessionListeners.delete(fn);
}

export function setSession(s: Session | null) {
  accessToken = s?.access_token ?? null;
  sessionListeners.forEach((fn) => fn(s));
}

export function getAccessToken() {
  return accessToken;
}

async function parse(res: Response): Promise<unknown> {
  if (res.status === 204) return undefined;
  const text = await res.text();
  return text ? JSON.parse(text) : undefined;
}

async function raw(path: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers);
  if (init.body !== undefined && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
  if (accessToken) headers.set('Authorization', `Bearer ${accessToken}`);
  return fetch(BASE + path, { ...init, headers, credentials: 'same-origin' });
}

/** Exchanges the refresh cookie for a new access token (deduplicated). */
export function refreshSession(): Promise<Session | null> {
  refreshing ??= (async () => {
    try {
      const res = await fetch(`${BASE}/auth/refresh`, { method: 'POST', credentials: 'same-origin' });
      if (!res.ok) {
        setSession(null);
        return null;
      }
      const s = (await res.json()) as Session;
      setSession(s);
      return s;
    } catch {
      return null;
    } finally {
      refreshing = null;
    }
  })();
  return refreshing;
}

export async function api<T>(path: string, init: RequestInit & { json?: unknown } = {}): Promise<T> {
  const { json, ...rest } = init;
  if (json !== undefined) rest.body = JSON.stringify(json);
  let res = await raw(path, rest);
  if (res.status === 401 && !path.startsWith('/auth/')) {
    const s = await refreshSession();
    if (s) res = await raw(path, rest);
  }
  const body = await parse(res);
  if (!res.ok) {
    const err = (body as { error?: ApiErrorBody } | undefined)?.error ?? { code: 'http', message: res.statusText };
    throw new ApiError(res.status, err);
  }
  return body as T;
}

export const get = <T>(path: string) => api<T>(path);
export const post = <T>(path: string, json?: unknown) => api<T>(path, { method: 'POST', json });
export const patch = <T>(path: string, json: unknown) => api<T>(path, { method: 'PATCH', json });
export const put = <T>(path: string, json: unknown) => api<T>(path, { method: 'PUT', json });
export const del = <T = void>(path: string) => api<T>(path, { method: 'DELETE' });

export function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

/** Opens an authenticated SSE stream; returns a function that closes it. */
export function streamEvents(onEvent: (type: string, data: unknown) => void, onOpen: () => void): () => void {
  const ctrl = new AbortController();
  let delay = 1000;
  const loop = async () => {
    while (!ctrl.signal.aborted) {
      try {
        let res = await raw('/events', { signal: ctrl.signal });
        if (res.status === 401 && (await refreshSession())) res = await raw('/events', { signal: ctrl.signal });
        if (!res.ok || !res.body) throw new Error(`events: ${res.status}`);
        delay = 1000;
        onOpen();
        const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
        let buf = '';
        for (;;) {
          const { value, done } = await reader.read();
          if (done) break;
          buf += value;
          let idx;
          while ((idx = buf.indexOf('\n\n')) >= 0) {
            const chunk = buf.slice(0, idx);
            buf = buf.slice(idx + 2);
            let type = 'message';
            let data = '';
            for (const line of chunk.split('\n')) {
              if (line.startsWith('event: ')) type = line.slice(7);
              else if (line.startsWith('data: ')) data += line.slice(6);
            }
            if (data || type !== 'message') onEvent(type, data ? JSON.parse(data) : null);
          }
        }
      } catch {
        if (ctrl.signal.aborted) return;
      }
      await new Promise((r) => setTimeout(r, delay));
      delay = Math.min(delay * 2, 30000);
    }
  };
  void loop();
  return () => ctrl.abort();
}
