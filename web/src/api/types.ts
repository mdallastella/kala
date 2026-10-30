export interface User {
  id: number;
  email: string;
  timezone: string;
  created_at: string;
}

export interface Session {
  access_token: string;
  expires_at: string;
  user: User;
}

export interface Task {
  id: number;
  title: string;
  notes: string;
  tags: string[];
  due_date: string | null;
  remind_at: string | null;
  reminded_at: string | null;
  closed_at: string | null;
  deleted_at?: string | null;
  created_at: string;
  updated_at: string;
  running_since: string | null;
  tracked_seconds: number;
  last_tracked_at: string | null;
  /** Priority value; derived from the due date when there is no priority tag. */
  priority?: string;
  priority_derived?: boolean;
}

export interface Tag {
  id: number;
  name: string;
  color: string | null;
  pinned: boolean;
  position: number;
  open: number;
  total: number;
}

export interface NamespaceValue {
  value: string;
  terminal: boolean;
}

export interface Namespace {
  id: number;
  name: string;
  exclusive: boolean;
  default_value?: string;
  values: NamespaceValue[];
}

export interface TimeEntry {
  id: number;
  task_id: number;
  task_title: string;
  task_tags: string[];
  started_at: string;
  stopped_at: string | null;
  note: string;
  seconds: number;
}

export interface ReportRow {
  key: string;
  label: string;
  seconds: number;
}

export interface Report {
  group: string;
  from: string;
  to: string;
  total_seconds: number;
  rows: ReportRow[];
}

export interface QuickParse {
  title: string;
  tags: string[];
  due_date?: string;
  remind_at?: string;
  /** Time already spent, logged as a completed time entry. */
  spent?: { started_at: string; stopped_at: string };
}

export interface WorkingDay {
  weekday: number;
  start_min: number;
  end_min: number;
}

export interface AccessToken {
  id: number;
  name: string;
  prefix: string;
  created_at: string;
  last_used_at: string | null;
  token?: string;
}

export interface Briefing {
  date: string;
  overdue: Task[];
  due_today: Task[];
  reminders: Task[];
}

export interface Wrapup {
  date: string;
  open: Task[];
  next: Task[];
  next_work_date: string;
  running: TimeEntry[];
}

export interface RenamePreview {
  renames: { from: string; to: string }[];
  merges: string[];
  affected_tasks: number;
  conflicts: number[];
  applied: boolean;
}

export interface ApiErrorBody {
  code: string;
  message: string;
  details?: unknown;
}
