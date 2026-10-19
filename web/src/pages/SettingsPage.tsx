import {
  Alert,
  Autocomplete,
  Box,
  Button,
  Checkbox,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  IconButton,
  List,
  ListItem,
  ListItemText,
  MenuItem,
  Paper,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@mui/material';
import ArrowDownwardIcon from '@mui/icons-material/ArrowDownward';
import ArrowUpwardIcon from '@mui/icons-material/ArrowUpward';
import DeleteOutlinedIcon from '@mui/icons-material/DeleteOutlined';
import EditOutlinedIcon from '@mui/icons-material/EditOutlined';
import PushPinIcon from '@mui/icons-material/PushPin';
import PushPinOutlinedIcon from '@mui/icons-material/PushPinOutlined';
import { useEffect, useState, type ReactNode } from 'react';
import { ApiError, errorMessage, get, post } from '../api/client';
import { useActions, useMe, useNamespaces, useTags, useTokens, useViews, useWorkingHours } from '../api/hooks';
import type { Namespace, NamespaceValue, RenamePreview, Tag, WorkingDay } from '../api/types';
import { TagChip } from '../components/TagChip';
import { useUi } from '../components/ui';
import { clockToMinutes, minutesToClock } from '../lib/format';

function Section({ title, children, hint }: { title: string; hint?: string; children: ReactNode }) {
  return (
    <Paper variant="outlined" sx={{ p: 2 }}>
      <Typography variant="subtitle1" sx={{ fontWeight: 700 }}>
        {title}
      </Typography>
      {hint && (
        <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
          {hint}
        </Typography>
      )}
      {children}
    </Paper>
  );
}

export function SettingsPage() {
  return (
    <Box sx={{ p: 2, overflowY: 'auto' }}>
      <Stack spacing={2} sx={{ maxWidth: 860 }}>
        <Typography variant="h6" sx={{ fontWeight: 700 }}>
          Settings
        </Typography>
        <Profile />
        <WorkingHours />
        <Namespaces />
        <TagsSection />
        <Views />
        <Tokens />
        <Data />
      </Stack>
    </Box>
  );
}

function Profile() {
  const { data: me } = useMe();
  const { updateMe } = useActions();
  const ui = useUi();
  const zones = Intl.supportedValuesOf('timeZone');
  const [perm, setPerm] = useState(typeof Notification === 'undefined' ? 'unsupported' : Notification.permission);
  return (
    <Section title="Profile" hint={me?.email}>
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
        {me && <Autocomplete
          sx={{ width: 300 }}
          options={zones}
          value={me.timezone}
          disableClearable
          onChange={(_, tz: string) => updateMe.mutate({ timezone: tz }, { onSuccess: () => ui.notify(`Timezone set to ${tz}`) })}
          renderInput={(params) => <TextField {...params} label="Timezone" />}
        />}
        <Button
          variant="outlined"
          disabled={perm !== 'default'}
          onClick={() => void Notification.requestPermission().then(setPerm)}
        >
          {perm === 'granted' ? 'Browser notifications on' : perm === 'denied' ? 'Notifications blocked' : perm === 'unsupported' ? 'Notifications unsupported' : 'Enable browser notifications'}
        </Button>
      </Stack>
    </Section>
  );
}

const WEEKDAYS = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];
const weekdayNumber = (i: number) => (i + 1) % 7; // display Monday first; 0 = Sunday

function WorkingHours() {
  const { data } = useWorkingHours();
  return data ? <WorkingHoursForm key={JSON.stringify(data)} initial={data} /> : null;
}

function WorkingHoursForm({ initial }: { initial: WorkingDay[] }) {
  const { setWorkingHours } = useActions();
  const ui = useUi();
  const [days, setDays] = useState<Record<number, WorkingDay | undefined>>(() => Object.fromEntries(initial.map((d) => [d.weekday, d])));
  const set = (wd: number, d: WorkingDay | undefined) => setDays((s) => ({ ...s, [wd]: d }));
  const save = () =>
    setWorkingHours.mutate(Object.values(days).filter((d): d is WorkingDay => !!d), {
      onSuccess: () => ui.notify('Working hours saved'),
      onError: (e) => ui.notify(errorMessage(e)),
    });
  return (
    <Section title="Working hours" hint="The morning briefing appears when your day starts and the wrap-up when it ends. Reminders without a time default to the start of the day.">
      {WEEKDAYS.map((name, i) => {
        const wd = weekdayNumber(i);
        const d = days[wd];
        return (
          <Stack key={wd} direction="row" spacing={2} sx={{ alignItems: 'center', py: 0.5 }}>
            <FormControlLabel
              sx={{ width: 160 }}
              control={<Switch checked={!!d} onChange={(e) => set(wd, e.target.checked ? { weekday: wd, start_min: 540, end_min: 1080 } : undefined)} />}
              label={name}
            />
            {d && (
              <>
                <TextField type="time" label="Start" value={minutesToClock(d.start_min)} onChange={(e) => e.target.value && set(wd, { ...d, start_min: clockToMinutes(e.target.value) })} />
                <TextField type="time" label="End" value={minutesToClock(d.end_min)} onChange={(e) => e.target.value && set(wd, { ...d, end_min: clockToMinutes(e.target.value) })} />
              </>
            )}
          </Stack>
        );
      })}
      <Button variant="contained" sx={{ mt: 1 }} onClick={save}>
        Save hours
      </Button>
    </Section>
  );
}

function Namespaces() {
  const { data: nss = [] } = useNamespaces();
  const [editing, setEditing] = useState<Partial<Namespace> | null>(null);
  return (
    <Section
      title="Namespaces"
      hint="A namespace is the first part of a tag (status:open → status). Exclusive namespaces allow one value per task; terminal values close the task."
    >
      <List dense disablePadding>
        {nss.map((n) => (
          <ListItem
            key={n.id}
            disableGutters
            secondaryAction={
              <IconButton onClick={() => setEditing(n)} aria-label={`Edit ${n.name}`}>
                <EditOutlinedIcon fontSize="small" />
              </IconButton>
            }
          >
            <ListItemText
              primary={
                <>
                  <b>{n.name}</b>
                  {n.exclusive && ' · exclusive'}
                  {n.default_value && ` · default ${n.default_value}`}
                </>
              }
              secondary={n.values.map((v) => v.value + (v.terminal ? ' (terminal)' : '')).join(', ') || 'free values'}
            />
          </ListItem>
        ))}
      </List>
      <Button onClick={() => setEditing({ name: '', exclusive: false, values: [] })}>New namespace</Button>
      {editing && <NamespaceDialog ns={editing} onClose={() => setEditing(null)} />}
    </Section>
  );
}

function NamespaceDialog({ ns, onClose }: { ns: Partial<Namespace>; onClose: () => void }) {
  const { saveNamespace, deleteNamespace } = useActions();
  const [name, setName] = useState(ns.name ?? '');
  const [exclusive, setExclusive] = useState(ns.exclusive ?? false);
  const [def, setDef] = useState(ns.default_value ?? '');
  const [values, setValues] = useState<NamespaceValue[]>(ns.values ?? []);
  const [newValue, setNewValue] = useState('');
  const [error, setError] = useState('');

  const move = (i: number, d: number) => {
    const v = [...values];
    const j = i + d;
    if (j < 0 || j >= v.length) return;
    [v[i], v[j]] = [v[j], v[i]];
    setValues(v);
  };
  const add = () => {
    const v = newValue.trim().toLowerCase();
    if (v && !values.some((x) => x.value === v)) setValues([...values, { value: v, terminal: false }]);
    setNewValue('');
  };
  const save = () =>
    saveNamespace.mutate(
      { id: ns.id, name, exclusive, default_value: exclusive && def ? def : undefined, values },
      {
        onSuccess: onClose,
        onError: (e) => {
          const ids = e instanceof ApiError ? (e.details as { task_ids?: number[] } | undefined)?.task_ids : undefined;
          setError(errorMessage(e) + (ids ? ` (tasks ${ids.slice(0, 10).join(', ')}${ids.length > 10 ? '…' : ''})` : ''));
        },
      },
    );

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>{ns.id ? `Namespace “${ns.name}”` : 'New namespace'}</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          {error && <Alert severity="error">{error}</Alert>}
          <TextField label="Name" value={name} disabled={!!ns.id} onChange={(e) => setName(e.target.value.toLowerCase())} autoFocus={!ns.id} />
          <FormControlLabel control={<Switch checked={exclusive} onChange={(e) => setExclusive(e.target.checked)} />} label="Exclusive (one value per task)" />
          <Box>
            <Typography variant="overline" color="text.secondary">
              Values (in sort order)
            </Typography>
            {values.map((v, i) => (
              <Stack key={v.value} direction="row" sx={{ alignItems: 'center' }}>
                <Typography sx={{ flex: 1 }}>{v.value}</Typography>
                <FormControlLabel
                  control={<Checkbox size="small" checked={v.terminal} onChange={(e) => setValues(values.map((x, j) => (j === i ? { ...x, terminal: e.target.checked } : x)))} />}
                  label="terminal"
                />
                <IconButton size="small" onClick={() => move(i, -1)} aria-label="Move up">
                  <ArrowUpwardIcon fontSize="small" />
                </IconButton>
                <IconButton size="small" onClick={() => move(i, 1)} aria-label="Move down">
                  <ArrowDownwardIcon fontSize="small" />
                </IconButton>
                <IconButton size="small" onClick={() => setValues(values.filter((_, j) => j !== i))} aria-label="Remove value">
                  <DeleteOutlinedIcon fontSize="small" />
                </IconButton>
              </Stack>
            ))}
            <TextField
              fullWidth
              placeholder="Add value and press Enter"
              value={newValue}
              onChange={(e) => setNewValue(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault();
                  add();
                }
              }}
              sx={{ mt: 1 }}
            />
          </Box>
          {exclusive && (
            <TextField select label="Default value for new tasks" value={def} onChange={(e) => setDef(e.target.value)}>
              <MenuItem value="">(none)</MenuItem>
              {values.map((v) => (
                <MenuItem key={v.value} value={v.value}>
                  {v.value}
                </MenuItem>
              ))}
            </TextField>
          )}
        </Stack>
      </DialogContent>
      <DialogActions>
        {ns.id && (
          <Button color="error" sx={{ mr: 'auto' }} onClick={() => window.confirm(`Delete the “${ns.name}” namespace? Its tags stay on tasks as free tags.`) && deleteNamespace.mutate(ns.id!, { onSuccess: onClose })}>
            Delete
          </Button>
        )}
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" onClick={save} disabled={!name}>
          Save
        </Button>
      </DialogActions>
    </Dialog>
  );
}

function TagsSection() {
  const { data: tags = [] } = useTags();
  const { setTagMeta, deleteTag } = useActions();
  const ui = useUi();
  const [renaming, setRenaming] = useState<Tag | null>(null);
  const [filter, setFilter] = useState('');
  const shown = tags.filter((t) => t.name.includes(filter.toLowerCase()));
  return (
    <Section title="Tags" hint="Colors are inherited by descendant tags. Renaming moves the whole subtree and merges into existing tags.">
      <TextField placeholder="Filter tags" value={filter} onChange={(e) => setFilter(e.target.value)} sx={{ mb: 1 }} />
      <List dense disablePadding sx={{ maxHeight: 360, overflowY: 'auto' }}>
        {shown.map((t) => (
          <ListItem key={t.id} disableGutters>
            <Box sx={{ flex: 1, display: 'flex', alignItems: 'center', gap: 1 }}>
              <TagChip name={t.name} />
              <Typography variant="caption" color="text.secondary">
                {t.open} open · {t.total} total
              </Typography>
            </Box>
            <input
              type="color"
              aria-label={`Color of ${t.name}`}
              value={t.color ?? '#888888'}
              onChange={(e) => setTagMeta.mutate({ name: t.name, color: e.target.value })}
              style={{ width: 28, height: 24, border: 'none', background: 'none' }}
            />
            <IconButton size="small" onClick={() => setTagMeta.mutate({ name: t.name, pinned: !t.pinned })} aria-label="Pin to sidebar">
              {t.pinned ? <PushPinIcon fontSize="small" /> : <PushPinOutlinedIcon fontSize="small" />}
            </IconButton>
            <IconButton size="small" onClick={() => setRenaming(t)} aria-label="Rename">
              <EditOutlinedIcon fontSize="small" />
            </IconButton>
            <IconButton
              size="small"
              aria-label="Delete"
              onClick={() =>
                window.confirm(`Remove “${t.name}” from all ${t.total} tasks?`) &&
                deleteTag.mutate(t.id, { onError: (e) => ui.notify(errorMessage(e)) })
              }
            >
              <DeleteOutlinedIcon fontSize="small" />
            </IconButton>
          </ListItem>
        ))}
      </List>
      {renaming && <RenameDialog tag={renaming} onClose={() => setRenaming(null)} />}
    </Section>
  );
}

function RenameDialog({ tag, onClose }: { tag: Tag; onClose: () => void }) {
  const [to, setTo] = useState(tag.name);
  const [result, setResult] = useState<{ to: string; preview?: RenamePreview; error?: string }>({ to: '' });
  const [applyError, setApplyError] = useState('');
  const { renameTag } = useActions();
  useEffect(() => {
    if (!to || to === tag.name) return;
    const t = setTimeout(() => {
      post<RenamePreview>('/tags/rename', { from: tag.name, to, confirm: false })
        .then((p) => setResult({ to, preview: p }))
        .catch((e) => setResult({ to, error: errorMessage(e) }));
    }, 200);
    return () => clearTimeout(t);
  }, [to, tag.name]);
  const current = result.to === to && to !== tag.name;
  const preview = current ? (result.preview ?? null) : null;
  const error = applyError || (current ? (result.error ?? '') : '');
  const setError = setApplyError;
  const apply = () =>
    renameTag.mutate({ from: tag.name, to, confirm: true }, { onSuccess: onClose, onError: (e) => setError(errorMessage(e)) });
  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>Rename “{tag.name}”</DialogTitle>
      <DialogContent>
        <TextField autoFocus fullWidth label="New name" value={to} onChange={(e) => setTo(e.target.value.toLowerCase())} onKeyDown={(e) => e.key === 'Enter' && preview && !preview.conflicts.length && apply()} sx={{ mt: 1 }} />
        {error && <Alert severity="error" sx={{ mt: 2 }}>{error}</Alert>}
        {preview && (
          <Box sx={{ mt: 2 }}>
            <Typography variant="body2">
              {preview.renames.length} tag(s) renamed, {preview.affected_tasks} task(s) affected.
            </Typography>
            {preview.renames.slice(0, 8).map((r) => (
              <Typography key={r.from} variant="caption" sx={{ display: 'block', fontFamily: 'monospace' }}>
                {r.from} → {r.to}
                {preview.merges.includes(r.to) && ' (merge)'}
              </Typography>
            ))}
            {preview.conflicts.length > 0 && (
              <Alert severity="warning" sx={{ mt: 1 }}>
                {preview.conflicts.length} task(s) would end up with two values of an exclusive namespace.
              </Alert>
            )}
          </Box>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" disabled={!preview || preview.conflicts.length > 0} onClick={apply}>
          Rename
        </Button>
      </DialogActions>
    </Dialog>
  );
}

function Views() {
  const { data: views = [] } = useViews();
  const { updateView, deleteView } = useActions();
  const ui = useUi();
  return (
    <Section title="Views" hint="Saved filters shown in the sidebar. Save the current filter with Ctrl/⌘+S.">
      {views.map((v, i) => (
        <Stack key={v.id} direction="row" spacing={1} sx={{ alignItems: 'center', py: 0.5 }}>
          <TextField defaultValue={v.name} sx={{ width: 160 }} onBlur={(e) => e.target.value !== v.name && updateView.mutate({ id: v.id, name: e.target.value }, { onError: (err) => ui.notify(errorMessage(err)) })} />
          <TextField
            defaultValue={v.query}
            sx={{ flex: 1 }}
            slotProps={{ input: { sx: { fontFamily: 'monospace' } } }}
            onBlur={(e) => e.target.value !== v.query && updateView.mutate({ id: v.id, query: e.target.value }, { onError: (err) => ui.notify(errorMessage(err)) })}
          />
          <IconButton size="small" disabled={i === 0} onClick={() => { updateView.mutate({ id: v.id, position: views[i - 1].position - 1 }); }} aria-label="Move up">
            <ArrowUpwardIcon fontSize="small" />
          </IconButton>
          <IconButton size="small" onClick={() => window.confirm(`Delete view “${v.name}”?`) && deleteView.mutate(v.id)} aria-label="Delete view">
            <DeleteOutlinedIcon fontSize="small" />
          </IconButton>
        </Stack>
      ))}
    </Section>
  );
}

function Tokens() {
  const { data: tokens = [] } = useTokens();
  const { createToken, revokeToken } = useActions();
  const [name, setName] = useState('');
  const [created, setCreated] = useState<string | null>(null);
  const origin = window.location.origin;
  return (
    <Section title="Access tokens & MCP" hint="Personal access tokens authenticate scripts and MCP clients such as Claude. They have full access to your data.">
      {created && (
        <Alert severity="success" sx={{ mb: 2 }}>
          Copy this token now — it won't be shown again:
          <Box component="pre" sx={{ m: 0, mt: 1, fontSize: 12, whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
            {created}
          </Box>
          <Typography variant="caption" sx={{ display: 'block', mt: 1 }}>
            Claude Code:
          </Typography>
          <Box component="pre" sx={{ m: 0, fontSize: 12, whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
            {`claude mcp add --transport http kala ${origin}/mcp --header "Authorization: Bearer ${created}"`}
          </Box>
        </Alert>
      )}
      <List dense disablePadding>
        {tokens.map((t) => (
          <ListItem
            key={t.id}
            disableGutters
            secondaryAction={
              <Button color="error" size="small" onClick={() => window.confirm(`Revoke “${t.name}”?`) && revokeToken.mutate(t.id)}>
                Revoke
              </Button>
            }
          >
            <ListItemText
              primary={`${t.name} · ${t.prefix}…`}
              secondary={`Created ${new Date(t.created_at).toLocaleDateString()} · ${t.last_used_at ? `last used ${new Date(t.last_used_at).toLocaleString()}` : 'never used'}`}
            />
          </ListItem>
        ))}
      </List>
      <Stack direction="row" spacing={1} sx={{ mt: 1 }}>
        <TextField placeholder="Token name, e.g. claude" value={name} onChange={(e) => setName(e.target.value)} />
        <Button
          variant="outlined"
          disabled={!name.trim()}
          onClick={() =>
            createToken.mutate(name.trim(), {
              onSuccess: (t) => {
                setCreated(t.token ?? null);
                setName('');
              },
            })
          }
        >
          Create token
        </Button>
      </Stack>
    </Section>
  );
}

function Data() {
  const ui = useUi();
  const exportData = async () => {
    const data = await get<unknown>('/export');
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = `kala-export-${new Date().toISOString().slice(0, 10)}.json`;
    a.click();
    URL.revokeObjectURL(a.href);
  };
  const importData = async (file: File) => {
    try {
      const res = await post<{ tasks: number; entries: number }>('/import', JSON.parse(await file.text()));
      ui.notify(`Imported ${res.tasks} tasks and ${res.entries} time entries`);
    } catch (e) {
      ui.notify(errorMessage(e));
    }
  };
  return (
    <Section title="Data" hint="Export everything (tasks, trash, tags, namespaces, views, time entries) as JSON. Importing adds to your existing data.">
      <Stack direction="row" spacing={1}>
        <Button variant="outlined" onClick={() => void exportData()}>
          Export JSON
        </Button>
        <Button variant="outlined" component="label">
          Import JSON
          <input hidden type="file" accept="application/json" onChange={(e) => e.target.files?.[0] && void importData(e.target.files[0])} />
        </Button>
      </Stack>
    </Section>
  );
}
