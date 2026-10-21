import { Box, Button, IconButton, Tooltip, Typography } from '@mui/material';
import ChevronLeftIcon from '@mui/icons-material/ChevronLeft';
import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import { useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router';
import { errorMessage } from '../api/client';
import { useActions, useTasks } from '../api/hooks';
import type { Task } from '../api/types';
import { useUi } from '../components/ui';
import { useHotkeys } from '../keys/hotkeys';
import { dragTasks, useTaskDrop } from '../lib/dnd';
import { addDays, isoDate, monthGrid } from '../lib/format';
import { PRIORITY_COLORS } from '../lib/tags';
import { tasksPath } from '../lib/useCurrentQuery';

const WEEKDAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];

/** Sets (or with null clears) the due date of the dropped tasks. */
function useDueDrop(due: string | null) {
  const { bulk } = useActions();
  const ui = useUi();
  return useTaskDrop((ids) => bulk.mutate({ ids, op: 'set_due', due_date: due }, { onError: (e) => ui.notify(errorMessage(e)) }));
}

function CalendarTask({ task }: { task: Task }) {
  const navigate = useNavigate();
  const closed = task.closed_at != null;
  return (
    <Box
      draggable
      onDragStart={(e) => dragTasks(e, [task.id])}
      onClick={() => navigate(tasksPath(undefined, task.id))}
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 0.5,
        px: 0.5,
        borderRadius: 0.5,
        cursor: 'pointer',
        '&:hover': { bgcolor: 'action.hover' },
      }}
    >
      <Box
        sx={{
          width: 6,
          height: 6,
          borderRadius: '50%',
          flexShrink: 0,
          bgcolor: (task.priority && PRIORITY_COLORS[task.priority]) || 'text.disabled',
          opacity: task.priority_derived ? 0.35 : 1,
        }}
      />
      <Typography variant="caption" noWrap sx={{ textDecoration: closed ? 'line-through' : undefined, color: closed ? 'text.disabled' : 'text.primary' }}>
        {task.title}
      </Typography>
    </Box>
  );
}

function DayCell({ day, tasks, inMonth, today, focused, onFocus }: { day: string; tasks: Task[]; inMonth: boolean; today: boolean; focused: boolean; onFocus: () => void }) {
  const drop = useDueDrop(day);
  return (
    <Box
      onClick={onFocus}
      {...drop.props}
      sx={{
        minHeight: 0,
        overflowY: 'auto',
        borderRight: 1,
        borderBottom: 1,
        borderColor: 'divider',
        p: 0.5,
        bgcolor: drop.over ? 'action.selected' : inMonth ? 'background.paper' : 'action.hover',
        outline: focused ? 2 : 0,
        outlineColor: 'primary.main',
        outlineOffset: -2,
      }}
    >
      <Typography
        variant="caption"
        sx={{
          display: 'inline-block',
          px: 0.5,
          borderRadius: 1,
          fontWeight: today ? 700 : 400,
          color: today ? 'primary.contrastText' : inMonth ? 'text.secondary' : 'text.disabled',
          bgcolor: today ? 'primary.main' : undefined,
        }}
      >
        {Number(day.slice(8))}
      </Typography>
      {tasks.map((t) => (
        <CalendarTask key={t.id} task={t} />
      ))}
    </Box>
  );
}

function Unscheduled({ q }: { q: string }) {
  const { data: tasks = [] } = useTasks(`${q} due:none`);
  const drop = useDueDrop(null);
  return (
    <Box
      {...drop.props}
      sx={{ width: 240, flexShrink: 0, borderLeft: 1, borderColor: 'divider', overflowY: 'auto', p: 1, bgcolor: drop.over ? 'action.selected' : undefined }}
    >
      <Typography variant="overline" color="text.secondary">
        Unscheduled · {tasks.length}
      </Typography>
      <Typography variant="caption" color="text.disabled" sx={{ display: 'block', mb: 1 }}>
        Drag onto a day to set the due date, or here to clear it.
      </Typography>
      {tasks.map((t) => (
        <CalendarTask key={t.id} task={t} />
      ))}
    </Box>
  );
}

export function CalendarPage() {
  const [params] = useSearchParams();
  const q = params.get('q') ?? 'is:open';
  const navigate = useNavigate();
  const [focus, setFocus] = useState(() => new Date());
  const days = monthGrid(focus);
  const first = isoDate(days[0]);
  const last = isoDate(days[days.length - 1]);
  const { data: tasks = [] } = useTasks(`${q} due:>=${first} due:<=${last} sort:due,priority`);
  const byDay = new Map<string, Task[]>();
  for (const t of tasks) if (t.due_date) byDay.set(t.due_date, [...(byDay.get(t.due_date) ?? []), t]);

  const month = focus.getMonth();
  const today = isoDate(new Date());
  const focusDay = isoDate(focus);
  const shiftMonth = (n: number) => setFocus((f) => new Date(f.getFullYear(), f.getMonth() + n, Math.min(f.getDate(), 28)));
  const move = (n: number) => setFocus((f) => addDays(f, n));

  const g = 'Calendar';
  useHotkeys([
    { keys: '[', description: 'Previous month', group: g, run: () => shiftMonth(-1) },
    { keys: ']', description: 'Next month', group: g, run: () => shiftMonth(1) },
    { keys: '.', description: 'Today', group: g, run: () => setFocus(new Date()) },
    { keys: 'left', description: 'Previous day', group: g, run: () => move(-1) },
    { keys: 'right', description: 'Next day', group: g, run: () => move(1) },
    { keys: 'up', description: 'Previous week', group: g, run: () => move(-7) },
    { keys: 'down', description: 'Next week', group: g, run: () => move(7) },
    { keys: 'enter', description: 'Show tasks due that day', group: g, run: () => navigate(tasksPath(`${q} due:${focusDay}`)) },
  ]);

  return (
    <Box sx={{ display: 'flex', flex: 1, minHeight: 0 }}>
      <Box sx={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        <Box sx={{ px: 2, py: 1.5, display: 'flex', alignItems: 'center', gap: 1, borderBottom: 1, borderColor: 'divider' }}>
          <Typography variant="h6" sx={{ fontWeight: 700, mr: 1 }}>
            Calendar
          </Typography>
          <Tooltip title="Previous month ([)">
            <IconButton size="small" onClick={() => shiftMonth(-1)}>
              <ChevronLeftIcon />
            </IconButton>
          </Tooltip>
          <Typography variant="body2" sx={{ minWidth: 130, textAlign: 'center' }}>
            {focus.toLocaleDateString(undefined, { month: 'long', year: 'numeric' })}
          </Typography>
          <Tooltip title="Next month (])">
            <IconButton size="small" onClick={() => shiftMonth(1)}>
              <ChevronRightIcon />
            </IconButton>
          </Tooltip>
          <Button size="small" onClick={() => setFocus(new Date())}>
            Today (.)
          </Button>
          <Typography variant="caption" color="text.disabled" sx={{ fontFamily: 'monospace', ml: 1 }} noWrap>
            {q}
          </Typography>
        </Box>
        <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(7, 1fr)', borderBottom: 1, borderColor: 'divider' }}>
          {WEEKDAYS.map((d) => (
            <Typography key={d} variant="caption" color="text.secondary" sx={{ px: 1, py: 0.5 }}>
              {d}
            </Typography>
          ))}
        </Box>
        <Box sx={{ flex: 1, minHeight: 0, display: 'grid', gridTemplateColumns: 'repeat(7, minmax(0, 1fr))', gridAutoRows: 'minmax(0, 1fr)' }}>
          {days.map((d) => {
            const day = isoDate(d);
            return (
              <DayCell
                key={day}
                day={day}
                tasks={byDay.get(day) ?? []}
                inMonth={d.getMonth() === month}
                today={day === today}
                focused={day === focusDay}
                onFocus={() => setFocus(d)}
              />
            );
          })}
        </Box>
      </Box>
      <Unscheduled q={q} />
    </Box>
  );
}
