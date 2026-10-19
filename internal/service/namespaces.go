package service

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"kala/internal/tags"
)

// StatusNS is the namespace used by close/reopen shortcuts and timers.
const StatusNS = "status"

func (s *Service) listNamespaces(ctx context.Context, q querier, userID int64) ([]tags.Namespace, error) {
	rows, err := q.QueryContext(ctx, `SELECT n.id, n.name, n.exclusive, COALESCE(n.default_value, ''), v.value, v.terminal
		FROM namespaces n LEFT JOIN namespace_values v ON v.namespace_id = n.id
		WHERE n.user_id = ? ORDER BY n.name, v.position`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []tags.Namespace{}
	for rows.Next() {
		var (
			ns       tags.Namespace
			value    sql.NullString
			terminal sql.NullBool
		)
		if err := rows.Scan(&ns.ID, &ns.Name, &ns.Exclusive, &ns.Default, &value, &terminal); err != nil {
			return nil, err
		}
		if len(out) == 0 || out[len(out)-1].ID != ns.ID {
			ns.Values = []tags.Value{}
			out = append(out, ns)
		}
		if value.Valid {
			last := &out[len(out)-1]
			last.Values = append(last.Values, tags.Value{Value: value.String, Terminal: terminal.Bool})
		}
	}
	return out, rows.Err()
}

// Namespaces lists the namespace definitions of a user.
func (s *Service) Namespaces(ctx context.Context, userID int64) ([]tags.Namespace, error) {
	return s.listNamespaces(ctx, s.db, userID)
}

func insertNamespace(ctx context.Context, q querier, userID int64, ns tags.Namespace) (int64, error) {
	var id int64
	var def any
	if ns.Default != "" {
		def = ns.Default
	}
	if err := q.QueryRowContext(ctx, "INSERT INTO namespaces (user_id, name, exclusive, default_value) VALUES (?, ?, ?, ?) RETURNING id",
		userID, ns.Name, ns.Exclusive, def).Scan(&id); err != nil {
		return 0, err
	}
	return id, insertValues(ctx, q, id, ns.Values)
}

func insertValues(ctx context.Context, q querier, nsID int64, values []tags.Value) error {
	for i, v := range values {
		if _, err := q.ExecContext(ctx, "INSERT INTO namespace_values (namespace_id, value, position, terminal) VALUES (?, ?, ?, ?)",
			nsID, v.Value, i, v.Terminal); err != nil {
			return err
		}
	}
	return nil
}

// CreateNamespace defines a new namespace.
func (s *Service) CreateNamespace(ctx context.Context, userID int64, ns tags.Namespace) (*tags.Namespace, error) {
	return s.saveNamespace(ctx, userID, 0, ns)
}

// UpdateNamespace replaces a namespace definition (its name cannot change).
func (s *Service) UpdateNamespace(ctx context.Context, userID, id int64, ns tags.Namespace) (*tags.Namespace, error) {
	return s.saveNamespace(ctx, userID, id, ns)
}

func (s *Service) saveNamespace(ctx context.Context, userID, id int64, ns tags.Namespace) (*tags.Namespace, error) {
	if err := ns.Validate(); err != nil {
		return nil, errf(CodeInvalid, "%s", err.Error())
	}
	err := s.tx(ctx, func(tx *sql.Tx) error {
		if id == 0 {
			var n int
			if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM namespaces WHERE user_id = ? AND name = ?", userID, ns.Name).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				return errf(CodeConflict, "namespace %q already exists", ns.Name)
			}
		} else {
			var name string
			err := tx.QueryRowContext(ctx, "SELECT name FROM namespaces WHERE id = ? AND user_id = ?", id, userID).Scan(&name)
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			if err != nil {
				return err
			}
			ns.Name = name
		}
		if ns.Exclusive {
			if err := checkExclusive(ctx, tx, userID, &ns); err != nil {
				return err
			}
		}
		if id == 0 {
			var err error
			id, err = insertNamespace(ctx, tx, userID, ns)
			if err != nil {
				return err
			}
		} else {
			var def any
			if ns.Default != "" {
				def = ns.Default
			}
			if _, err := tx.ExecContext(ctx, "UPDATE namespaces SET exclusive = ?, default_value = ? WHERE id = ?", ns.Exclusive, def, id); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, "DELETE FROM namespace_values WHERE namespace_id = ?", id); err != nil {
				return err
			}
			if err := insertValues(ctx, tx, id, ns.Values); err != nil {
				return err
			}
		}
		if ns.Default != "" {
			if err := applyDefault(ctx, tx, userID, ns.Name+":"+ns.Default); err != nil {
				return err
			}
		}
		return s.recomputeClosed(ctx, tx, userID, nil)
	})
	if err != nil {
		return nil, err
	}
	s.publish(userID, "namespaces", "tags", "tasks", "timers")
	ns.ID = id
	return &ns, nil
}

// checkExclusive refuses to make a namespace exclusive when existing tasks
// would violate it.
func checkExclusive(ctx context.Context, q querier, userID int64, ns *tags.Namespace) error {
	rows, err := q.QueryContext(ctx, `SELECT tt.task_id, g.name FROM task_tags tt JOIN tags g ON g.id = tt.tag_id
		WHERE g.user_id = ? AND g.name LIKE ? ESCAPE '\'`, userID, escapeLike(ns.Name)+":%")
	if err != nil {
		return err
	}
	defer rows.Close()
	perTask := map[int64]int{}
	var bad []int64
	for rows.Next() {
		var (
			id   int64
			name string
		)
		if err := rows.Scan(&id, &name); err != nil {
			return err
		}
		perTask[id]++
		_, v := tags.Split(name)
		if (perTask[id] > 1 || !ns.Has(v)) && !slices.Contains(bad, id) {
			bad = append(bad, id)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(bad) > 0 {
		return &Error{Code: CodeConflict, Message: "some tasks have several or undefined values in this namespace; fix them first",
			Details: map[string]any{"task_ids": bad}}
	}
	return nil
}

// applyDefault gives every task without a value in the tag's namespace the
// default tag.
func applyDefault(ctx context.Context, q querier, userID int64, tag string) error {
	tagID, err := ensureTag(ctx, q, userID, tag)
	if err != nil {
		return err
	}
	ns, _ := tags.Split(tag)
	_, err = q.ExecContext(ctx, `INSERT INTO task_tags (task_id, tag_id)
		SELECT t.id, ? FROM tasks t WHERE t.user_id = ? AND NOT EXISTS (
			SELECT 1 FROM task_tags tt JOIN tags g ON g.id = tt.tag_id WHERE tt.task_id = t.id AND g.name LIKE ? ESCAPE '\')`,
		tagID, userID, escapeLike(ns)+":%")
	return err
}

// DeleteNamespace removes a namespace definition. Its tags stay on tasks as
// free-form tags.
func (s *Service) DeleteNamespace(ctx context.Context, userID, id int64) error {
	err := s.tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, "DELETE FROM namespaces WHERE id = ? AND user_id = ?", id, userID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return s.recomputeClosed(ctx, tx, userID, nil)
	})
	if err == nil {
		s.publish(userID, "namespaces", "tasks")
	}
	return err
}
