package schedule

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kala/internal/dates"
	"kala/internal/events"
	"kala/internal/service"
	"kala/internal/store"
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

// drain returns how many reminder events are waiting on ch.
func drain(ch <-chan events.Event) int {
	n := 0
	for {
		select {
		case e := <-ch:
			if e.Type == "reminder" {
				n++
			}
		default:
			return n
		}
	}
}

func TestReminderPushes(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC) // Wednesday 10:00 in Rome
	hub := events.NewHub()
	svc := service.New(db, hub, func() time.Time { return now })
	sched := New(svc, hub, slog.New(slog.DiscardHandler))
	u, err := svc.Register(ctx, "a@example.com", "password123", "Europe/Rome", true)
	if err != nil {
		t.Fatal(err)
	}
	at := now.Add(-time.Minute)
	task, err := svc.CreateTask(ctx, u.ID, service.TaskInput{Title: "call", RemindAt: &at})
	if err != nil {
		t.Fatal(err)
	}
	ch, cancel := hub.Subscribe(u.ID)
	defer cancel()
	if n := drain(ch); n != 1 {
		t.Fatalf("catch-up on connect: %d reminders, want 1", n)
	}
	tick := func(after time.Duration) int {
		now = now.Add(after)
		sched.Tick(ctx)
		return drain(ch)
	}
	if n := tick(0); n != 1 {
		t.Fatalf("first tick: %d reminders, want 1", n)
	}
	if n := tick(Interval) + tick(23*time.Hour); n != 0 {
		t.Fatalf("within a day: %d reminders, want none", n)
	}
	// Left unacknowledged, a reminder comes back once a day.
	if n := tick(time.Hour); n != 1 {
		t.Fatalf("a day later: %d reminders, want 1", n)
	}
	if n := tick(Interval) + tick(25*time.Hour); n != 1 {
		t.Fatalf("two days later: %d reminders, want 1", n)
	}
	// A snooze is a new reminder; an acknowledged one is gone for good.
	later := now.Add(time.Hour)
	if _, err := svc.AckReminder(ctx, u.ID, task.ID, &later); err != nil {
		t.Fatal(err)
	}
	if n := tick(Interval) + tick(time.Hour); n != 1 {
		t.Fatalf("after the snooze: %d reminders, want 1", n)
	}
	if _, err := svc.AckReminder(ctx, u.ID, task.ID, nil); err != nil {
		t.Fatal(err)
	}
	if n := tick(25*time.Hour) + tick(72*time.Hour) + tick(Interval); n != 0 {
		t.Fatalf("after the acknowledgement: %d reminders, want none", n)
	}
	// What was pushed days ago is forgotten.
	if len(sched.sentReminders) != 0 {
		t.Errorf("sentReminders keeps %d stale entries", len(sched.sentReminders))
	}
	for key := range sched.sentDigests {
		if strings.Contains(key, "/2026-10-21/") {
			t.Errorf("sentDigests keeps %q", key)
		}
	}
}
