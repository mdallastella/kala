import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  List,
  ListItemButton,
  ListItemText,
  Snackbar,
  Typography,
} from '@mui/material';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router';
import { get, post, streamEvents } from '../api/client';
import { invalidateKinds, useActions } from '../api/hooks';
import type { Briefing, Task, Wrapup } from '../api/types';
import { useHotkeys } from '../keys/hotkeys';
import { clock, dueLabel, isoDate, addDays } from '../lib/format';
import { TagChip } from './TagChip';
import { chipTags } from '../lib/tags';

type Digest = 'briefing' | 'wrapup';

function notifyBrowser(title: string, body: string) {
  if (typeof Notification === 'undefined' || Notification.permission !== 'granted') return;
  try {
    new Notification(title, { body, tag: `kala-${title}-${body}` });
  } catch {
    // Some browsers only allow notifications from service workers.
  }
}

/** Connects the SSE stream and renders reminders and daily digests. */
export function Notifications({ onConnected }: { onConnected: (ok: boolean) => void }) {
  const qc = useQueryClient();
  const [reminders, setReminders] = useState<Task[]>([]);
  // A pushed digest is announced first and opened on request, so a dialog
  // with single-key actions never pops up while the user is typing.
  const [pending, setPending] = useState<Digest | null>(null);
  const [digest, setDigest] = useState<Digest | null>(null);

  useEffect(
    () =>
      streamEvents(
        (type, data) => {
          if (type === 'changed') {
            invalidateKinds(qc, (data as { kinds: string[] }).kinds);
          } else if (type === 'reminder') {
            const t = data as Task;
            setReminders((rs) => (rs.some((r) => r.id === t.id) ? rs : [...rs, t]));
            notifyBrowser('Reminder', t.title);
          } else if (type === 'briefing' || type === 'wrapup') {
            setPending(type);
            notifyBrowser(type === 'briefing' ? 'Good morning' : 'End of the working day', 'Open Kala to review your day');
          }
        },
        () => {
          onConnected(true);
          // Catch up on anything missed while disconnected.
          invalidateKinds(qc, ['tasks', 'tags', 'timers', 'namespaces']);
        },
      ),
    [qc, onConnected],
  );

  const openDigest = () => {
    setDigest(pending);
    setPending(null);
  };
  const dismissDigest = () => {
    if (pending) void post(`/digest/${pending}/seen`);
    setPending(null);
  };
  useHotkeys(pending ? [{ keys: 'b', description: 'Open daily briefing / wrap-up', group: 'General', run: openDigest }] : [], !!pending);

  return (
    <>
      <ReminderSnackbar task={reminders[0] ?? null} onDone={() => setReminders((rs) => rs.slice(1))} />
      <Snackbar open={!!pending} anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}>
        <Alert
          severity={pending === 'wrapup' ? 'warning' : 'success'}
          sx={{ alignItems: 'center' }}
          action={
            <>
              <Button color="inherit" size="small" onClick={openDigest}>
                Open (b)
              </Button>
              <Button color="inherit" size="small" onClick={dismissDigest}>
                Dismiss
              </Button>
            </>
          }
        >
          {pending === 'wrapup' ? 'Your working day is over — time to wrap up.' : 'Your morning briefing is ready.'}
        </Alert>
      </Snackbar>
      {digest && <DigestDialog kind={digest} onClose={() => setDigest(null)} />}
    </>
  );
}

function ReminderSnackbar({ task, onDone }: { task: Task | null; onDone: () => void }) {
  const { ackReminder } = useActions();
  const navigate = useNavigate();
  const ack = (snooze?: number) => {
    if (!task) return;
    ackReminder.mutate({ id: task.id, snooze_until: snooze ? new Date(Date.now() + snooze * 60000).toISOString() : undefined });
    onDone();
  };
  return (
    <Snackbar open={!!task} anchorOrigin={{ vertical: 'top', horizontal: 'center' }}>
      <Alert
        severity="info"
        variant="filled"
        sx={{ alignItems: 'center' }}
        action={
          <>
            <Button color="inherit" size="small" onClick={() => { ack(); if (task) navigate(`/?task=${task.id}`); }}>
              Open
            </Button>
            <Button color="inherit" size="small" onClick={() => ack(10)}>
              Snooze 10m
            </Button>
            <Button color="inherit" size="small" onClick={() => ack()}>
              Dismiss
            </Button>
          </>
        }
      >
        {task?.title}
      </Alert>
    </Snackbar>
  );
}

function TaskLine({ t, focused, onClick }: { t: Task; focused: boolean; onClick: () => void }) {
  return (
    <ListItemButton selected={focused} onClick={onClick} dense>
      <ListItemText
        primary={t.title}
        secondary={
          <Box component="span" sx={{ display: 'inline-flex', gap: 0.5, flexWrap: 'wrap', alignItems: 'center' }}>
            {t.due_date && <span>{dueLabel(t.due_date)}</span>}
            {chipTags(t.tags).map((x) => (
              <TagChip key={x} name={x} component="span" />
            ))}
          </Box>
        }
        slotProps={{ secondary: { component: 'div' } }}
      />
    </ListItemButton>
  );
}

function Section({ title, tasks, offset, focus, setFocus }: { title: string; tasks: Task[]; offset: number; focus: number; setFocus: (i: number) => void }) {
  if (tasks.length === 0) return null;
  return (
    <>
      <Typography variant="overline" color="text.secondary">
        {title} · {tasks.length}
      </Typography>
      <List dense disablePadding>
        {tasks.map((t, i) => (
          <TaskLine key={t.id} t={t} focused={focus === offset + i} onClick={() => setFocus(offset + i)} />
        ))}
      </List>
    </>
  );
}

function DigestDialog({ kind, onClose }: { kind: Digest; onClose: () => void }) {
  const { data } = useQuery({ queryKey: ['digest', kind], queryFn: () => get<Briefing | Wrapup>(`/digest/${kind}`) });
  const { update, stopAll } = useActions();
  const [focus, setFocus] = useState(0);
  const navigate = useNavigate();

  const close = () => {
    void post(`/digest/${kind}/seen`);
    onClose();
  };

  const b = kind === 'briefing' ? (data as Briefing | undefined) : undefined;
  const w = kind === 'wrapup' ? (data as Wrapup | undefined) : undefined;
  const actionable = b ? [...b.overdue, ...b.due_today] : (w?.open ?? []);
  const current = actionable[focus];

  const act = (patch: { closed?: boolean; due_date?: string }) => {
    if (current) update.mutate({ id: current.id, ...patch });
  };
  const tomorrow = isoDate(addDays(new Date(), 1));

  useHotkeys([
    { keys: 'j', description: 'Next task', group: 'Digest', allowInDialog: true, run: () => setFocus((f) => Math.min(f + 1, actionable.length - 1)) },
    { keys: 'down', description: 'Next task', group: 'Digest', allowInDialog: true, hidden: true, run: () => setFocus((f) => Math.min(f + 1, actionable.length - 1)) },
    { keys: 'k', description: 'Previous task', group: 'Digest', allowInDialog: true, run: () => setFocus((f) => Math.max(f - 1, 0)) },
    { keys: 'up', description: 'Previous task', group: 'Digest', allowInDialog: true, hidden: true, run: () => setFocus((f) => Math.max(f - 1, 0)) },
    { keys: 'x', description: 'Close task', group: 'Digest', allowInDialog: true, run: () => act({ closed: true }) },
    { keys: 't', description: 'Postpone to tomorrow', group: 'Digest', allowInDialog: true, run: () => act({ due_date: tomorrow }) },
    { keys: 'n', description: 'Postpone to next working day', group: 'Digest', allowInDialog: true, run: () => w && act({ due_date: w.next_work_date }) },
    { keys: 'S', description: 'Stop all timers', group: 'Digest', allowInDialog: true, run: () => stopAll.mutate() },
    { keys: 'enter', description: 'Open task', group: 'Digest', allowInDialog: true, run: () => { if (current) { close(); navigate(`/?task=${current.id}`); } } },
  ]);

  return (
    <Dialog open onClose={close} fullWidth maxWidth="sm">
      <DialogTitle>{kind === 'briefing' ? 'Good morning ☀️' : 'Wrapping up the day'}</DialogTitle>
      <DialogContent dividers>
        {!data && <Typography>Loading…</Typography>}
        {b && (
          <>
            {b.overdue.length + b.due_today.length === 0 && <Typography>Nothing due today. Enjoy!</Typography>}
            <Section title="Overdue" tasks={b.overdue} offset={0} focus={focus} setFocus={setFocus} />
            <Section title="Due today" tasks={b.due_today} offset={b.overdue.length} focus={focus} setFocus={setFocus} />
            {b.reminders.length > 0 && (
              <>
                <Typography variant="overline" color="text.secondary">
                  Reminders today
                </Typography>
                {b.reminders.map((t) => (
                  <Typography key={t.id} variant="body2">
                    {new Date(t.remind_at!).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })} · {t.title}
                  </Typography>
                ))}
              </>
            )}
          </>
        )}
        {w && (
          <>
            {w.running.length > 0 && (
              <Alert severity="warning" sx={{ mb: 2 }} action={<Button color="inherit" size="small" onClick={() => stopAll.mutate()}>Stop all (S)</Button>}>
                {w.running.length} timer{w.running.length > 1 ? 's' : ''} still running:{' '}
                {w.running.map((e) => `${e.task_title} (${clock(e.seconds)})`).join(', ')}
              </Alert>
            )}
            {w.open.length === 0 && <Typography>All of today's tasks are done. 🎉</Typography>}
            <Section title="Still open" tasks={w.open} offset={0} focus={focus} setFocus={setFocus} />
            {w.next.length > 0 && (
              <>
                <Typography variant="overline" color="text.secondary">
                  Next working day · {dueLabel(w.next_work_date)}
                </Typography>
                {w.next.map((t) => (
                  <Typography key={t.id} variant="body2">
                    {t.title}
                  </Typography>
                ))}
              </>
            )}
          </>
        )}
      </DialogContent>
      <DialogActions sx={{ justifyContent: 'space-between' }}>
        <Typography variant="caption" color="text.secondary" sx={{ pl: 1 }}>
          j/k move · x close · t tomorrow · n next working day · Enter open
        </Typography>
        <Button onClick={close}>Done</Button>
      </DialogActions>
    </Dialog>
  );
}
