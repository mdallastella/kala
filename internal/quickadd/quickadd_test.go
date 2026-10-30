package quickadd

import (
	"slices"
	"testing"
	"time"

	"kala/internal/dates"
)

func TestParse(t *testing.T) {
	rome, _ := time.LoadLocation("Europe/Rome")
	cal := dates.Calendar{Loc: rome, Days: map[time.Weekday]dates.Hours{
		time.Monday: {Start: 540, End: 1080}, time.Friday: {Start: 480, End: 840},
	}}
	// Wednesday 2026-10-21 10:00 Rome.
	now := time.Date(2026, 10, 21, 10, 0, 0, 0, rome)
	cases := []struct {
		in     string
		title  string
		tags   []string
		due    string
		remind string
	}{
		{"Write report #client:acme @kala !1 due:fri", "Write report", []string{"client:acme", "project:kala", "priority:high"}, "2026-10-23", ""},
		{"call bob due:tomorrow", "call bob", nil, "2026-10-22", ""},
		{"x due:nwd", "x", nil, "2026-10-23", ""},
		{"x due:+2w", "x", nil, "2026-11-04", ""},
		{"x remind:fri", "x", nil, "", "2026-10-23T08:00:00+02:00"},
		{"x remind:mon 14:30", "x", nil, "", "2026-10-26T14:30:00+01:00"},
		{"x remind:9:00", "x", nil, "", "2026-10-22T09:00:00+02:00"},
		{"x remind:+90m", "x", nil, "", "2026-10-21T11:30:00+02:00"},
		{"Fix !important things", "Fix things", []string{"priority:important"}, "", ""},
	}
	for _, tc := range cases {
		r, err := Parse(tc.in, now, cal)
		if err != nil {
			t.Errorf("%q: %v", tc.in, err)
			continue
		}
		if r.Title != tc.title || !slices.Equal(r.Tags, append([]string{}, tc.tags...)) || r.DueDate != tc.due {
			t.Errorf("%q = %+v", tc.in, r)
		}
		got := ""
		if r.RemindAt != nil {
			got = r.RemindAt.In(rome).Format(time.RFC3339)
		}
		if got != tc.remind {
			t.Errorf("%q remind = %s, want %s", tc.in, got, tc.remind)
		}
	}
	for _, bad := range []string{"", "#only-tags", "x due:someday", "x #bad,tag",
		"x spent:", "x spent:0m", "x spent:25h", "x spent:1.5h", "x spent:9:00", "x spent:1h 9:30"} {
		if _, err := Parse(bad, now, cal); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
}

func TestParseSpent(t *testing.T) {
	rome, _ := time.LoadLocation("Europe/Rome")
	cal := dates.Calendar{Loc: rome}
	// Wednesday 2026-10-21 10:00:30 Rome.
	now := time.Date(2026, 10, 21, 10, 0, 30, 0, rome)
	cases := []struct{ in, title, from, to string }{
		{"Call bob spent:45m", "Call bob", "09:15", "10:00"},
		{"Call bob spent:1h30m 8:00 #client", "Call bob", "08:00", "09:30"},
		{"Call bob spent:2h", "Call bob", "08:00", "10:00"},
		{"Call bob spent:1h 9:00", "Call bob", "09:00", "10:00"},
		{"spent:30m Call 11:00", "Call 11:00", "09:30", "10:00"},
	}
	for _, tc := range cases {
		r, err := Parse(tc.in, now, cal)
		if err != nil {
			t.Errorf("%q: %v", tc.in, err)
			continue
		}
		if r.Spent == nil {
			t.Errorf("%q: no spent", tc.in)
			continue
		}
		from, to := r.Spent.StartedAt.In(rome).Format("15:04"), r.Spent.StoppedAt.In(rome).Format("15:04")
		if r.Title != tc.title || from != tc.from || to != tc.to {
			t.Errorf("%q = %q %s-%s, want %q %s-%s", tc.in, r.Title, from, to, tc.title, tc.from, tc.to)
		}
	}
}
