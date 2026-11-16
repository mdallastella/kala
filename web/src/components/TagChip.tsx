import { Chip, type ChipProps } from '@mui/material';
import { alpha } from '@mui/material/styles';
import { useMemo } from 'react';
import { useTags } from '../api/hooks';
import { tagColor } from '../lib/tags';

interface Props extends Omit<ChipProps, 'label' | 'color'> {
  name: string;
  /** Hide the namespace for compact display (project:kala → kala). */
  short?: boolean;
}

export function useTagMap() {
  const { data } = useTags();
  return useMemo(() => new Map((data ?? []).map((t) => [t.name, t])), [data]);
}

export function TagChip({ name, short, sx, ...rest }: Props) {
  const map = useTagMap();
  const color = tagColor(name, map);
  const label = short && name.startsWith('project:') ? name.slice(8) : name;
  return (
    <Chip
      label={label}
      variant="outlined"
      sx={[
        (theme) => ({
          borderColor: alpha(color, 0.5),
          // Tag colors are arbitrary: shade the text toward the theme's text color so the
          // small label stays readable (>= 4.5:1 for the default palette in both schemes).
          color: `color-mix(in srgb, ${color} 75%, black)`,
          bgcolor: alpha(color, 0.08),
          height: 20,
          fontSize: 11.5,
          '& .MuiChip-label': { px: 0.75 },
          ...theme.applyStyles('dark', { color: `color-mix(in srgb, ${color} 45%, white)` }),
        }),
        ...(Array.isArray(sx) ? sx : [sx]),
      ]}
      {...rest}
    />
  );
}
