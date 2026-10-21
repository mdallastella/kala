// Package filter compiles the task filter language into parameterised SQL.
//
// Terms are separated by spaces and combined with AND; a leading "-" negates a
// term. Supported terms:
//
//	project:kala         task has the tag or a descendant (project:kala:api)
//	project:a,b          either project:a or project:b
//	project:             task has any tag in the namespace
//	#urgent              tag without namespace
//	is:open|closed|running|reminder
//	due:today|tomorrow|overdue|week|none|any, due:<=fri, due:2026-11-01
//	remind:today|due|none|any
//	closed:today|week, closed:>=2026-10-01
//	sort:priority,due,-created   (namespace names sort by value order; also tracked, activity)
//	word "quoted text"   matches title or notes
//
// The compiled condition refers to the tasks table as "t".
package filter

import (
	"fmt"
	"strings"
	"time"

	"kala/internal/dates"
	"kala/internal/tags"
)

// Env carries the context needed to resolve relative dates.
type Env struct {
	Now time.Time
	Cal dates.Calendar
}

func (e Env) today() dates.Date { return dates.On(e.Now, e.Cal.Loc) }

// startOf returns the UTC timestamp string at local midnight of d.
func (e Env) startOf(d dates.Date) string { return Timestamp(d.At(0, e.Cal.Loc)) }

// Timestamp formats t the way the store saves instants, so that string
// comparison orders them correctly.
func Timestamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

// Query is a compiled filter.
type Query struct {
	Where string
	Args  []any
	Order string
	// OrderArgs are bound after Args.
	OrderArgs []any
}

// DefaultSort is used when the filter has no sort term.
const DefaultSort = "priority,due,-created"

// Compile parses q.
func Compile(q string, env Env) (*Query, error) {
	toks, err := tokenize(q)
	if err != nil {
		return nil, err
	}
	var (
		conds []string
		args  []any
		sort  = DefaultSort
	)
	for _, tok := range toks {
		neg := false
		text := tok.text
		if !tok.quoted && len(text) > 1 && text[0] == '-' {
			neg = true
			text = text[1:]
		}
		var (
			cond string
			a    []any
		)
		key, val, hasColon := strings.Cut(text, ":")
		key = strings.ToLower(key)
		switch {
		case tok.quoted:
			cond, a = textCond(text)
		case hasColon && key == "sort":
			if neg {
				return nil, fmt.Errorf("sort cannot be negated")
			}
			sort = val
			continue
		case hasColon && key == "is":
			cond, err = isCond(strings.ToLower(val))
		case hasColon && key == "due":
			cond, a, err = dueCond(strings.ToLower(val), env)
		case hasColon && key == "remind":
			cond, a, err = remindCond(strings.ToLower(val), env)
		case hasColon && key == "closed":
			cond, a, err = closedCond(strings.ToLower(val), env)
		case strings.HasPrefix(text, "#") || hasColon:
			cond, a, err = tagCond(text)
		default:
			cond, a = textCond(text)
		}
		if err != nil {
			return nil, err
		}
		if neg {
			cond = "NOT COALESCE((" + cond + "), 0)"
		}
		conds = append(conds, cond)
		args = append(args, a...)
	}
	order, oargs, err := orderBy(sort, env)
	if err != nil {
		return nil, err
	}
	where := "1"
	if len(conds) > 0 {
		where = strings.Join(conds, " AND ")
	}
	return &Query{Where: where, Args: args, Order: order, OrderArgs: oargs}, nil
}

type token struct {
	text   string
	quoted bool
}

func tokenize(q string) ([]token, error) {
	var (
		out []token
		cur strings.Builder
		inQ bool
	)
	flush := func(quoted bool) {
		if cur.Len() > 0 || quoted {
			if s := cur.String(); s != "" {
				out = append(out, token{text: s, quoted: quoted})
			}
			cur.Reset()
		}
	}
	for _, r := range q {
		switch {
		case r == '"':
			if inQ {
				flush(true)
			} else {
				flush(false)
			}
			inQ = !inQ
		case !inQ && (r == ' ' || r == '\t' || r == '\n'):
			flush(false)
		default:
			cur.WriteRune(r)
		}
	}
	if inQ {
		return nil, fmt.Errorf("unterminated quote")
	}
	flush(false)
	return out, nil
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

func textCond(s string) (string, []any) {
	p := "%" + escapeLike(s) + "%"
	return `(t.title LIKE ? ESCAPE '\' OR t.notes LIKE ? ESCAPE '\')`, []any{p, p}
}

const tagExists = `EXISTS (SELECT 1 FROM task_tags tt JOIN tags g ON g.id = tt.tag_id WHERE tt.task_id = t.id AND (%s))`

func tagCond(s string) (string, []any, error) {
	s = strings.TrimPrefix(s, "#")
	var (
		alts []string
		args []any
		ns   string
	)
	for i, alt := range strings.Split(s, ",") {
		alt = strings.ToLower(strings.TrimSpace(alt))
		if alt == "" {
			continue
		}
		if i == 0 {
			ns, _ = tags.Split(alt)
			if !strings.Contains(alt, ":") {
				ns = ""
			}
		} else if ns != "" && !strings.Contains(alt, ":") {
			alt = ns + ":" + alt
		}
		if root, ok := strings.CutSuffix(alt, ":"); ok {
			if _, err := tags.Normalize(root); err != nil {
				return "", nil, err
			}
			alts = append(alts, `g.name LIKE ? ESCAPE '\'`)
			args = append(args, escapeLike(root)+":%")
			continue
		}
		n, err := tags.Normalize(alt)
		if err != nil {
			return "", nil, err
		}
		alts = append(alts, `g.name = ? OR g.name LIKE ? ESCAPE '\'`)
		args = append(args, n, escapeLike(n)+":%")
	}
	if len(alts) == 0 {
		return "", nil, fmt.Errorf("empty tag term %q", s)
	}
	return fmt.Sprintf(tagExists, strings.Join(alts, " OR ")), args, nil
}

func isCond(v string) (string, error) {
	switch v {
	case "open":
		return "t.closed_at IS NULL", nil
	case "closed", "done":
		return "t.closed_at IS NOT NULL", nil
	case "running":
		return "EXISTS (SELECT 1 FROM time_entries e WHERE e.task_id = t.id AND e.stopped_at IS NULL)", nil
	case "reminder":
		return "(t.remind_at IS NOT NULL AND t.reminded_at IS NULL)", nil
	}
	return "", fmt.Errorf("unknown is:%s (use open, closed, running, reminder)", v)
}

// splitOp separates a comparison operator from a value.
func splitOp(v string) (op, rest string) {
	for _, o := range []string{"<=", ">=", "<", ">", "="} {
		if strings.HasPrefix(v, o) {
			return o, v[len(o):]
		}
	}
	return "", v
}

func dueCond(v string, env Env) (string, []any, error) {
	today := env.today()
	switch v {
	case "none":
		return "t.due_date IS NULL", nil, nil
	case "any":
		return "t.due_date IS NOT NULL", nil, nil
	case "overdue":
		return "t.due_date < ?", []any{today.String()}, nil
	case "week":
		return "t.due_date BETWEEN ? AND ?", []any{today.String(), today.AddDays(6).String()}, nil
	}
	op, rest := splitOp(v)
	d, err := dates.ParseRelative(rest, today, env.Cal)
	if err != nil {
		return "", nil, fmt.Errorf("due: %w", err)
	}
	if op == "" {
		op = "="
	}
	return "t.due_date " + op + " ?", []any{d.String()}, nil
}

func remindCond(v string, env Env) (string, []any, error) {
	today := env.today()
	switch v {
	case "none":
		return "t.remind_at IS NULL", nil, nil
	case "any":
		return "t.remind_at IS NOT NULL", nil, nil
	case "due":
		return "(t.remind_at <= ? AND t.reminded_at IS NULL)", []any{Timestamp(env.Now)}, nil
	case "today":
		return "(t.remind_at >= ? AND t.remind_at < ?)", []any{env.startOf(today), env.startOf(today.AddDays(1))}, nil
	}
	return "", nil, fmt.Errorf("unknown remind:%s (use today, due, none, any)", v)
}

func closedCond(v string, env Env) (string, []any, error) {
	today := env.today()
	switch v {
	case "today":
		return "t.closed_at >= ?", []any{env.startOf(today)}, nil
	case "week":
		return "t.closed_at >= ?", []any{env.startOf(today.AddDays(-6))}, nil
	}
	op, rest := splitOp(v)
	d, err := dates.ParseRelative(rest, today, env.Cal)
	if err != nil {
		return "", nil, fmt.Errorf("closed: %w", err)
	}
	start, next := env.startOf(d), env.startOf(d.AddDays(1))
	switch op {
	case "<=":
		return "t.closed_at < ?", []any{next}, nil
	case "<":
		return "t.closed_at < ?", []any{start}, nil
	case ">=":
		return "t.closed_at >= ?", []any{start}, nil
	case ">":
		return "t.closed_at >= ?", []any{next}, nil
	default:
		return "(t.closed_at >= ? AND t.closed_at < ?)", []any{start, next}, nil
	}
}

const nsPosition = `(SELECT MIN(nv.position) FROM task_tags tt
	JOIN tags g ON g.id = tt.tag_id
	JOIN namespaces n ON n.user_id = t.user_id AND n.name = ?
	JOIN namespace_values nv ON nv.namespace_id = n.id AND g.name = n.name || ':' || nv.value
	WHERE tt.task_id = t.id)`

// PriorityNS is the namespace whose order a task without a tag in it takes
// from its due date.
const PriorityNS = "priority"

// PriorityHorizon is how many days ahead a due date implies the second
// priority value.
const PriorityHorizon = 3

// DerivedPriority returns the position of the priority value implied by a due
// date (YYYY-MM-DD, empty for none): overdue or today is the first value,
// within PriorityHorizon days the second, otherwise the third.
func DerivedPriority(due string, today dates.Date) int {
	switch {
	case due == "":
		return 2
	case due <= today.String():
		return 0
	case due <= today.AddDays(PriorityHorizon).String():
		return 1
	}
	return 2
}

// derivedPriority mirrors DerivedPriority in SQL; closed tasks have none.
const derivedPriority = `CASE WHEN t.closed_at IS NOT NULL THEN NULL WHEN t.due_date IS NULL THEN 2
	WHEN t.due_date <= ? THEN 0 WHEN t.due_date <= ? THEN 1 ELSE 2 END`

// lastTracked is the start of a task's most recent time entry.
const lastTracked = `(SELECT MAX(e.started_at) FROM time_entries e WHERE e.task_id = t.id)`

var columns = map[string]string{
	"due":     "t.due_date",
	"created": "t.created_at",
	"updated": "t.updated_at",
	"closed":  "t.closed_at",
	"title":   "t.title COLLATE NOCASE",
	"remind":  "t.remind_at",
	"tracked": lastTracked,
	// activity is the later of creation and the last time entry start.
	"activity": "MAX(t.created_at, COALESCE(" + lastTracked + ", ''))",
}

func orderBy(spec string, env Env) (string, []any, error) {
	var (
		parts []string
		args  []any
	)
	for key := range strings.SplitSeq(strings.ToLower(spec), ",") {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		dir := "ASC"
		if strings.HasPrefix(key, "-") {
			dir = "DESC"
			key = key[1:]
		}
		expr, ok := columns[key]
		if !ok {
			if _, err := tags.Normalize(key); err != nil || strings.Contains(key, ":") {
				return "", nil, fmt.Errorf("cannot sort by %q", key)
			}
			// expr appears twice below, so its arguments are bound twice.
			expr = nsPosition
			args = append(args, key, key)
			if key == PriorityNS {
				expr = "COALESCE(" + nsPosition + ", " + derivedPriority + ")"
				today, horizon := env.today().String(), env.today().AddDays(PriorityHorizon).String()
				args = append(args[:len(args)-2], key, today, horizon, key, today, horizon)
			}
		}
		parts = append(parts, expr+" IS NULL, "+expr+" "+dir)
	}
	parts = append(parts, "t.id DESC")
	return strings.Join(parts, ", "), args, nil
}
