import { useEffect, useMemo, useRef, useState } from 'preact/hooks';
import type { TargetedEvent } from 'preact';
import { errorMessage, post, send } from '../lib/api';
import { dateTimeLabel, dueLabel } from '../lib/format';
import { currentToken, tagColor } from '../lib/quickadd';
import type { QuickParse, Tag, Task } from '../lib/types';

interface Props {
  tags: Tag[];
  /** Why the extension cannot reach kala yet; Enter then opens the settings. */
  setupError: string;
  onDone: (message: string) => void;
}

/** Port of web/src/components/QuickAdd.tsx: Enter adds, Shift+Enter also starts a timer. */
export function QuickAdd({ tags, setupError, onDone }: Props) {
  const [text, setText] = useState('');
  // Parse result for a given text; stale results are ignored when rendering.
  const [parsed, setParsed] = useState<{ text: string; preview?: QuickParse; error?: string }>({ text: '' });
  const [submitError, setSubmitError] = useState('');
  const [busy, setBusy] = useState(false);
  const [shift, setShift] = useState(false);
  const [sel, setSel] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const colors = useMemo(() => new Map(tags.map((t) => [t.name, t.color])), [tags]);

  useEffect(() => {
    if (!text.trim() || setupError) return;
    const t = setTimeout(() => {
      post<QuickParse>('/tasks/parse', { quick: text })
        .then((p) => setParsed({ text, preview: p }))
        .catch((e) => setParsed({ text, error: errorMessage(e) }));
    }, 150);
    return () => clearTimeout(t);
  }, [text, setupError]);
  const empty = !text.trim();
  const preview = empty ? null : (parsed.preview ?? null);
  const error = setupError || submitError || (!empty && parsed.text === text ? (parsed.error ?? '') : '');

  const token = currentToken(text);
  const suggestions = useMemo(() => {
    if (!token) return [];
    const names = tags.map((t) => t.name);
    const pool = token.prefix === '@' ? names.filter((n) => n.startsWith('project:')).map((n) => n.slice(8)) : names;
    return pool.filter((n) => n.includes(token.partial) && n !== token.partial).slice(0, 8);
  }, [token, tags]);

  const complete = (value: string) => {
    setText(text.replace(/([#@])(\S*)$/, `$1${value} `));
    setSel(0);
  };

  const submit = async (timer: boolean) => {
    if (setupError) {
      void send({ type: 'options' });
      return;
    }
    if (!text.trim() || error || busy) return;
    setBusy(true);
    try {
      const t = timer ? await post<Task>('/timers/start-new', { quick: text }) : await post<Task>('/tasks', { quick: text });
      onDone(timer ? `Started timer on “${t.title}”` : `Created “${t.title}”`);
    } catch (e) {
      setSubmitError(errorMessage(e));
      setBusy(false);
    }
  };

  const onKeyDown = (e: KeyboardEvent) => {
    setShift(e.shiftKey);
    if (suggestions.length > 0 && (e.key === 'Tab' || e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
      e.preventDefault();
      if (e.key === 'Tab') complete(suggestions[sel] ?? suggestions[0]);
      else setSel((s) => (s + (e.key === 'ArrowDown' ? 1 : suggestions.length - 1)) % suggestions.length);
      return;
    }
    if (e.key === 'Tab') e.preventDefault(); // keep focus in the input
    if (e.key === 'Enter') {
      e.preventDefault();
      void submit(e.shiftKey);
    }
  };

  return (
    <div class="quick">
      <div class="head">
        {shift && !setupError && <span class="badge">timer</span>}
        <input
          ref={inputRef}
          autoFocus
          class="input"
          placeholder="Write report #client:acme @kala !high due:fri remind:thu 9:00"
          aria-label="Quick add"
          value={text}
          readOnly={busy}
          onInput={(e: TargetedEvent<HTMLInputElement>) => {
            setText(e.currentTarget.value);
            setSubmitError('');
          }}
          onKeyDown={onKeyDown}
          onKeyUp={(e) => setShift(e.shiftKey)}
          // Pages sometimes grab focus (their own autofocus, focus traps); take it back.
          onBlur={() => {
            setShift(false);
            requestAnimationFrame(() => inputRef.current?.focus());
          }}
        />
      </div>
      {suggestions.length > 0 && (
        <ul class="list suggestions">
          {suggestions.map((s, i) => (
            <li
              key={s}
              class={i === sel ? 'item selected' : 'item'}
              onMouseDown={(e) => {
                e.preventDefault();
                complete(s);
              }}
            >
              <span class="label">
                {token?.prefix}
                {s}
              </span>
            </li>
          ))}
        </ul>
      )}
      <div class="preview">
        {error ? (
          <span class="error">{setupError ? `${setupError} · Enter opens settings` : error}</span>
        ) : preview ? (
          <>
            <strong>{preview.title}</strong>
            {preview.tags.map((t) => {
              const c = tagColor(t, colors);
              return (
                <span key={t} class="chip" style={{ color: c, borderColor: `color-mix(in srgb, ${c} 50%, transparent)`, background: `color-mix(in srgb, ${c} 8%, transparent)` }}>
                  {t}
                </span>
              );
            })}
            {preview.due_date && <span class="chip plain">due {dueLabel(preview.due_date)}</span>}
            {preview.remind_at && <span class="chip plain">remind {dateTimeLabel(preview.remind_at)}</span>}
          </>
        ) : (
          <span class="caption">#tag · @project · !high/!1 · due:fri · remind:thu 9:00 · Tab completes · Enter adds · Shift+Enter adds and starts a timer</span>
        )}
      </div>
    </div>
  );
}
