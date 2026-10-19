// Package quickadd parses the one-line task syntax:
//
//	Write report #client:acme @kala !high due:fri remind:thu 9:00
//
// "#tag" adds a tag, "@x" adds project:x, "!x" sets priority:x (!1..!3 map to
// high, medium, low), "due:" takes a date expression and "remind:" a date
// expression optionally followed by HH:MM (default: start of working hours),
// a bare HH:MM, or a relative +30m / +2h. Every other word is the title.
package quickadd

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"kala/internal/dates"
	"kala/internal/tags"
)

// Result is a parsed quick-add line.
type Result struct {
	Title    string     `json:"title"`
	Tags     []string   `json:"tags"`
	DueDate  string     `json:"due_date,omitempty"`
	RemindAt *time.Time `json:"remind_at,omitempty"`
}

var (
	priorities = map[string]string{"1": "high", "2": "medium", "3": "low"}
	durRe      = regexp.MustCompile(`^\+(\d{1,4})([mh])$`)
)

// Parse parses line relative to now in the calendar's timezone.
func Parse(line string, now time.Time, cal dates.Calendar) (Result, error) {
	var (
		res   = Result{Tags: []string{}}
		title []string
	)
	today := dates.On(now, cal.Loc)
	words := strings.Fields(line)
	addTag := func(t string) error {
		n, err := tags.Normalize(t)
		if err != nil {
			return err
		}
		res.Tags = append(res.Tags, n)
		return nil
	}
	for i := 0; i < len(words); i++ {
		w := words[i]
		lw := strings.ToLower(w)
		var err error
		switch {
		case len(w) > 1 && w[0] == '#':
			err = addTag(w[1:])
		case len(w) > 1 && w[0] == '@':
			err = addTag("project:" + w[1:])
		case len(w) > 1 && w[0] == '!':
			p := strings.ToLower(w[1:])
			if m, ok := priorities[p]; ok {
				p = m
			}
			err = addTag("priority:" + p)
		case strings.HasPrefix(lw, "due:"):
			var d dates.Date
			d, err = dates.ParseRelative(lw[4:], today, cal)
			res.DueDate = d.String()
		case strings.HasPrefix(lw, "remind:"):
			var t time.Time
			var consumed bool
			next := ""
			if i+1 < len(words) {
				next = words[i+1]
			}
			t, consumed, err = parseRemind(lw[7:], next, now, today, cal)
			if consumed {
				i++
			}
			res.RemindAt = &t
		default:
			title = append(title, w)
		}
		if err != nil {
			return Result{}, err
		}
	}
	res.Title = strings.Join(title, " ")
	if res.Title == "" {
		return Result{}, fmt.Errorf("a task needs a title")
	}
	return res, nil
}

func parseRemind(v, next string, now time.Time, today dates.Date, cal dates.Calendar) (time.Time, bool, error) {
	if m := durRe.FindStringSubmatch(v); m != nil {
		n, _ := strconv.Atoi(m[1])
		unit := time.Minute
		if m[2] == "h" {
			unit = time.Hour
		}
		return now.Add(time.Duration(n) * unit).Truncate(time.Minute), false, nil
	}
	if minute, ok := dates.ParseClock(v); ok {
		t := today.At(minute, cal.Loc)
		if !t.After(now) {
			t = today.AddDays(1).At(minute, cal.Loc)
		}
		return t, false, nil
	}
	d, err := dates.ParseRelative(v, today, cal)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("remind: %w", err)
	}
	if minute, ok := dates.ParseClock(next); ok {
		return d.At(minute, cal.Loc), true, nil
	}
	return d.At(cal.StartMinute(d), cal.Loc), false, nil
}
