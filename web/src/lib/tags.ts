import type { Namespace, Tag } from '../api/types';

export interface TagNode {
  name: string; // full path
  label: string; // last segment
  tag?: Tag;
  children: TagNode[];
  open: number; // open tasks in the subtree
}

export function namespaceOf(tag: string): string {
  const i = tag.indexOf(':');
  return i < 0 ? tag : tag.slice(0, i);
}

export function valueOf(tag: string): string {
  const i = tag.indexOf(':');
  return i < 0 ? '' : tag.slice(i + 1);
}

/** Builds the tag forest; intermediate paths without a tag row become virtual nodes. */
export function buildTree(tags: Tag[]): TagNode[] {
  const roots: TagNode[] = [];
  const index = new Map<string, TagNode>();
  const ensure = (path: string): TagNode => {
    let node = index.get(path);
    if (node) return node;
    const i = path.lastIndexOf(':');
    node = { name: path, label: i < 0 ? path : path.slice(i + 1), children: [], open: 0 };
    index.set(path, node);
    if (i < 0) roots.push(node);
    else ensure(path.slice(0, i)).children.push(node);
    return node;
  };
  for (const t of tags) ensure(t.name).tag = t;
  const sum = (n: TagNode): number => {
    n.children.sort((a, b) => (a.tag?.position ?? 0) - (b.tag?.position ?? 0) || a.label.localeCompare(b.label));
    n.open = (n.tag?.open ?? 0) + n.children.reduce((acc, c) => acc + sum(c), 0);
    return n.open;
  };
  roots.forEach(sum);
  roots.sort((a, b) => a.label.localeCompare(b.label));
  return roots;
}

const palette = ['#5b5bd6', '#0e7c86', '#c2410c', '#15803d', '#be185d', '#7c3aed', '#0369a1', '#a16207', '#b91c1c', '#4d7c0f'];

function hash(s: string): number {
  let h = 0;
  for (const c of s) h = (h * 31 + c.charCodeAt(0)) | 0;
  return Math.abs(h);
}

/** A tag's color: its own, the nearest ancestor's, or a stable hash color. */
export function tagColor(name: string, tags: Map<string, Tag>): string {
  for (let p = name; p; ) {
    const c = tags.get(p)?.color;
    if (c) return c;
    const i = p.lastIndexOf(':');
    p = i < 0 ? '' : p.slice(0, i);
  }
  return palette[hash(name) % palette.length];
}

export const PRIORITY_COLORS: Record<string, string> = { high: '#dc2626', medium: '#ea580c', low: '#2563eb' };

export function priorityOf(tags: string[]): string | null {
  const t = tags.find((x) => x.startsWith('priority:'));
  return t ? valueOf(t) : null;
}

export function statusOf(tags: string[]): string | null {
  const t = tags.find((x) => x.startsWith('status:'));
  return t ? valueOf(t) : null;
}

/** Tags displayed as chips (status and priority are shown as icons). */
export function chipTags(tags: string[]): string[] {
  return tags.filter((t) => !t.startsWith('status:') && !t.startsWith('priority:'));
}

export function namespaceValues(nss: Namespace[] | undefined, name: string): string[] {
  return nss?.find((n) => n.name === name)?.values.map((v) => v.value) ?? [];
}

/** Extracts the project of a single-project filter, for quick-add context. */
export function projectFromQuery(q: string): string | null {
  const m = q.match(/(?:^|\s)project:([^\s,-][^\s,]*)(?=\s|$)/);
  return m ? m[1] : null;
}
