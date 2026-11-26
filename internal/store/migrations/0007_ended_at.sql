-- closed_at becomes ended_at: it is set while a task carries any terminal
-- value (closed, cancelled, …), not only status:closed.
ALTER TABLE tasks RENAME COLUMN closed_at TO ended_at;

-- Sub-values of a terminal value are terminal too (status:closed:won).
-- Before this migration only exact values ended a task, so end the ones
-- carrying a terminal sub-value now, stopping their timers first.
CREATE TEMP TABLE newly_ended AS
SELECT t.id FROM tasks t
WHERE t.ended_at IS NULL AND EXISTS (
    SELECT 1 FROM task_tags tt
    JOIN tags g ON g.id = tt.tag_id
    JOIN namespaces n ON n.user_id = g.user_id
    JOIN namespace_values nv ON nv.namespace_id = n.id AND nv.terminal = 1
    WHERE tt.task_id = t.id
      AND substr(g.name, 1, length(n.name) + length(nv.value) + 2) = n.name || ':' || nv.value || ':'
);

UPDATE time_entries SET stopped_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE stopped_at IS NULL AND task_id IN (SELECT id FROM newly_ended);

UPDATE tasks SET ended_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id IN (SELECT id FROM newly_ended);

DROP TABLE newly_ended;
