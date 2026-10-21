-- Status set: open, pending (was waiting), delegated, closed†, cancelled†.
-- Only status namespaces that have "waiting" and no "pending" are renamed.
CREATE TEMP TABLE rename_ns AS
SELECT n.id, n.user_id FROM namespaces n
WHERE n.name = 'status'
  AND EXISTS (SELECT 1 FROM namespace_values v WHERE v.namespace_id = n.id AND v.value = 'waiting')
  AND NOT EXISTS (SELECT 1 FROM namespace_values v WHERE v.namespace_id = n.id AND v.value = 'pending');

-- Tasks keep their tag: task_tags references tags by id. A stray
-- status:pending tag (possible if it once existed) absorbs status:waiting.
INSERT OR IGNORE INTO task_tags (task_id, tag_id)
SELECT tt.task_id, p.id FROM task_tags tt
JOIN tags w ON w.id = tt.tag_id AND w.name = 'status:waiting'
JOIN tags p ON p.user_id = w.user_id AND p.name = 'status:pending'
WHERE w.user_id IN (SELECT user_id FROM rename_ns);

DELETE FROM tags
WHERE name = 'status:waiting'
  AND user_id IN (SELECT user_id FROM rename_ns)
  AND EXISTS (SELECT 1 FROM tags p WHERE p.user_id = tags.user_id AND p.name = 'status:pending');

UPDATE tags SET name = 'status:pending'
WHERE name = 'status:waiting' AND user_id IN (SELECT user_id FROM rename_ns);

UPDATE namespace_values SET value = 'pending'
WHERE value = 'waiting' AND namespace_id IN (SELECT id FROM rename_ns);

UPDATE namespaces SET default_value = 'pending'
WHERE default_value = 'waiting' AND id IN (SELECT id FROM rename_ns);

DROP TABLE rename_ns;

INSERT OR IGNORE INTO namespace_values (namespace_id, value, position, terminal)
SELECT id, 'delegated', 0, 0 FROM namespaces WHERE name = 'status';

INSERT OR IGNORE INTO namespace_values (namespace_id, value, position, terminal)
SELECT id, 'cancelled', 0, 1 FROM namespaces WHERE name = 'status';

-- Known values first, in canonical order; custom values keep their relative order after them.
UPDATE namespace_values SET position = CASE value
    WHEN 'open' THEN 0
    WHEN 'pending' THEN 1
    WHEN 'delegated' THEN 2
    WHEN 'closed' THEN 3
    WHEN 'cancelled' THEN 4
    ELSE 5 + position
END
WHERE namespace_id IN (SELECT id FROM namespaces WHERE name = 'status');
