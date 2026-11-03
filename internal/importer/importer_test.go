package importer

import (
	"archive/zip"
	"bytes"
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"kala/internal/service"
	"kala/internal/store"
)

var (
	rome = mustLoc("Europe/Rome")
	now  = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
)

func mustLoc(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

const vikunjaJSON = `[
 {"id": 1, "title": "Inbox", "tasks": [
  {"id": 10, "title": "Call Bob", "project_id": 1, "done": false, "done_at": "0001-01-01T00:00:00Z",
   "due_date": "2026-11-04T23:30:00Z", "priority": 0, "labels": [{"title": "Rainbow"}, {"title": "Call Me"}],
   "description": "", "created": "2026-10-01T08:00:00Z", "updated": "2026-10-02T08:00:00Z"}
 ]},
 {"id": 2, "title": "Solunet", "parent_project_id": 0, "tasks": null},
 {"id": 3, "title": "Rainbow", "parent_project_id": 2, "is_archived": true, "tasks": [
  {"id": 20, "title": "Supporto Zaitex", "project_id": 3, "done": true, "done_at": "0001-01-01T00:00:00Z",
   "due_date": "0001-01-01T00:00:00Z", "priority": 3, "labels": [{"title": "rainbow"}, {"title": "Solunet"}],
   "description": "Import da Clockify — progetto Clockify: Rainbow (1 entry, 1.00 h)\nKeep this line",
   "created": "2026-10-01T08:00:00Z", "updated": "2026-10-15T08:00:00Z"},
  {"id": 21, "title": "Supporto Zaitex", "project_id": 3, "done": true, "done_at": "2026-09-01T10:00:00Z",
   "due_date": "0001-01-01T00:00:00Z", "priority": 5,
   "description": "<p>See <a href=\"https://x.test/a\">the doc</a> &amp; <a href=\"https://x.test\">https://x.test</a></p><ul><li>one</li><li>two</li></ul>",
   "reminders": [{"reminder": "2026-05-08T13:00:00Z", "relative_to": "due_date"}],
   "repeat_after": 604800, "related_tasks": {"related": [{"id": 10, "title": ""}]},
   "created": "2026-08-01T08:00:00Z", "updated": "2026-09-01T10:00:00Z"}
 ]}
]`

func zipped(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func byID(tasks []*task, title string, nth int) *task {
	for _, t := range tasks {
		if t.Title == title {
			if nth == 0 {
				return t
			}
			nth--
		}
	}
	return nil
}

func TestVikunja(t *testing.T) {
	for name, data := range map[string][]byte{
		"json": []byte(vikunjaJSON),
		"zip":  zipped(t, "data.json", vikunjaJSON),
	} {
		t.Run(name, func(t *testing.T) {
			tasks, err := vikunja(data, rome, now)
			if err != nil {
				t.Fatal(err)
			}
			if len(tasks) != 3 {
				t.Fatalf("got %d tasks", len(tasks))
			}

			call := byID(tasks, "Call Bob", 0)
			// Inbox gives no project; a label naming a project is promoted.
			if want := []string{"project:rainbow", "call-me", "status:open"}; !slices.Equal(call.Tags, want) {
				t.Errorf("Call Bob tags = %v, want %v", call.Tags, want)
			}
			if call.DueDate == nil || *call.DueDate != "2026-11-05" {
				t.Errorf("due = %v, want 2026-11-05 (Rome)", call.DueDate)
			}
			if call.ClosedAt != nil {
				t.Error("open task has closed_at")
			}

			first := byID(tasks, "Supporto Zaitex", 0)
			// Own project label dropped, other project label promoted.
			if want := []string{"project:rainbow", "project:solunet", "status:closed", "priority:high"}; !slices.Equal(first.Tags, want) {
				t.Errorf("tags = %v, want %v", first.Tags, want)
			}
			if first.Notes != "Keep this line" {
				t.Errorf("notes = %q", first.Notes)
			}
			if !first.closedGuessed || first.ClosedAt == nil || !first.ClosedAt.Equal(first.UpdatedAt) {
				t.Errorf("closed_at = %v, guessed = %v", first.ClosedAt, first.closedGuessed)
			}
			if first.DueDate != nil {
				t.Error("zero due date imported")
			}

			second := byID(tasks, "Supporto Zaitex", 1)
			wantNotes := "See the doc (https://x.test/a) & https://x.test\n\n- one\n- two\n\nRepeats every 7d (from Vikunja)\nRelated: Call Bob"
			if second.Notes != wantNotes {
				t.Errorf("notes = %q, want %q", second.Notes, wantNotes)
			}
			if second.RemindAt == nil || second.RemindedAt == nil {
				t.Error("past reminder not imported as already sent")
			}
			if second.closedGuessed || !second.ClosedAt.Equal(time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)) {
				t.Errorf("closed_at = %v", second.ClosedAt)
			}
		})
	}
}

func TestVikunjaErrors(t *testing.T) {
	if _, err := vikunja(zipped(t, "other.json", "[]"), rome, now); err == nil || !strings.Contains(err.Error(), "data.json") {
		t.Errorf("err = %v", err)
	}
	if _, err := vikunja([]byte("{"), rome, now); err == nil {
		t.Error("invalid JSON accepted")
	}
}

const clockifyCSV = "\xef\xbb\xbf" + `"Project","Client","Description","Task","Tags","Start Date","Start Time","End Date","End Time"
"Rainbow","Acme","Supporto Zaitex","","","15/10/2026","08:00:00","15/10/2026","09:30:00"
"Rainbow","Acme","supporto zaitex","","meeting","14/10/2026","23:00:00","15/10/2026","00:30:00"
"COSO","Acme","Supporto Zaitex","","","14/10/2026","10:00:00","14/10/2026","11:00:00"
"","","","","","13/10/2026","10:00:00","13/10/2026","10:15:00"
"Rainbow","Acme","Running","","","13/10/2026","10:00:00","",""
`

func TestClockify(t *testing.T) {
	groups, err := clockify([]byte(clockifyCSV), rome)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 3 {
		t.Fatalf("got %d groups: %+v", len(groups), groups)
	}
	g := groups[0]
	if g.Project != "project:rainbow" || g.Title != "Supporto Zaitex" || len(g.Entries) != 2 {
		t.Fatalf("group = %+v", g)
	}
	if want := time.Date(2026, 10, 15, 6, 0, 0, 0, time.UTC); !g.Entries[0].StartedAt.Equal(want) {
		t.Errorf("start = %v, want %v", g.Entries[0].StartedAt.UTC(), want)
	}
	if d := g.Entries[1].StoppedAt.Sub(g.Entries[1].StartedAt); d != 90*time.Minute {
		t.Errorf("entry across midnight lasts %v", d)
	}
	if groups[2].Project != "" || groups[2].Title != "(no description)" {
		t.Errorf("empty group = %+v", groups[2])
	}
}

func TestClockifyDateOrder(t *testing.T) {
	csv := `Project,Description,Start Date,Start Time,End Date,End Time
P,x,03/04/2026,08:00,03/04/2026,09:00
P,x,10/25/2026,1:00 PM,10/25/2026,2:30 PM
`
	groups, err := clockify([]byte(csv), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	e := groups[0].Entries
	if want := time.Date(2026, 3, 4, 8, 0, 0, 0, time.UTC); !e[0].StartedAt.Equal(want) {
		t.Errorf("month-first start = %v, want %v", e[0].StartedAt, want)
	}
	if want := time.Date(2026, 10, 25, 13, 0, 0, 0, time.UTC); !e[1].StartedAt.Equal(want) {
		t.Errorf("12h start = %v, want %v", e[1].StartedAt, want)
	}
}

func TestClockifyMissingColumn(t *testing.T) {
	if _, err := clockify([]byte("Project,Description\nP,x\n"), time.UTC); err == nil || !strings.Contains(err.Error(), "start date") {
		t.Errorf("err = %v", err)
	}
}

func TestConvert(t *testing.T) {
	ex, st, err := Convert([]byte(vikunjaJSON), []byte(clockifyCSV), rome, now)
	if err != nil {
		t.Fatal(err)
	}
	if st != (Stats{Tasks: 5, Entries: 4, Matched: 1, Created: 2}) {
		t.Fatalf("stats = %+v", st)
	}
	var matched, coso *service.ExportTask
	for i := range ex.Tasks {
		et := &ex.Tasks[i]
		switch {
		case et.Title == "Supporto Zaitex" && len(et.Entries) > 0 && slices.Contains(et.Tags, "project:rainbow"):
			matched = et
		case slices.Contains(et.Tags, "project:coso"):
			coso = et
		}
	}
	// The most recently updated of the two candidates wins, and its guessed
	// closing time moves to the end of the last entry.
	if matched == nil || !matched.UpdatedAt.Equal(time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)) || len(matched.Entries) != 2 {
		t.Fatalf("matched = %+v", matched)
	}
	if want := time.Date(2026, 10, 15, 7, 30, 0, 0, time.UTC); !matched.ClosedAt.Equal(want) {
		t.Errorf("closed_at = %v, want %v", matched.ClosedAt.UTC(), want)
	}
	// Same title in another project: a new closed task.
	if coso == nil || !slices.Equal(coso.Tags, []string{"project:coso", "status:closed"}) || coso.ClosedAt == nil {
		t.Fatalf("coso = %+v", coso)
	}

	// The output loads with the regular import.
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc := service.New(db, nil, func() time.Time { return now })
	u, err := svc.Register(context.Background(), "a@example.com", "password123", "Europe/Rome", true)
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Import(context.Background(), u.ID, ex)
	if err != nil {
		t.Fatal(err)
	}
	if res.Tasks != 5 || res.Entries != 4 {
		t.Fatalf("import = %+v", res)
	}
	open, err := svc.Tasks(context.Background(), u.ID, service.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var openTitles []string
	for _, ot := range open {
		if ot.ClosedAt == nil {
			openTitles = append(openTitles, ot.Title)
		}
	}
	if !slices.Equal(openTitles, []string{"Call Bob"}) {
		t.Errorf("open tasks = %v", openTitles)
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Migrated from TickTick": "migrated-from-ticktick",
		"  a:b, c#d!! ":          "a-b-c-d",
		"--x--":                  "x",
		"!!":                     "",
		"Ünïcode Name":           "ünïcode-name",
	} {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}
