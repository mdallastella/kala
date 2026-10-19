import { Box, Chip, Dialog, DialogContent, List, ListItemButton, Stack, TextField, Typography } from '@mui/material';
import AlarmIcon from '@mui/icons-material/Alarm';
import EventIcon from '@mui/icons-material/Event';
import TimerIcon from '@mui/icons-material/Timer';
import { useEffect, useMemo, useState } from 'react';
import { errorMessage } from '../api/client';
import { parseQuick, useActions, useTags } from '../api/hooks';
import type { QuickParse } from '../api/types';
import { dateTimeLabel, dueLabel } from '../lib/format';
import { TagChip } from './TagChip';

interface Props {
  timer: boolean;
  prefill: string;
  onClose: () => void;
  onDone: (message: string) => void;
}

/** The token being typed at the end of the input, if it is a tag token. */
function currentToken(text: string): { prefix: '#' | '@'; partial: string } | null {
  const m = text.match(/(?:^|\s)([#@])(\S*)$/);
  return m ? { prefix: m[1] as '#' | '@', partial: m[2].toLowerCase() } : null;
}

export function QuickAdd({ timer, prefill, onClose, onDone }: Props) {
  const [text, setText] = useState(prefill);
  // Parse result for a given text; stale results are ignored when rendering.
  const [parsed, setParsed] = useState<{ text: string; preview?: QuickParse; error?: string }>({ text: '' });
  const [submitError, setSubmitError] = useState('');
  const [sel, setSel] = useState(0);
  const { data: tags = [] } = useTags();
  const { create, startNew } = useActions();

  useEffect(() => {
    if (!text.trim()) return;
    const t = setTimeout(() => {
      parseQuick(text)
        .then((p) => setParsed({ text, preview: p }))
        .catch((e) => setParsed({ text, error: errorMessage(e) }));
    }, 150);
    return () => clearTimeout(t);
  }, [text]);
  const empty = !text.trim();
  const preview = empty ? null : (parsed.preview ?? null);
  const error = submitError || (!empty && parsed.text === text ? (parsed.error ?? '') : '');

  const token = currentToken(text);
  const suggestions = useMemo(() => {
    if (!token) return [];
    const names = tags.map((t) => t.name);
    const pool = token.prefix === '@' ? names.filter((n) => n.startsWith('project:')).map((n) => n.slice(8)) : names;
    return pool.filter((n) => n.includes(token.partial) && n !== token.partial).slice(0, 8);
  }, [token, tags]);

  const complete = (value: string) => {
    setText(text.replace(/([#@])(\S*)$/, `$1${value} `));
    setSel(0);
  };

  const submit = async () => {
    if (!text.trim() || error) return;
    try {
      const t = timer ? await startNew.mutateAsync(text) : await create.mutateAsync({ quick: text });
      onDone(timer ? `Started timer on “${t.title}”` : `Created “${t.title}”`);
      onClose();
    } catch (e) {
      setSubmitError(errorMessage(e));
    }
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (suggestions.length > 0 && (e.key === 'Tab' || e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
      e.preventDefault();
      if (e.key === 'Tab') complete(suggestions[sel] ?? suggestions[0]);
      else setSel((s) => (s + (e.key === 'ArrowDown' ? 1 : suggestions.length - 1)) % suggestions.length);
      return;
    }
    if (e.key === 'Enter') {
      e.preventDefault();
      void submit();
    }
  };

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="sm" slotProps={{ paper: { sx: { position: 'fixed', top: '12vh', m: 0 } } }}>
      <DialogContent sx={{ p: 2 }}>
        <TextField
          autoFocus
          fullWidth
          placeholder={timer ? 'What are you working on? (starts a timer)' : 'Write report #client:acme @kala !high due:fri remind:thu 9:00'}
          value={text}
          onChange={(e) => {
            setText(e.target.value);
            setSubmitError('');
          }}
          onKeyDown={onKeyDown}
          slotProps={{
            input: { startAdornment: timer ? <TimerIcon fontSize="small" sx={{ mr: 1, color: 'primary.main' }} /> : undefined },
            htmlInput: { 'aria-label': 'Quick add' },
          }}
        />
        {suggestions.length > 0 && (
          <List dense sx={{ mt: 0.5, border: 1, borderColor: 'divider', borderRadius: 1, py: 0 }}>
            {suggestions.map((s, i) => (
              <ListItemButton key={s} selected={i === sel} onMouseDown={(e) => { e.preventDefault(); complete(s); }}>
                <Typography variant="body2">
                  {token?.prefix}
                  {s}
                </Typography>
              </ListItemButton>
            ))}
          </List>
        )}
        <Box sx={{ mt: 1.5, minHeight: 28 }}>
          {error ? (
            <Typography variant="body2" color="error">
              {error}
            </Typography>
          ) : preview ? (
            <Stack direction="row" spacing={0.75} useFlexGap sx={{ flexWrap: 'wrap', alignItems: 'center' }}>
              <Typography variant="body2" sx={{ fontWeight: 600, mr: 0.5 }}>
                {preview.title}
              </Typography>
              {preview.tags.map((t) => (
                <TagChip key={t} name={t} />
              ))}
              {preview.due_date && <Chip icon={<EventIcon />} label={dueLabel(preview.due_date)} variant="outlined" />}
              {preview.remind_at && <Chip icon={<AlarmIcon />} label={dateTimeLabel(preview.remind_at)} variant="outlined" />}
            </Stack>
          ) : (
            <Typography variant="caption" color="text.secondary">
              #tag · @project · !high/!1 · due:fri · remind:thu 9:00 · Tab completes · Enter saves
            </Typography>
          )}
        </Box>
      </DialogContent>
    </Dialog>
  );
}
