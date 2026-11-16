import { Button, Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle } from '@mui/material';
import { useId } from 'react';
import type { ConfirmOptions } from './ui';

interface Props extends ConfirmOptions {
  onAnswer: (ok: boolean) => void;
}

/** Asks before a destructive action; focus starts on Cancel. */
export function ConfirmDialog({ title, message, confirmLabel = 'Delete', onAnswer }: Props) {
  const id = useId();
  return (
    <Dialog open onClose={() => onAnswer(false)} maxWidth="xs" fullWidth aria-labelledby={`${id}-title`} aria-describedby={message ? `${id}-message` : undefined}>
      <DialogTitle id={`${id}-title`}>{title}</DialogTitle>
      {message && (
        <DialogContent>
          <DialogContentText id={`${id}-message`} sx={{ whiteSpace: 'pre-line' }}>
            {message}
          </DialogContentText>
        </DialogContent>
      )}
      <DialogActions>
        <Button autoFocus onClick={() => onAnswer(false)}>
          Cancel
        </Button>
        <Button variant="contained" color="error" onClick={() => onAnswer(true)}>
          {confirmLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
