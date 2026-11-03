package importer

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"kala/internal/service"
)

// maxTitle mirrors the service's title limit, so long titles are truncated
// rather than failing the whole import.
const maxTitle = 500

type vikProject struct {
	ID    int64     `json:"id"`
	Title string    `json:"title"`
	Tasks []vikTask `json:"tasks"`
}

type vikRef struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

type vikTask struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	ProjectID   int64     `json:"project_id"`
	Done        bool      `json:"done"`
	DoneAt      time.Time `json:"done_at"`
	DueDate     time.Time `json:"due_date"`
	Reminders   []struct {
		Reminder time.Time `json:"reminder"`
	} `json:"reminders"`
	RepeatAfter int64 `json:"repeat_after"`
	RepeatMode  int   `json:"repeat_mode"`
	Priority    int   `json:"priority"`
	Labels      []struct {
		Title string `json:"title"`
	} `json:"labels"`
	RelatedTasks map[string][]vikRef `json:"related_tasks"`
	Created      time.Time           `json:"created"`
	Updated      time.Time           `json:"updated"`
}

// task is a converted task plus what the merge step needs to know about it.
type task struct {
	service.ExportTask
	// closedGuessed is set when the source had no completion time, so
	// ClosedAt is only an approximation that later time entries can improve.
	closedGuessed bool
}

// vikunja converts a Vikunja export (the zip file, or its data.json) into
// tasks. Due dates are taken as calendar days in loc; reminders before now
// are marked as already sent.
func vikunja(data []byte, loc *time.Location, now time.Time) ([]*task, error) {
	if bytes.HasPrefix(data, []byte("PK")) {
		var err error
		if data, err = readZipEntry(data, "data.json"); err != nil {
			return nil, err
		}
	}
	var projects []vikProject
	if err := json.Unmarshal(data, &projects); err != nil {
		return nil, fmt.Errorf("vikunja: invalid data.json: %w", err)
	}

	projectTags := map[int64]string{}
	known := map[string]bool{}
	titles := map[int64]string{}
	for _, p := range projects {
		if tag := projectTag(p.Title); tag != "" && !strings.EqualFold(strings.TrimSpace(p.Title), "inbox") {
			known[tag] = true
			projectTags[p.ID] = tag
		}
		for _, t := range p.Tasks {
			titles[t.ID] = t.Title
		}
	}

	var out []*task
	for _, p := range projects {
		for _, vt := range p.Tasks {
			out = append(out, convertVikunjaTask(vt, projectTags[vt.ProjectID], known, titles, loc, now))
		}
	}
	return out, nil
}

func convertVikunjaTask(vt vikTask, project string, known map[string]bool, titles map[int64]string, loc *time.Location, now time.Time) *task {
	t := &task{ExportTask: service.ExportTask{Entries: []service.ExportEntry{}}}
	t.Title = truncate(strings.TrimSpace(vt.Title), maxTitle)
	if t.Title == "" {
		t.Title = "(untitled)"
	}
	t.CreatedAt, t.UpdatedAt = vt.Created, vt.Updated

	tagSet := []string{}
	add := func(tag string) {
		if tag != "" && !slices.Contains(tagSet, tag) {
			tagSet = append(tagSet, tag)
		}
	}
	add(project)
	for _, l := range vt.Labels {
		if pt := projectTag(l.Title); known[pt] {
			add(pt)
		} else {
			add(slug(l.Title))
		}
	}
	if vt.Done {
		add("status:closed")
		closed := vt.DoneAt
		if isZero(closed) {
			closed, t.closedGuessed = vt.Updated, true
		}
		t.ClosedAt = &closed
	} else {
		add("status:open")
	}
	switch vt.Priority {
	case 0:
	case 1:
		add("priority:low")
	case 2:
		add("priority:medium")
	default:
		add("priority:high")
	}
	t.Tags = tagSet

	if !isZero(vt.DueDate) {
		d := vt.DueDate.In(loc).Format(time.DateOnly)
		t.DueDate = &d
	}
	for _, r := range vt.Reminders {
		if isZero(r.Reminder) {
			continue
		}
		at := r.Reminder
		t.RemindAt = &at
		if at.Before(now) {
			t.RemindedAt = &at
		}
		break
	}
	t.Notes = vikunjaNotes(vt, titles)
	return t
}

// vikunjaNotes builds the notes: the description as text without the lines
// left by an earlier Clockify import, then what kala cannot represent.
func vikunjaNotes(vt vikTask, titles map[int64]string) string {
	var lines []string
	for line := range strings.SplitSeq(htmlToText(vt.Description), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "Import da Clockify") {
			lines = append(lines, line)
		}
	}
	notes := strings.TrimSpace(reBlanks.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))

	var extra []string
	if vt.RepeatMode == 1 {
		extra = append(extra, "Repeats monthly (from Vikunja)")
	} else if vt.RepeatAfter > 0 {
		extra = append(extra, "Repeats every "+repeatInterval(vt.RepeatAfter)+" (from Vikunja)")
	}
	kinds := make([]string, 0, len(vt.RelatedTasks))
	for k := range vt.RelatedTasks {
		kinds = append(kinds, k)
	}
	slices.Sort(kinds)
	for _, k := range kinds {
		for _, ref := range vt.RelatedTasks[k] {
			title := ref.Title
			if title == "" {
				title = titles[ref.ID]
			}
			if title == "" {
				title = fmt.Sprintf("#%d", ref.ID)
			}
			extra = append(extra, relationName(k)+": "+title)
		}
	}
	if len(extra) == 0 {
		return notes
	}
	if notes != "" {
		notes += "\n\n"
	}
	return notes + strings.Join(extra, "\n")
}

func repeatInterval(secs int64) string {
	d := time.Duration(secs) * time.Second
	switch {
	case d%(24*time.Hour) == 0:
		return fmt.Sprintf("%dd", d/(24*time.Hour))
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	default:
		return d.String()
	}
}

var relationNames = map[string]string{
	"subtask":     "Subtask",
	"parenttask":  "Parent task",
	"related":     "Related",
	"duplicateof": "Duplicate of",
	"duplicates":  "Duplicates",
	"blocking":    "Blocking",
	"blocked":     "Blocked by",
	"precedes":    "Precedes",
	"follows":     "Follows",
	"copiedfrom":  "Copied from",
	"copiedto":    "Copied to",
}

func relationName(kind string) string {
	if n, ok := relationNames[kind]; ok {
		return n
	}
	return kind
}

// isZero reports Go's and Vikunja's "no date" value (0001-01-01).
func isZero(t time.Time) bool { return t.Year() <= 1 }

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func readZipEntry(data []byte, name string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("vikunja: invalid zip: %w", err)
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer func() { _ = rc.Close() }()
		return io.ReadAll(io.LimitReader(rc, 256<<20))
	}
	return nil, errors.New("vikunja: zip has no " + name)
}
