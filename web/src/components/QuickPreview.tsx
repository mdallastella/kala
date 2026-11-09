import { Chip, Stack, Typography } from '@mui/material';
import AlarmIcon from '@mui/icons-material/Alarm';
import EventIcon from '@mui/icons-material/Event';
import TimerIcon from '@mui/icons-material/Timer';
import { useEffect, useState } from 'react';
import { errorMessage } from '../api/client';
import { parseQuick } from '../api/hooks';
import type { QuickParse } from '../api/types';
import { dateTimeLabel, dueLabel, duration, timeLabel } from '../lib/format';
import { TagChip } from './TagChip';

/**
 * Parses a quick-add line on the server as it is typed. Results for older text are ignored, so
 * preview and error always describe the current text (or are empty while it is being parsed).
 */
export function useQuickParse(text: string | null): { preview: QuickParse | null; error: string } {
  const [parsed, setParsed] = useState<{ text: string; preview?: QuickParse; error?: string }>({ text: '' });
  useEffect(() => {
    if (!text?.trim()) return;
    const t = setTimeout(() => {
      parseQuick(text)
        .then((p) => setParsed({ text, preview: p }))
        .catch((e) => setParsed({ text, error: errorMessage(e) }));
    }, 150);
    return () => clearTimeout(t);
  }, [text]);
  if (!text?.trim() || parsed.text !== text) return { preview: null, error: '' };
  return { preview: parsed.preview ?? null, error: parsed.error ?? '' };
}

function spentLabel(s: NonNullable<QuickParse['spent']>): string {
  const secs = (new Date(s.stopped_at).getTime() - new Date(s.started_at).getTime()) / 1000;
  return `${duration(secs)} · ${timeLabel(s.started_at)}–${timeLabel(s.stopped_at)}`;
}

/** Shows what a parsed quick-add line will do. */
export function QuickPreview({ preview, showTitle = true }: { preview: QuickParse; showTitle?: boolean }) {
  return (
    <Stack direction="row" spacing={0.75} useFlexGap sx={{ flexWrap: 'wrap', alignItems: 'center' }}>
      {showTitle && (
        <Typography variant="body2" sx={{ fontWeight: 600, mr: 0.5 }}>
          {preview.title}
        </Typography>
      )}
      {preview.tags.map((t) => (
        <TagChip key={t} name={t} />
      ))}
      {preview.due_date && <Chip icon={<EventIcon />} label={dueLabel(preview.due_date)} variant="outlined" />}
      {preview.remind_at && <Chip icon={<AlarmIcon />} label={dateTimeLabel(preview.remind_at)} variant="outlined" />}
      {preview.spent && <Chip icon={<TimerIcon />} label={spentLabel(preview.spent)} variant="outlined" />}
    </Stack>
  );
}

/** The error starting a timer would give for this line, which the server only reports on submit. */
export const TIMER_SPENT_ERROR = 'spent: logs time already spent and can’t be combined with a timer';
