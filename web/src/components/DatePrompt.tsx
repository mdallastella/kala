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
  const { update, bulk } = useActions();

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
    const t = setTimeout(() => {
      parseQuick(`x ${kind}:${e}`)
        .then((p) => {
          setValue(kind === 'due' ? (p.due_date ?? null) : (p.remind_at ?? null));
          setError('');
        })
        .catch((err) => setError(errorMessage(err)));
    }, 120);
    return () => clearTimeout(t);
  }, [expr, kind]);

  const save = async () => {
    try {
      if (kind === 'due' && tasks.length > 1) {
        await bulk.mutateAsync({ ids: tasks.map((t) => t.id), op: 'set_due', due_date: value });
      } else {
        for (const t of tasks) await update.mutateAsync({ id: t.id, [kind === 'due' ? 'due_date' : 'remind_at']: value });
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
              if (e.key === 'Enter' && !error) {
                e.preventDefault();
                void save();
              }
            }}
            error={!!error}
            helperText={error || shown}
          />
          {kind === 'due' ? (
            <TextField type="date" label="Pick a date" value={value ?? ''} onChange={(e) => setValue(e.target.value || null)} slotProps={{ inputLabel: { shrink: true } }} />
          ) : (
            <DateTimeField label="Pick a time" value={toLocalInput(value)} onChange={(v) => setValue(fromLocalInput(v))} />
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
