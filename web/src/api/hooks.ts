import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query';
import { del, get, patch, post, put } from './client';
import type {
  AccessToken,
  Namespace,
  QuickParse,
  RenamePreview,
  Report,
  Tag,
  Task,
  TimeEntry,
  User,
  WorkingDay,
} from './types';

/** Query keys grouped by the resource kinds the server reports in SSE events. */
const kindKeys: Record<string, string[][]> = {
  tasks: [['tasks'], ['task'], ['trash'], ['digest']],
  tags: [['tags']],
  timers: [['timers'], ['entries'], ['report'], ['tasks'], ['task'], ['digest']],
  namespaces: [['namespaces']],
  me: [['me'], ['workingHours']],
  tokens: [['tokens']],
};

export function invalidateKinds(qc: QueryClient, kinds: string[]) {
  for (const k of kinds) for (const key of kindKeys[k] ?? []) void qc.invalidateQueries({ queryKey: key });
}

export const useMe = () => useQuery({ queryKey: ['me'], queryFn: () => get<User>('/me') });

export const useTasks = (q: string, limit?: number) =>
  useQuery({
    queryKey: ['tasks', q, limit],
    queryFn: () => get<Task[]>(`/tasks?q=${encodeURIComponent(q)}${limit ? `&limit=${limit}` : ''}`),
    placeholderData: (prev) => prev,
  });

export const useTask = (id: number | null) =>
  useQuery({ queryKey: ['task', id], queryFn: () => get<Task>(`/tasks/${id}`), enabled: id != null });

export const useTrash = () => useQuery({ queryKey: ['trash'], queryFn: () => get<Task[]>('/trash') });
export const useTags = () => useQuery({ queryKey: ['tags'], queryFn: () => get<Tag[]>('/tags') });
export const useNamespaces = () => useQuery({ queryKey: ['namespaces'], queryFn: () => get<Namespace[]>('/namespaces') });
export const useActiveTimers = () => useQuery({ queryKey: ['timers'], queryFn: () => get<TimeEntry[]>('/timers/active') });
export const useWorkingHours = () =>
  useQuery({ queryKey: ['workingHours'], queryFn: () => get<WorkingDay[]>('/me/working-hours') });
export const useTokens = () => useQuery({ queryKey: ['tokens'], queryFn: () => get<AccessToken[]>('/tokens') });

export const useEntries = (from: string, to: string, task?: number) =>
  useQuery({
    queryKey: ['entries', from, to, task],
    queryFn: () =>
      get<TimeEntry[]>(
        `/time-entries?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}${task ? `&task=${task}` : ''}`,
      ),
  });

export const useReport = (from: string, to: string, group: string, q = '') =>
  useQuery({
    queryKey: ['report', from, to, group, q],
    queryFn: () =>
      get<Report>(
        `/reports/time?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}&group=${encodeURIComponent(group)}&q=${encodeURIComponent(q)}`,
      ),
  });

export const parseQuick = (quick: string) => post<QuickParse>('/tasks/parse', { quick });

export interface TaskPatch {
  title?: string;
  notes?: string;
  due_date?: string | null;
  remind_at?: string | null;
  tags?: string[];
  add_tags?: string[];
  remove_tags?: string[];
  closed?: boolean;
}

export interface BulkInput {
  ids: number[];
  op: 'add_tags' | 'remove_tags' | 'set_due' | 'close' | 'reopen' | 'delete' | 'restore';
  tags?: string[];
  due_date?: string | null;
}

/** Mutations invalidate locally as well, in case the event stream is down. */
export function useActions() {
  const qc = useQueryClient();
  const done =
    (...kinds: string[]) =>
    () =>
      invalidateKinds(qc, kinds);
  const m = <A, R>(fn: (a: A) => Promise<R>, ...kinds: string[]) =>
    // eslint-disable-next-line react-hooks/rules-of-hooks -- fixed call order
    useMutation({ mutationFn: fn, onSuccess: done(...kinds) });
  return {
    create: m((in_: { quick?: string; title?: string; tags?: string[] }) => post<Task>('/tasks', in_), 'tasks', 'tags'),
    update: m(({ id, ...p }: TaskPatch & { id: number }) => patch<Task>(`/tasks/${id}`, p), 'tasks', 'tags', 'timers'),
    remove: m((id: number) => del(`/tasks/${id}`), 'tasks', 'tags', 'timers'),
    restore: m((id: number) => post<Task>(`/tasks/${id}/restore`), 'tasks', 'tags'),
    purge: m((id: number) => del(`/trash/${id}`), 'tasks'),
    bulk: m((b: BulkInput) => post('/tasks/bulk', b), 'tasks', 'tags', 'timers'),
    start: m((id: number) => post<Task>(`/tasks/${id}/timer/start`), 'timers', 'tasks'),
    stop: m((id: number) => post<Task>(`/tasks/${id}/timer/stop`), 'timers', 'tasks'),
    startNew: m((quick: string) => post<Task>('/timers/start-new', { quick }), 'timers', 'tasks', 'tags'),
    stopAll: m(() => post<{ stopped: number }>('/timers/stop-all'), 'timers', 'tasks'),
    createEntry: m(
      (e: { task_id: number; started_at: string; stopped_at: string; note?: string }) => post<TimeEntry>('/time-entries', e),
      'timers',
    ),
    updateEntry: m(
      ({ id, ...e }: { id: number; started_at?: string; stopped_at?: string; note?: string }) =>
        patch<TimeEntry>(`/time-entries/${id}`, e),
      'timers',
    ),
    deleteEntry: m((id: number) => del(`/time-entries/${id}`), 'timers'),
    setTagMeta: m(
      (t: { name: string; color?: string | null; pinned?: boolean; position?: number }) => put<Tag>('/tags', t),
      'tags',
    ),
    renameTag: m((r: { from: string; to: string; confirm: boolean }) => post<RenamePreview>('/tags/rename', r), 'tags', 'tasks'),
    deleteTag: m((id: number) => del(`/tags/${id}`), 'tags', 'tasks'),
    saveNamespace: m(
      (n: Omit<Namespace, 'id'> & { id?: number }) =>
        n.id ? put<Namespace>(`/namespaces/${n.id}`, { ...n, id: undefined }) : post<Namespace>('/namespaces', n),
      'namespaces', 'tasks', 'tags',
    ),
    deleteNamespace: m((id: number) => del(`/namespaces/${id}`), 'namespaces', 'tasks'),
    ackReminder: m(({ id, snooze_until }: { id: number; snooze_until?: string }) => post<Task>(`/reminders/${id}/ack`, snooze_until ? { snooze_until } : undefined), 'tasks'),
    updateMe: m((p: { timezone: string }) => patch<User>('/me', p), 'me', 'tasks'),
    setWorkingHours: m((days: WorkingDay[]) => put<WorkingDay[]>('/me/working-hours', days), 'me'),
    createToken: m((name: string) => post<AccessToken>('/tokens', { name }), 'tokens'),
    revokeToken: m((id: number) => del(`/tokens/${id}`), 'tokens'),
  };
}

export type Actions = ReturnType<typeof useActions>;
