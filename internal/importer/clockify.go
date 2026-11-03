package importer

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"kala/internal/service"
)

// group is the set of Clockify entries logged on one (project, description).
type group struct {
	Project string // project tag, "" for none
	Title   string
	Entries []service.ExportEntry
}

// clockify parses a Clockify detailed CSV export, reading local times in loc,
// and groups its entries by project and task (or description).
func clockify(data []byte, loc *time.Location) ([]*group, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("clockify: read header: %w", err)
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	need := []string{"project", "description", "start date", "start time", "end date", "end time"}
	for _, h := range need {
		if _, ok := col[h]; !ok {
			return nil, fmt.Errorf("clockify: missing column %q (is this a detailed time entry CSV?)", h)
		}
	}
	taskCol, hasTask := col["task"]

	var records [][]string
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("clockify: %w", err)
		}
		records = append(records, rec)
	}
	field := func(rec []string, i int) string {
		if i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}

	var dates []string
	for _, rec := range records {
		dates = append(dates, field(rec, col["start date"]), field(rec, col["end date"]))
	}
	dayFirst := detectDayFirst(dates)

	var groups []*group
	byKey := map[string]*group{}
	for n, rec := range records {
		line := n + 2
		endDate, endTime := field(rec, col["end date"]), field(rec, col["end time"])
		if endDate == "" || endTime == "" {
			continue // still running when exported
		}
		started, err := parseDateTime(field(rec, col["start date"]), field(rec, col["start time"]), dayFirst, loc)
		if err != nil {
			return nil, fmt.Errorf("clockify: line %d: start: %w", line, err)
		}
		stopped, err := parseDateTime(endDate, endTime, dayFirst, loc)
		if err != nil {
			return nil, fmt.Errorf("clockify: line %d: end: %w", line, err)
		}
		if !stopped.After(started) {
			continue
		}
		title := field(rec, col["description"])
		if hasTask && field(rec, taskCol) != "" {
			title = field(rec, taskCol)
		}
		title = truncate(title, maxTitle)
		if title == "" {
			title = "(no description)"
		}
		project := projectTag(field(rec, col["project"]))
		key := project + "\x00" + strings.ToLower(title)
		g := byKey[key]
		if g == nil {
			g = &group{Project: project, Title: title}
			byKey[key] = g
			groups = append(groups, g)
		}
		g.Entries = append(g.Entries, service.ExportEntry{StartedAt: started, StoppedAt: &stopped})
	}
	return groups, nil
}

// detectDayFirst decides between DD/MM/YYYY and MM/DD/YYYY from all dates of
// the file; when nothing disambiguates it assumes day first.
func detectDayFirst(dates []string) bool {
	for _, d := range dates {
		parts := splitDate(d)
		if len(parts) != 3 || len(parts[0]) == 4 {
			continue
		}
		if a, _ := strconv.Atoi(parts[0]); a > 12 {
			return true
		}
		if b, _ := strconv.Atoi(parts[1]); b > 12 {
			return false
		}
	}
	return true
}

func splitDate(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == '/' || r == '.' || r == '-' })
}

var timeLayouts = []string{"15:04:05", "15:04", "3:04:05 PM", "3:04 PM", "3:04:05PM", "3:04PM"}

func parseDateTime(date, clock string, dayFirst bool, loc *time.Location) (time.Time, error) {
	parts := splitDate(date)
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid date %q", date)
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid date %q", date)
		}
		nums[i] = n
	}
	var y, m, d int
	switch {
	case len(parts[0]) == 4:
		y, m, d = nums[0], nums[1], nums[2]
	case dayFirst:
		d, m, y = nums[0], nums[1], nums[2]
	default:
		m, d, y = nums[0], nums[1], nums[2]
	}
	if m < 1 || m > 12 || d < 1 || d > 31 {
		return time.Time{}, fmt.Errorf("invalid date %q", date)
	}
	for _, layout := range timeLayouts {
		if c, err := time.Parse(layout, strings.ToUpper(clock)); err == nil {
			return time.Date(y, time.Month(m), d, c.Hour(), c.Minute(), c.Second(), 0, loc), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time %q", clock)
}
