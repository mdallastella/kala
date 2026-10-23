import { useEffect, useState } from 'preact/hooks';
import { ApiError, get, send } from '../lib/api';
import type { CachedTags } from '../lib/messages';
import type { Tag } from '../lib/types';
import { QuickAdd } from './QuickAdd';

interface Props {
  onClose: () => void;
}

/** Errors that only the extension settings can fix. */
function setupMessage(e: unknown): string {
  if (!(e instanceof ApiError)) return '';
  if (e.code === 'unconfigured' || e.code === 'permission') return e.message;
  return e.status === 401 ? 'Token invalid' : '';
}

export function App({ onClose }: Props) {
  const [tags, setTags] = useState<Tag[]>([]);
  const [setupError, setSetupError] = useState('');
  const [toast, setToast] = useState('');

  // Tags feed the #/@ completion: cached ones right away, then fresh ones.
  useEffect(() => {
    void send<CachedTags>({ type: 'cached-tags' }).then((t) => t && setTags(t));
    get<Tag[]>('/tags')
      .then(setTags)
      .catch((e: unknown) => setSetupError(setupMessage(e)));
  }, []);

  useEffect(() => {
    if (!toast) return;
    const t = setTimeout(onClose, 1500);
    return () => clearTimeout(t);
  }, [toast, onClose]);

  return (
    <div
      class="root"
      onKeyDown={(e) => {
        if (e.key === 'Escape') {
          e.preventDefault();
          onClose();
        }
      }}
    >
      {toast ? <div class="toast">{toast}</div> : <QuickAdd tags={tags} setupError={setupError} onDone={setToast} />}
    </div>
  );
}
