package service_test

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"kala/internal/events"
	"kala/internal/service"
	"kala/internal/store"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

// Wednesday 2026-10-21 10:00 in Europe/Rome.
var start = time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)

func newService(t *testing.T) (*service.Service, *clock) {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	c := &clock{t: start}
	return service.New(db, events.NewHub(), c.now), c
}

func register(t *testing.T, s *service.Service, email string) int64 {
	t.Helper()
	u, err := s.Register(context.Background(), email, "password123", "Europe/Rome", true)
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func mustCreate(t *testing.T, s *service.Service, uid int64, in service.TaskInput) *service.Task {
	t.Helper()
	task, err := s.CreateTask(context.Background(), uid, in)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func titles(ts []service.Task) []string {
	out := []string{}
	for _, t := range ts {
		out = append(out, t.Title)
	}
	slices.Sort(out)
	return out
}

func TestSignupGate(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "a@example.com", "password123", "UTC", false); err != nil {
		t.Fatalf("first user must always register: %v", err)
	}
	if _, err := s.Register(ctx, "b@example.com", "password123", "UTC", false); !service.IsCode(err, service.CodeDenied) {
		t.Fatalf("want denied, got %v", err)
	}
	if _, err := s.Register(ctx, "a@example.com", "password123", "UTC", true); !service.IsCode(err, service.CodeConflict) {
		t.Fatalf("want conflict for duplicate email, got %v", err)
	}
}

func TestQuickAddAndNamespaces(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	uid := register(t, s, "a@example.com")

	task := mustCreate(t, s, uid, service.TaskInput{Quick: "Write report #client:acme @kala !1 due:fri remind:thu 9:30"})
	if task.Title != "Write report" {
		t.Errorf("title = %q", task.Title)
	}
	want := []string{"client:acme", "priority:high", "project:kala", "status:open"}
	if !slices.Equal(task.Tags, want) {
		t.Errorf("tags = %v, want %v", task.Tags, want)
	}
	if task.DueDate == nil || *task.DueDate != "2026-10-23" {
		t.Errorf("due = %v", task.DueDate)
	}
	if task.RemindAt == nil || !task.RemindAt.Equal(time.Date(2026, 10, 22, 7, 30, 0, 0, time.UTC)) {
		t.Errorf("remind = %v", task.RemindAt)
	}

	// Exclusive replacement and terminal status.
	task, err := s.UpdateTask(ctx, uid, task.ID, service.TaskPatch{AddTags: []string{"status:closed", "priority:low"}})
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"client:acme", "priority:low", "project:kala", "status:closed"}
	if !slices.Equal(task.Tags, want) || task.ClosedAt == nil {
		t.Fatalf("tags = %v closed = %v", task.Tags, task.ClosedAt)
	}
	// Removing the status falls back to the default.
	task, err = s.UpdateTask(ctx, uid, task.ID, service.TaskPatch{RemoveTags: []string{"status:closed"}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(task.Tags, "status:open") || task.ClosedAt != nil {
		t.Fatalf("tags = %v closed = %v", task.Tags, task.ClosedAt)
	}
	if _, err := s.UpdateTask(ctx, uid, task.ID, service.TaskPatch{AddTags: []string{"status:bogus"}}); !service.IsCode(err, service.CodeInvalid) {
		t.Fatalf("undefined exclusive value: want invalid, got %v", err)
	}
	if _, err := s.UpdateTask(ctx, uid, task.ID, service.TaskPatch{Tags: service.Some([]string{"status:open", "status:closed"})}); !service.IsCode(err, service.CodeInvalid) {
		t.Fatalf("two exclusive values: want invalid, got %v", err)
	}
}

func TestTimers(t *testing.T) {
	s, c := newService(t)
	ctx := context.Background()
	uid := register(t, s, "a@example.com")
	a := mustCreate(t, s, uid, service.TaskInput{Title: "A"})
	b := mustCreate(t, s, uid, service.TaskInput{Title: "B", Tags: []string{"project:x"}})

	if _, err := s.StartTimer(ctx, uid, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartTimer(ctx, uid, b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartTimer(ctx, uid, a.ID); !service.IsCode(err, service.CodeConflict) {
		t.Fatalf("second timer on same task: want conflict, got %v", err)
	}
	active, err := s.ActiveTimers(ctx, uid)
	if err != nil || len(active) != 2 {
		t.Fatalf("active = %v, %v", active, err)
	}

	c.advance(30 * time.Minute)
	closed, err := s.UpdateTask(ctx, uid, a.ID, service.TaskPatch{Closed: service.Some(true)})
	if err != nil {
		t.Fatal(err)
	}
	if closed.RunningSince != nil || closed.TrackedSeconds != 1800 {
		t.Fatalf("closing must stop the timer: running=%v tracked=%d", closed.RunningSince, closed.TrackedSeconds)
	}

	c.advance(30 * time.Minute)
	reopened, err := s.StartTimer(ctx, uid, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.ClosedAt != nil || !slices.Contains(reopened.Tags, "status:open") {
		t.Fatalf("starting a timer must reopen: %+v", reopened)
	}
	c.advance(15 * time.Minute)
	if _, err := s.StopAllTimers(ctx, uid); err != nil {
		t.Fatal(err)
	}

	rep, err := s.TimeReport(ctx, uid, service.EntryFilter{From: start.Add(-time.Hour), To: c.t.Add(time.Hour)}, "project")
	if err != nil {
		t.Fatal(err)
	}
	// A: 30m + 15m, B: 75m.
	if rep.Total != 120*60 {
		t.Errorf("total = %d", rep.Total)
	}
	got := map[string]int64{}
	for _, r := range rep.Rows {
		got[r.Key] = r.Seconds
	}
	if got["project:x"] != 75*60 || got[service.NoValue] != 45*60 {
		t.Errorf("rows = %+v", rep.Rows)
	}

	// Deleted tasks disappear from reports.
	if err := s.DeleteTask(ctx, uid, b.ID); err != nil {
		t.Fatal(err)
	}
	rep, _ = s.TimeReport(ctx, uid, service.EntryFilter{From: start.Add(-time.Hour), To: c.t.Add(time.Hour)}, "day")
	if rep.Total != 45*60 {
		t.Errorf("total after delete = %d", rep.Total)
	}
}

func TestFilters(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	uid := register(t, s, "a@example.com")
	mustCreate(t, s, uid, service.TaskInput{Quick: "api work @kala:api due:today"})
	mustCreate(t, s, uid, service.TaskInput{Quick: "web work @kala:web due:yesterday !high"})
	mustCreate(t, s, uid, service.TaskInput{Quick: "other @infra due:+3d"})
	mustCreate(t, s, uid, service.TaskInput{Quick: "loose idea #someday"})
	done := mustCreate(t, s, uid, service.TaskInput{Quick: "finished @kala"})
	if _, err := s.UpdateTask(ctx, uid, done.ID, service.TaskPatch{Closed: service.Some(true)}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		q    string
		want []string
	}{
		{"project:kala", []string{"api work", "finished", "web work"}},
		{"project:kala is:open", []string{"api work", "web work"}},
		{"project:kala:api,infra", []string{"api work", "other"}},
		{"-project: is:open", []string{"loose idea"}},
		{"#someday", []string{"loose idea"}},
		{"is:open due:<=today", []string{"api work", "web work"}},
		{"due:overdue", []string{"web work"}},
		{"due:week", []string{"api work", "other"}},
		{"due:none is:open", []string{"loose idea"}},
		{"priority:high", []string{"web work"}},
		{`"work"`, []string{"api work", "web work"}},
		{"-work is:open", []string{"loose idea", "other"}},
		{"closed:today", []string{"finished"}},
	}
	for _, tc := range cases {
		got, err := s.Tasks(ctx, uid, service.ListOptions{Query: tc.q})
		if err != nil {
			t.Errorf("%q: %v", tc.q, err)
			continue
		}
		if !slices.Equal(titles(got), tc.want) {
			t.Errorf("%q = %v, want %v", tc.q, titles(got), tc.want)
		}
	}

	sorted, err := s.Tasks(ctx, uid, service.ListOptions{Query: "is:open sort:priority,due"})
	if err != nil {
		t.Fatal(err)
	}
	// "api work" has no priority tag but is due today, so it ranks as high.
	var order []string
	for _, task := range sorted {
		order = append(order, task.Title+"="+task.Priority)
		if want := !slices.Contains(task.Tags, "priority:"+task.Priority); task.PriorityDerived != want {
			t.Errorf("%s: derived = %v", task.Title, task.PriorityDerived)
		}
	}
	if want := []string{"web work=high", "api work=high", "other=medium", "loose idea=low"}; !slices.Equal(order, want) {
		t.Errorf("priority sort = %v, want %v", order, want)
	}
	if closed, _ := s.Task(ctx, uid, done.ID); closed.Priority != "" {
		t.Errorf("closed task priority = %q", closed.Priority)
	}
	if _, err := s.Tasks(ctx, uid, service.ListOptions{Query: "due:whenever"}); !service.IsCode(err, service.CodeInvalid) {
		t.Errorf("bad filter: want invalid, got %v", err)
	}
}

func TestIsolation(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	a := register(t, s, "a@example.com")
	b := register(t, s, "b@example.com")
	task := mustCreate(t, s, a, service.TaskInput{Title: "secret", Tags: []string{"project:x"}})
	if _, err := s.Task(ctx, b, task.ID); !service.IsCode(err, service.CodeNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
	if _, err := s.UpdateTask(ctx, b, task.ID, service.TaskPatch{Title: service.Some("pwned")}); !service.IsCode(err, service.CodeNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
	if _, err := s.StartTimer(ctx, b, task.ID); !service.IsCode(err, service.CodeNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
	if err := s.DeleteTask(ctx, b, task.ID); !service.IsCode(err, service.CodeNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
	list, _ := s.Tasks(ctx, b, service.ListOptions{})
	tagsB, _ := s.Tags(ctx, b)
	for _, tg := range tagsB {
		if tg.Name == "project:x" {
			t.Fatal("tags leaked across users")
		}
	}
	if len(list) != 0 {
		t.Fatalf("tasks leaked: %v", list)
	}
}

func TestRenameTag(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	uid := register(t, s, "a@example.com")
	t1 := mustCreate(t, s, uid, service.TaskInput{Title: "one", Tags: []string{"project:old:api"}})
	t2 := mustCreate(t, s, uid, service.TaskInput{Title: "two", Tags: []string{"project:old", "project:new"}})

	prev, err := s.RenameTag(ctx, uid, service.RenameInput{From: "project:old", To: "project:new"})
	if err != nil {
		t.Fatal(err)
	}
	if prev.Applied || len(prev.Renames) != 2 || !slices.Equal(prev.Merges, []string{"project:new"}) {
		t.Fatalf("preview = %+v", prev)
	}
	if _, err := s.RenameTag(ctx, uid, service.RenameInput{From: "project:old", To: "project:new", Confirm: true}); err != nil {
		t.Fatal(err)
	}
	got1, _ := s.Task(ctx, uid, t1.ID)
	got2, _ := s.Task(ctx, uid, t2.ID)
	if !slices.Contains(got1.Tags, "project:new:api") {
		t.Errorf("subtree not renamed: %v", got1.Tags)
	}
	if !slices.Equal(got2.Tags, []string{"project:new", "status:open"}) {
		t.Errorf("merge: %v", got2.Tags)
	}

	// Renaming into an exclusive namespace that would clash is refused.
	mustCreate(t, s, uid, service.TaskInput{Title: "three", Tags: []string{"wip"}})
	if _, err := s.RenameTag(ctx, uid, service.RenameInput{From: "wip", To: "status:pending", Confirm: true}); !service.IsCode(err, service.CodeConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
}

func TestTrashAndExport(t *testing.T) {
	s, c := newService(t)
	ctx := context.Background()
	uid := register(t, s, "a@example.com")
	task := mustCreate(t, s, uid, service.TaskInput{Title: "temp", Tags: []string{"project:x"}})
	if _, err := s.StartTimer(ctx, uid, task.ID); err != nil {
		t.Fatal(err)
	}
	c.advance(time.Hour)
	if err := s.DeleteTask(ctx, uid, task.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.Tasks(ctx, uid, service.ListOptions{}); len(list) != 0 {
		t.Fatal("deleted task still listed")
	}
	if trash, _ := s.Tasks(ctx, uid, service.ListOptions{Deleted: true}); len(trash) != 1 || trash[0].RunningSince != nil {
		t.Fatalf("trash = %+v", trash)
	}

	ex, err := s.Export(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	other := register(t, s, "b@example.com")
	res, err := s.Import(ctx, other, ex)
	if err != nil {
		t.Fatal(err)
	}
	if res.Tasks != 1 || res.Entries != 1 {
		t.Fatalf("import = %+v", res)
	}
	trash, _ := s.Tasks(ctx, other, service.ListOptions{Deleted: true})
	if len(trash) != 1 || trash[0].TrackedSeconds != 3600 || !slices.Contains(trash[0].Tags, "project:x") {
		t.Fatalf("imported trash = %+v", trash)
	}

	if _, err := s.RestoreTask(ctx, uid, task.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTask(ctx, uid, task.ID); err != nil {
		t.Fatal(err)
	}
	c.advance(service.TrashRetention + time.Hour)
	if err := s.PurgeExpired(ctx); err != nil {
		t.Fatal(err)
	}
	if trash, _ := s.Tasks(ctx, uid, service.ListOptions{Deleted: true}); len(trash) != 0 {
		t.Fatal("expired trash not purged")
	}
}

func TestRefreshRotation(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	uid := register(t, s, "a@example.com")
	tok, err := s.IssueRefresh(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	got, next, err := s.RotateRefresh(ctx, tok)
	if err != nil || got != uid {
		t.Fatalf("rotate: %v %v", got, err)
	}
	// Reusing the old token revokes the whole family.
	if _, _, err := s.RotateRefresh(ctx, tok); !service.IsCode(err, service.CodeAuth) {
		t.Fatalf("reuse: want auth error, got %v", err)
	}
	if _, _, err := s.RotateRefresh(ctx, next); !service.IsCode(err, service.CodeAuth) {
		t.Fatalf("after reuse the new token must be revoked too, got %v", err)
	}
}

func TestDigests(t *testing.T) {
	s, c := newService(t)
	ctx := context.Background()
	uid := register(t, s, "a@example.com")
	mustCreate(t, s, uid, service.TaskInput{Quick: "late due:yesterday"})
	mustCreate(t, s, uid, service.TaskInput{Quick: "now due:today remind:+5m"})
	mustCreate(t, s, uid, service.TaskInput{Quick: "friday due:fri"})
	t4 := mustCreate(t, s, uid, service.TaskInput{Quick: "thursday due:thu"})

	b, err := s.Briefing(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(titles(b.Overdue), []string{"late"}) || !slices.Equal(titles(b.DueToday), []string{"now"}) || len(b.Reminders) != 1 {
		t.Fatalf("briefing = %+v", b)
	}
	if _, err := s.StartTimer(ctx, uid, t4.ID); err != nil {
		t.Fatal(err)
	}
	w, err := s.Wrapup(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(titles(w.Open), []string{"late", "now"}) || !slices.Equal(titles(w.Next), []string{"thursday"}) || len(w.Running) != 1 {
		t.Fatalf("wrapup = %+v", w)
	}

	if due, _ := s.DueReminders(ctx, uid); len(due) != 0 {
		t.Fatal("reminder fired early")
	}
	c.advance(10 * time.Minute)
	due, _ := s.DueReminders(ctx, uid)
	if len(due) != 1 {
		t.Fatal("reminder not due")
	}
	snooze := c.t.Add(10 * time.Minute)
	if _, err := s.AckReminder(ctx, uid, due[0].ID, &snooze); err != nil {
		t.Fatal(err)
	}
	if due, _ := s.DueReminders(ctx, uid); len(due) != 0 {
		t.Fatal("snoozed reminder still due")
	}
}
