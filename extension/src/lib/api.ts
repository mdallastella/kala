import type { ApiResponse, Request } from './messages';
import type { ApiErrorBody } from './types';

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, body: ApiErrorBody) {
    super(status === 401 ? 'Token invalid — open Extension settings' : body.message);
    this.status = status;
    this.code = body.code;
  }
}

export function send<T>(msg: Request): Promise<T> {
  return chrome.runtime.sendMessage(msg);
}

/** Calls the kala REST API through the service worker, which holds the token and host permission. */
export async function api<T>(method: string, path: string, body?: unknown): Promise<T> {
  const r = await send<ApiResponse>({ type: 'api', method, path, body });
  if (!r.ok) throw new ApiError(r.status, r.error ?? { code: 'http', message: `HTTP ${r.status}` });
  return r.body as T;
}

export const get = <T>(path: string) => api<T>('GET', path);
export const post = <T>(path: string, body?: unknown) => api<T>('POST', path, body);

export function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}
