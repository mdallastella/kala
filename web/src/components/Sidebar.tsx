import { Box, Collapse, IconButton, List, ListItem, ListItemButton, ListItemIcon, ListItemText, ListSubheader, Typography } from '@mui/material';
import AccessTimeIcon from '@mui/icons-material/AccessTime';
import CalendarMonthOutlinedIcon from '@mui/icons-material/CalendarMonthOutlined';
import ChecklistIcon from '@mui/icons-material/Checklist';
import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import DeleteOutlinedIcon from '@mui/icons-material/DeleteOutlined';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import FolderOutlinedIcon from '@mui/icons-material/FolderOutlined';
import HomeOutlinedIcon from '@mui/icons-material/HomeOutlined';
import LabelOutlinedIcon from '@mui/icons-material/LabelOutlined';
import LocalOfferOutlinedIcon from '@mui/icons-material/LocalOfferOutlined';
import PushPinOutlinedIcon from '@mui/icons-material/PushPinOutlined';
import SettingsOutlinedIcon from '@mui/icons-material/SettingsOutlined';
import { useMemo, useState } from 'react';
import { useLocation, useNavigate, useSearchParams } from 'react-router';
import { errorMessage } from '../api/client';
import { useActions, useNamespaces, useTags } from '../api/hooks';
import { useTaskDrop } from '../lib/dnd';
import { tasksPath } from '../lib/useCurrentQuery';
import type { Tag } from '../api/types';
import { buildTree, tagColor, withNamespaceValues, type TagNode } from '../lib/tags';
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

function DeleteTagButton({ tag, selected }: { tag: Tag; selected: boolean }) {
  const { deleteTag } = useActions();
  const navigate = useNavigate();
  const ui = useUi();
  return (
    <IconButton
      className="tag-delete"
      size="small"
      sx={{ p: 0.25, mx: 0.5, opacity: 0, '&:focus-visible': { opacity: 1 } }}
      aria-label={`Delete tag ${tag.name}`}
      onClick={() => {
        if (!window.confirm(`Remove “${tag.name}” from all ${tag.total} tasks?`)) return;
        deleteTag.mutate(tag.id, {
          onSuccess: () => {
            ui.notify(`Deleted ${tag.name}`);
            if (selected) navigate('/tasks');
          },
          onError: (err) => ui.notify(errorMessage(err)),
        });
      }}
    >
      <DeleteOutlinedIcon sx={{ fontSize: 16 }} />
    </IconButton>
  );
}

/** `listed` holds the tags that are listed namespace values: managed in Settings, so no bin. */
function TreeItem({ node, depth, current, listed }: { node: TagNode; depth: number; current: string; listed: Set<string> }) {
  const [open, setOpen] = useState(false);
  const navigate = useNavigate();
  const map = useTagMap();
  const q = tagQuery(node.name);
  const hasKids = node.children.length > 0;
  // A bare namespace (status, client…) is not a tag you can apply.
  const drop = useTagDrop(node.name, node.name.includes(':') || !hasKids);
  return (
    <>
      {/* Expand and delete sit beside the row button, not inside it. */}
      <ListItem disablePadding sx={{ pl: 1 + depth * 1.5, '&:hover .tag-delete, &:focus-within .tag-delete': { opacity: 1 } }}>
        <Box sx={{ width: 24, display: 'flex', flexShrink: 0 }}>
          {hasKids && (
            <IconButton size="small" sx={{ p: 0 }} onClick={() => setOpen(!open)} aria-expanded={open} aria-label={`${open ? 'Collapse' : 'Expand'} ${node.label}`}>
              {open ? <ExpandMoreIcon fontSize="small" /> : <ChevronRightIcon fontSize="small" />}
            </IconButton>
          )}
        </Box>
        <ListItemButton dense selected={current === q} sx={{ pl: 0.5, py: 0.25, ...dropSx(drop.over) }} onClick={() => navigate(tasksPath(q))} {...drop.props}>
          <Box sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: tagColor(node.name, map), mr: 1, flexShrink: 0 }} />
          <ListItemText primary={node.label} slotProps={{ primary: { noWrap: true, variant: 'body2' } }} />
          <Count n={node.open} />
        </ListItemButton>
        {node.tag && !listed.has(node.name) && <DeleteTagButton tag={node.tag} selected={current === q} />}
      </ListItem>
      {hasKids && (
        <Collapse in={open} unmountOnExit>
          {node.children.map((c) => (
            <TreeItem key={c.name} node={c} depth={depth + 1} current={current} listed={listed} />
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

function readOpen(key: string): boolean {
  try {
    return localStorage.getItem(key) === '1';
  } catch {
    return false;
  }
}

/** A sidebar list whose header toggles it; collapsed until opened, remembered per browser. */
function Section({ id, icon, label, children }: { id: string; icon: React.ReactNode; label: string; children: React.ReactNode }) {
  const key = `sidebar.open.${id}`;
  const [open, setOpen] = useState(() => readOpen(key));
  const toggle = () => {
    const next = !open;
    setOpen(next);
    try {
      localStorage.setItem(key, next ? '1' : '0');
    } catch {
      // storage blocked: state just won't persist
    }
  };
  return (
    <List
      dense
      subheader={
        <ListSubheader
          role="button"
          tabIndex={0}
          aria-expanded={open}
          onClick={toggle}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              toggle();
            }
          }}
          sx={{ display: 'flex', alignItems: 'center', gap: 1, cursor: 'pointer', userSelect: 'none' }}
        >
          {icon} {label}
          <Box sx={{ ml: 'auto', display: 'flex' }}>
            {open ? <ExpandMoreIcon fontSize="small" /> : <ChevronRightIcon fontSize="small" />}
          </Box>
        </ListSubheader>
      }
    >
      <Collapse in={open} unmountOnExit>
        {children}
      </Collapse>
    </List>
  );
}

export function Sidebar() {
  const { data: tags = [] } = useTags();
  const navigate = useNavigate();
  const location = useLocation();
  const [params] = useSearchParams();
  const current = location.pathname === '/tasks' ? (params.get('q') ?? '') : '';
  const { data: namespaces = [] } = useNamespaces();
  const tree = useMemo(() => buildTree(tags), [tags]);
  // One section per namespace: project first, then the user's own A–Z, with
  // status and priority after Tags. Roots that are not a namespace go to Tags.
  const { sections, others, listed } = useMemo(() => {
    const names = new Set(namespaces.map((n) => n.name));
    const rank = (name: string) => (name === 'project' ? 0 : name === 'status' ? 2 : name === 'priority' ? 3 : 1);
    const sections = [...namespaces]
      .sort((a, b) => rank(a.name) - rank(b.name) || a.name.localeCompare(b.name))
      .map((ns) => ({ ns: ns.name, node: withNamespaceValues(tree.find((n) => n.name === ns.name), ns) }));
    if (!names.has('project')) {
      const root = tree.find((n) => n.name === 'project');
      sections.unshift({ ns: 'project', node: root ?? { name: 'project', label: 'project', children: [], open: 0 } });
      names.add('project');
    }
    const listed = new Set(namespaces.flatMap((n) => n.values.map((v) => `${n.name}:${v.value}`)));
    return { sections, others: tree.filter((n) => !names.has(n.name)), listed };
  }, [namespaces, tree]);
  const pinned = tags.filter((t) => t.pinned);

  const section = ({ ns, node }: (typeof sections)[number]) => {
    if (ns !== 'project' && node.children.length === 0) return null;
    return (
      <Section
        key={ns}
        id={`ns:${ns}`}
        icon={ns === 'project' ? <FolderOutlinedIcon fontSize="inherit" /> : <LabelOutlinedIcon fontSize="inherit" />}
        label={ns === 'project' ? 'Projects' : ns.charAt(0).toUpperCase() + ns.slice(1)}
      >
        {node.children.length > 0 ? (
          node.children.map((n) => <TreeItem key={n.name} node={n} depth={0} current={current} listed={listed} />)
        ) : (
          <Typography variant="caption" color="text.secondary" sx={{ px: 2, display: 'block' }}>
            Add “@name” to a task to create a project
          </Typography>
        )}
      </Section>
    );
  };
  const isSystem = (ns: string) => ns === 'status' || ns === 'priority';

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
        {nav('/calendar', 'Calendar', <CalendarMonthOutlinedIcon fontSize="small" />, 'g c')}
        {nav('/time', 'Time', <AccessTimeIcon fontSize="small" />, 'g m')}
      </List>
      {pinned.length > 0 && (
        <List dense subheader={<ListSubheader>Pinned</ListSubheader>}>
          {pinned.map((t) => (
            <PinnedItem key={t.id} tag={t} current={current} />
          ))}
        </List>
      )}
      {sections.filter((s) => !isSystem(s.ns)).map(section)}
      {others.length > 0 && (
        <Section id="tags" icon={<LocalOfferOutlinedIcon fontSize="inherit" />} label="Tags">
          {others.map((n) => (
            <TreeItem key={n.name} node={n} depth={0} current={current} listed={listed} />
          ))}
        </Section>
      )}
      {sections.filter((s) => isSystem(s.ns)).map(section)}
      <List dense>
        {nav('/trash', 'Trash', <DeleteOutlinedIcon fontSize="small" />, 'g x')}
        {nav('/settings', 'Settings', <SettingsOutlinedIcon fontSize="small" />, 'g ,')}
      </List>
    </Box>
  );
}
