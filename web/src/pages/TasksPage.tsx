import { Alert, Box, Chip, List, Typography } from '@mui/material';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useCurrentQuery } from '../lib/useCurrentQuery';
import { useSearchParams } from 'react-router';
import { errorMessage } from '../api/client';
import { useActions, useNamespaces, useTasks } from '../api/hooks';
import type { Task } from '../api/types';
import { TaskDetail } from '../components/TaskDetail';
import { TaskRow } from '../components/TaskRow';
import { useNow } from '../components/TimerStrip';
import { useUi } from '../components/ui';
import { useHotkeys, type Binding } from '../keys/hotkeys';
import { dragTasks } from '../lib/dnd';
import { namespaceValues, statusOf } from '../lib/tags';

export function TasksPage() {
  const [params, setParams] = useSearchParams();
  const { q } = useCurrentQuery();
  const ui = useUi();
  const taskId = Number(params.get('task')) || null;

  const { data: tasks = [], error, isPending } = useTasks(q);
  const { data: nss } = useNamespaces();
  const actions = useActions();
  const now = useNow();

  // Focus follows a task id; when that task leaves the list (closed,
  // deleted) it falls back to the row at the last known position.
  const [focusState, setFocusState] = useState<{ id: number | null; index: number }>({ id: null, index: 0 });
  const [selection, setSelection] = useState<Set<number>>(new Set());
  const [anchor, setAnchor] = useState<number | null>(null);
  const [focusTitle, setFocusTitle] = useState(0);
  const listRef = useRef<HTMLUListElement>(null);

  // Changing the filter clears the selection.
  const [selectionQuery, setSelectionQuery] = useState(q);
  if (selectionQuery !== q) {
    setSelectionQuery(q);
    setSelection(new Set());
  }

  let index = tasks.findIndex((t) => t.id === focusState.id);
  if (index < 0) index = Math.min(focusState.index, tasks.length - 1);
  const focused: Task | undefined = tasks[index];
  const setFocusId = (id: number) => setFocusState({ id, index: Math.max(0, tasks.findIndex((t) => t.id === id)) });

  useEffect(() => {
    if (focused) listRef.current?.querySelector(`[data-task-id="${focused.id}"]`)?.scrollIntoView({ block: 'nearest' });
  }, [focused]);

  const openTask = useCallback(
    (id: number | null) =>
      setParams(
        (p) => {
          const n = new URLSearchParams(p);
          if (id == null) n.delete('task');
          else n.set('task', String(id));
          return n;
        },
        { replace: true },
      ),
    [setParams],
  );

  const targets = (): Task[] => {
    if (selection.size > 0) return tasks.filter((t) => selection.has(t.id));
    return focused ? [focused] : [];
  };
  const fail = (e: unknown) => ui.notify(errorMessage(e));
  const move = (delta: number) => {
    const t = tasks[Math.max(0, Math.min(tasks.length - 1, index + delta))];
    if (t) setFocusId(t.id);
  };
  const bulkOrSingle = (ts: Task[], op: 'close' | 'reopen') => {
    if (ts.length === 1) actions.update.mutate({ id: ts[0].id, closed: op === 'close' }, { onError: fail });
    else actions.bulk.mutate({ ids: ts.map((t) => t.id), op }, { onError: fail });
  };
  const toggleClosed = (ts = targets()) => {
    if (ts.length) bulkOrSingle(ts, ts.every((t) => t.closed_at) ? 'reopen' : 'close');
  };
  const addTags = (tags: string[]) => {
    const ts = targets();
    if (ts.length) actions.bulk.mutate({ ids: ts.map((t) => t.id), op: 'add_tags', tags }, { onError: fail });
  };
  const priorities = namespaceValues(nss, 'priority');
  const setPriority = (n: number) => {
    const ts = targets();
    if (!ts.length) return;
    if (n === 0) {
      actions.bulk.mutate({ ids: ts.map((t) => t.id), op: 'remove_tags', tags: priorities.map((p) => `priority:${p}`) }, { onError: fail });
    } else if (priorities[n - 1]) addTags([`priority:${priorities[n - 1]}`]);
  };
  const toggleStatus = (status: string) => {
    const ts = targets();
    if (!ts.length) return;
    if (ts.every((t) => statusOf(t.tags) === status)) bulkOrSingle(ts, 'reopen');
    else addTags([`status:${status}`]);
  };
  const toggleTimer = () => {
    for (const t of targets()) (t.running_since ? actions.stop : actions.start).mutate(t.id, { onError: fail });
  };
  const deleteTargets = () => {
    const ts = targets();
    if (!ts.length) return;
    const ids = ts.map((t) => t.id);
    actions.bulk.mutate(
      { ids, op: 'delete' },
      {
        onSuccess: () => {
          setSelection(new Set());
          if (taskId && ids.includes(taskId)) openTask(null);
          ui.notify(ids.length === 1 ? `Deleted “${ts[0].title}”` : `Deleted ${ids.length} tasks`, {
            label: 'Undo',
            run: () => actions.bulk.mutate({ ids, op: 'restore' }),
          });
        },
        onError: fail,
      },
    );
  };
  const toggleSelect = () => {
    if (!focused) return;
    setSelection((s) => {
      const n = new Set(s);
      if (n.has(focused.id)) n.delete(focused.id);
      else n.add(focused.id);
      return n;
    });
    setAnchor(focused.id);
    move(1);
  };
  const rangeSelect = () => {
    if (!focused) return;
    const a = tasks.findIndex((t) => t.id === anchor);
    const [from, to] = a < 0 ? [index, index] : [Math.min(a, index), Math.max(a, index)];
    setSelection(new Set(tasks.slice(from, to + 1).map((t) => t.id)));
    if (a < 0) setAnchor(focused.id);
  };

  const g = 'Tasks';
  const bindings: Binding[] = [
    { keys: 'j', description: 'Next task', group: g, run: () => move(1) },
    { keys: 'down', description: 'Next task', group: g, hidden: true, run: () => move(1) },
    { keys: 'k', description: 'Previous task', group: g, run: () => move(-1) },
    { keys: 'up', description: 'Previous task', group: g, hidden: true, run: () => move(-1) },
    { keys: 'home', description: 'First task', group: g, run: () => tasks[0] && setFocusId(tasks[0].id) },
    { keys: 'G', description: 'Last task', group: g, run: () => tasks.length && setFocusId(tasks[tasks.length - 1].id) },
    { keys: 'end', description: 'Last task', group: g, hidden: true, run: () => tasks.length && setFocusId(tasks[tasks.length - 1].id) },
    { keys: 'enter', description: 'Open task details', group: g, run: () => focused && openTask(focused.id) },
    { keys: 'o', description: 'Open task details', group: g, hidden: true, run: () => focused && openTask(focused.id) },
    { keys: 'e', description: 'Edit title', group: g, run: () => { if (focused) { openTask(focused.id); setFocusTitle((n) => n + 1); } } },
    { keys: 'escape', description: 'Close details / clear selection', group: g, run: () => (taskId ? openTask(null) : setSelection(new Set())) },
    { keys: 'x', description: 'Close / reopen', group: g, run: () => toggleClosed() },
    { keys: 'w', description: 'Toggle pending', group: g, run: () => toggleStatus('pending') },
    { keys: 'D', description: 'Toggle delegated', group: g, run: () => toggleStatus('delegated') },
    { keys: 'c', description: 'Cancel / reopen', group: g, run: () => toggleStatus('cancelled') },
    { keys: '1', description: 'Priority high', group: g, run: () => setPriority(1) },
    { keys: '2', description: 'Priority medium', group: g, run: () => setPriority(2) },
    { keys: '3', description: 'Priority low', group: g, run: () => setPriority(3) },
    { keys: '0', description: 'Clear priority', group: g, run: () => setPriority(0) },
    { keys: 's', description: 'Start / stop timer', group: g, run: toggleTimer },
    { keys: 'd', description: 'Set due date', group: g, run: () => targets().length && ui.pickDate(targets(), 'due') },
    { keys: 'r', description: 'Set reminder', group: g, run: () => targets().length && ui.pickDate(targets(), 'remind') },
    { keys: 't', description: 'Edit tags', group: g, run: () => targets().length && ui.pickTags(targets()) },
    { keys: 'p', description: 'Edit project', group: g, run: () => targets().length && ui.pickTags(targets(), 'project') },
    { keys: 'delete', description: 'Delete (undo available)', group: g, run: deleteTargets },
    { keys: '#', description: 'Delete (undo available)', group: g, hidden: true, run: deleteTargets },
    { keys: 'space', description: 'Toggle selection', group: 'Selection', run: toggleSelect },
    { keys: 'V', description: 'Select range from last selected', group: 'Selection', run: rangeSelect },
    { keys: '* a', description: 'Select all', group: 'Selection', run: () => setSelection(new Set(tasks.map((t) => t.id))) },
    { keys: '* n', description: 'Select none', group: 'Selection', run: () => setSelection(new Set()) },
  ];
  useHotkeys(bindings);

  const selectedCount = selection.size;
  const rows = useMemo(() => tasks, [tasks]);

  return (
    <Box sx={{ display: 'flex', flex: 1, minHeight: 0 }}>
      <Box sx={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        <Box sx={{ px: 2, py: 1.5, display: 'flex', alignItems: 'baseline', gap: 1.5, borderBottom: 1, borderColor: 'divider' }}>
          <Typography variant="h6" sx={{ fontWeight: 700 }}>
            Tasks
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {tasks.length} task{tasks.length === 1 ? '' : 's'}
          </Typography>
          <Typography variant="caption" color="text.disabled" sx={{ fontFamily: 'monospace' }} noWrap>
            {q}
          </Typography>
          <Box sx={{ flex: 1 }} />
          {selectedCount > 0 && (
            <Chip color="primary" label={`${selectedCount} selected · x t p d 1-3 Del · Esc clears`} onDelete={() => setSelection(new Set())} />
          )}
        </Box>
        {error && (
          <Alert severity="error" sx={{ m: 2 }}>
            {errorMessage(error)}
          </Alert>
        )}
        <List ref={listRef} disablePadding sx={{ overflowY: 'auto', flex: 1 }}>
          {rows.map((t) => (
            <TaskRow
              key={t.id}
              task={t}
              now={t.running_since ? now : 0}
              focused={t.id === focused?.id}
              selected={selection.has(t.id)}
              open={t.id === taskId}
              onClick={(e) => {
                setFocusId(t.id);
                if (e.shiftKey || e.metaKey || e.ctrlKey) {
                  setSelection((s) => {
                    const n = new Set(s);
                    if (n.has(t.id)) n.delete(t.id);
                    else n.add(t.id);
                    return n;
                  });
                } else openTask(t.id);
              }}
              onToggle={() => toggleClosed([t])}
              onDragStart={(e) => dragTasks(e, selection.has(t.id) ? [...selection] : [t.id])}
            />
          ))}
          {!isPending && !error && tasks.length === 0 && (
            <Box sx={{ p: 4, textAlign: 'center', color: 'text.secondary' }}>
              <Typography>Nothing here.</Typography>
              <Typography variant="body2">
                Press <b>q</b> to add a task, <b>/</b> to change the filter or <b>?</b> for all shortcuts.
              </Typography>
            </Box>
          )}
        </List>
      </Box>
      {taskId && (
        <Box sx={{ width: 420, flexShrink: 0, borderLeft: 1, borderColor: 'divider', bgcolor: 'background.paper' }}>
          <TaskDetail key={taskId} id={taskId} focusTitle={focusTitle} onClose={() => openTask(null)} />
        </Box>
      )}
    </Box>
  );
}
