import { useLocation, useSearchParams } from 'react-router';

/** Filter shown when the task page is opened without ?q=…. */
const DEFAULT_QUERY = 'is:open';

/** Path of the task list, optionally filtered and with a task open. */
export function tasksPath(q?: string, task?: number): string {
  const p = new URLSearchParams();
  if (q) p.set('q', q);
  if (task != null) p.set('task', String(task));
  const s = p.toString();
  return s ? `/tasks?${s}` : '/tasks';
}

/**
 * The task filter shown by the current URL: ?q=…, or DEFAULT_QUERY when the
 * task page is opened without one. Empty on other pages.
 */
export function useCurrentQuery(): { q: string; explicit: boolean } {
  const [params] = useSearchParams();
  const { pathname } = useLocation();
  const explicit = params.get('q');
  if (pathname !== '/tasks') return { q: '', explicit: false };
  return { q: explicit ?? DEFAULT_QUERY, explicit: explicit != null };
}
