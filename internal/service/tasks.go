package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"kala/internal/dates"
	"kala/internal/filter"
	"kala/internal/quickadd"
	"kala/internal/tags"
)

// Task is a task with its tags and tracking summary.
type Task struct {
	ID         int64      `json:"id"`
	Title      string     `json:"title"`
	Notes      string     `json:"notes"`
	Tags       []string   `json:"tags"`
	DueDate    *string    `json:"due_date"`
	RemindAt   *time.Time `json:"remind_at"`
	RemindedAt *time.Time `json:"reminded_at"`
	ClosedAt   *time.Time `json:"closed_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	// RunningSince is the start of the task's running timer, if any.
	RunningSince *time.Time `json:"running_since"`
	// TrackedSeconds is the total of the task's stopped time entries.
	TrackedSeconds int64 `json:"tracked_seconds"`
}

const maxTitle = 500

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// ensureTag returns the id of a tag, creating it if needed.
func ensureTag(ctx context.Context, q querier, userID int64, name string) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `INSERT INTO tags (user_id, name) VALUES (?, ?)
		ON CONFLICT (user_id, name) DO UPDATE SET name = excluded.name RETURNING id`, userID, name).Scan(&id)
	return id, err
}

const terminalExists = `EXISTS (SELECT 1 FROM task_tags tt JOIN tags g ON g.id = tt.tag_id
	JOIN namespaces n ON n.user_id = g.user_id
	JOIN namespace_values nv ON nv.namespace_id = n.id AND nv.terminal = 1 AND g.name = n.name || ':' || nv.value
	WHERE tt.task_id = tasks.id)`

// recomputeClosed brings closed_at in line with the tasks' tags: a task is
// closed while it carries a terminal value. Newly closed tasks get their
// running timer stopped. taskIDs nil means every task of the user.
func (s *Service) recomputeClosed(ctx context.Context, q querier, userID int64, taskIDs []int64) error {
	scope := ""
	args := []any{userID}
	if taskIDs != nil {
		scope = " AND id IN (SELECT value FROM json_each(?))"
		args = append(args, idsJSON(taskIDs))
	}
	now := ts(s.now())
	if _, err := q.ExecContext(ctx, `UPDATE time_entries SET stopped_at = ? WHERE stopped_at IS NULL AND task_id IN (
		SELECT id FROM tasks WHERE user_id = ? AND closed_at IS NULL AND `+terminalExists+scope+`)`,
		append([]any{now}, args...)...); err != nil {
		return err
	}
	if _, err := q.ExecContext(ctx, `UPDATE tasks SET closed_at = ? WHERE user_id = ? AND closed_at IS NULL AND `+terminalExists+scope,
		append([]any{now}, args...)...); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx, `UPDATE tasks SET closed_at = NULL WHERE user_id = ? AND closed_at IS NOT NULL AND NOT `+terminalExists+scope, args...)
	return err
}

func taskTags(ctx context.Context, q querier, taskID int64) ([]string, error) {
	rows, err := q.QueryContext(ctx, "SELECT g.name FROM task_tags tt JOIN tags g ON g.id = tt.tag_id WHERE tt.task_id = ? ORDER BY g.name", taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// setTaskTags makes the task's tags exactly set.
func setTaskTags(ctx context.Context, q querier, userID, taskID int64, set []string) error {
	current, err := taskTags(ctx, q, taskID)
	if err != nil {
		return err
	}
	for _, t := range current {
		if !slices.Contains(set, t) {
			if _, err := q.ExecContext(ctx, `DELETE FROM task_tags WHERE task_id = ? AND tag_id = (SELECT id FROM tags WHERE user_id = ? AND name = ?)`,
				taskID, userID, t); err != nil {
				return err
			}
		}
	}
	for _, t := range set {
		if slices.Contains(current, t) {
			continue
		}
		id, err := ensureTag(ctx, q, userID, t)
		if err != nil {
			return err
		}
		if _, err := q.ExecContext(ctx, "INSERT OR IGNORE INTO task_tags (task_id, tag_id) VALUES (?, ?)", taskID, id); err != nil {
			return err
		}
	}
	return nil
}

// mutateTags adds and removes tags on a task, enforcing namespace rules.
func (s *Service) mutateTags(ctx context.Context, q querier, rules tags.Rules, userID, taskID int64, add, remove []string) error {
	current, err := taskTags(ctx, q, taskID)
	if err != nil {
		return err
	}
	set, err := rules.Apply(current, add, remove)
	if err != nil {
		return errf(CodeInvalid, "%s", err.Error())
	}
	return setTaskTags(ctx, q, userID, taskID, set)
}

// statusTag returns the tag that closes (terminal) or reopens (default) tasks.
func statusTag(rules tags.Rules, closed bool) (string, error) {
	ns, ok := rules[StatusNS]
	if !ok {
		return "", errf(CodeInvalid, "no %q namespace is defined", StatusNS)
	}
	if closed {
		v, ok := ns.FirstTerminal()
		if !ok {
			return "", errf(CodeInvalid, "the %q namespace has no terminal value", StatusNS)
		}
		return StatusNS + ":" + v, nil
	}
	if ns.Default != "" {
		return StatusNS + ":" + ns.Default, nil
	}
	for _, v := range ns.Values {
		if !v.Terminal {
			return StatusNS + ":" + v.Value, nil
		}
	}
	return "", errf(CodeInvalid, "the %q namespace has no open value", StatusNS)
}

const taskColumns = `t.id, t.title, t.notes, t.due_date, t.remind_at, t.reminded_at, t.closed_at, t.deleted_at, t.created_at, t.updated_at,
	(SELECT e.started_at FROM time_entries e WHERE e.task_id = t.id AND e.stopped_at IS NULL),
	(SELECT COALESCE(SUM(CAST(ROUND((julianday(e.stopped_at) - julianday(e.started_at)) * 86400) AS INTEGER)), 0)
		FROM time_entries e WHERE e.task_id = t.id AND e.stopped_at IS NOT NULL)`

func scanTasks(ctx context.Context, q querier, query string, args ...any) ([]Task, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Task{}
	var ids []int64
	for rows.Next() {
		var (
			t                                        Task
			due, remind, reminded, closed, del, runs sql.NullString
			created, updated                         string
		)
		if err := rows.Scan(&t.ID, &t.Title, &t.Notes, &due, &remind, &reminded, &closed, &del, &created, &updated,
			&runs, &t.TrackedSeconds); err != nil {
			return nil, err
		}
		t.DueDate = nullString(due)
		t.RemindAt, t.RemindedAt, t.ClosedAt, t.DeletedAt = nullTime(remind), nullTime(reminded), nullTime(closed), nullTime(del)
		t.RunningSince = nullTime(runs)
		t.CreatedAt, t.UpdatedAt = parseTS(created), parseTS(updated)
		t.Tags = []string{}
		out = append(out, t)
		ids = append(ids, t.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return out, nil
	}
	trows, err := q.QueryContext(ctx, `SELECT tt.task_id, g.name FROM task_tags tt JOIN tags g ON g.id = tt.tag_id
		WHERE tt.task_id IN (SELECT value FROM json_each(?)) ORDER BY g.name`, idsJSON(ids))
	if err != nil {
		return nil, err
	}
	defer trows.Close()
	index := make(map[int64]int, len(out))
	for i, t := range out {
		index[t.ID] = i
	}
	for trows.Next() {
		var (
			id   int64
			name string
		)
		if err := trows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[index[id]].Tags = append(out[index[id]].Tags, name)
	}
	return out, trows.Err()
}

func (s *Service) getTask(ctx context.Context, q querier, userID, id int64, withDeleted bool) (*Task, error) {
	cond := " AND t.deleted_at IS NULL"
	if withDeleted {
		cond = ""
	}
	list, err := scanTasks(ctx, q, "SELECT "+taskColumns+" FROM tasks t WHERE t.id = ? AND t.user_id = ?"+cond, id, userID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return &list[0], nil
}

// Task returns one task.
func (s *Service) Task(ctx context.Context, userID, id int64) (*Task, error) {
	return s.getTask(ctx, s.db, userID, id, false)
}

// ListOptions controls task listing.
type ListOptions struct {
	Query string
	Limit int
	// Deleted lists the trash instead of live tasks.
	Deleted bool
}

// MaxList is the maximum number of tasks returned by one listing.
const MaxList = 2000

// Tasks lists tasks matching a filter query.
func (s *Service) Tasks(ctx context.Context, userID int64, opts ListOptions) ([]Task, error) {
	env, err := s.Env(ctx, userID)
	if err != nil {
		return nil, err
	}
	fq, err := filter.Compile(opts.Query, env)
	if err != nil {
		return nil, errf(CodeInvalid, "filter: %s", err.Error())
	}
	if opts.Limit <= 0 || opts.Limit > MaxList {
		opts.Limit = MaxList
	}
	deleted, order := "t.deleted_at IS NULL", fq.Order
	if opts.Deleted {
		deleted, order = "t.deleted_at IS NOT NULL", "t.deleted_at DESC"
		fq.OrderArgs = nil
	}
	args := append([]any{userID}, fq.Args...)
	args = append(args, fq.OrderArgs...)
	args = append(args, opts.Limit)
	return scanTasks(ctx, s.db, "SELECT "+taskColumns+" FROM tasks t WHERE t.user_id = ? AND "+deleted+
		" AND ("+fq.Where+") ORDER BY "+order+" LIMIT ?", args...)
}

// TaskInput creates a task. When Quick is set it is parsed with the
// quick-add syntax and merged with the other fields.
type TaskInput struct {
	Quick    string     `json:"quick,omitempty"`
	Title    string     `json:"title,omitempty"`
	Notes    string     `json:"notes,omitempty"`
	Tags     []string   `json:"tags,omitempty"`
	DueDate  *string    `json:"due_date,omitempty"`
	RemindAt *time.Time `json:"remind_at,omitempty"`
}

// ParseQuick parses a quick-add line for a user.
func (s *Service) ParseQuick(ctx context.Context, userID int64, line string) (*quickadd.Result, error) {
	cal, err := s.calendar(ctx, s.db, userID)
	if err != nil {
		return nil, err
	}
	res, err := quickadd.Parse(line, s.now(), cal)
	if err != nil {
		return nil, errf(CodeInvalid, "%s", err.Error())
	}
	return &res, nil
}

func validateDue(d *string) error {
	if d == nil {
		return nil
	}
	if _, err := dates.Parse(*d); err != nil {
		return errf(CodeInvalid, "%s", err.Error())
	}
	return nil
}

func validateTitle(t string) (string, error) {
	t = strings.TrimSpace(t)
	if t == "" {
		return "", errf(CodeInvalid, "title is required")
	}
	if len(t) > maxTitle {
		return "", errf(CodeInvalid, "title is too long")
	}
	return t, nil
}

func (s *Service) resolveInput(ctx context.Context, userID int64, in TaskInput) (TaskInput, error) {
	if in.Quick != "" {
		res, err := s.ParseQuick(ctx, userID, in.Quick)
		if err != nil {
			return in, err
		}
		if in.Title == "" {
			in.Title = res.Title
		}
		in.Tags = append(in.Tags, res.Tags...)
		if in.DueDate == nil && res.DueDate != "" {
			in.DueDate = &res.DueDate
		}
		if in.RemindAt == nil {
			in.RemindAt = res.RemindAt
		}
	}
	var err error
	if in.Title, err = validateTitle(in.Title); err != nil {
		return in, err
	}
	if in.Tags, err = tags.NormalizeAll(in.Tags); err != nil {
		return in, errf(CodeInvalid, "%s", err.Error())
	}
	return in, validateDue(in.DueDate)
}

func (s *Service) createTask(ctx context.Context, tx *sql.Tx, userID int64, in TaskInput) (int64, error) {
	rules, err := s.rules(ctx, tx, userID)
	if err != nil {
		return 0, err
	}
	set, err := rules.Apply(nil, in.Tags, nil)
	if err != nil {
		return 0, errf(CodeInvalid, "%s", err.Error())
	}
	now := ts(s.now())
	var id int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO tasks (user_id, title, notes, due_date, remind_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`, userID, in.Title, in.Notes, in.DueDate, tsOrNil(in.RemindAt), now, now).Scan(&id); err != nil {
		return 0, err
	}
	if err := setTaskTags(ctx, tx, userID, id, set); err != nil {
		return 0, err
	}
	return id, s.recomputeClosed(ctx, tx, userID, []int64{id})
}

// CreateTask creates a task.
func (s *Service) CreateTask(ctx context.Context, userID int64, in TaskInput) (*Task, error) {
	in, err := s.resolveInput(ctx, userID, in)
	if err != nil {
		return nil, err
	}
	var id int64
	if err := s.tx(ctx, func(tx *sql.Tx) error {
		id, err = s.createTask(ctx, tx, userID, in)
		return err
	}); err != nil {
		return nil, err
	}
	s.publish(userID, "tasks", "tags")
	return s.Task(ctx, userID, id)
}

// TaskPatch is a partial task update. Tags replaces the whole tag set;
// AddTags/RemoveTags edit it incrementally.
type TaskPatch struct {
	Title      Optional[string]    `json:"title"`
	Notes      Optional[string]    `json:"notes"`
	DueDate    Optional[string]    `json:"due_date"`
	RemindAt   Optional[time.Time] `json:"remind_at"`
	Tags       Optional[[]string]  `json:"tags"`
	AddTags    []string            `json:"add_tags"`
	RemoveTags []string            `json:"remove_tags"`
	// Closed closes (true) or reopens (false) the task via the status
	// namespace.
	Closed Optional[bool] `json:"closed"`
}

// UpdateTask applies a patch.
func (s *Service) UpdateTask(ctx context.Context, userID, id int64, p TaskPatch) (*Task, error) {
	err := s.tx(ctx, func(tx *sql.Tx) error { return s.updateTask(ctx, tx, userID, id, p) })
	if err != nil {
		return nil, err
	}
	s.publish(userID, "tasks", "tags", "timers")
	return s.Task(ctx, userID, id)
}

func (s *Service) updateTask(ctx context.Context, tx *sql.Tx, userID, id int64, p TaskPatch) error {
	if _, err := s.getTask(ctx, tx, userID, id, false); err != nil {
		return err
	}
	var (
		sets []string
		args []any
	)
	if p.Title.Set {
		t, err := validateTitle(p.Title.Value)
		if err != nil {
			return err
		}
		sets, args = append(sets, "title = ?"), append(args, t)
	}
	if p.Notes.Set {
		sets, args = append(sets, "notes = ?"), append(args, p.Notes.Value)
	}
	if p.DueDate.Set {
		if p.DueDate.Null {
			sets, args = append(sets, "due_date = ?"), append(args, nil)
		} else {
			if err := validateDue(&p.DueDate.Value); err != nil {
				return err
			}
			sets, args = append(sets, "due_date = ?"), append(args, p.DueDate.Value)
		}
	}
	if p.RemindAt.Set {
		var v any
		if !p.RemindAt.Null {
			v = ts(p.RemindAt.Value)
		}
		sets, args = append(sets, "remind_at = ?", "reminded_at = NULL"), append(args, v)
	}
	sets, args = append(sets, "updated_at = ?"), append(args, ts(s.now()))
	// sets only contains constant column assignments; values are bound.
	if _, err := tx.ExecContext(ctx, "UPDATE tasks SET "+strings.Join(sets, ", ")+" WHERE id = ?", append(args, id)...); err != nil { //nolint:gosec // see above
		return err
	}
	if !p.Tags.Set && len(p.AddTags) == 0 && len(p.RemoveTags) == 0 && !p.Closed.Set {
		return nil
	}
	rules, err := s.rules(ctx, tx, userID)
	if err != nil {
		return err
	}
	if p.Tags.Set {
		set, err := tags.NormalizeAll(p.Tags.Value)
		if err != nil {
			return errf(CodeInvalid, "%s", err.Error())
		}
		if set, err = rules.Apply(nil, set, nil); err != nil {
			return errf(CodeInvalid, "%s", err.Error())
		}
		if err := setTaskTags(ctx, tx, userID, id, set); err != nil {
			return err
		}
	}
	add, err := tags.NormalizeAll(p.AddTags)
	if err != nil {
		return errf(CodeInvalid, "%s", err.Error())
	}
	remove, err := tags.NormalizeAll(p.RemoveTags)
	if err != nil {
		return errf(CodeInvalid, "%s", err.Error())
	}
	if p.Closed.Set {
		t, err := statusTag(rules, p.Closed.Value)
		if err != nil {
			return err
		}
		add = append(add, t)
	}
	if len(add) > 0 || len(remove) > 0 {
		if err := s.mutateTags(ctx, tx, rules, userID, id, add, remove); err != nil {
			return err
		}
	}
	return s.recomputeClosed(ctx, tx, userID, []int64{id})
}

// DeleteTask moves a task to the trash, stopping its timer.
func (s *Service) DeleteTask(ctx context.Context, userID, id int64) error {
	err := s.tx(ctx, func(tx *sql.Tx) error { return s.deleteTask(ctx, tx, userID, id) })
	if err == nil {
		s.publish(userID, "tasks", "tags", "timers")
	}
	return err
}

func (s *Service) deleteTask(ctx context.Context, tx *sql.Tx, userID, id int64) error {
	now := ts(s.now())
	res, err := tx.ExecContext(ctx, "UPDATE tasks SET deleted_at = ?, updated_at = ? WHERE id = ? AND user_id = ? AND deleted_at IS NULL", now, now, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	_, err = tx.ExecContext(ctx, "UPDATE time_entries SET stopped_at = ? WHERE task_id = ? AND stopped_at IS NULL", now, id)
	return err
}

// RestoreTask takes a task out of the trash.
func (s *Service) RestoreTask(ctx context.Context, userID, id int64) (*Task, error) {
	err := s.tx(ctx, func(tx *sql.Tx) error { return s.restoreTask(ctx, tx, userID, id) })
	if err != nil {
		return nil, err
	}
	s.publish(userID, "tasks", "tags")
	return s.Task(ctx, userID, id)
}

func (s *Service) restoreTask(ctx context.Context, tx *sql.Tx, userID, id int64) error {
	res, err := tx.ExecContext(ctx, "UPDATE tasks SET deleted_at = NULL, updated_at = ? WHERE id = ? AND user_id = ? AND deleted_at IS NOT NULL",
		ts(s.now()), id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// PurgeTask permanently deletes a task from the trash.
func (s *Service) PurgeTask(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM tasks WHERE id = ? AND user_id = ? AND deleted_at IS NOT NULL", id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	s.publish(userID, "tasks", "tags")
	return nil
}

// TrashRetention is how long deleted tasks are kept.
const TrashRetention = 30 * 24 * time.Hour

// PurgeExpired deletes trashed tasks past retention and stale refresh tokens
// for every user.
func (s *Service) PurgeExpired(ctx context.Context) error {
	now := s.now()
	if _, err := s.db.ExecContext(ctx, "DELETE FROM tasks WHERE deleted_at IS NOT NULL AND deleted_at < ?", ts(now.Add(-TrashRetention))); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, "DELETE FROM refresh_tokens WHERE expires_at < ? OR revoked_at < ?", ts(now), ts(now.Add(-7*24*time.Hour)))
	return err
}

// Bulk operations.
const (
	BulkAddTags    = "add_tags"
	BulkRemoveTags = "remove_tags"
	BulkSetDue     = "set_due"
	BulkClose      = "close"
	BulkReopen     = "reopen"
	BulkDelete     = "delete"
	BulkRestore    = "restore"
)

// BulkInput applies one operation to several tasks atomically.
type BulkInput struct {
	IDs     []int64          `json:"ids"`
	Op      string           `json:"op"`
	Tags    []string         `json:"tags,omitempty"`
	DueDate Optional[string] `json:"due_date"`
}

// Bulk applies an operation to many tasks in one transaction.
func (s *Service) Bulk(ctx context.Context, userID int64, in BulkInput) error {
	if len(in.IDs) == 0 {
		return errf(CodeInvalid, "no task ids")
	}
	if len(in.IDs) > MaxList {
		return errf(CodeInvalid, "too many tasks")
	}
	err := s.tx(ctx, func(tx *sql.Tx) error {
		for _, id := range in.IDs {
			var err error
			switch in.Op {
			case BulkAddTags:
				err = s.updateTask(ctx, tx, userID, id, TaskPatch{AddTags: in.Tags})
			case BulkRemoveTags:
				err = s.updateTask(ctx, tx, userID, id, TaskPatch{RemoveTags: in.Tags})
			case BulkSetDue:
				if !in.DueDate.Set {
					return errf(CodeInvalid, "due_date is required (null clears it)")
				}
				err = s.updateTask(ctx, tx, userID, id, TaskPatch{DueDate: in.DueDate})
			case BulkClose:
				err = s.updateTask(ctx, tx, userID, id, TaskPatch{Closed: Some(true)})
			case BulkReopen:
				err = s.updateTask(ctx, tx, userID, id, TaskPatch{Closed: Some(false)})
			case BulkDelete:
				err = s.deleteTask(ctx, tx, userID, id)
			case BulkRestore:
				err = s.restoreTask(ctx, tx, userID, id)
			default:
				return errf(CodeInvalid, "unknown bulk op %q", in.Op)
			}
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					return errf(CodeNotFound, "task %d not found", id)
				}
				var de *Error
				if errors.As(err, &de) {
					return &Error{Code: de.Code, Message: fmt.Sprintf("task %d: %s", id, de.Message)}
				}
				return err
			}
		}
		return nil
	})
	if err == nil {
		s.publish(userID, "tasks", "tags", "timers")
	}
	return err
}
