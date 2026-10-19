package schedule

import (
	"testing"
	"time"

	"kala/internal/dates"
	"kala/internal/service"
)

func TestDigestDue(t *testing.T) {
	rome, _ := time.LoadLocation("Europe/Rome")
	cal := dates.Calendar{Loc: rome, Days: map[time.Weekday]dates.Hours{
		time.Wednesday: {Start: 9 * 60, End: 18 * 60},
		time.Friday:    {Start: 9 * 60, End: 13 * 60},
	}}
	at := func(d, h, m int) time.Time { return time.Date(2026, 10, d, h, m, 0, 0, rome) }
	cases := []struct {
		now          time.Time
		lastB, lastW string
		want         string
	}{
		{at(21, 8, 59), "", "", ""},
		{at(21, 9, 0), "", "", service.DigestBriefing},
		{at(21, 12, 0), "2026-10-21", "", ""},
		{at(21, 18, 0), "2026-10-21", "", service.DigestWrapup},
		{at(21, 23, 0), "", "", service.DigestWrapup}, // missed briefing is skipped once the day is over
		{at(21, 19, 0), "2026-10-21", "2026-10-21", ""},
		{at(22, 10, 0), "", "", ""},                   // Thursday is a day off
		{at(23, 13, 0), "", "", service.DigestWrapup}, // short Friday
	}
	for _, tc := range cases {
		st := &service.DigestState{Cal: cal, LastBriefing: tc.lastB, LastWrapup: tc.lastW}
		if got := DigestDue(st, tc.now); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.now, got, tc.want)
		}
	}
}
