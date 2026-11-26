import { useLocation, useSearchParams } from 'react-router';
import { useOpenQuery } from '../api/hooks';

/** Path of the task list, optionally filtered and with a task open. */
export function tasksPath(q?: string, task?: number): string {
  const p = new URLSearchParams();
  if (q) p.set('q', q);
  if (task != null) p.set('task', String(task));
  const s = p.toString();
  return s ? `/tasks?${s}` : '/tasks';
}

/**
 * The task filter shown by the current URL: ?q=…, or the open query when the
 * task page is opened without one. Empty on other pages. ready is false while
 * the open query is still loading, so the list should wait.
 */
export function useCurrentQuery(): { q: string; explicit: boolean; ready: boolean } {
  const [params] = useSearchParams();
  const { pathname } = useLocation();
  const { data: open } = useOpenQuery();
  const explicit = params.get('q');
  if (pathname !== '/tasks') return { q: '', explicit: false, ready: true };
  if (explicit != null) return { q: explicit, explicit: true, ready: true };
  return { q: open ?? '', explicit: false, ready: open !== undefined };
}
