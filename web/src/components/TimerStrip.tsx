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

// The favicon from index.html, plus a red dot while a timer runs.
const logo =
  "<circle cx='16' cy='16' r='14' fill='%235b5bd6'/><path d='M11 9v14M11 16l9-7M11 16l9 7' stroke='white' stroke-width='3' fill='none' stroke-linecap='round'/>";
const dot = "<circle cx='25' cy='7' r='6' fill='%23e5484d' stroke='white' stroke-width='2'/>";
const favicon = (running: boolean) =>
  `data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'>${logo}${running ? dot : ''}</svg>`;

/** Marks the favicon while any timer runs. */
export function useTimerFavicon() {
  const { data: timers = [] } = useActiveTimers();
  const running = timers.length > 0;
  useEffect(() => {
    const link = document.querySelector<HTMLLinkElement>("link[rel='icon']");
    if (link) link.href = favicon(running);
  }, [running]);
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
