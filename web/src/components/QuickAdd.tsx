import { Box, Dialog, DialogContent, List, ListItemButton, TextField, Typography } from '@mui/material';
import TimerIcon from '@mui/icons-material/Timer';
import { useMemo, useState } from 'react';
import { errorMessage } from '../api/client';
import { useActions, useNamespaces, useTags } from '../api/hooks';
import { completeToken, currentToken, suggestions as suggest } from '../lib/quickadd';
import { QuickPreview, TIMER_SPENT_ERROR, useQuickParse } from './QuickPreview';

interface Props {
  timer: boolean;
  prefill: string;
  onClose: () => void;
  onDone: (message: string) => void;
}

/** Enter adds the task; Shift+Enter (or opening it with Q) also starts a timer on it. */
export function QuickAdd({ timer, prefill, onClose, onDone }: Props) {
  const [text, setText] = useState(prefill);
  const [submitError, setSubmitError] = useState('');
  const [busy, setBusy] = useState(false);
  const [shift, setShift] = useState(false);
  const [sel, setSel] = useState(0);
  const { data: tags = [] } = useTags();
  const { data: nss = [] } = useNamespaces();
  const { create, startNew } = useActions();
  const { preview, error: parseError } = useQuickParse(text);
  const withTimer = timer || shift;
  const error = submitError || parseError || (withTimer && preview?.spent ? TIMER_SPENT_ERROR : '');

  const token = currentToken(text);
  const suggestions = useMemo(() => suggest(token, tags.map((t) => t.name), nss), [token, tags, nss]);

  const complete = (value: string) => {
    setText(completeToken(text, value));
    setSel(0);
  };

  const submit = async (startTimer: boolean) => {
    if (!text.trim() || busy || submitError || parseError || (startTimer && preview?.spent)) return;
    setBusy(true);
    try {
      const t = startTimer ? await startNew.mutateAsync(text) : await create.mutateAsync({ quick: text });
      onDone(startTimer ? `Started timer on “${t.title}”` : `Created “${t.title}”`);
      onClose();
    } catch (e) {
      setSubmitError(errorMessage(e));
      setBusy(false);
    }
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    setShift(e.shiftKey);
    if (suggestions.length > 0 && (e.key === 'Tab' || e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
      e.preventDefault();
      if (e.key === 'Tab') complete(suggestions[sel] ?? suggestions[0]);
      else setSel((s) => (s + (e.key === 'ArrowDown' ? 1 : suggestions.length - 1)) % suggestions.length);
      return;
    }
    if (e.key === 'Enter') {
      e.preventDefault();
      void submit(timer || e.shiftKey);
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
          onKeyUp={(e) => setShift(e.shiftKey)}
          onBlur={() => setShift(false)}
          slotProps={{
            input: {
              readOnly: busy,
              startAdornment: withTimer ? <TimerIcon fontSize="small" sx={{ mr: 1, color: 'primary.main' }} /> : undefined,
            },
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
            <QuickPreview preview={preview} />
          ) : (
            <Typography variant="caption" color="text.secondary">
              #tag · @project · !high/!1 · due:fri · remind:thu 9:00
              {timer ? ' · Tab completes · Enter adds and starts a timer' : ' · spent:1h30m 9:00 · Tab completes · Enter adds · Shift+Enter adds and starts a timer'}
            </Typography>
          )}
        </Box>
      </DialogContent>
    </Dialog>
  );
}
