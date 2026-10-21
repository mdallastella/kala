package filter

import (
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
