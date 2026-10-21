import { Box, Paper, Tooltip, Typography } from '@mui/material';
import { useMemo } from 'react';
import { useWorkingHours } from '../api/hooks';
import type { TimeEntry } from '../api/types';
import { addDays, duration, timeLabel } from '../lib/format';
import { chipTags, tagColor } from '../lib/tags';
import { hourRange, segments } from '../lib/timeline';
import { useTagMap } from './TagChip';

interface Props {
  entries: TimeEntry[];
  week: Date;
  now: number;
  /** The report grouping: entries are colored by their value in it. */
  group: string;
  onSelect: (e: TimeEntry) => void;
}

/** Week of time entries laid out by time of day, one row per day. */
export function WeekTimeline({ entries, week, now, group, onSelect }: Props) {
  const { data: hours = [] } = useWorkingHours();
  const map = useTagMap();
  const segs = useMemo(() => segments(entries, week, now), [entries, week, now]);
  const [h0, h1] = hourRange(segs, hours);
  const pct = (min: number) => `${((min - h0 * 60) / ((h1 - h0) * 60)) * 100}%`;
  const span = (a: number, b: number) => ({ left: pct(a), width: `calc(${pct(b)} - ${pct(a)})` });

  const colorKey = (e: TimeEntry): string => {
    if (group === 'task' || group === 'day') return e.task_title;
    if (group === 'tag') return chipTags(e.task_tags)[0] ?? e.task_title;
    return e.task_tags.find((t) => t.startsWith(group + ':')) ?? '(none)';
  };

  const ticks = Array.from({ length: h1 - h0 + 1 }, (_, i) => h0 + i);
  return (
    <Paper variant="outlined" sx={{ p: 2 }}>
      <Box sx={{ display: 'grid', gridTemplateColumns: '56px 1fr', rowGap: 0.5 }}>
        <Box />
        <Box sx={{ position: 'relative', height: 16 }}>
          {ticks.map((h) => (
            <Typography key={h} variant="caption" color="text.secondary" sx={{ position: 'absolute', left: pct(h * 60), transform: 'translateX(-50%)', lineHeight: 1 }}>
              {h}
            </Typography>
          ))}
        </Box>
        {Array.from({ length: 7 }, (_, day) => {
          const date = addDays(week, day);
          const work = hours.find((h) => h.weekday === date.getDay());
          return [
            <Typography key={`l${day}`} variant="caption" color="text.secondary" sx={{ alignSelf: 'center' }}>
              {date.toLocaleDateString(undefined, { weekday: 'short', day: 'numeric' })}
            </Typography>,
            <Box key={`r${day}`} sx={{ position: 'relative', height: 22, bgcolor: 'action.hover', borderRadius: 0.5, overflow: 'hidden' }}>
              {work && <Box sx={{ position: 'absolute', top: 0, bottom: 0, ...span(work.start_min, work.end_min), bgcolor: 'action.selected' }} />}
              {ticks.map((h) => (
                <Box key={h} sx={{ position: 'absolute', top: 0, bottom: 0, left: pct(h * 60), borderLeft: 1, borderColor: 'divider' }} />
              ))}
              {segs
                .filter((s) => s.day === day)
                .map((s) => (
                  <Tooltip
                    key={`${s.entry.id}-${s.day}`}
                    title={`${s.entry.task_title} · ${timeLabel(s.entry.started_at)}–${s.entry.stopped_at ? timeLabel(s.entry.stopped_at) : 'now'} · ${duration((s.end - s.start) * 60)}`}
                  >
                    <Box
                      onClick={() => onSelect(s.entry)}
                      sx={{
                        position: 'absolute',
                        top: 3,
                        bottom: 3,
                        ...span(s.start, s.end),
                        minWidth: 2,
                        borderRadius: 0.5,
                        cursor: 'pointer',
                        bgcolor: tagColor(colorKey(s.entry), map),
                        opacity: s.entry.stopped_at ? 0.85 : 1,
                        '&:hover': { opacity: 1, outline: 1, outlineColor: 'text.primary' },
                      }}
                    />
                  </Tooltip>
                ))}
            </Box>,
          ];
        })}
      </Box>
    </Paper>
  );
}
