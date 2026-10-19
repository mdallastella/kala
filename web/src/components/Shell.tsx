import {
  AppBar,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Drawer,
  IconButton,
  InputAdornment,
  Snackbar,
  TextField,
  Toolbar,
  Tooltip,
  Typography,
} from '@mui/material';
import { useColorScheme } from '@mui/material/styles';
import AddIcon from '@mui/icons-material/Add';
import DarkModeOutlinedIcon from '@mui/icons-material/DarkModeOutlined';
import KeyboardIcon from '@mui/icons-material/Keyboard';
import LightModeOutlinedIcon from '@mui/icons-material/LightModeOutlined';
import LogoutIcon from '@mui/icons-material/Logout';
import SearchIcon from '@mui/icons-material/Search';
import { useCallback, useMemo, useRef, useState, type ReactNode } from 'react';
import { Outlet, useNavigate } from 'react-router';
import { errorMessage } from '../api/client';
import { useActions, useTags, useViews } from '../api/hooks';
import type { Task } from '../api/types';
import { useAuth } from '../auth/AuthContext';
import { HelpDialog } from '../keys/HelpDialog';
import { useHotkeys, type Binding } from '../keys/hotkeys';
import { projectFromQuery } from '../lib/tags';
import { useCurrentQuery } from '../lib/useCurrentQuery';
import { CommandPalette, type Command } from './CommandPalette';
import { DatePrompt } from './DatePrompt';
import { Notifications } from './Notifications';
import { QuickAdd } from './QuickAdd';
import { Sidebar, tagQuery } from './Sidebar';
import { TagPicker } from './TagPicker';
import { TimerStrip } from './TimerStrip';
import { UiContext, type NotifyAction, type Ui } from './ui';

const DRAWER = 248;

interface Toast {
  key: number;
  message: string;
  action?: NotifyAction;
}

export function Shell() {
  const navigate = useNavigate();
  const { logout, user } = useAuth();
  const { mode, setMode, systemMode } = useColorScheme();
  const dark = (mode === 'system' ? systemMode : mode) === 'dark';
  const { data: views = [] } = useViews();
  const { data: tags = [] } = useTags();
  const { stopAll, createView } = useActions();

  const [toast, setToast] = useState<Toast | null>(null);
  const [quick, setQuick] = useState<{ open: boolean; timer: boolean; prefill: string }>({ open: false, timer: false, prefill: '' });
  const [palette, setPalette] = useState<{ open: boolean; prefill: string }>({ open: false, prefill: '' });
  const [help, setHelp] = useState(false);
  const [tagPick, setTagPick] = useState<{ tasks: Task[]; mode: 'tags' | 'project' } | null>(null);
  const [datePick, setDatePick] = useState<{ tasks: Task[]; kind: 'due' | 'remind' } | null>(null);
  const [saveView, setSaveView] = useState(false);
  const { q: currentQuery } = useCurrentQuery();
  // Draft filter text while the user edits it; null shows the current query.
  const [draft, setDraft] = useState<string | null>(null);
  const filterText = draft ?? currentQuery;
  const filterRef = useRef<HTMLInputElement>(null);

  const notify = useCallback((message: string, action?: NotifyAction) => setToast({ key: Date.now(), message, action }), []);

  const ui: Ui = useMemo(
    () => ({
      notify,
      quickAdd: (o) => {
        const project = projectFromQuery(currentQuery);
        setQuick({ open: true, timer: !!o?.timer, prefill: o?.prefill ?? (project ? `@${project} ` : '') });
      },
      palette: (prefill = '') => setPalette({ open: true, prefill }),
      help: () => setHelp(true),
      pickTags: (tasks, mode = 'tags') => setTagPick({ tasks, mode }),
      pickDate: (tasks, kind) => setDatePick({ tasks, kind }),
      currentQuery,
    }),
    [notify, currentQuery],
  );

  const goView = (name: string) => {
    const v = views.find((x) => x.name.toLowerCase() === name.toLowerCase());
    if (v) navigate(`/?q=${encodeURIComponent(v.query)}&view=${v.id}`);
  };
  const toggleTheme = () => setMode(dark ? 'light' : 'dark');
  const stopAllTimers = () => stopAll.mutate(undefined, { onSuccess: (r) => notify(`Stopped ${r.stopped} timer(s)`) });

  const global: Binding[] = [
    { keys: 'mod+k', description: 'Command palette', group: 'General', allowInInput: true, run: () => ui.palette() },
    { keys: 'q', description: 'Quick add task', group: 'General', run: () => ui.quickAdd() },
    { keys: 'Q', description: 'Start timer on a new task', group: 'General', run: () => ui.quickAdd({ timer: true }) },
    { keys: '/', description: 'Filter tasks', group: 'General', run: () => filterRef.current?.focus() },
    { keys: '?', description: 'Keyboard shortcuts', group: 'General', run: () => setHelp(true) },
    { keys: 'mod+s', description: 'Save filter as view', group: 'General', allowInInput: true, run: () => setSaveView(true) },
    { keys: 'g t', description: 'Go to Today', group: 'Navigation', run: () => goView('Today') },
    { keys: 'g u', description: 'Go to Upcoming', group: 'Navigation', run: () => goView('Upcoming') },
    { keys: 'g o', description: 'Go to Overdue', group: 'Navigation', run: () => goView('Overdue') },
    { keys: 'g i', description: 'Go to Inbox', group: 'Navigation', run: () => goView('Inbox') },
    { keys: 'g a', description: 'Go to All open', group: 'Navigation', run: () => goView('All open') },
    { keys: 'g p', description: 'Jump to project…', group: 'Navigation', run: () => ui.palette('project:') },
    { keys: 'g m', description: 'Go to Time', group: 'Navigation', run: () => navigate('/time') },
    { keys: 'g x', description: 'Go to Trash', group: 'Navigation', run: () => navigate('/trash') },
    { keys: 'g ,', description: 'Go to Settings', group: 'Navigation', run: () => navigate('/settings') },
  ];
  useHotkeys(global);

  const commands: Command[] = useMemo(() => {
    const cmds: Command[] = [
      { id: 'quick', label: 'Add task', group: 'Actions', keys: 'q', run: () => ui.quickAdd() },
      { id: 'quick-timer', label: 'Start timer on a new task', group: 'Actions', keys: 'Q', run: () => ui.quickAdd({ timer: true }) },
      { id: 'stop-all', label: 'Stop all timers', group: 'Actions', run: stopAllTimers },
      { id: 'save-view', label: 'Save current filter as view', group: 'Actions', keys: 'mod+s', run: () => setSaveView(true) },
      { id: 'theme', label: dark ? 'Switch to light theme' : 'Switch to dark theme', group: 'Actions', run: toggleTheme },
      { id: 'help', label: 'Keyboard shortcuts', group: 'Actions', keys: '?', run: () => setHelp(true) },
      { id: 'logout', label: 'Sign out', group: 'Actions', run: () => void logout() },
      { id: 'time', label: 'Time tracking', group: 'Go to', keys: 'g m', run: () => navigate('/time') },
      { id: 'trash', label: 'Trash', group: 'Go to', keys: 'g x', run: () => navigate('/trash') },
      { id: 'settings', label: 'Settings', group: 'Go to', keys: 'g ,', run: () => navigate('/settings') },
    ];
    for (const v of views)
      cmds.push({ id: `view-${v.id}`, label: v.name, hint: v.query, group: 'Views', run: () => navigate(`/?q=${encodeURIComponent(v.query)}&view=${v.id}`) });
    for (const t of tags)
      cmds.push({ id: `tag-${t.id}`, label: t.name, hint: `${t.open} open`, group: 'Tags', run: () => navigate(`/?q=${encodeURIComponent(tagQuery(t.name))}`) });
    return cmds;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [views, tags, dark, ui]);

  const applyFilter = () => {
    navigate(`/?q=${encodeURIComponent(filterText.trim())}`);
    setDraft(null);
    filterRef.current?.blur();
  };

  const [connected, setConnected] = useState(false);
  const onConnected = useCallback((ok: boolean) => setConnected(ok), []);

  return (
    <UiContext.Provider value={ui}>
      <Box sx={{ display: 'flex', height: '100vh', bgcolor: 'background.default' }}>
        <AppBar position="fixed" color="inherit" elevation={0} sx={{ borderBottom: 1, borderColor: 'divider', zIndex: (t) => t.zIndex.drawer + 1 }}>
          <Toolbar variant="dense" sx={{ gap: 2 }}>
            <Typography variant="h6" sx={{ fontWeight: 700, width: DRAWER - 40, color: 'primary.main', letterSpacing: -0.5 }}>
              kala
            </Typography>
            <TextField
              inputRef={filterRef}
              value={filterText}
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') applyFilter();
                if (e.key === 'Escape') {
                  setDraft(null);
                  filterRef.current?.blur();
                }
              }}
              onBlur={() => setDraft(null)}
              placeholder="Filter: project:kala is:open due:<=today   ( / )"
              sx={{ flex: 1, maxWidth: 560 }}
              slotProps={{
                input: {
                  startAdornment: (
                    <InputAdornment position="start">
                      <SearchIcon fontSize="small" />
                    </InputAdornment>
                  ),
                  sx: { fontFamily: 'ui-monospace, monospace', fontSize: 13 },
                },
              }}
            />
            <Box sx={{ flex: 1 }} />
            <TimerStrip />
            <Tooltip title="Add task (q)">
              <IconButton onClick={() => ui.quickAdd()} color="primary">
                <AddIcon />
              </IconButton>
            </Tooltip>
            <Tooltip title="Shortcuts (?)">
              <IconButton onClick={() => setHelp(true)}>
                <KeyboardIcon />
              </IconButton>
            </Tooltip>
            <Tooltip title="Toggle theme">
              <IconButton onClick={toggleTheme}>{dark ? <LightModeOutlinedIcon /> : <DarkModeOutlinedIcon />}</IconButton>
            </Tooltip>
            <Tooltip title={`Sign out ${user?.email ?? ''}`}>
              <IconButton onClick={() => void logout()}>
                <LogoutIcon />
              </IconButton>
            </Tooltip>
            <Tooltip title={connected ? 'Live updates connected' : 'Reconnecting…'}>
              <Box sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: connected ? 'success.main' : 'warning.main' }} />
            </Tooltip>
          </Toolbar>
        </AppBar>
        <Drawer variant="permanent" sx={{ width: DRAWER, flexShrink: 0, '& .MuiDrawer-paper': { width: DRAWER, boxSizing: 'border-box', pt: '48px' } }}>
          <Sidebar />
        </Drawer>
        <Box component="main" sx={{ flex: 1, minWidth: 0, pt: '48px', display: 'flex', flexDirection: 'column' }}>
          <Outlet />
        </Box>
      </Box>

      {/* Dialogs mount when opened, so each opening starts from fresh state. */}
      {quick.open && (
        <QuickAdd timer={quick.timer} prefill={quick.prefill} onClose={() => setQuick((s) => ({ ...s, open: false }))} onDone={notify} />
      )}
      {palette.open && (
        <CommandPalette
          prefill={palette.prefill}
          commands={commands}
          onOpenTask={(t) => navigate(`/?q=${encodeURIComponent(currentQuery)}&task=${t.id}`)}
          onClose={() => setPalette((s) => ({ ...s, open: false }))}
        />
      )}
      <HelpDialog open={help} onClose={() => setHelp(false)} />
      {tagPick && <TagPicker tasks={tagPick.tasks} mode={tagPick.mode} onClose={() => setTagPick(null)} onError={notify} />}
      {datePick && <DatePrompt tasks={datePick.tasks} kind={datePick.kind} onClose={() => setDatePick(null)} onError={notify} />}
      <SaveViewDialog
        key={String(saveView)}
        open={saveView}
        query={currentQuery}
        onClose={() => setSaveView(false)}
        onSave={(name) =>
          createView.mutate(
            { name, query: currentQuery },
            { onSuccess: (v) => { notify(`Saved view “${v.name}”`); setSaveView(false); }, onError: (e) => notify(errorMessage(e)) },
          )
        }
      />
      <Notifications onConnected={onConnected} />
      <Snackbar
        key={toast?.key}
        open={!!toast}
        autoHideDuration={toast?.action ? 6000 : 3000}
        onClose={(_, reason) => reason !== 'clickaway' && setToast(null)}
        message={toast?.message}
        action={
          toast?.action && (
            <Button color="primary" size="small" onClick={() => { toast.action?.run(); setToast(null); }}>
              {toast.action.label}
            </Button>
          )
        }
      />
    </UiContext.Provider>
  );
}

function SaveViewDialog({ open, query, onClose, onSave }: { open: boolean; query: string; onClose: () => void; onSave: (name: string) => void }): ReactNode {
  const [name, setName] = useState('');
  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Save view</DialogTitle>
      <DialogContent>
        <TextField
          autoFocus
          fullWidth
          label="Name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && name.trim() && onSave(name.trim())}
          helperText={query || '(empty filter)'}
          sx={{ mt: 1 }}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" disabled={!name.trim()} onClick={() => onSave(name.trim())}>
          Save
        </Button>
      </DialogActions>
    </Dialog>
  );
}
