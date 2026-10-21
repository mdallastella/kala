import { useState } from 'react';

/** Drag data type carrying task ids from the task list to drop targets. */
const TASK_MIME = 'application/x-kala-tasks';

export function dragTasks(e: React.DragEvent, ids: number[]) {
  e.dataTransfer.setData(TASK_MIME, JSON.stringify(ids));
  e.dataTransfer.effectAllowed = 'move';
}

function draggedTasks(e: React.DragEvent): number[] {
  try {
    const ids: unknown = JSON.parse(e.dataTransfer.getData(TASK_MIME));
    return Array.isArray(ids) ? ids.filter((x): x is number => typeof x === 'number') : [];
  } catch {
    return [];
  }
}

/**
 * Makes an element a drop target for dragged tasks. Spread `props` on the
 * element; `over` is true while tasks hover it.
 */
export function useTaskDrop(onDrop: (ids: number[]) => void, enabled = true) {
  const [over, setOver] = useState(false);
  const accepts = (e: React.DragEvent) => enabled && e.dataTransfer.types.includes(TASK_MIME);
  const props = {
    onDragOver: (e: React.DragEvent) => {
      if (!accepts(e)) return;
      e.preventDefault();
      e.dataTransfer.dropEffect = 'move';
      if (!over) setOver(true);
    },
    onDragLeave: (e: React.DragEvent) => {
      if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setOver(false);
    },
    onDrop: (e: React.DragEvent) => {
      setOver(false);
      if (!accepts(e)) return;
      e.preventDefault();
      const ids = draggedTasks(e);
      if (ids.length) onDrop(ids);
    },
  };
  return { over, props };
}
