package filter

import (
	"slices"
	"testing"
	"time"

	"kala/internal/dates"
)

func TestDerivedPriority(t *testing.T) {
	today := dates.NewDate(2026, time.October, 21)
	cases := map[string]int{
		"":           2,
		"2026-10-01": 0,
		"2026-10-21": 0,
		"2026-10-22": 1,
		"2026-10-24": 1,
		"2026-10-25": 2,
	}
	for due, want := range cases {
		if got := DerivedPriority(due, today); got != want {
			t.Errorf("DerivedPriority(%q) = %d, want %d", due, got, want)
		}
	}
}

func TestNegatedQuotedText(t *testing.T) {
	env := Env{Now: time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC), Cal: dates.Calendar{Loc: time.UTC}}
	text := `(t.title LIKE ? ESCAPE '\' OR t.notes LIKE ? ESCAPE '\')`
	cases := []struct {
		q, where string
		args     []any
	}{
		{`"foo bar"`, text, []any{"%foo bar%", "%foo bar%"}},
		{`-"foo bar"`, "NOT COALESCE((" + text + "), 0)", []any{"%foo bar%", "%foo bar%"}},
		{`- "foo"`, text + " AND " + text, []any{"%-%", "%-%", "%foo%", "%foo%"}},
		{`a-"b"`, text + " AND " + text, []any{"%a-%", "%a-%", "%b%", "%b%"}},
	}
	for _, tc := range cases {
		q, err := Compile(tc.q, env)
		if err != nil {
			t.Errorf("%s: %v", tc.q, err)
			continue
		}
		if q.Where != tc.where || !slices.Equal(q.Args, tc.args) {
			t.Errorf("%s: where = %s args = %v", tc.q, q.Where, q.Args)
		}
	}
}
