package service

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"slices"

	"kala/internal/tags"
)

// Tag is a tag with presentation metadata and usage counts.
type Tag struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Color    *string `json:"color"`
	Pinned   bool    `json:"pinned"`
	Position int     `json:"position"`
	// Open counts the open, non-deleted tasks carrying exactly this tag.
	Open int `json:"open"`
	// Total counts every non-deleted task carrying exactly this tag.
	Total int `json:"total"`
}

// Tags lists every tag of a user.
func (s *Service) Tags(ctx context.Context, userID int64) ([]Tag, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT g.id, g.name, g.color, g.pinned, g.position,
			COALESCE(SUM(CASE WHEN t.id IS NOT NULL AND t.closed_at IS NULL THEN 1 ELSE 0 END), 0),
			COUNT(t.id)
		FROM tags g
		LEFT JOIN task_tags tt ON tt.tag_id = g.id
		LEFT JOIN tasks t ON t.id = tt.task_id AND t.deleted_at IS NULL
		WHERE g.user_id = ? GROUP BY g.id ORDER BY g.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Tag{}
	for rows.Next() {
		var (
			t     Tag
			color sql.NullString
		)
		if err := rows.Scan(&t.ID, &t.Name, &color, &t.Pinned, &t.Position, &t.Open, &t.Total); err != nil {
			return nil, err
		}
		t.Color = nullString(color)
		out = append(out, t)
	}
	return out, rows.Err()
}

// TagMeta sets presentation metadata on a tag, creating the tag if needed
// (so that a tree node such as "project" can get a color).
type TagMeta struct {
	Name     string           `json:"name"`
	Color    Optional[string] `json:"color"`
	Pinned   Optional[bool]   `json:"pinned"`
	Position Optional[int]    `json:"position"`
}

// SetTagMeta updates tag metadata.
func (s *Service) SetTagMeta(ctx context.Context, userID int64, m TagMeta) (*Tag, error) {
	name, err := tags.Normalize(m.Name)
	if err != nil {
		return nil, errf(CodeInvalid, "%s", err.Error())
	}
	if m.Color.Set && !m.Color.Null && !validColor(m.Color.Value) {
		return nil, errf(CodeInvalid, "color must be #rrggbb")
	}
	err = s.tx(ctx, func(tx *sql.Tx) error {
		id, err := ensureTag(ctx, tx, userID, name)
		if err != nil {
			return err
		}
		if m.Color.Set {
			var v any
			if !m.Color.Null {
				v = m.Color.Value
			}
			if _, err := tx.ExecContext(ctx, "UPDATE tags SET color = ? WHERE id = ?", v, id); err != nil {
				return err
			}
		}
		if m.Pinned.Set {
			if _, err := tx.ExecContext(ctx, "UPDATE tags SET pinned = ? WHERE id = ?", m.Pinned.Value, id); err != nil {
				return err
			}
		}
		if m.Position.Set {
			if _, err := tx.ExecContext(ctx, "UPDATE tags SET position = ? WHERE id = ?", m.Position.Value, id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.publish(userID, "tags")
	all, err := s.Tags(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, t := range all {
		if t.Name == name {
			return &t, nil
		}
	}
	return nil, ErrNotFound
}

func validColor(c string) bool {
	if len(c) != 7 || c[0] != '#' {
		return false
	}
	for _, r := range c[1:] {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

// RenameInput renames a tag subtree.
type RenameInput struct {
	From string `json:"from"`
	To   string `json:"to"`
	// Confirm applies the rename; otherwise only a preview is returned.
	Confirm bool `json:"confirm"`
}

// RenamePreview describes the effect of a rename.
type RenamePreview struct {
	Renames []RenamePair `json:"renames"`
	// Merges lists the target tags that already exist.
	Merges        []string `json:"merges"`
	AffectedTasks int      `json:"affected_tasks"`
	// Conflicts lists tasks that would break exclusive namespaces.
	Conflicts []int64 `json:"conflicts"`
	Applied   bool    `json:"applied"`
}

// RenamePair is one tag renamed.
type RenamePair struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// RenameTag renames a tag and all its descendants, merging into existing
// tags. Without Confirm it only previews.
func (s *Service) RenameTag(ctx context.Context, userID int64, in RenameInput) (*RenamePreview, error) {
	from, err := tags.Normalize(in.From)
	if err != nil {
		return nil, errf(CodeInvalid, "from: %s", err.Error())
	}
	to, err := tags.Normalize(in.To)
	if err != nil {
		return nil, errf(CodeInvalid, "to: %s", err.Error())
	}
	if from == to {
		return nil, errf(CodeInvalid, "from and to are the same")
	}
	if tags.InSubtree(to, from) {
		return nil, errf(CodeInvalid, "cannot rename %q into its own subtree", from)
	}
	var prev RenamePreview
	err = s.tx(ctx, func(tx *sql.Tx) error {
		all, err := allTags(ctx, tx, userID)
		if err != nil {
			return err
		}
		sources := map[int64]string{}
		var srcIDs []int64
		for id, name := range all {
			if tags.InSubtree(name, from) {
				sources[id] = name
				srcIDs = append(srcIDs, id)
			}
		}
		if len(sources) == 0 {
			return errf(CodeNotFound, "no tag %q", from)
		}
		slices.SortFunc(srcIDs, func(a, b int64) int { return cmp.Compare(sources[a], sources[b]) })
		remaining := map[string]bool{}
		for id, name := range all {
			if _, ok := sources[id]; !ok {
				remaining[name] = true
			}
		}
		mapping := map[string]string{}
		prev.Renames, prev.Merges, prev.Conflicts = []RenamePair{}, []string{}, []int64{}
		for _, id := range srcIDs {
			n, _ := tags.Rename(sources[id], from, to)
			mapping[sources[id]] = n
			prev.Renames = append(prev.Renames, RenamePair{From: sources[id], To: n})
			if remaining[n] {
				prev.Merges = append(prev.Merges, n)
			}
		}
		rules, err := s.rules(ctx, tx, userID)
		if err != nil {
			return err
		}
		affected, err := tasksWithTags(ctx, tx, srcIDs)
		if err != nil {
			return err
		}
		prev.AffectedTasks = len(affected)
		for taskID, names := range affected {
			var set []string
			for _, n := range names {
				if m, ok := mapping[n]; ok {
					n = m
				}
				if !slices.Contains(set, n) {
					set = append(set, n)
				}
			}
			if rules.Validate(set) != nil {
				prev.Conflicts = append(prev.Conflicts, taskID)
			}
		}
		slices.Sort(prev.Conflicts)
		if !in.Confirm {
			return nil
		}
		if len(prev.Conflicts) > 0 {
			return &Error{Code: CodeConflict, Message: "the rename would give some tasks several values of an exclusive namespace",
				Details: prev}
		}
		// Move sources out of the way first, so chains such as a:b:b → a:b
		// cannot collide mid-way.
		for _, id := range srcIDs {
			if _, err := tx.ExecContext(ctx, "UPDATE tags SET name = ' renaming ' || id WHERE id = ?", id); err != nil {
				return err
			}
		}
		for _, id := range srcIDs {
			target := mapping[sources[id]]
			var existing int64
			err := tx.QueryRowContext(ctx, "SELECT id FROM tags WHERE user_id = ? AND name = ?", userID, target).Scan(&existing)
			switch {
			case errors.Is(err, sql.ErrNoRows):
				if _, err := tx.ExecContext(ctx, "UPDATE tags SET name = ? WHERE id = ?", target, id); err != nil {
					return err
				}
			case err != nil:
				return err
			default:
				if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO task_tags (task_id, tag_id) SELECT task_id, ? FROM task_tags WHERE tag_id = ?",
					existing, id); err != nil {
					return err
				}
				if _, err := tx.ExecContext(ctx, "UPDATE tags SET color = COALESCE((SELECT color FROM tags WHERE id = ?), color) WHERE id = ?", id, existing); err != nil {
					return err
				}
				if _, err := tx.ExecContext(ctx, "DELETE FROM tags WHERE id = ?", id); err != nil {
					return err
				}
			}
		}
		taskIDs := make([]int64, 0, len(affected))
		for id := range affected {
			taskIDs = append(taskIDs, id)
		}
		prev.Applied = true
		return s.recomputeClosed(ctx, tx, userID, taskIDs)
	})
	if err != nil {
		return nil, err
	}
	if prev.Applied {
		s.publish(userID, "tags", "tasks", "timers", "views")
	}
	return &prev, nil
}

func allTags(ctx context.Context, q querier, userID int64) (map[int64]string, error) {
	rows, err := q.QueryContext(ctx, "SELECT id, name FROM tags WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var (
			id   int64
			name string
		)
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

// tasksWithTags returns the full tag sets of the tasks carrying any of tagIDs.
func tasksWithTags(ctx context.Context, q querier, tagIDs []int64) (map[int64][]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT tt.task_id, g.name FROM task_tags tt JOIN tags g ON g.id = tt.tag_id
		WHERE tt.task_id IN (SELECT task_id FROM task_tags WHERE tag_id IN (SELECT value FROM json_each(?)))`, idsJSON(tagIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]string{}
	for rows.Next() {
		var (
			id   int64
			name string
		)
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = append(out[id], name)
	}
	return out, rows.Err()
}

// DeleteTag removes a tag from every task. Values of exclusive namespaces
// must be removed from the namespace definition instead.
func (s *Service) DeleteTag(ctx context.Context, userID, id int64) error {
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var name string
		err := tx.QueryRowContext(ctx, "SELECT name FROM tags WHERE id = ? AND user_id = ?", id, userID).Scan(&name)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		rules, err := s.rules(ctx, tx, userID)
		if err != nil {
			return err
		}
		ns, v := tags.Split(name)
		if def, ok := rules[ns]; ok && def.Exclusive && def.Has(v) {
			return errf(CodeInvalid, "%q is a value of the exclusive namespace %q; edit the namespace instead", name, ns)
		}
		affected, err := tasksWithTags(ctx, tx, []int64{id})
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM tags WHERE id = ?", id); err != nil {
			return err
		}
		taskIDs := make([]int64, 0, len(affected))
		for t := range affected {
			taskIDs = append(taskIDs, t)
		}
		return s.recomputeClosed(ctx, tx, userID, taskIDs)
	})
	if err == nil {
		s.publish(userID, "tags", "tasks")
	}
	return err
}
