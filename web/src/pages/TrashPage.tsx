import { Box, Button, List, ListItem, ListItemButton, ListItemText, Typography } from '@mui/material';
import { useState } from 'react';
import { errorMessage } from '../api/client';
import { useActions, useTrash } from '../api/hooks';
import { TagChip } from '../components/TagChip';
import { useUi } from '../components/ui';
import { useHotkeys } from '../keys/hotkeys';
import { fullDateTime } from '../lib/format';
import { chipTags } from '../lib/tags';
import { usePageTitle } from '../lib/usePageTitle';

export function TrashPage() {
  const { data: tasks = [] } = useTrash();
  const { restore, purge } = useActions();
  const ui = useUi();
  usePageTitle('Trash');
  const [rawFocus, setFocus] = useState(0);
  const focus = Math.min(rawFocus, Math.max(tasks.length - 1, 0));
  const current = tasks[focus];
  const fail = (e: unknown) => ui.notify(errorMessage(e));

  const doRestore = () => current && restore.mutate(current.id, { onSuccess: () => ui.notify(`Restored “${current.title}”`), onError: fail });
  const doPurge = async () => {
    if (!current) return;
    if (await ui.confirm({ title: `Delete “${current.title}” forever?`, message: 'Its time entries are deleted too.', confirmLabel: 'Delete forever' }))
      purge.mutate(current.id, { onError: fail });
  };

  useHotkeys([
    { keys: 'j', description: 'Next', group: 'Trash', run: () => setFocus((f) => Math.min(f + 1, tasks.length - 1)) },
    { keys: 'k', description: 'Previous', group: 'Trash', run: () => setFocus((f) => Math.max(f - 1, 0)) },
    { keys: 'u', description: 'Restore task', group: 'Trash', run: doRestore },
    { keys: 'delete', description: 'Delete forever', group: 'Trash', run: doPurge },
  ]);

  return (
    <Box sx={{ p: 2, overflowY: 'auto' }}>
      <Typography variant="h6" sx={{ fontWeight: 700 }}>
        Trash
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Deleted tasks are kept for 30 days. j/k move · u restore · Del delete forever
      </Typography>
      <List disablePadding>
        {tasks.map((t, i) => (
          <ListItem
            key={t.id}
            disablePadding
            sx={{ borderBottom: 1, borderColor: 'divider' }}
            secondaryAction={
              <Button size="small" onClick={() => { setFocus(i); restore.mutate(t.id, { onError: fail }); }}>
                Restore
              </Button>
            }
          >
            <ListItemButton data-hotkeys selected={i === focus} onClick={() => setFocus(i)} sx={{ pr: '96px !important' }}>
              <ListItemText
                primary={t.title}
                secondary={`Deleted ${fullDateTime(t.deleted_at!)}`}
              />
              <Box sx={{ display: 'flex', gap: 0.5 }}>
                {chipTags(t.tags).map((x) => (
                  <TagChip key={x} name={x} />
                ))}
              </Box>
            </ListItemButton>
          </ListItem>
        ))}
      </List>
      {tasks.length === 0 && <Typography color="text.secondary">The trash is empty.</Typography>}
    </Box>
  );
}
