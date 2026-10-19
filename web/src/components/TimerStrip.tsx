import { Box, Chip, Tooltip } from '@mui/material';
import StopIcon from '@mui/icons-material/Stop';
import { useEffect, useState } from 'react';
import { useActions, useActiveTimers } from '../api/hooks';
import { clock } from '../lib/format';

export function useNow(intervalMs = 1000): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(t);
  }, [intervalMs]);
  return now;
}

/** Running timers with live elapsed time; clicking one stops it. */
export function TimerStrip() {
  const { data: timers = [] } = useActiveTimers();
  const { stop } = useActions();
  const now = useNow();
  if (timers.length === 0) return null;
  return (
    <Box sx={{ display: 'flex', gap: 0.75, overflowX: 'auto', minWidth: 0, maxWidth: '45vw' }}>
      {timers.map((e) => (
        <Tooltip key={e.id} title="Stop timer">
          <Chip
            color="primary"
            variant="outlined"
            icon={<StopIcon />}
            onClick={() => stop.mutate(e.task_id)}
            label={
              <Box component="span" sx={{ display: 'inline-flex', gap: 1 }}>
                <Box component="span" sx={{ maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {e.task_title}
                </Box>
                <Box component="span" sx={{ fontVariantNumeric: 'tabular-nums', fontWeight: 600 }}>
                  {clock((now - new Date(e.started_at).getTime()) / 1000)}
                </Box>
              </Box>
            }
          />
        </Tooltip>
      ))}
    </Box>
  );
}
