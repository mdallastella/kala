package service

import (
	"context"
	"database/sql"
	"time"

	"kala/internal/tags"
)

// ExportVersion is the current export format version.
const ExportVersion = 1

// Export is the portable dump of one user's data.
type Export struct {
	Version      int              `json:"version"`
	ExportedAt   time.Time        `json:"exported_at"`
	Timezone     string           `json:"timezone"`
	WorkingHours []WorkingDay     `json:"working_hours"`
	Namespaces   []tags.Namespace `json:"namespaces"`
	Tags         []Tag            `json:"tags"`
	Views        []View           `json:"views"`
	Tasks        []ExportTask     `json:"tasks"`
}

// ExportTask is a task with its time entries.
type ExportTask struct {
	Task
	Entries []ExportEntry `json:"entries"`
}

// ExportEntry is a time entry in an export.
type ExportEntry struct {
	StartedAt time.Time  `json:"started_at"`
	StoppedAt *time.Time `json:"stopped_at"`
	Note      string     `json:"note"`
}

// Export dumps all data of a user, trash included.
func (s *Service) Export(ctx context.Context, userID int64) (*Export, error) {
	me, err := s.Me(ctx, userID)
	if err != nil {
		return nil, err
	}
	ex := &Export{Version: ExportVersion, ExportedAt: s.now(), Timezone: me.Timezone}
	if ex.WorkingHours, err = s.WorkingHours(ctx, userID); err != nil {
		return nil, err
	}
	if ex.Namespaces, err = s.Namespaces(ctx, userID); err != nil {
		return nil, err
	}
	if ex.Tags, err = s.Tags(ctx, userID); err != nil {
		return nil, err
	}
	if ex.Views, err = s.Views(ctx, userID); err != nil {
		return nil, err
	}
	tasks, err := scanTasks(ctx, s.db, "SELECT "+taskColumns+" FROM tasks t WHERE t.user_id = ? ORDER BY t.id", userID)
	if err != nil {
		return nil, err
	}
	entries := map[int64][]ExportEntry{}
	rows, err := s.db.QueryContext(ctx, "SELECT task_id, started_at, stopped_at, note FROM time_entries WHERE user_id = ? ORDER BY started_at", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id      int64
			e       ExportEntry
			started string
			stopped sql.NullString
		)
		if err := rows.Scan(&id, &started, &stopped, &e.Note); err != nil {
			return nil, err
		}
		e.StartedAt, e.StoppedAt = parseTS(started), nullTime(stopped)
		entries[id] = append(entries[id], e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ex.Tasks = make([]ExportTask, 0, len(tasks))
	for _, t := range tasks {
		e := entries[t.ID]
		if e == nil {
			e = []ExportEntry{}
		}
		ex.Tasks = append(ex.Tasks, ExportTask{Task: t, Entries: e})
	}
	return ex, nil
}

// ImportResult summarises an import.
type ImportResult struct {
	Tasks      int `json:"tasks"`
	Entries    int `json:"entries"`
	Namespaces int `json:"namespaces"`
	Views      int `json:"views"`
}

// Import adds the content of an export to a user's data. Existing
// namespaces and views with the same name are kept; tasks are always added.
// Imported tasks keep their tags as-is (no namespace defaults are applied).
func (s *Service) Import(ctx context.Context, userID int64, ex *Export) (*ImportResult, error) {
	if ex.Version != ExportVersion {
		return nil, errf(CodeInvalid, "unsupported export version %d", ex.Version)
	}
	res := &ImportResult{}
	err := s.tx(ctx, func(tx *sql.Tx) error {
		existing, err := s.rules(ctx, tx, userID)
		if err != nil {
			return err
		}
		for _, ns := range ex.Namespaces {
			if _, ok := existing[ns.Name]; ok {
				continue
			}
			if err := ns.Validate(); err != nil {
				return errf(CodeInvalid, "namespace %q: %s", ns.Name, err.Error())
			}
			if _, err := insertNamespace(ctx, tx, userID, ns); err != nil {
				return err
			}
			res.Namespaces++
		}
		for _, t := range ex.Tags {
			name, err := tags.Normalize(t.Name)
			if err != nil {
				return errf(CodeInvalid, "%s", err.Error())
			}
			id, err := ensureTag(ctx, tx, userID, name)
			if err != nil {
				return err
			}
			if t.Color != nil && !validColor(*t.Color) {
				t.Color = nil
			}
			if _, err := tx.ExecContext(ctx, "UPDATE tags SET color = COALESCE(?, color), pinned = ?, position = ? WHERE id = ?",
				t.Color, t.Pinned, t.Position, id); err != nil {
				return err
			}
		}
		for _, v := range ex.Views {
			r, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO views (user_id, name, query, position, builtin) VALUES (?, ?, ?, ?, ?)`,
				userID, v.Name, v.Query, v.Position, v.Builtin)
			if err != nil {
				return err
			}
			if n, _ := r.RowsAffected(); n > 0 {
				res.Views++
			}
		}
		for _, t := range ex.Tasks {
			title, err := validateTitle(t.Title)
			if err != nil {
				return err
			}
			if err := validateDue(t.DueDate); err != nil {
				return err
			}
			set, err := tags.NormalizeAll(t.Tags)
			if err != nil {
				return errf(CodeInvalid, "%s", err.Error())
			}
			var id int64
			if err := tx.QueryRowContext(ctx, `INSERT INTO tasks (user_id, title, notes, due_date, remind_at, reminded_at, closed_at, deleted_at, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`, userID, title, t.Notes, t.DueDate, tsOrNil(t.RemindAt), tsOrNil(t.RemindedAt),
				tsOrNil(t.ClosedAt), tsOrNil(t.DeletedAt), ts(t.CreatedAt), ts(t.UpdatedAt)).Scan(&id); err != nil {
				return err
			}
			if err := setTaskTags(ctx, tx, userID, id, set); err != nil {
				return err
			}
			for _, e := range t.Entries {
				if e.StoppedAt != nil && !e.StoppedAt.After(e.StartedAt) {
					continue
				}
				if _, err := tx.ExecContext(ctx, "INSERT INTO time_entries (user_id, task_id, started_at, stopped_at, note) VALUES (?, ?, ?, ?, ?)",
					userID, id, ts(e.StartedAt), tsOrNil(e.StoppedAt), e.Note); err != nil {
					if uniqueViolation(err) {
						return errf(CodeInvalid, "task %q has several running timers", t.Title)
					}
					return err
				}
				res.Entries++
			}
			res.Tasks++
		}
		return s.recomputeClosed(ctx, tx, userID, nil)
	})
	if err != nil {
		return nil, err
	}
	s.publish(userID, "tasks", "tags", "timers", "views", "namespaces")
	return res, nil
}
