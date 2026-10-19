import { useLocation, useSearchParams } from 'react-router';
import { useViews } from '../api/hooks';
import type { View } from '../api/types';

/**
 * The task filter shown by the current URL: ?q=…, or the first view when the
 * task page is opened without one. Empty on other pages.
 */
export function useCurrentQuery(): { q: string; view: View | undefined; explicit: boolean; ready: boolean } {
  const [params] = useSearchParams();
  const { pathname } = useLocation();
  const { data: views } = useViews();
  const explicit = params.get('q');
  const viewId = Number(params.get('view')) || null;
  if (pathname !== '/') return { q: '', view: undefined, explicit: false, ready: true };
  const q = explicit ?? views?.[0]?.query ?? 'is:open';
  const view = views?.find((v) => (viewId ? v.id === viewId : v.query === q));
  return { q, view, explicit: explicit != null, ready: explicit != null || views != null };
}
