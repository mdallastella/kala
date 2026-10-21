import {
  Autocomplete,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  LinearProgress,
  List,
  ListItemButton,
  ListItemText,
  MenuItem,
  Paper,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import ChevronLeftIcon from '@mui/icons-material/ChevronLeft';
import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import DeleteOutlinedIcon from '@mui/icons-material/DeleteOutlined';
import { useMemo, useState } from 'react';
import { errorMessage, get } from '../api/client';
import { useActions, useEntries, useNamespaces, useReport } from '../api/hooks';
import type { Task, TimeEntry } from '../api/types';
import { TagChip } from '../components/TagChip';
import { useNow } from '../components/TimerStrip';
import { WeekTimeline } from '../components/WeekTimeline';
import { useUi } from '../components/ui';
import { useHotkeys } from '../keys/hotkeys';
import { addDays, dueLabel, duration, fromLocalInput, isoDate, startOfWeek, timeLabel, toLocalInput } from '../lib/format';
import { chipTags } from '../lib/tags';

export function TimePage() {
  const [week, setWeek] = useState(() => startOfWeek(new Date()));
  const [group, setGroup] = useState('project');
  const [editing, setEditing] = useState<TimeEntry | 'new' | null>(null);
  const [focus, setFocus] = useState(0);
  const { data: nss = [] } = useNamespaces();
  const ui = useUi();
  const now = useNow();
  const from = week.toISOString();
  const to = addDays(week, 7).toISOString();
  const { data: report } = useReport(from, to, group);
  const { data: entries = [] } = useEntries(from, to);
  const { deleteEntry } = useActions();
  const groups = ['day', 'task', 'tag', ...nss.filter((n) => !n.exclusive).map((n) => n.name), ...nss.filter((n) => n.exclusive).map((n) => n.name)];

  const byDay = useMemo(() => {
    const m = new Map<string, TimeEntry[]>();
    for (const e of entries) {
      const d = isoDate(new Date(e.started_at));
      m.set(d, [...(m.get(d) ?? []), e]);
    }
    return [...m];
  }, [entries]);
  const flat = byDay.flatMap(([, es]) => es);
  const current = flat[focus];

  const del = (e: TimeEntry) => deleteEntry.mutate(e.id, { onSuccess: () => ui.notify('Time entry deleted'), onError: (err) => ui.notify(errorMessage(err)) });

  useHotkeys([
    { keys: '[', description: 'Previous week', group: 'Time', run: () => setWeek((w) => addDays(w, -7)) },
    { keys: ']', description: 'Next week', group: 'Time', run: () => setWeek((w) => addDays(w, 7)) },
    { keys: '.', description: 'This week', group: 'Time', run: () => setWeek(startOfWeek(new Date())) },
    { keys: 'v', description: 'Cycle report grouping', group: 'Time', run: () => setGroup((g) => groups[(groups.indexOf(g) + 1) % groups.length]) },
    { keys: 'a', description: 'Add time entry', group: 'Time', run: () => setEditing('new') },
    { keys: 'j', description: 'Next entry', group: 'Time', run: () => setFocus((f) => Math.min(f + 1, flat.length - 1)) },
    { keys: 'k', description: 'Previous entry', group: 'Time', run: () => setFocus((f) => Math.max(f - 1, 0)) },
    { keys: 'enter', description: 'Edit entry', group: 'Time', run: () => current && setEditing(current) },
    { keys: 'delete', description: 'Delete entry', group: 'Time', run: () => current && del(current) },
  ]);

  const max = Math.max(1, ...(report?.rows.map((r) => r.seconds) ?? []));
  let i = -1;

  return (
    <Box sx={{ p: 2, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Typography variant="h6" sx={{ fontWeight: 700, mr: 1 }}>
          Time
        </Typography>
        <Tooltip title="Previous week ([)">
          <IconButton size="small" onClick={() => setWeek(addDays(week, -7))}>
            <ChevronLeftIcon />
          </IconButton>
        </Tooltip>
        <Typography variant="body2" sx={{ minWidth: 190, textAlign: 'center' }}>
          {week.toLocaleDateString(undefined, { day: 'numeric', month: 'short' })} – {addDays(week, 6).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' })}
        </Typography>
        <Tooltip title="Next week (])">
          <IconButton size="small" onClick={() => setWeek(addDays(week, 7))}>
            <ChevronRightIcon />
          </IconButton>
        </Tooltip>
        <Button size="small" onClick={() => setWeek(startOfWeek(new Date()))}>
          This week (.)
        </Button>
        <Box sx={{ flex: 1 }} />
        <TextField select label="Group by (v)" value={group} onChange={(e) => setGroup(e.target.value)} sx={{ minWidth: 160 }}>
          {groups.map((g) => (
            <MenuItem key={g} value={g}>
              {g}
            </MenuItem>
          ))}
        </TextField>
        <Button variant="outlined" onClick={() => setEditing('new')}>
          Add entry (a)
        </Button>
      </Box>

      <WeekTimeline entries={entries} week={week} now={now} group={group} onSelect={setEditing} />

      <Paper variant="outlined" sx={{ p: 2 }}>
        <Typography variant="overline" color="text.secondary">
          Total {duration(report?.total_seconds ?? 0)}
        </Typography>
        {report?.rows.map((r) => (
          <Box key={r.key} sx={{ display: 'grid', gridTemplateColumns: '220px 1fr 80px', alignItems: 'center', gap: 2, py: 0.5 }}>
            {group === 'day' || group === 'task' || r.key === '(none)' ? (
              <Typography variant="body2" noWrap>
                {group === 'day' ? dueLabel(r.label) : r.label}
              </Typography>
            ) : (
              <Box>
                <TagChip name={r.label} />
              </Box>
            )}
            <LinearProgress variant="determinate" value={(r.seconds / max) * 100} sx={{ height: 8, borderRadius: 4 }} />
            <Typography variant="body2" sx={{ textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>
              {duration(r.seconds)}
            </Typography>
          </Box>
        ))}
        {report && report.rows.length === 0 && (
          <Typography variant="body2" color="text.secondary">
            No time tracked this week. Press <b>s</b> on a task or <b>Q</b> to start a timer.
          </Typography>
        )}
        {(group === 'tag' || nss.some((n) => n.name === group)) && (
          <Typography variant="caption" color="text.secondary">
            Tasks with several matching tags count in each row, so rows can add up to more than the total.
          </Typography>
        )}
      </Paper>

      {byDay.map(([day, es]) => (
        <Box key={day}>
          <Typography variant="overline" color="text.secondary">
            {dueLabel(day)} · {duration(es.reduce((a, e) => a + (e.stopped_at ? e.seconds : (now - new Date(e.started_at).getTime()) / 1000), 0))}
          </Typography>
          <List dense disablePadding>
            {es.map((e) => {
              i++;
              const idx = i;
              return (
                <ListItemButton key={e.id} selected={idx === focus} onClick={() => setFocus(idx)} onDoubleClick={() => setEditing(e)} sx={{ borderBottom: 1, borderColor: 'divider' }}>
                  <Typography variant="body2" sx={{ width: 120, fontVariantNumeric: 'tabular-nums', color: 'text.secondary' }}>
                    {timeLabel(e.started_at)}–{e.stopped_at ? timeLabel(e.stopped_at) : 'now'}
                  </Typography>
                  <ListItemText primary={e.task_title} secondary={e.note || undefined} />
                  <Box sx={{ display: 'flex', gap: 0.5, mr: 2 }}>
                    {chipTags(e.task_tags).map((t) => (
                      <TagChip key={t} name={t} short />
                    ))}
                  </Box>
                  <Typography variant="body2" sx={{ width: 70, textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>
                    {duration(e.stopped_at ? e.seconds : (now - new Date(e.started_at).getTime()) / 1000)}
                  </Typography>
                  <IconButton size="small" onClick={(ev) => { ev.stopPropagation(); del(e); }} aria-label="Delete entry">
                    <DeleteOutlinedIcon fontSize="small" />
                  </IconButton>
                </ListItemButton>
              );
            })}
          </List>
        </Box>
      ))}

      {editing && <EntryDialog entry={editing === 'new' ? null : editing} onClose={() => setEditing(null)} />}
    </Box>
  );
}

function EntryDialog({ entry, onClose }: { entry: TimeEntry | null; onClose: () => void }) {
  const { createEntry, updateEntry } = useActions();
  const ui = useUi();
  const [start, setStart] = useState(() => toLocalInput(entry?.started_at ?? new Date(Date.now() - 3600000).toISOString()));
  const [stop, setStop] = useState(() => toLocalInput(entry?.stopped_at ?? (entry ? null : new Date().toISOString())));
  const [note, setNote] = useState(entry?.note ?? '');
  const [task, setTask] = useState<Task | null>(null);
  const [options, setOptions] = useState<Task[]>([]);

  const search = (text: string) => {
    get<Task[]>(`/tasks?limit=20&q=${encodeURIComponent(text ? `"${text.replace(/"/g, '')}"` : 'is:open')}`)
      .then(setOptions)
      .catch(() => setOptions([]));
  };

  const save = () => {
    const started_at = fromLocalInput(start)!;
    const stopped_at = fromLocalInput(stop) ?? undefined;
    const opts = { onSuccess: onClose, onError: (e: unknown) => ui.notify(errorMessage(e)) };
    if (entry) updateEntry.mutate({ id: entry.id, started_at, stopped_at, note }, opts);
    else if (task && stopped_at) createEntry.mutate({ task_id: task.id, started_at, stopped_at, note }, opts);
  };

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="xs">
      <DialogTitle>{entry ? `Edit · ${entry.task_title}` : 'Add time entry'}</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          {!entry && (
            <Autocomplete
              autoHighlight
              options={options}
              value={task}
              onChange={(_, v) => setTask(v)}
              onOpen={() => search('')}
              onInputChange={(_, v, reason) => reason === 'input' && search(v)}
              getOptionLabel={(t) => t.title}
              filterOptions={(x) => x}
              renderInput={(params) => <TextField {...params} autoFocus label="Task" />}
            />
          )}
          <TextField type="datetime-local" label="Start" value={start} onChange={(e) => setStart(e.target.value)} slotProps={{ inputLabel: { shrink: true } }} />
          <TextField
            type="datetime-local"
            label="End"
            value={stop}
            onChange={(e) => setStop(e.target.value)}
            helperText={entry && !entry.stopped_at ? 'Leave empty to keep it running' : undefined}
            slotProps={{ inputLabel: { shrink: true } }}
          />
          <TextField label="Note" value={note} onChange={(e) => setNote(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && save()} />
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" onClick={save} disabled={!entry && (!task || !stop)}>
          Save
        </Button>
      </DialogActions>
    </Dialog>
  );
}
