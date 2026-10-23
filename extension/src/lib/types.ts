// Subsets of the kala API types (web/src/api/types.ts) the extension uses.

export interface User {
  id: number;
  email: string;
}

export interface Task {
  id: number;
  title: string;
  tags: string[];
}

export interface Tag {
  id: number;
  name: string;
  color: string | null;
  open: number;
}

export interface QuickParse {
  title: string;
  tags: string[];
  due_date?: string;
  remind_at?: string;
}

export interface ApiErrorBody {
  code: string;
  message: string;
}
