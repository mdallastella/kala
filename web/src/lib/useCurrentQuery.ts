import { useLocation, useSearchParams } from 'react-router';

/** Filter shown when the task page is opened without ?q=…. */
const DEFAULT_QUERY = 'is:open';

/**
 * The task filter shown by the current URL: ?q=…, or DEFAULT_QUERY when the
 * task page is opened without one. Empty on other pages.
 */
export function useCurrentQuery(): { q: string; explicit: boolean } {
  const [params] = useSearchParams();
  const { pathname } = useLocation();
  const explicit = params.get('q');
  if (pathname !== '/') return { q: '', explicit: false };
  return { q: explicit ?? DEFAULT_QUERY, explicit: explicit != null };
}
