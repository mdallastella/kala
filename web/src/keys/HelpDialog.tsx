import { Box, Dialog, DialogContent, DialogTitle, Typography } from '@mui/material';
import { useRegisteredBindings } from './hotkeys';
import { keyLabel } from './matcher';

export function HelpDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const bindings = useRegisteredBindings().filter((b) => !b.hidden);
  const groups = new Map<string, { keys: string[]; description: string }[]>();
  for (const b of bindings) {
    const list = groups.get(b.group) ?? [];
    const same = list.find((x) => x.description === b.description);
    if (same) {
      if (!same.keys.includes(b.keys)) same.keys.push(b.keys);
    } else list.push({ keys: [b.keys], description: b.description });
    groups.set(b.group, list);
  }
  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>Keyboard shortcuts</DialogTitle>
      <DialogContent>
        <Box sx={{ columnCount: { xs: 1, md: 2 }, columnGap: 4 }}>
          {[...groups].map(([group, list]) => (
            <Box key={group} sx={{ breakInside: 'avoid', mb: 2 }}>
              <Typography variant="overline" color="text.secondary">
                {group}
              </Typography>
              {list.map((b) => (
                <Box key={b.description} sx={{ display: 'flex', justifyContent: 'space-between', gap: 2, py: 0.25 }}>
                  <Typography variant="body2">{b.description}</Typography>
                  <Typography variant="body2" sx={{ fontFamily: 'monospace', color: 'text.secondary', whiteSpace: 'nowrap' }}>
                    {b.keys.map(keyLabel).join(' / ')}
                  </Typography>
                </Box>
              ))}
            </Box>
          ))}
        </Box>
        <Typography variant="caption" color="text.secondary">
          Filter syntax: project:kala · -project: (inbox) · is:open · due:&lt;=today · priority:high · "text" · sort:due
        </Typography>
      </DialogContent>
    </Dialog>
  );
}
