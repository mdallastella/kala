import {
  Box,
  Button,
  Chip,
  Divider,
  IconButton,
  List,
  ListItem,
  ListItemText,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@mui/material';
import AlarmIcon from '@mui/icons-material/Alarm';
import CloseIcon from '@mui/icons-material/Close';
import DeleteOutlinedIcon from '@mui/icons-material/DeleteOutlined';
import EventIcon from '@mui/icons-material/Event';
import FlagIcon from '@mui/icons-material/Flag';
import PlayArrowIcon from '@mui/icons-material/PlayArrow';
import StopIcon from '@mui/icons-material/Stop';
import { useEffect, useRef, useState } from 'react';
import Markdown from 'react-markdown';
import { errorMessage } from '../api/client';
import { useActions, useEntries, useNamespaces, useTask } from '../api/hooks';
import { clock, dateTimeLabel, dueLabel, duration, isoDate, isOverdue, timeLabel } from '../lib/format';
import { namespaceValues, priorityOf, PRIORITY_COLORS, statusOf } from '../lib/tags';
import { TagChip } from './TagChip';
import { useNow } from './TimerStrip';
import { useUi } from './ui';

const FAR_PAST = '2000-01-01T00:00:00Z';

interface Props {
  id: number;
  focusTitle: number;
  onClose: () => void;
}

/** Blurs the field on Escape so the next Escape closes the panel. */
const escBlur = (e: React.KeyboardEvent) => {
  if (e.key === 'Escape') {
    e.stopPropagation();
    (e.target as HTMLElement).blur();
  }
};

export function TaskDetail({ id, focusTitle, onClose }: Props) {
  const { data: task, error } = useTask(id);
  const { data: nss } = useNamespaces();
  const { update, remove, restore, start, stop } = useActions();
  const ui = useUi();
  const now = useNow();
  // Edit buffers are only set while a field is being edited, so server
  // updates show through otherwise.
  const [titleDraft, setTitleDraft] = useState<string | null>(null);
  const [notesDraft, setNotesDraft] = useState<string | null>(null);
  const titleRef = useRef<HTMLInputElement>(null);
  const until = new Date(now + 86400000).toISOString().slice(0, 13) + ':00:00Z';
  const { data: entries = [] } = useEntries(FAR_PAST, until, id);

  useEffect(() => {
    if (focusTitle > 0) titleRef.current?.focus();
  }, [focusTitle]);

  if (error) return <Box sx={{ p: 2 }}>Task not found.</Box>;
  if (!task) return null;

  const title = titleDraft ?? task.title;
  const save = (p: Parameters<typeof update.mutate>[0]) => update.mutate(p, { onError: (e) => ui.notify(errorMessage(e)) });
  const closed = task.closed_at != null;
  const prio = priorityOf(task.tags) ?? '';
  const status = statusOf(task.tags) ?? '';
  const statuses = namespaceValues(nss, 'status');
  const priorities = namespaceValues(nss, 'priority');
  const otherTags = task.tags.filter((t) => !t.startsWith('status:') && !t.startsWith('priority:'));
  const running = task.running_since ? (now - new Date(task.running_since).getTime()) / 1000 : 0;

  return (
    <Box sx={{ p: 2, display: 'flex', flexDirection: 'column', gap: 2, height: '100%', overflowY: 'auto' }}>
      <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1 }}>
        <TextField
          inputRef={titleRef}
          variant="standard"
          fullWidth
          multiline
          value={title}
          onChange={(e) => setTitleDraft(e.target.value)}
          onBlur={() => {
            if (title.trim() && title !== task.title) save({ id, title });
            setTitleDraft(null);
          }}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault();
              (e.target as HTMLElement).blur();
            }
            escBlur(e);
          }}
          slotProps={{ input: { disableUnderline: true, sx: { fontSize: 18, fontWeight: 600 } } }}
        />
        <Tooltip title="Close panel (Esc)">
          <IconButton size="small" onClick={onClose}>
            <CloseIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Box>

      <Stack direction="row" spacing={1} useFlexGap sx={{ flexWrap: 'wrap' }}>
        <ToggleButtonGroup
          size="small"
          exclusive
          value={status}
          onChange={(_, v: string | null) => v && save({ id, add_tags: [`status:${v}`] })}
        >
          {statuses.map((s) => (
            <ToggleButton key={s} value={s} sx={{ px: 1.25, py: 0.25, textTransform: 'none' }}>
              {s}
            </ToggleButton>
          ))}
        </ToggleButtonGroup>
        <ToggleButtonGroup
          size="small"
          exclusive
          value={prio}
          onChange={(_, v: string | null) =>
            save(v ? { id, add_tags: [`priority:${v}`] } : { id, remove_tags: prio ? [`priority:${prio}`] : [] })
          }
        >
          {priorities.map((p) => (
            <ToggleButton key={p} value={p} sx={{ px: 1, py: 0.25 }} aria-label={p}>
              <Tooltip title={p}>
                <FlagIcon sx={{ fontSize: 16, color: PRIORITY_COLORS[p] }} />
              </Tooltip>
            </ToggleButton>
          ))}
        </ToggleButtonGroup>
      </Stack>

      <Box>
        <Typography variant="overline" color="text.secondary">
          Tags
        </Typography>
        <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap', alignItems: 'center' }}>
          {otherTags.map((t) => (
            <TagChip key={t} name={t} onDelete={() => save({ id, remove_tags: [t] })} />
          ))}
          <Button size="small" onClick={() => ui.pickTags([task])}>
            Edit (t)
          </Button>
          <Button size="small" onClick={() => ui.pickTags([task], 'project')}>
            Project (p)
          </Button>
        </Box>
      </Box>

      <Stack direction="row" spacing={1}>
        <Chip
          icon={<EventIcon />}
          label={task.due_date ? dueLabel(task.due_date) : 'No due date'}
          color={!closed && isOverdue(task.due_date) ? 'error' : 'default'}
          variant="outlined"
          onClick={() => ui.pickDate([task], 'due')}
          onDelete={task.due_date ? () => save({ id, due_date: null }) : undefined}
        />
        <Chip
          icon={<AlarmIcon />}
          label={task.remind_at ? dateTimeLabel(task.remind_at) : 'No reminder'}
          variant="outlined"
          onClick={() => ui.pickDate([task], 'remind')}
          onDelete={task.remind_at ? () => save({ id, remind_at: null }) : undefined}
        />
      </Stack>

      <Box>
        <Typography variant="overline" color="text.secondary">
          Notes
        </Typography>
        {notesDraft != null ? (
          <TextField
            autoFocus
            fullWidth
            multiline
            minRows={4}
            value={notesDraft}
            onChange={(e) => setNotesDraft(e.target.value)}
            onBlur={() => {
              if (notesDraft !== task.notes) save({ id, notes: notesDraft });
              setNotesDraft(null);
            }}
            onKeyDown={escBlur}
            placeholder="Markdown supported"
            slotProps={{ input: { sx: { fontFamily: 'ui-monospace, monospace', fontSize: 13 } } }}
          />
        ) : (
          <Box
            tabIndex={0}
            onClick={() => setNotesDraft(task.notes)}
            onKeyDown={(e) => e.key === 'Enter' && setNotesDraft(task.notes)}
            sx={{
              minHeight: 48,
              p: 1,
              borderRadius: 1,
              border: 1,
              borderColor: 'divider',
              cursor: 'text',
              '& p': { my: 0.5 },
              '& pre': { overflowX: 'auto' },
              fontSize: 13,
            }}
          >
            {task.notes ? (
              <Markdown>{task.notes}</Markdown>
            ) : (
              <Typography variant="body2" color="text.disabled">
                Click to add notes…
              </Typography>
            )}
          </Box>
        )}
      </Box>

      <Box>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <Typography variant="overline" color="text.secondary">
            Time · {duration(task.tracked_seconds + running)}
          </Typography>
          {task.running_since ? (
            <Button size="small" startIcon={<StopIcon />} onClick={() => stop.mutate(id)}>
              Stop {clock(running)} (s)
            </Button>
          ) : (
            <Button size="small" startIcon={<PlayArrowIcon />} onClick={() => start.mutate(id)}>
              Start (s)
            </Button>
          )}
        </Box>
        <List dense disablePadding>
          {entries.slice(0, 20).map((e) => (
            <ListItem key={e.id} disableGutters sx={{ py: 0 }}>
              <ListItemText
                primary={`${dueLabel(isoDate(new Date(e.started_at)))} ${timeLabel(e.started_at)}–${e.stopped_at ? timeLabel(e.stopped_at) : 'now'}`}
                secondary={e.note || undefined}
              />
              <Typography variant="caption" sx={{ fontVariantNumeric: 'tabular-nums' }}>
                {duration(e.stopped_at ? e.seconds : (now - new Date(e.started_at).getTime()) / 1000)}
              </Typography>
            </ListItem>
          ))}
        </List>
      </Box>

      <Box sx={{ flex: 1 }} />
      <Divider />
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Typography variant="caption" color="text.secondary">
          Created {new Date(task.created_at).toLocaleString()}
          {task.closed_at && ` · closed ${new Date(task.closed_at).toLocaleString()}`}
        </Typography>
        <Button
          size="small"
          color="error"
          startIcon={<DeleteOutlinedIcon />}
          onClick={() =>
            remove.mutate(id, {
              onSuccess: () => {
                onClose();
                ui.notify(`Deleted “${task.title}”`, { label: 'Undo', run: () => restore.mutate(id) });
              },
            })
          }
        >
          Delete
        </Button>
      </Box>
    </Box>
  );
}
