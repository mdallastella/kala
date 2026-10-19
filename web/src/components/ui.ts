import { createContext, useContext } from 'react';
import type { Task } from '../api/types';

export interface NotifyAction {
  label: string;
  run: () => void;
}

export interface Ui {
  notify: (message: string, action?: NotifyAction) => void;
  quickAdd: (opts?: { timer?: boolean; prefill?: string }) => void;
  palette: (prefill?: string) => void;
  help: () => void;
  pickTags: (tasks: Task[], mode?: 'tags' | 'project') => void;
  pickDate: (tasks: Task[], kind: 'due' | 'remind') => void;
  /** The filter currently shown, used as quick-add context. */
  currentQuery: string;
}

export const UiContext = createContext<Ui | null>(null);

export function useUi(): Ui {
  const v = useContext(UiContext);
  if (!v) throw new Error('useUi outside UiProvider');
  return v;
}
