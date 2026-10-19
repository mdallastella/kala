package service

import (
	"cmp"
	"context"
	"database/sql"
	"slices"
	"strconv"
	"strings"
	"time"

	"kala/internal/dates"
	"kala/internal/filter"
	"kala/internal/tags"
)

// TimeEntry is a tracked time interval of a task.
type TimeEntry struct {
	ID        int64      `json:"id"`
	TaskID    int64      `json:"task_id"`
	TaskTitle string     `json:"task_title"`
	TaskTags  []string   `json:"task_tags"`
	StartedAt time.Time  `json:"started_at"`
	StoppedAt *time.Time `json:"stopped_at"`
	Note      string     `json:"note"`
	// Seconds is the duration so far (up to now for running entries).
	Seconds int64 `json:"seconds"`
}

func (s *Service) startTimer(ctx context.Context, tx *sql.Tx, userID, taskID int64) error {
	task, err := s.getTask(ctx, tx, userID, taskID, false)
	if err != nil {
		return err
	}
	if task.RunningSince != nil {
		return errf(CodeConflict, "a timer is already running for this task")
	}
	if task.ClosedAt != nil {
		rules, err := s.rules(ctx, tx, userID)
		if err != nil {
			return err
		}
		open, err := statusTag(rules, false)
		if err != nil {
			return err
		}
		if err := s.mutateTags(ctx, tx, rules, userID, taskID, []string{open}, nil); err != nil {
			return err
		}
		if err := s.recomputeClosed(ctx, tx, userID, []int64{taskID}); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO time_entries (user_id, task_id, started_at) VALUES (?, ?, ?)", userID, taskID, ts(s.now()))
	return err
}

// StartTimer starts a timer on a task, reopening it if it is closed.
func (s *Service) StartTimer(ctx context.Context, userID, taskID int64) (*Task, error) {
	if err := s.tx(ctx, func(tx *sql.Tx) error { return s.startTimer(ctx, tx, userID, taskID) }); err != nil {
		return nil, err
	}
	s.publish(userID, "timers", "tasks")
	return s.Task(ctx, userID, taskID)
}

// StartTimerNewTask creates a task and starts its timer.
func (s *Service) StartTimerNewTask(ctx context.Context, userID int64, in TaskInput) (*Task, error) {
	in, err := s.resolveInput(ctx, userID, in)
	if err != nil {
		return nil, err
	}
	var id int64
	err = s.tx(ctx, func(tx *sql.Tx) error {
		if id, err = s.createTask(ctx, tx, userID, in); err != nil {
			return err
		}
		return s.startTimer(ctx, tx, userID, id)
	})
	if err != nil {
		return nil, err
	}
	s.publish(userID, "timers", "tasks", "tags")
	return s.Task(ctx, userID, id)
}

// StopTimer stops the running timer of a task.
func (s *Service) StopTimer(ctx context.Context, userID, taskID int64) (*Task, error) {
	res, err := s.db.ExecContext(ctx, "UPDATE time_entries SET stopped_at = ? WHERE task_id = ? AND user_id = ? AND stopped_at IS NULL",
		ts(s.now()), taskID, userID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := s.Task(ctx, userID, taskID); err != nil {
			return nil, err
		}
		return nil, errf(CodeConflict, "no timer is running for this task")
	}
	s.publish(userID, "timers", "tasks")
	return s.Task(ctx, userID, taskID)
}

// StopAllTimers stops every running timer of a user and returns how many.
func (s *Service) StopAllTimers(ctx context.Context, userID int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, "UPDATE time_entries SET stopped_at = ? WHERE user_id = ? AND stopped_at IS NULL", ts(s.now()), userID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		s.publish(userID, "timers", "tasks")
	}
	return n, nil
}

const entryColumns = `e.id, e.task_id, t.title, e.started_at, e.stopped_at, e.note`

func (s *Service) scanEntries(ctx context.Context, q querier, query string, args ...any) ([]TimeEntry, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := s.now()
	out := []TimeEntry{}
	var taskIDs []int64
	for rows.Next() {
		var (
			e       TimeEntry
			started string
			stopped sql.NullString
		)
		if err := rows.Scan(&e.ID, &e.TaskID, &e.TaskTitle, &started, &stopped, &e.Note); err != nil {
			return nil, err
		}
		e.StartedAt, e.StoppedAt = parseTS(started), nullTime(stopped)
		end := now
		if e.StoppedAt != nil {
			end = *e.StoppedAt
		}
		e.Seconds = int64(end.Sub(e.StartedAt).Seconds())
		e.TaskTags = []string{}
		out = append(out, e)
		taskIDs = append(taskIDs, e.TaskID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	trows, err := q.QueryContext(ctx, `SELECT tt.task_id, g.name FROM task_tags tt JOIN tags g ON g.id = tt.tag_id
		WHERE tt.task_id IN (SELECT value FROM json_each(?)) ORDER BY g.name`, idsJSON(taskIDs))
	if err != nil {
		return nil, err
	}
	defer trows.Close()
	byTask := map[int64][]string{}
	for trows.Next() {
		var (
			id   int64
			name string
		)
		if err := trows.Scan(&id, &name); err != nil {
			return nil, err
		}
		byTask[id] = append(byTask[id], name)
	}
	for i := range out {
		if t := byTask[out[i].TaskID]; t != nil {
			out[i].TaskTags = t
		}
	}
	return out, trows.Err()
}

// ActiveTimers lists the running time entries of a user.
func (s *Service) ActiveTimers(ctx context.Context, userID int64) ([]TimeEntry, error) {
	return s.scanEntries(ctx, s.db, "SELECT "+entryColumns+` FROM time_entries e JOIN tasks t ON t.id = e.task_id
		WHERE e.user_id = ? AND e.stopped_at IS NULL AND t.deleted_at IS NULL ORDER BY e.started_at`, userID)
}

// EntryFilter selects time entries overlapping [From, To).
type EntryFilter struct {
	From, To time.Time
	TaskID   int64
	// Query filters the entries' tasks with the filter language.
	Query string
}

func (s *Service) entryQuery(ctx context.Context, userID int64, f EntryFilter) (string, []any, error) {
	cond := `e.user_id = ? AND t.deleted_at IS NULL AND e.started_at < ? AND (e.stopped_at IS NULL OR e.stopped_at > ?)`
	args := []any{userID, ts(f.To), ts(f.From)}
	if f.TaskID != 0 {
		cond += " AND e.task_id = ?"
		args = append(args, f.TaskID)
	}
	if strings.TrimSpace(f.Query) != "" {
		env, err := s.Env(ctx, userID)
		if err != nil {
			return "", nil, err
		}
		fq, err := filter.Compile(f.Query, env)
		if err != nil {
			return "", nil, errf(CodeInvalid, "filter: %s", err.Error())
		}
		cond += " AND (" + fq.Where + ")"
		args = append(args, fq.Args...)
	}
	return cond, args, nil
}

// TimeEntries lists the entries overlapping a range, newest first.
func (s *Service) TimeEntries(ctx context.Context, userID int64, f EntryFilter) ([]TimeEntry, error) {
	if !f.From.Before(f.To) {
		return nil, errf(CodeInvalid, "from must be before to")
	}
	cond, args, err := s.entryQuery(ctx, userID, f)
	if err != nil {
		return nil, err
	}
	return s.scanEntries(ctx, s.db, "SELECT "+entryColumns+" FROM time_entries e JOIN tasks t ON t.id = e.task_id WHERE "+cond+
		" ORDER BY e.started_at DESC LIMIT 5000", args...)
}

// EntryInput creates or updates a time entry.
type EntryInput struct {
	TaskID    int64               `json:"task_id"`
	StartedAt Optional[time.Time] `json:"started_at"`
	StoppedAt Optional[time.Time] `json:"stopped_at"`
	Note      Optional[string]    `json:"note"`
}

func (s *Service) entry(ctx context.Context, userID, id int64) (*TimeEntry, error) {
	list, err := s.scanEntries(ctx, s.db, "SELECT "+entryColumns+" FROM time_entries e JOIN tasks t ON t.id = e.task_id WHERE e.id = ? AND e.user_id = ?", id, userID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return &list[0], nil
}

// CreateTimeEntry adds a manual, completed time entry.
func (s *Service) CreateTimeEntry(ctx context.Context, userID int64, in EntryInput) (*TimeEntry, error) {
	if !in.StartedAt.Set || in.StartedAt.Null || !in.StoppedAt.Set || in.StoppedAt.Null {
		return nil, errf(CodeInvalid, "started_at and stopped_at are required")
	}
	if !in.StoppedAt.Value.After(in.StartedAt.Value) {
		return nil, errf(CodeInvalid, "stopped_at must be after started_at")
	}
	if _, err := s.Task(ctx, userID, in.TaskID); err != nil {
		return nil, err
	}
	var id int64
	if err := s.db.QueryRowContext(ctx, "INSERT INTO time_entries (user_id, task_id, started_at, stopped_at, note) VALUES (?, ?, ?, ?, ?) RETURNING id",
		userID, in.TaskID, ts(in.StartedAt.Value), ts(in.StoppedAt.Value), in.Note.Value).Scan(&id); err != nil {
		return nil, err
	}
	s.publish(userID, "timers", "tasks")
	return s.entry(ctx, userID, id)
}

// UpdateTimeEntry edits an entry's bounds or note. A running entry keeps
// running unless stopped_at is given.
func (s *Service) UpdateTimeEntry(ctx context.Context, userID, id int64, in EntryInput) (*TimeEntry, error) {
	cur, err := s.entry(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	start, stop, note := cur.StartedAt, cur.StoppedAt, cur.Note
	if in.StartedAt.Set && !in.StartedAt.Null {
		start = in.StartedAt.Value
	}
	if in.StoppedAt.Set {
		if in.StoppedAt.Null {
			return nil, errf(CodeInvalid, "stopped_at cannot be cleared")
		}
		stop = &in.StoppedAt.Value
	}
	if in.Note.Set {
		note = in.Note.Value
	}
	end := s.now()
	if stop != nil {
		end = *stop
	}
	if !end.After(start) {
		return nil, errf(CodeInvalid, "the entry must end after it starts")
	}
	if _, err := s.db.ExecContext(ctx, "UPDATE time_entries SET started_at = ?, stopped_at = ?, note = ? WHERE id = ?",
		ts(start), tsOrNil(stop), note, id); err != nil {
		return nil, err
	}
	s.publish(userID, "timers", "tasks")
	return s.entry(ctx, userID, id)
}

// DeleteTimeEntry permanently deletes an entry.
func (s *Service) DeleteTimeEntry(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM time_entries WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	s.publish(userID, "timers", "tasks")
	return nil
}

// ReportRow is one group of a time report.
type ReportRow struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Seconds int64  `json:"seconds"`
}

// Report totals tracked time per group.
type Report struct {
	Group string      `json:"group"`
	From  time.Time   `json:"from"`
	To    time.Time   `json:"to"`
	Total int64       `json:"total_seconds"`
	Rows  []ReportRow `json:"rows"`
}

// NoValue labels report time of tasks without a tag in the namespace.
const NoValue = "(none)"

// TimeReport totals the time tracked in [from, to) grouped by "day", "task",
// "tag" or a namespace name. Time is clipped to the range. With tag and
// namespace grouping, a task with several matching tags counts in each.
func (s *Service) TimeReport(ctx context.Context, userID int64, f EntryFilter, group string) (*Report, error) {
	if group == "" {
		group = "day"
	}
	entries, err := s.TimeEntries(ctx, userID, f)
	if err != nil {
		return nil, err
	}
	cal, err := s.calendar(ctx, s.db, userID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	rep := &Report{Group: group, From: f.From, To: f.To, Rows: []ReportRow{}}
	totals := map[string]*ReportRow{}
	add := func(key, label string, secs int64) {
		r, ok := totals[key]
		if !ok {
			r = &ReportRow{Key: key, Label: label}
			totals[key] = r
		}
		r.Seconds += secs
	}
	for _, e := range entries {
		start := e.StartedAt
		if start.Before(f.From) {
			start = f.From
		}
		end := now
		if e.StoppedAt != nil {
			end = *e.StoppedAt
		}
		if end.After(f.To) {
			end = f.To
		}
		secs := int64(end.Sub(start).Seconds())
		if secs <= 0 {
			continue
		}
		rep.Total += secs
		switch group {
		case "day":
			d := dates.On(start, cal.Loc).String()
			add(d, d, secs)
		case "task":
			add(strconv.FormatInt(e.TaskID, 10), e.TaskTitle, secs)
		case "tag":
			if len(e.TaskTags) == 0 {
				add(NoValue, NoValue, secs)
			}
			for _, t := range e.TaskTags {
				add(t, t, secs)
			}
		default:
			found := false
			for _, t := range e.TaskTags {
				if ns, _ := tags.Split(t); ns == group && strings.Contains(t, ":") {
					add(t, t, secs)
					found = true
				}
			}
			if !found {
				add(NoValue, NoValue, secs)
			}
		}
	}
	for _, r := range totals {
		rep.Rows = append(rep.Rows, *r)
	}
	slices.SortFunc(rep.Rows, func(a, b ReportRow) int {
		if group == "day" {
			return cmp.Compare(a.Key, b.Key)
		}
		return cmp.Or(cmp.Compare(b.Seconds, a.Seconds), cmp.Compare(a.Key, b.Key))
	})
	return rep, nil
}
