-- Per-task lookups of time entries (last tracked, totals).
CREATE INDEX time_entries_task ON time_entries (task_id, started_at);
