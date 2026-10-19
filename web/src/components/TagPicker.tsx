import { Autocomplete, Button, Dialog, DialogActions, DialogContent, DialogTitle, TextField, Typography } from '@mui/material';
import { useMemo, useState } from 'react';
import { errorMessage } from '../api/client';
import { useActions, useNamespaces, useTags } from '../api/hooks';
import type { Task } from '../api/types';
import { TagChip } from './TagChip';

interface Props {
  tasks: Task[];
  mode: 'tags' | 'project';
  onClose: () => void;
  onError: (msg: string) => void;
}

const PROJECT = 'project:';

/** Edits the tags (or only the projects) of one or more tasks. */
export function TagPicker({ tasks, mode, onClose, onError }: Props) {
  const { data: tags = [] } = useTags();
  const { data: nss = [] } = useNamespaces();
  const { update, bulk } = useActions();
  const inScope = (t: string) => (mode === 'project' ? t.startsWith(PROJECT) : true);

  // Tags common to all targets are editable; the rest stay untouched.
  const initial = useMemo(() => {
    if (tasks.length === 0) return [];
    return tasks[0].tags.filter((t) => inScope(t) && tasks.every((x) => x.tags.includes(t)));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tasks, mode]);
  const [value, setValue] = useState<string[]>(initial);
  const [input, setInput] = useState('');

  const options = useMemo(() => {
    const names = new Set(tags.map((t) => t.name));
    // Offer every defined namespace value too, even if unused so far.
    for (const ns of nss) for (const v of ns.values) names.add(`${ns.name}:${v.value}`);
    return [...names].filter(inScope).sort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tags, nss, mode]);

  const normalize = (s: string) => {
    const t = s.trim().replace(/^#/, '').toLowerCase();
    if (!t) return '';
    return mode === 'project' && !t.startsWith(PROJECT) ? PROJECT + t : t;
  };

  const save = async () => {
    const pending = normalize(input);
    const final = pending && !value.includes(pending) ? [...value, pending] : value;
    const add = final.filter((t) => !initial.includes(t));
    const remove = initial.filter((t) => !final.includes(t));
    try {
      if (tasks.length === 1) {
        if (add.length || remove.length) await update.mutateAsync({ id: tasks[0].id, add_tags: add, remove_tags: remove });
      } else {
        const ids = tasks.map((t) => t.id);
        if (remove.length) await bulk.mutateAsync({ ids, op: 'remove_tags', tags: remove });
        if (add.length) await bulk.mutateAsync({ ids, op: 'add_tags', tags: add });
      }
      onClose();
    } catch (e) {
      onError(errorMessage(e));
    }
  };

  const label = (t: string) => (mode === 'project' ? t.slice(PROJECT.length) : t);
  const title = mode === 'project' ? 'Projects' : 'Tags';

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle sx={{ pb: 1 }}>
        {title}
        {tasks.length > 1 ? ` · ${tasks.length} tasks` : tasks[0] ? ` · ${tasks[0].title}` : ''}
      </DialogTitle>
      <DialogContent>
        <Autocomplete
          multiple
          freeSolo
          autoHighlight
          options={options}
          value={value}
          inputValue={input}
          onInputChange={(_, v) => setInput(v)}
          onChange={(_, v) => setValue([...new Set(v.map((x) => normalize(x)).filter(Boolean))])}
          getOptionLabel={label}
          renderValue={(vals, getItemProps) =>
            vals.map((t, index) => {
              const { key, ...itemProps } = getItemProps({ index });
              return <TagChip key={key} name={t} short={mode === 'project'} {...itemProps} />;
            })
          }
          renderInput={(params) => (
            <TextField
              {...params}
              autoFocus
              placeholder={mode === 'project' ? 'kala:api' : 'client:acme, status:waiting…'}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && (e.ctrlKey || e.metaKey || input === '')) {
                  e.preventDefault();
                  e.stopPropagation();
                  void save();
                }
              }}
            />
          )}
          sx={{ mt: 1 }}
        />
        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 1 }}>
          Enter adds · Backspace removes · Enter on empty input (or Ctrl+Enter) saves · Esc cancels
        </Typography>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" onClick={() => void save()}>
          Save
        </Button>
      </DialogActions>
    </Dialog>
  );
}
