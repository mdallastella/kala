import { Box, IconButton, List, ListItemButton, ListItemText, Paper, Tooltip, Typography } from '@mui/material';
import PlayArrowIcon from '@mui/icons-material/PlayArrow';
import StopIcon from '@mui/icons-material/Stop';
import { Navigate, useSearchParams } from 'react-router';
import { errorMessage } from '../api/client';
import { useActions, useActiveTimers, useTasks } from '../api/hooks';
import type { Task } from '../api/types';
import { useNow } from '../components/TimerStrip';
import { useUi } from '../components/ui';
import { ago, clock } from '../lib/format';
import { tasksPath } from '../lib/useCurrentQuery';
import { TasksPage } from './TasksPage';

const RECENT = 8;

/** When a task was last created or tracked, whichever is later. */
function lastActivity(t: Task): { at: string; tracked: boolean } {
  return t.last_tracked_at && t.last_tracked_at > t.created_at ? { at: t.last_tracked_at, tracked: true } : { at: t.created_at, tracked: false };
}

function Column({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <Paper variant="outlined" sx={{ display: 'flex', flexDirection: 'column', minHeight: 0, maxHeight: 280 }}>
      <Typography variant="overline" color="text.secondary" sx={{ px: 2, pt: 1 }}>
        {title}
      </Typography>
      <Box sx={{ overflowY: 'auto' }}>{children}</Box>
    </Paper>
  );
}

function Empty({ children }: { children: React.ReactNode }) {
  return (
    <Typography variant="body2" color="text.secondary" sx={{ px: 2, pb: 2 }}>
      {children}
    </Typography>
  );
}

export function DashboardPage() {
  const [params, setParams] = useSearchParams();
  const { data: timers = [] } = useActiveTimers();
  const { data: recent = [] } = useTasks('sort:-activity', RECENT);
  const { start, stop } = useActions();
  const ui = useUi();
  const now = useNow();

  // The task list used to live here; keep old filter links working.
  const legacyQuery = params.get('q');
  if (legacyQuery != null) return <Navigate to={tasksPath(legacyQuery, Number(params.get('task')) || undefined)} replace />;

  const openTask = (id: number) => setParams({ task: String(id) }, { replace: true });
  const fail = (e: unknown) => ui.notify(errorMessage(e));

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0 }}>
      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, gap: 2, p: 2, borderBottom: 1, borderColor: 'divider' }}>
        <Column title={`Running timers · ${timers.length}`}>
          {timers.length === 0 ? (
            <Empty>
              No timer running. Press <b>s</b> on a task or <b>Q</b> to start one on a new task.
            </Empty>
          ) : (
            <List dense disablePadding>
              {timers.map((e) => (
                <ListItemButton key={e.id} onClick={() => openTask(e.task_id)}>
                  <ListItemText primary={e.task_title} slotProps={{ primary: { noWrap: true } }} />
                  <Typography variant="body2" sx={{ color: 'primary.main', fontWeight: 600, fontVariantNumeric: 'tabular-nums', mx: 1 }}>
                    {clock((now - new Date(e.started_at).getTime()) / 1000)}
                  </Typography>
                  <Tooltip title="Stop timer">
                    <IconButton
                      size="small"
                      aria-label="Stop timer"
                      onClick={(ev) => {
                        ev.stopPropagation();
                        stop.mutate(e.task_id, { onError: fail });
                      }}
                    >
                      <StopIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                </ListItemButton>
              ))}
            </List>
          )}
        </Column>
        <Column title="Recently created or tracked">
          {recent.length === 0 ? (
            <Empty>
              No tasks yet. Press <b>q</b> to add one.
            </Empty>
          ) : (
            <List dense disablePadding>
              {recent.map((t) => {
                const { at, tracked } = lastActivity(t);
                const closed = t.closed_at != null;
                return (
                  <ListItemButton key={t.id} onClick={() => openTask(t.id)}>
                    <ListItemText
                      primary={t.title}
                      secondary={`${tracked ? 'tracked' : 'created'} ${ago(at, now)}`}
                      slotProps={{
                        primary: { noWrap: true, sx: { textDecoration: closed ? 'line-through' : undefined, color: closed ? 'text.disabled' : undefined } },
                      }}
                    />
                    <Tooltip title={t.running_since ? 'Stop timer' : 'Start timer'}>
                      <IconButton
                        size="small"
                        aria-label={t.running_since ? 'Stop timer' : 'Start timer'}
                        onClick={(ev) => {
                          ev.stopPropagation();
                          (t.running_since ? stop : start).mutate(t.id, { onError: fail });
                        }}
                      >
                        {t.running_since ? <StopIcon fontSize="small" color="primary" /> : <PlayArrowIcon fontSize="small" />}
                      </IconButton>
                    </Tooltip>
                  </ListItemButton>
                );
              })}
            </List>
          )}
        </Column>
      </Box>
      <TasksPage query="is:open sort:-created" title="Latest tasks" />
    </Box>
  );
}
