import { Button, Dialog, DialogActions, DialogContent, DialogTitle, Stack, TextField, Typography } from '@mui/material';
import { useEffect, useState } from 'react';
import { errorMessage } from '../api/client';
import { parseQuick, useActions } from '../api/hooks';
import type { Task } from '../api/types';
import { dateTimeLabel, dueLabel, fromLocalInput, toLocalInput } from '../lib/format';
import { DateTimeField } from './DateTimeField';

interface Props {
  tasks: Task[];
  kind: 'due' | 'remind';
  onClose: () => void;
  onError: (msg: string) => void;
}

const CLEAR = ['none', 'no', '-', 'clear'];

/**
 * Sets the due date or reminder of tasks. Accepts the same expressions as
 * quick-add (fri, +3d, nwd, thu 9:00, +30m) or a picked value.
 */
export function DatePrompt({ tasks, kind, onClose, onError }: Props) {
  const current = tasks.length === 1 ? (kind === 'due' ? tasks[0].due_date : tasks[0].remind_at) : null;
  const [expr, setExpr] = useState('');
  const [value, setValue] = useState<string | null>(current); // due: YYYY-MM-DD, remind: ISO
  const [error, setError] = useState('');
  // The expression value was last taken from; typing ahead of the server leaves them apart.
  const [resolved, setResolved] = useState('');
  const { update, bulk } = useActions();

  const resolve = (e: string) => parseQuick(`x ${kind}:${e}`).then((p) => (kind === 'due' ? (p.due_date ?? null) : (p.remind_at ?? null)));
  const pick = (v: string | null) => {
    setValue(v);
    setResolved(expr.trim());
    setError('');
  };

  const onExpr = (v: string) => {
    setExpr(v);
    if (CLEAR.includes(v.trim().toLowerCase())) {
      setValue(null);
      setError('');
    }
  };

  useEffect(() => {
    const e = expr.trim();
    if (!e || CLEAR.includes(e.toLowerCase())) return;
    let stale = false;
    const t = setTimeout(() => {
      resolve(e)
        .then((v) => {
          if (stale) return;
          setValue(v);
          setResolved(e);
          setError('');
        })
        .catch((err) => !stale && setError(errorMessage(err)));
    }, 120);
    return () => {
      stale = true;
      clearTimeout(t);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- resolve only depends on kind
  }, [expr, kind]);

  const save = async () => {
    // Enter can come before the typed expression was parsed: parse it now rather than save the
    // value of an earlier one.
    let v = value;
    const e = expr.trim();
    if (e && !CLEAR.includes(e.toLowerCase()) && e !== resolved) {
      try {
        v = await resolve(e);
      } catch (err) {
        setError(errorMessage(err));
        return;
      }
    }
    try {
      const ids = tasks.map((t) => t.id);
      if (ids.length > 1) {
        await bulk.mutateAsync(kind === 'due' ? { ids, op: 'set_due', due_date: v } : { ids, op: 'set_remind', remind_at: v });
      } else if (ids.length === 1) {
        await update.mutateAsync({ id: ids[0], [kind === 'due' ? 'due_date' : 'remind_at']: v });
      }
      onClose();
    } catch (e) {
      onError(errorMessage(e));
    }
  };

  const shown = value ? (kind === 'due' ? dueLabel(value) : dateTimeLabel(value)) : 'No ' + (kind === 'due' ? 'due date' : 'reminder');

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="xs">
      <DialogTitle sx={{ pb: 1 }}>{kind === 'due' ? 'Due date' : 'Reminder'}</DialogTitle>
      <DialogContent>
        <Stack spacing={1.5} sx={{ mt: 0.5 }}>
          <TextField
            autoFocus
            fullWidth
            label={kind === 'due' ? 'fri, tomorrow, +3d, nwd, 2026-11-02, none' : 'thu 9:00, 14:30, +30m, tomorrow, none'}
            value={expr}
            onChange={(e) => onExpr(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault();
                void save();
              }
            }}
            error={!!error}
            helperText={error || shown}
          />
          {kind === 'due' ? (
            <TextField type="date" label="Pick a date" value={value ?? ''} onChange={(e) => pick(e.target.value || null)} slotProps={{ inputLabel: { shrink: true } }} />
          ) : (
            <DateTimeField label="Pick a time" value={toLocalInput(value)} onChange={(v) => pick(fromLocalInput(v))} />
          )}
          {tasks.length > 1 && (
            <Typography variant="caption" color="text.secondary">
              Applies to {tasks.length} tasks
            </Typography>
          )}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => { setValue(null); setExpr(''); }}>Clear</Button>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" disabled={!!error} onClick={() => void save()}>
          Save
        </Button>
      </DialogActions>
    </Dialog>
  );
}
