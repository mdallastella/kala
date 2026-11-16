import { createContext, useContext } from 'react';
import type { Task } from '../api/types';

export interface NotifyAction {
  label: string;
  run: () => void;
}

export interface ConfirmOptions {
  title: string;
  message?: string;
  /** Label of the destructive button; defaults to "Delete". */
  confirmLabel?: string;
}

export interface Ui {
  notify: (message: string, action?: NotifyAction) => void;
  quickAdd: (opts?: { timer?: boolean; prefill?: string }) => void;
  palette: (prefill?: string) => void;
  help: () => void;
  pickTags: (tasks: Task[], mode?: 'tags' | 'project') => void;
  pickDate: (tasks: Task[], kind: 'due' | 'remind') => void;
  /** Asks before a destructive action; resolves true when confirmed. */
  confirm: (opts: ConfirmOptions) => Promise<boolean>;
  /** The filter currently shown, used as quick-add context. */
  currentQuery: string;
}

export const UiContext = createContext<Ui | null>(null);

export function useUi(): Ui {
  const v = useContext(UiContext);
  if (!v) throw new Error('useUi outside UiProvider');
  return v;
}
