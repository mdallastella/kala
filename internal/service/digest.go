package service

import (
	"context"
	"database/sql"
	"time"

	"kala/internal/dates"
)

// Digest kinds.
const (
	DigestBriefing = "briefing"
	DigestWrapup   = "wrapup"
)

// Briefing is the start-of-day summary.
type Briefing struct {
	Date      string `json:"date"`
	Overdue   []Task `json:"overdue"`
	DueToday  []Task `json:"due_today"`
	Reminders []Task `json:"reminders"`
}

// Wrapup is the end-of-day summary.
type Wrapup struct {
	Date string `json:"date"`
	// Open lists open tasks due today or earlier.
	Open []Task `json:"open"`
	// Next lists open tasks due on the next working day.
	Next         []Task      `json:"next"`
	NextWorkDate string      `json:"next_work_date"`
	Running      []TimeEntry `json:"running"`
}

func (s *Service) list(ctx context.Context, userID int64, q string) ([]Task, error) {
	return s.Tasks(ctx, userID, ListOptions{Query: q, Limit: 200})
}

// Briefing builds today's briefing.
func (s *Service) Briefing(ctx context.Context, userID int64) (*Briefing, error) {
	cal, err := s.calendar(ctx, s.db, userID)
	if err != nil {
		return nil, err
	}
	b := &Briefing{Date: dates.On(s.now(), cal.Loc).String()}
	if b.Overdue, err = s.list(ctx, userID, "is:open due:overdue sort:due,priority"); err != nil {
		return nil, err
	}
	if b.DueToday, err = s.list(ctx, userID, "is:open due:today"); err != nil {
		return nil, err
	}
	if b.Reminders, err = s.list(ctx, userID, "is:open remind:today sort:remind"); err != nil {
		return nil, err
	}
	return b, nil
}

// Wrapup builds today's wrap-up.
func (s *Service) Wrapup(ctx context.Context, userID int64) (*Wrapup, error) {
	cal, err := s.calendar(ctx, s.db, userID)
	if err != nil {
		return nil, err
	}
	today := dates.On(s.now(), cal.Loc)
	w := &Wrapup{Date: today.String(), NextWorkDate: cal.NextWorkingDay(today).String()}
	if w.Open, err = s.list(ctx, userID, "is:open due:<=today"); err != nil {
		return nil, err
	}
	if w.Next, err = s.list(ctx, userID, "is:open due:nwd"); err != nil {
		return nil, err
	}
	if w.Running, err = s.ActiveTimers(ctx, userID); err != nil {
		return nil, err
	}
	return w, nil
}

// DigestState is what the scheduler needs to decide on digests.
type DigestState struct {
	Cal          dates.Calendar
	LastBriefing string
	LastWrapup   string
}

// DigestState loads a user's digest bookkeeping.
func (s *Service) DigestState(ctx context.Context, userID int64) (*DigestState, error) {
	cal, err := s.calendar(ctx, s.db, userID)
	if err != nil {
		return nil, err
	}
	st := &DigestState{Cal: cal}
	var b, w sql.NullString
	if err := s.db.QueryRowContext(ctx, "SELECT last_briefing_on, last_wrapup_on FROM users WHERE id = ?", userID).Scan(&b, &w); err != nil {
		return nil, err
	}
	st.LastBriefing, st.LastWrapup = b.String, w.String
	return st, nil
}

// MarkDigestSeen records that today's digest of kind was shown, so it is not
// pushed again on any device.
func (s *Service) MarkDigestSeen(ctx context.Context, userID int64, kind string) error {
	col := map[string]string{DigestBriefing: "last_briefing_on", DigestWrapup: "last_wrapup_on"}[kind]
	if col == "" {
		return errf(CodeInvalid, "unknown digest %q", kind)
	}
	cal, err := s.calendar(ctx, s.db, userID)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "UPDATE users SET "+col+" = ? WHERE id = ?", dates.On(s.now(), cal.Loc).String(), userID)
	return err
}

// DueReminders lists open tasks whose reminder time has come and that were
// not acknowledged.
func (s *Service) DueReminders(ctx context.Context, userID int64) ([]Task, error) {
	return s.list(ctx, userID, "is:open remind:due sort:remind")
}

// AckReminder acknowledges a reminder, or snoozes it until a later time.
func (s *Service) AckReminder(ctx context.Context, userID, taskID int64, snoozeUntil *time.Time) (*Task, error) {
	var (
		res sql.Result
		err error
	)
	if snoozeUntil != nil {
		if !snoozeUntil.After(s.now()) {
			return nil, errf(CodeInvalid, "snooze_until must be in the future")
		}
		res, err = s.db.ExecContext(ctx, "UPDATE tasks SET remind_at = ?, reminded_at = NULL WHERE id = ? AND user_id = ? AND deleted_at IS NULL",
			ts(*snoozeUntil), taskID, userID)
	} else {
		res, err = s.db.ExecContext(ctx, "UPDATE tasks SET reminded_at = ? WHERE id = ? AND user_id = ? AND deleted_at IS NULL",
			ts(s.now()), taskID, userID)
	}
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	s.publish(userID, "tasks")
	return s.Task(ctx, userID, taskID)
}
