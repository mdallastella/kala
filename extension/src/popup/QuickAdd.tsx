import { useEffect, useMemo, useRef, useState } from 'preact/hooks';
import type { TargetedEvent } from 'preact';
import { errorMessage, get, post, send } from '../lib/api';
import { dateTimeLabel, dueLabel, duration, timeLabel } from '../lib/format';
import { currentToken, searchQuery, tagColor } from '../lib/quickadd';
import type { QuickParse, Tag, Task, TimeEntry } from '../lib/types';

interface Props {
  tags: Tag[];
  /** Why the extension cannot reach kala yet; Enter then opens the settings. */
  setupError: string;
  onDone: (message: string) => void;
}

/** Open tasks most recently created or timed. */
const recentQuery = 'is:open sort:-activity';

/**
 * Port of web/src/components/QuickAdd.tsx: Enter adds, Shift+Enter also starts a timer.
 * Running timers and the latest open tasks (or, once typing, those matching the title) are
 * listed below; on a selected one Enter opens it in kala and Shift+Enter starts or stops its
 * timer. Alt+n picks the nth task.
 */
export function QuickAdd({ tags, setupError, onDone }: Props) {
  const [text, setText] = useState('');
  // Parse result for a given text; stale results are ignored when rendering.
  const [parsed, setParsed] = useState<{ text: string; preview?: QuickParse; error?: string }>({ text: '' });
  const [submitError, setSubmitError] = useState('');
  const [busy, setBusy] = useState(false);
  const [shift, setShift] = useState(false);
  const [sel, setSel] = useState(0);
  // Search results for a query; pick is the selected match, -1 while adding a new task.
  const [found, setFound] = useState<{ q: string; tasks: Task[] }>({ q: '', tasks: [] });
  const [pick, setPick] = useState(-1);
  const [recent, setRecent] = useState<Task[]>([]);
  const [timers, setTimers] = useState<TimeEntry[]>([]);
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

  // Search by the parsed title so tag, due and remind tokens don't narrow the matches.
  const query = !empty && !setupError && preview ? searchQuery(preview.title) : null;
  useEffect(() => {
    if (!query) return;
    get<Task[]>(`/tasks?limit=6&q=${encodeURIComponent(query)}`)
      .then((tasks) => setFound({ q: query, tasks }))
      .catch(() => setFound({ q: query, tasks: [] }));
  }, [query]);
  useEffect(() => {
    if (setupError) return;
    get<Task[]>(`/tasks?limit=5&q=${encodeURIComponent(recentQuery)}`)
      .then(setRecent)
      .catch(() => setRecent([]));
    get<TimeEntry[]>('/timers/active')
      .then(setTimers)
      .catch(() => setTimers([]));
  }, [setupError]);
  const running = useMemo(() => new Set(timers.map((e) => e.task_id)), [timers]);
  // Previous matches stay up while the next search runs, to avoid flicker.
  const matches = (empty ? recent : query ? found.tasks : []).filter((t) => !running.has(t.id));
  // Selectable rows: running timers, then matches; pick indexes into this.
  const rows: Task[] = [...timers.map((e) => ({ id: e.task_id, title: e.task_title, tags: e.task_tags })), ...matches];
  useEffect(() => setPick(-1), [found]);

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

  const act = async (task: Task, timer: boolean) => {
    if (busy) return;
    if (!timer) {
      void send({ type: 'open-task', id: task.id });
      return;
    }
    const stop = running.has(task.id);
    setBusy(true);
    try {
      const t = await post<Task>(`/tasks/${task.id}/timer/${stop ? 'stop' : 'start'}`);
      onDone(`${stop ? 'Stopped' : 'Started'} timer on “${t.title}”`);
    } catch (e) {
      setSubmitError(errorMessage(e));
      setBusy(false);
    }
  };

  const onKeyDown = (e: KeyboardEvent) => {
    setShift(e.shiftKey);
    // e.code, not e.key: Shift turns the digit into a symbol on most layouts.
    const n = e.altKey && /^Digit[1-9]$/.test(e.code) ? Number(e.code.slice(5)) : 0;
    if (n) {
      e.preventDefault();
      const task = matches[n - 1];
      if (task) void act(task, e.shiftKey);
      return;
    }
    if (suggestions.length > 0 && (e.key === 'Tab' || e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
      e.preventDefault();
      if (e.key === 'Tab') complete(suggestions[sel] ?? suggestions[0]);
      else setSel((s) => (s + (e.key === 'ArrowDown' ? 1 : suggestions.length - 1)) % suggestions.length);
      return;
    }
    if (rows.length > 0 && (e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
      e.preventDefault();
      // Cycles through the rows and back to adding a new task (-1).
      const n = rows.length + 1;
      setPick((p) => ((p + 1 + (e.key === 'ArrowDown' ? 1 : n - 1)) % n) - 1);
      return;
    }
    if (e.key === 'Tab') e.preventDefault(); // keep focus in the input
    if (e.key === 'Enter') {
      e.preventDefault();
      const task = rows[pick];
      void (task ? act(task, e.shiftKey) : submit(e.shiftKey));
    }
  };

  const row = (t: Task, i: number, key: string, extra?: string) => {
    const hint = [extra, ...t.tags.filter((x) => x.startsWith('project:')).map((x) => `@${x.slice(8)}`)].filter(Boolean).join(' · ');
    return (
      <li
        key={t.id}
        class={i === pick ? 'item task selected' : 'item task'}
        onMouseMove={() => setPick(i)}
        onMouseDown={(e) => {
          e.preventDefault();
          void act(t, e.shiftKey);
        }}
      >
        <span class="key">{key}</span>
        <span class="body">
          <span class="label">{t.title}</span>
          {hint && <span class="label hint">{hint}</span>}
        </span>
      </li>
    );
  };

  return (
    <div class="quick">
      <div class="head">
        {shift && !setupError && <span class="badge">{rows[pick] && running.has(rows[pick].id) ? 'stop' : 'timer'}</span>}
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
            setPick(-1);
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
        ) : rows[pick] ? (
          <span class="caption">
            Enter opens “{rows[pick].title}” in kala · Shift+Enter {running.has(rows[pick].id) ? 'stops' : 'starts'} its timer · ↑ back to adding
          </span>
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
            {preview.spent && (
              <span class="chip plain">
                spent {duration((new Date(preview.spent.stopped_at).getTime() - new Date(preview.spent.started_at).getTime()) / 1000)} ·{' '}
                {timeLabel(preview.spent.started_at)}–{timeLabel(preview.spent.stopped_at)}
              </span>
            )}
          </>
        ) : (
          <span class="caption">#tag · @project · !high/!1 · due:fri · remind:thu 9:00 · spent:1h30m 9:00 · Tab completes · Enter adds · Shift+Enter adds and starts a timer</span>
        )}
      </div>
      {timers.length > 0 && (
        <ul class="list matches" aria-label="Running timers">
          <li class="section">Running timers · Shift+Enter stops</li>
          {timers.map((e, i) => row(rows[i], i, '▶', `since ${timeLabel(e.started_at)}`))}
        </ul>
      )}
      {matches.length > 0 && (
        <ul class="list matches" aria-label="Existing tasks">
          <li class="section">{empty ? 'Recent tasks' : 'Existing tasks'} · ↓ or Alt+number to pick</li>
          {matches.map((t, i) => row(t, timers.length + i, String(i + 1)))}
        </ul>
      )}
    </div>
  );
}
