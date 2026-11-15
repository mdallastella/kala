import type { ApiErrorBody, Tag } from './types';

/** Messages the overlay and extension pages send to the service worker. */
export type Request =
  | { type: 'api'; method: string; path: string; body?: unknown }
  | { type: 'options' }
  | { type: 'open-task'; id: number }
  | { type: 'cached-tags' }
  | { type: 'cached-zone' };

export interface ApiResponse {
  ok: boolean;
  status: number;
  body?: unknown;
  error?: ApiErrorBody;
}

export type CachedTags = Tag[] | null;

/** The profile timezone last seen from the server. */
export type CachedZone = string | null;
