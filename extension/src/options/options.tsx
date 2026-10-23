import { render } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import type { TargetedEvent } from 'preact';
import { errorMessage, get } from '../lib/api';
import { getSettings, normalizeBaseUrl, originPattern } from '../lib/settings';
import type { User } from '../lib/types';

type Status = { kind: 'idle' } | { kind: 'busy' } | { kind: 'ok'; email: string } | { kind: 'error'; message: string };

async function check(): Promise<Status> {
  try {
    const me = await get<User>('/me');
    return { kind: 'ok', email: me.email };
  } catch (e) {
    return { kind: 'error', message: errorMessage(e) };
  }
}

function Options() {
  const [baseUrl, setBaseUrl] = useState('');
  const [token, setToken] = useState('');
  const [status, setStatus] = useState<Status>({ kind: 'idle' });
  const [shortcut, setShortcut] = useState('');

  useEffect(() => {
    void getSettings().then(async (s) => {
      if (!s) return;
      setBaseUrl(s.baseUrl);
      setToken(s.token);
      setStatus({ kind: 'busy' });
      setStatus(await check());
    });
    void chrome.commands.getAll().then((cmds) => setShortcut(cmds.find((c) => c.name === '_execute_action')?.shortcut ?? ''));
  }, []);

  const save = async (e: TargetedEvent<HTMLFormElement>) => {
    e.preventDefault();
    let url: string;
    try {
      url = normalizeBaseUrl(baseUrl);
    } catch (err) {
      setStatus({ kind: 'error', message: err instanceof TypeError ? 'Enter a valid URL, e.g. https://kala.example.com' : errorMessage(err) });
      return;
    }
    if (!token.trim().startsWith('kala_pat_')) {
      setStatus({ kind: 'error', message: 'Personal access tokens start with kala_pat_' });
      return;
    }
    // Must be requested before any other await, while the click still counts as a user gesture.
    const granted = await chrome.permissions.request({ origins: [originPattern(url)] });
    if (!granted) {
      setStatus({ kind: 'error', message: `kala needs permission to reach ${url}` });
      return;
    }
    const prev = await getSettings();
    if (prev && originPattern(prev.baseUrl) !== originPattern(url)) await chrome.permissions.remove({ origins: [originPattern(prev.baseUrl)] });
    setBaseUrl(url);
    await chrome.storage.local.set({ baseUrl: url, token: token.trim() });
    setStatus({ kind: 'busy' });
    setStatus(await check());
  };

  return (
    <>
      <h1>kala</h1>
      <p>Connect quick add to your kala server.</p>
      <form onSubmit={(e) => void save(e)}>
        <label>
          Server URL
          <input
            type="url"
            required
            placeholder="https://kala.example.com"
            value={baseUrl}
            onInput={(e: TargetedEvent<HTMLInputElement>) => setBaseUrl(e.currentTarget.value)}
          />
        </label>
        <label>
          Personal access token
          <input
            type="password"
            required
            placeholder="kala_pat_…"
            autoComplete="off"
            value={token}
            onInput={(e: TargetedEvent<HTMLInputElement>) => setToken(e.currentTarget.value)}
          />
        </label>
        <p>
          Create one in kala under <strong>Settings → Access tokens &amp; MCP</strong>.
        </p>
        <button type="submit" disabled={status.kind === 'busy'}>
          Save &amp; test connection
        </button>
      </form>
      {status.kind === 'busy' && <p class="status">Connecting…</p>}
      {status.kind === 'ok' && <p class="status ok">Connected as {status.email}</p>}
      {status.kind === 'error' && <p class="status err">{status.message}</p>}
      <hr />
      <p>
        Open quick add with {shortcut ? <code>{shortcut}</code> : 'a shortcut (none set)'} or the toolbar icon.{' '}
        <a onClick={() => void chrome.tabs.create({ url: 'chrome://extensions/shortcuts' })}>Change shortcuts</a>
      </p>
    </>
  );
}

render(<Options />, document.getElementById('root')!);
