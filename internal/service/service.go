// Package service implements Kala's business logic on top of SQLite. Both the
// REST API and the MCP server call it, so every rule (namespace enforcement,
// timer/status coupling, change events) lives here.
package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"kala/internal/dates"
	"kala/internal/events"
	"kala/internal/filter"
	"kala/internal/tags"
)

// Service is the application core.
type Service struct {
	db  *sql.DB
	hub *events.Hub
	now func() time.Time
}

// New returns a service. now may be nil to use the wall clock.
func New(db *sql.DB, hub *events.Hub, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{db: db, hub: hub, now: now}
}

// Now returns the service clock's current time.
func (s *Service) Now() time.Time { return s.now() }

// Error codes.
const (
	CodeInvalid  = "invalid"
	CodeNotFound = "not_found"
	CodeConflict = "conflict"
	CodeAuth     = "unauthorized"
	CodeDenied   = "forbidden"
)

// Error is a domain error with a machine-readable code.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func (e *Error) Error() string { return e.Message }

func errf(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

func uniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// ErrNotFound is returned for missing (or foreign) resources.
var ErrNotFound = &Error{Code: CodeNotFound, Message: "not found"}

// IsCode reports whether err is a domain error with code.
func IsCode(err error, code string) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == code
}

// Optional distinguishes an absent JSON field from an explicit null.
type Optional[T any] struct {
	Set   bool
	Null  bool
	Value T
}

// UnmarshalJSON implements json.Unmarshaler.
func (o *Optional[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if string(b) == "null" {
		o.Null = true
		return nil
	}
	return json.Unmarshal(b, &o.Value)
}

// Some returns a set Optional.
func Some[T any](v T) Optional[T] { return Optional[T]{Set: true, Value: v} }

type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (s *Service) tx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Service) publish(userID int64, kinds ...string) {
	if s.hub != nil {
		s.hub.Publish(userID, events.Changed(kinds...))
	}
}

var ts = filter.Timestamp

func parseTS(s string) time.Time {
	t, _ := time.Parse("2006-01-02T15:04:05.000Z", s)
	return t
}

func nullTime(ns sql.NullString) *time.Time {
	if !ns.Valid {
		return nil
	}
	t := parseTS(ns.String)
	return &t
}

func nullString(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

func tsOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return ts(*t)
}

func idsJSON(ids []int64) string {
	b, _ := json.Marshal(ids)
	return string(b)
}

// calendar loads a user's timezone and working hours.
func (s *Service) calendar(ctx context.Context, q querier, userID int64) (dates.Calendar, error) {
	var tz string
	if err := q.QueryRowContext(ctx, "SELECT timezone FROM users WHERE id = ?", userID).Scan(&tz); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dates.Calendar{}, ErrNotFound
		}
		return dates.Calendar{}, err
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	cal := dates.Calendar{Loc: loc, Days: map[time.Weekday]dates.Hours{}}
	rows, err := q.QueryContext(ctx, "SELECT weekday, start_min, end_min FROM working_hours WHERE user_id = ?", userID)
	if err != nil {
		return cal, err
	}
	defer rows.Close()
	for rows.Next() {
		var wd int
		var h dates.Hours
		if err := rows.Scan(&wd, &h.Start, &h.End); err != nil {
			return cal, err
		}
		cal.Days[time.Weekday(wd)] = h
	}
	return cal, rows.Err()
}

// Env returns the filter environment of a user.
func (s *Service) Env(ctx context.Context, userID int64) (filter.Env, error) {
	cal, err := s.calendar(ctx, s.db, userID)
	return filter.Env{Now: s.now(), Cal: cal}, err
}

// rules loads the namespace definitions of a user.
func (s *Service) rules(ctx context.Context, q querier, userID int64) (tags.Rules, error) {
	nss, err := s.listNamespaces(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	r := tags.Rules{}
	for i := range nss {
		r[nss[i].Name] = &nss[i]
	}
	return r, nil
}
