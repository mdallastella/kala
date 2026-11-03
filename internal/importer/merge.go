package importer

import (
	"slices"
	"strings"
	"time"

	"kala/internal/service"
)

// Stats summarises a conversion.
type Stats struct {
	Tasks   int // tasks in the output
	Entries int // time entries in the output
	Matched int // Clockify groups attached to a Vikunja task
	Created int // Clockify groups that became a new task
}

// Convert merges a Vikunja export and a Clockify CSV (either may be nil) into
// a kala export. Clockify entries are attached to the Vikunja task with the
// same title and project; the others become new closed tasks. loc is the
// timezone of the Clockify times and of Vikunja due dates.
func Convert(vikunjaData, clockifyData []byte, loc *time.Location, now time.Time) (*service.Export, Stats, error) {
	var (
		tasks  []*task
		groups []*group
		st     Stats
		err    error
	)
	if vikunjaData != nil {
		if tasks, err = vikunja(vikunjaData, loc, now); err != nil {
			return nil, st, err
		}
	}
	if clockifyData != nil {
		if groups, err = clockify(clockifyData, loc); err != nil {
			return nil, st, err
		}
	}

	byTitle := map[string][]*task{}
	for _, t := range tasks {
		k := strings.ToLower(t.Title)
		byTitle[k] = append(byTitle[k], t)
	}
	for _, g := range groups {
		if t := match(byTitle[strings.ToLower(g.Title)], g.Project); t != nil {
			attach(t, g.Entries)
			st.Matched++
			continue
		}
		tasks = append(tasks, newClockifyTask(g))
		st.Created++
	}

	ex := &service.Export{Version: service.ExportVersion, ExportedAt: now, Tasks: make([]service.ExportTask, 0, len(tasks))}
	for _, t := range tasks {
		ex.Tasks = append(ex.Tasks, t.ExportTask)
		st.Entries += len(t.Entries)
	}
	st.Tasks = len(ex.Tasks)
	return ex, st, nil
}

// match picks, among tasks with the group's title, the most recently updated
// one in the group's project (or without project when the group has none).
func match(candidates []*task, project string) *task {
	var best *task
	for _, t := range candidates {
		if project == "" {
			if slices.ContainsFunc(t.Tags, func(tag string) bool { return strings.HasPrefix(tag, "project:") }) {
				continue
			}
		} else if !slices.Contains(t.Tags, project) {
			continue
		}
		if best == nil || t.UpdatedAt.After(best.UpdatedAt) {
			best = t
		}
	}
	return best
}

func attach(t *task, entries []service.ExportEntry) {
	t.Entries = append(t.Entries, entries...)
	slices.SortFunc(t.Entries, func(a, b service.ExportEntry) int { return a.StartedAt.Compare(b.StartedAt) })
	// Without a real completion time, the end of the last logged work is a
	// better guess than the last edit.
	if t.closedGuessed && t.ClosedAt != nil {
		last := lastStop(t.Entries)
		t.ClosedAt = &last
	}
}

func newClockifyTask(g *group) *task {
	slices.SortFunc(g.Entries, func(a, b service.ExportEntry) int { return a.StartedAt.Compare(b.StartedAt) })
	last := lastStop(g.Entries)
	t := &task{ExportTask: service.ExportTask{Entries: g.Entries}}
	t.Title = g.Title
	t.Tags = []string{}
	if g.Project != "" {
		t.Tags = append(t.Tags, g.Project)
	}
	t.Tags = append(t.Tags, "status:closed")
	t.CreatedAt, t.UpdatedAt, t.ClosedAt = g.Entries[0].StartedAt, last, &last
	return t
}

func lastStop(entries []service.ExportEntry) time.Time {
	var last time.Time
	for _, e := range entries {
		if e.StoppedAt != nil && e.StoppedAt.After(last) {
			last = *e.StoppedAt
		}
	}
	return last
}
