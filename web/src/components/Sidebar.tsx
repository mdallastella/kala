import { Box, Collapse, IconButton, List, ListItemButton, ListItemIcon, ListItemText, ListSubheader, Typography } from '@mui/material';
import AccessTimeIcon from '@mui/icons-material/AccessTime';
import CalendarMonthOutlinedIcon from '@mui/icons-material/CalendarMonthOutlined';
import ChecklistIcon from '@mui/icons-material/Checklist';
import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import DeleteOutlinedIcon from '@mui/icons-material/DeleteOutlined';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import FolderOutlinedIcon from '@mui/icons-material/FolderOutlined';
import HomeOutlinedIcon from '@mui/icons-material/HomeOutlined';
import LocalOfferOutlinedIcon from '@mui/icons-material/LocalOfferOutlined';
import PushPinOutlinedIcon from '@mui/icons-material/PushPinOutlined';
import SettingsOutlinedIcon from '@mui/icons-material/SettingsOutlined';
import { useMemo, useState } from 'react';
import { useLocation, useNavigate, useSearchParams } from 'react-router';
import { errorMessage } from '../api/client';
import { useActions, useTags } from '../api/hooks';
import { useTaskDrop } from '../lib/dnd';
import { tasksPath } from '../lib/useCurrentQuery';
import type { Tag } from '../api/types';
import { buildTree, tagColor, type TagNode } from '../lib/tags';
import { useTagMap } from './TagChip';
import { useUi } from './ui';

export function tagQuery(name: string): string {
  return name.includes(':') ? `${name} is:open` : `#${name} is:open`;
}

/** Drop target that adds a tag to the dropped tasks. */
function useTagDrop(tag: string, enabled = true) {
  const { bulk } = useActions();
  const ui = useUi();
  return useTaskDrop(
    (ids) =>
      bulk.mutate(
        { ids, op: 'add_tags', tags: [tag] },
        {
          onSuccess: () => ui.notify(`Tagged ${ids.length === 1 ? '1 task' : `${ids.length} tasks`} with ${tag}`),
          onError: (e) => ui.notify(errorMessage(e)),
        },
      ),
    enabled,
  );
}

const dropSx = (over: boolean) => (over ? { outline: 2, outlineColor: 'primary.main', outlineOffset: -2 } : {});

function Count({ n }: { n: number }) {
  return n > 0 ? (
    <Typography variant="caption" color="text.secondary" sx={{ ml: 1 }}>
      {n}
    </Typography>
  ) : null;
}

function TreeItem({ node, depth, current }: { node: TagNode; depth: number; current: string }) {
  const [open, setOpen] = useState(depth === 0);
  const navigate = useNavigate();
  const map = useTagMap();
  const q = tagQuery(node.name);
  const hasKids = node.children.length > 0;
  // A bare namespace (status, client…) is not a tag you can apply.
  const drop = useTagDrop(node.name, node.name.includes(':') || !hasKids);
  return (
    <>
      <ListItemButton
        dense
        selected={current === q}
        sx={{ pl: 1 + depth * 1.5, py: 0.25, ...dropSx(drop.over) }}
        onClick={() => navigate(tasksPath(q))}
        {...drop.props}
      >
        <Box sx={{ width: 24, display: 'flex' }}>
          {hasKids && (
            <IconButton
              size="small"
              sx={{ p: 0 }}
              onClick={(e) => {
                e.stopPropagation();
                setOpen(!open);
              }}
              aria-label={open ? 'Collapse' : 'Expand'}
            >
              {open ? <ExpandMoreIcon fontSize="small" /> : <ChevronRightIcon fontSize="small" />}
            </IconButton>
          )}
        </Box>
        <Box sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: tagColor(node.name, map), mr: 1, flexShrink: 0 }} />
        <ListItemText primary={node.label} slotProps={{ primary: { noWrap: true, variant: 'body2' } }} />
        <Count n={node.open} />
      </ListItemButton>
      {hasKids && (
        <Collapse in={open} unmountOnExit>
          {node.children.map((c) => (
            <TreeItem key={c.name} node={c} depth={depth + 1} current={current} />
          ))}
        </Collapse>
      )}
    </>
  );
}

function PinnedItem({ tag, current }: { tag: Tag; current: string }) {
  const navigate = useNavigate();
  const drop = useTagDrop(tag.name);
  const q = tagQuery(tag.name);
  return (
    <ListItemButton dense selected={current === q} sx={dropSx(drop.over)} onClick={() => navigate(tasksPath(q))} {...drop.props}>
      <ListItemIcon>
        <PushPinOutlinedIcon fontSize="small" />
      </ListItemIcon>
      <ListItemText primary={tag.name} slotProps={{ primary: { noWrap: true } }} />
      <Count n={tag.open} />
    </ListItemButton>
  );
}

export function Sidebar() {
  const { data: tags = [] } = useTags();
  const navigate = useNavigate();
  const location = useLocation();
  const [params] = useSearchParams();
  const current = location.pathname === '/tasks' ? (params.get('q') ?? '') : '';
  const tree = useMemo(() => buildTree(tags), [tags]);
  const projects = tree.find((n) => n.name === 'project');
  const others = tree.filter((n) => n.name !== 'project' && n.name !== 'status' && n.name !== 'priority');
  const statusTree = tree.filter((n) => n.name === 'status' || n.name === 'priority');
  const pinned = tags.filter((t) => t.pinned);

  const nav = (path: string, label: string, icon: React.ReactNode, hint: string) => (
    <ListItemButton dense selected={location.pathname === path && !current} onClick={() => navigate(path)}>
      <ListItemIcon>{icon}</ListItemIcon>
      <ListItemText primary={label} />
      <Typography variant="caption" color="text.disabled" sx={{ fontFamily: 'monospace' }}>
        {hint}
      </Typography>
    </ListItemButton>
  );

  return (
    <Box sx={{ overflowY: 'auto', height: '100%', pb: 2 }}>
      <List dense>
        {nav('/', 'Home', <HomeOutlinedIcon fontSize="small" />, 'g h')}
        {nav('/tasks', 'Tasks', <ChecklistIcon fontSize="small" />, 'g t')}
      </List>
      {pinned.length > 0 && (
        <List dense subheader={<ListSubheader>Pinned</ListSubheader>}>
          {pinned.map((t) => (
            <PinnedItem key={t.id} tag={t} current={current} />
          ))}
        </List>
      )}
      <List
        dense
        subheader={
          <ListSubheader sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <FolderOutlinedIcon fontSize="inherit" /> Projects
          </ListSubheader>
        }
      >
        {projects ? (
          projects.children.map((n) => <TreeItem key={n.name} node={n} depth={0} current={current} />)
        ) : (
          <Typography variant="caption" color="text.secondary" sx={{ px: 2, display: "block" }}>
            Add “@name” to a task to create a project
          </Typography>
        )}
      </List>
      {others.length > 0 && (
        <List
          dense
          subheader={
            <ListSubheader sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <LocalOfferOutlinedIcon fontSize="inherit" /> Tags
            </ListSubheader>
          }
        >
          {others.map((n) => (
            <TreeItem key={n.name} node={n} depth={0} current={current} />
          ))}
        </List>
      )}
      <List dense subheader={<ListSubheader>Status &amp; priority</ListSubheader>}>
        {statusTree.map((n) => (
          <TreeItem key={n.name} node={n} depth={0} current={current} />
        ))}
      </List>
      <List dense>
        {nav('/calendar', 'Calendar', <CalendarMonthOutlinedIcon fontSize="small" />, 'g c')}
        {nav('/time', 'Time', <AccessTimeIcon fontSize="small" />, 'g m')}
        {nav('/trash', 'Trash', <DeleteOutlinedIcon fontSize="small" />, 'g x')}
        {nav('/settings', 'Settings', <SettingsOutlinedIcon fontSize="small" />, 'g ,')}
      </List>
    </Box>
  );
}
