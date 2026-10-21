import { Box, Checkbox, ListItemButton, Tooltip, Typography } from '@mui/material';
import AlarmIcon from '@mui/icons-material/Alarm';
import BlockIcon from '@mui/icons-material/Block';
import FlagIcon from '@mui/icons-material/Flag';
import ForwardIcon from '@mui/icons-material/Forward';
import HourglassEmptyIcon from '@mui/icons-material/HourglassEmpty';
import NotesIcon from '@mui/icons-material/Notes';
import PlayArrowIcon from '@mui/icons-material/PlayArrow';
import { memo } from 'react';
import type { Task } from '../api/types';
import { clock, dateTimeLabel, dueLabel, duration, isOverdue } from '../lib/format';
import { chipTags, priorityOf, PRIORITY_COLORS, statusOf } from '../lib/tags';
import { TagChip } from './TagChip';

interface Props {
  task: Task;
  focused: boolean;
  selected: boolean;
  open: boolean;
  now: number;
  onClick: (e: React.MouseEvent) => void;
  onToggle: () => void;
}

export const TaskRow = memo(function TaskRow({ task, focused, selected, open, now, onClick, onToggle }: Props) {
  const closed = task.closed_at != null;
  const prio = priorityOf(task.tags);
  const status = statusOf(task.tags);
  const running = task.running_since ? (now - new Date(task.running_since).getTime()) / 1000 : 0;
  return (
    <ListItemButton
      data-task-id={task.id}
      className={focused ? 'kala-focused' : undefined}
      selected={selected || open}
      onClick={onClick}
      sx={{ py: 0.5, pl: 1, gap: 1, borderBottom: 1, borderColor: 'divider', alignItems: 'center', minHeight: 40 }}
    >
      <Checkbox
        size="small"
        checked={closed}
        tabIndex={-1}
        onClick={(e) => {
          e.stopPropagation();
          onToggle();
        }}
        sx={{ p: 0.5 }}
        slotProps={{ input: { 'aria-label': closed ? 'Reopen' : 'Close' } }}
      />
      <Box sx={{ width: 16, display: 'flex' }}>{prio && <FlagIcon sx={{ fontSize: 16, color: PRIORITY_COLORS[prio] ?? 'text.secondary' }} />}</Box>
      <Box sx={{ flex: 1, minWidth: 0, display: 'flex', alignItems: 'center', gap: 1 }}>
        <Typography
          variant="body2"
          noWrap
          sx={{ textDecoration: closed ? 'line-through' : undefined, color: closed ? 'text.disabled' : 'text.primary', flexShrink: 1, minWidth: 40 }}
        >
          {task.title}
        </Typography>
        {task.notes && <NotesIcon sx={{ fontSize: 14, color: 'text.disabled' }} />}
        {status === 'pending' && (
          <Tooltip title="Pending">
            <HourglassEmptyIcon sx={{ fontSize: 14, color: 'warning.main' }} />
          </Tooltip>
        )}
        {status === 'delegated' && (
          <Tooltip title="Delegated">
            <ForwardIcon sx={{ fontSize: 14, color: 'info.main' }} />
          </Tooltip>
        )}
        {status === 'cancelled' && (
          <Tooltip title="Cancelled">
            <BlockIcon sx={{ fontSize: 14, color: 'text.disabled' }} />
          </Tooltip>
        )}
        <Box sx={{ display: 'flex', gap: 0.5, overflow: 'hidden', flexShrink: 2 }}>
          {chipTags(task.tags).map((t) => (
            <TagChip key={t} name={t} short />
          ))}
        </Box>
      </Box>
      {task.remind_at && !task.reminded_at && (
        <Tooltip title={`Reminder ${dateTimeLabel(task.remind_at)}`}>
          <AlarmIcon sx={{ fontSize: 16, color: 'text.secondary' }} />
        </Tooltip>
      )}
      {task.running_since ? (
        <Typography variant="caption" sx={{ color: 'primary.main', fontWeight: 600, fontVariantNumeric: 'tabular-nums', display: 'flex', alignItems: 'center' }}>
          <PlayArrowIcon sx={{ fontSize: 14 }} />
          {clock(running)}
        </Typography>
      ) : (
        task.tracked_seconds > 0 && (
          <Typography variant="caption" color="text.secondary" sx={{ fontVariantNumeric: 'tabular-nums' }}>
            {duration(task.tracked_seconds)}
          </Typography>
        )
      )}
      {task.due_date && (
        <Typography
          variant="caption"
          sx={{ width: 84, textAlign: 'right', color: !closed && isOverdue(task.due_date) ? 'error.main' : 'text.secondary', fontWeight: !closed && isOverdue(task.due_date) ? 600 : 400 }}
        >
          {dueLabel(task.due_date)}
        </Typography>
      )}
    </ListItemButton>
  );
});
