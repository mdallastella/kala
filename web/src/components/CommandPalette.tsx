import { Box, Dialog, List, ListItemButton, ListItemText, ListSubheader, TextField, Typography } from '@mui/material';
import { useEffect, useMemo, useRef, useState } from 'react';
import { get } from '../api/client';
import type { Task } from '../api/types';
import { keyLabel } from '../keys/matcher';

export interface Command {
  id: string;
  label: string;
  group: string;
  keys?: string;
  hint?: string;
  run: () => void;
}

interface Props {
  prefill: string;
  commands: Command[];
  onOpenTask: (t: Task) => void;
  onClose: () => void;
}

function score(label: string, q: string): number {
  const l = label.toLowerCase();
  if (!q) return 1;
  if (l.startsWith(q)) return 3;
  if (l.includes(q)) return 2;
  // Subsequence match ("tdy" → "today").
  let i = 0;
  for (const c of l) if (c === q[i]) i++;
  return i === q.length ? 1 : 0;
}

export function CommandPalette({ prefill, commands, onOpenTask, onClose }: Props) {
  const [q, setQ] = useState(prefill);
  const [sel, setSel] = useState(0);
  const [found, setFound] = useState<Task[]>([]);
  const listRef = useRef<HTMLUListElement>(null);
  const searchable = q.trim().length >= 2;
  const tasks = useMemo(() => (searchable ? found : []), [searchable, found]);

  useEffect(() => {
    const text = q.trim();
    if (text.length < 2) return;
    const t = setTimeout(() => {
      get<Task[]>(`/tasks?limit=8&q=${encodeURIComponent(`"${text.replace(/"/g, '')}"`)}`)
        .then(setFound)
        .catch(() => setFound([]));
    }, 150);
    return () => clearTimeout(t);
  }, [q]);

  const items = useMemo(() => {
    const needle = q.trim().toLowerCase();
    const cmds = commands
      .map((c) => ({ c, s: score(c.label, needle) + (needle && c.hint?.toLowerCase().includes(needle) ? 1 : 0) }))
      .filter((x) => x.s > 0)
      .sort((a, b) => b.s - a.s)
      .slice(0, 40)
      .map((x) => x.c);
    const taskCmds: Command[] = tasks.map((t) => ({
      id: `task-${t.id}`,
      label: t.title,
      group: 'Tasks',
      hint: t.tags.filter((x) => x.startsWith('project:')).join(' '),
      run: () => onOpenTask(t),
    }));
    return [...cmds, ...taskCmds];
  }, [q, commands, tasks, onOpenTask]);

  useEffect(() => {
    listRef.current?.querySelector('.Mui-selected')?.scrollIntoView({ block: 'nearest' });
  }, [sel]);

  const run = (c: Command | undefined) => {
    if (!c) return;
    onClose();
    c.run();
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown' || (e.ctrlKey && e.key === 'n')) {
      e.preventDefault();
      setSel((s) => Math.min(s + 1, items.length - 1));
    } else if (e.key === 'ArrowUp' || (e.ctrlKey && e.key === 'p')) {
      e.preventDefault();
      setSel((s) => Math.max(s - 1, 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      run(items[sel]);
    }
  };

  let lastGroup = '';
  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="sm" slotProps={{ paper: { sx: { position: 'fixed', top: '10vh', m: 0 } } }}>
      <Box sx={{ p: 1.5, borderBottom: 1, borderColor: 'divider' }}>
        <TextField
          autoFocus
          fullWidth
          variant="standard"
          placeholder="Type a command, view, tag or task…"
          value={q}
          onChange={(e) => {
            setQ(e.target.value);
            setSel(0);
          }}
          onKeyDown={onKeyDown}
          slotProps={{ input: { disableUnderline: true, sx: { fontSize: 16 } } }}
        />
      </Box>
      <List ref={listRef} dense sx={{ maxHeight: '60vh', overflow: 'auto', py: 0 }}>
        {items.length === 0 && (
          <Typography variant="body2" color="text.secondary" sx={{ p: 2 }}>
            Nothing matches.
          </Typography>
        )}
        {items.map((c, i) => {
          const header = c.group !== lastGroup ? (lastGroup = c.group) : null;
          return (
            <Box key={c.id}>
              {header && <ListSubheader sx={{ lineHeight: '28px' }}>{header}</ListSubheader>}
              <ListItemButton selected={i === sel} onClick={() => run(c)} onMouseMove={() => setSel(i)}>
                <ListItemText primary={c.label} secondary={c.hint} slotProps={{ secondary: { noWrap: true } }} />
                {c.keys && (
                  <Typography variant="caption" color="text.secondary" sx={{ ml: 2, whiteSpace: 'nowrap', fontFamily: 'monospace' }}>
                    {keyLabel(c.keys)}
                  </Typography>
                )}
              </ListItemButton>
            </Box>
          );
        })}
      </List>
    </Dialog>
  );
}
