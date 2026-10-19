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
      sx={{
        borderColor: alpha(color, 0.5),
        color,
        bgcolor: alpha(color, 0.08),
        height: 20,
        fontSize: 11.5,
        '& .MuiChip-label': { px: 0.75 },
        ...sx,
      }}
      {...rest}
    />
  );
}
