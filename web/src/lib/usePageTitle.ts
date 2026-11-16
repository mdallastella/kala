import { useEffect } from 'react';

/** Sets the tab title to "<parts> · kala"; null leaves it to another component. */
export function usePageTitle(...parts: (string | null | undefined)[]) {
  const title = [...parts.filter(Boolean), 'kala'].join(' · ');
  const skip = parts.includes(null);
  useEffect(() => {
    if (!skip) document.title = title;
  }, [title, skip]);
}
