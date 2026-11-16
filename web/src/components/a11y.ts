import type { SxProps, Theme } from '@mui/material/styles';

/** Hides content visually but keeps it in the accessibility tree. */
export const visuallyHidden: SxProps<Theme> = {
  position: 'absolute',
  clipPath: 'inset(50%)',
  overflow: 'hidden',
  width: '1px',
  height: '1px',
  margin: '-1px',
  padding: 0,
  border: 0,
  whiteSpace: 'nowrap',
};
