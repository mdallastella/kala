// Package dates handles civil dates, working-hours calendars and the relative
// date expressions shared by quick-add and the filter language.
package dates

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Layout is the storage format of civil dates.
const Layout = "2006-01-02"

// Date is a civil date, represented as midnight UTC.
type Date struct{ t time.Time }

// NewDate builds a Date from its components.
func NewDate(y int, m time.Month, d int) Date {
	return Date{time.Date(y, m, d, 0, 0, 0, 0, time.UTC)}
}

// On returns the civil date of t in loc.
func On(t time.Time, loc *time.Location) Date {
	y, m, d := t.In(loc).Date()
	return NewDate(y, m, d)
}

// Parse parses a YYYY-MM-DD date.
func Parse(s string) (Date, error) {
	t, err := time.Parse(Layout, s)
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q, want YYYY-MM-DD", s)
	}
	return Date{t}, nil
}

// String formats the date as YYYY-MM-DD.
func (d Date) String() string { return d.t.Format(Layout) }

// AddDays returns the date n days later.
func (d Date) AddDays(n int) Date { return Date{d.t.AddDate(0, 0, n)} }

// Weekday returns the day of the week.
func (d Date) Weekday() time.Weekday { return d.t.Weekday() }

// Before reports whether d is before o.
func (d Date) Before(o Date) bool { return d.t.Before(o.t) }

// IsZero reports whether d is the zero date.
func (d Date) IsZero() bool { return d.t.IsZero() }

// At returns the instant at minute-of-day min on d in loc.
func (d Date) At(minute int, loc *time.Location) time.Time {
	y, m, dd := d.t.Date()
	return time.Date(y, m, dd, minute/60, minute%60, 0, 0, loc)
}

// Hours is a working-hours range in minutes after midnight.
type Hours struct {
	Start int `json:"start_min"`
	End   int `json:"end_min"`
}

// Calendar describes a user's working week.
type Calendar struct {
	Loc *time.Location
	// Days maps each weekday to its hours; absent weekdays are days off.
	Days map[time.Weekday]Hours
}

// Hours returns the working hours on d, if d is a working day.
func (c Calendar) Hours(d Date) (Hours, bool) {
	h, ok := c.Days[d.Weekday()]
	return h, ok
}

// NextWorkingDay returns the first working day strictly after d. With no
// working days configured it returns the next day.
func (c Calendar) NextWorkingDay(d Date) Date {
	for i := 1; i <= 7; i++ {
		n := d.AddDays(i)
		if _, ok := c.Days[n.Weekday()]; ok {
			return n
		}
	}
	return d.AddDays(1)
}

// StartMinute returns when the working day d starts, defaulting to 09:00 on
// days off.
func (c Calendar) StartMinute(d Date) int {
	if h, ok := c.Hours(d); ok {
		return h.Start
	}
	return 9 * 60
}

var (
	relRe  = regexp.MustCompile(`^\+(\d{1,3})([dw])$`)
	timeRe = regexp.MustCompile(`^(\d{1,2}):(\d{2})$`)
)

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "sunday": time.Sunday,
	"mon": time.Monday, "monday": time.Monday,
	"tue": time.Tuesday, "tuesday": time.Tuesday,
	"wed": time.Wednesday, "wednesday": time.Wednesday,
	"thu": time.Thursday, "thursday": time.Thursday,
	"fri": time.Friday, "friday": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday,
}

// ErrNotDate is returned by ParseRelative for unrecognised expressions.
var ErrNotDate = errors.New("not a date expression")

// ParseRelative resolves a date expression relative to today: today, tomorrow,
// yesterday, a weekday name (today or the next occurrence), +Nd, +Nw, nwd
// (next working day) or YYYY-MM-DD.
func ParseRelative(s string, today Date, cal Calendar) (Date, error) {
	s = strings.ToLower(s)
	switch s {
	case "today", "tod":
		return today, nil
	case "tomorrow", "tom":
		return today.AddDays(1), nil
	case "yesterday":
		return today.AddDays(-1), nil
	case "nwd":
		return cal.NextWorkingDay(today), nil
	}
	if wd, ok := weekdays[s]; ok {
		diff := (int(wd) - int(today.Weekday()) + 7) % 7
		return today.AddDays(diff), nil
	}
	if m := relRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		if m[2] == "w" {
			n *= 7
		}
		return today.AddDays(n), nil
	}
	if d, err := Parse(s); err == nil {
		return d, nil
	}
	return Date{}, fmt.Errorf("%w: %q", ErrNotDate, s)
}

// ParseClock parses HH:MM into minutes after midnight.
func ParseClock(s string) (int, bool) {
	m := timeRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	h, _ := strconv.Atoi(m[1])
	mi, _ := strconv.Atoi(m[2])
	if h > 23 || mi > 59 {
		return 0, false
	}
	return h*60 + mi, true
}
