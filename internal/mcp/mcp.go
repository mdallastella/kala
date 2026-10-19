// Package mcp exposes Kala to MCP clients (e.g. Claude) over Streamable HTTP.
// Every request carries a personal access token; the API layer authenticates
// it and this package builds a server bound to that user.
package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"kala/internal/auth"
	"kala/internal/service"
)

const instructions = `Kala is a task manager where projects, status and priority are all tags.

Tags are colon-separated paths: project:kala:api, status:open, priority:high, client:acme.
Namespaces such as status and priority are exclusive (one value per task); adding
status:closed replaces status:open and closes the task.

Filter language (search_tasks, time_report): space-separated terms, all must match.
  project:kala        tag or any descendant      -project:     no project tag (inbox)
  project:a,b         either                     #urgent       tag without namespace
  is:open|closed|running|reminder
  due:today|tomorrow|overdue|week|none|any  due:<=fri  due:2026-11-01
  remind:today|due    closed:today|week     "some words"  free text in title/notes
  sort:priority,due,-created
  Prefix a term with - to negate it.

Quick-add syntax (create_task.quick, start_timer_new_task.quick):
  Write report #client:acme @kala !high due:fri remind:thu 9:00
  #tag, @project, !priority (or !1-!3), due:<date>, remind:<date> [HH:MM] | remind:+30m
  Dates: today, tomorrow, mon..sun, +3d, +2w, nwd (next working day), YYYY-MM-DD.`

// Handler returns the Streamable HTTP handler. Requests must already carry
// the authenticated user in their context.
func Handler(svc *service.Service) http.Handler {
	cache := mcp.NewSchemaCache()
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		uid := auth.UserID(r.Context())
		if uid == 0 {
			return nil
		}
		srv := mcp.NewServer(&mcp.Implementation{Name: "kala", Version: "1.0.0"},
			&mcp.ServerOptions{Instructions: instructions, SchemaCache: cache})
		register(srv, &tools{svc: svc, uid: uid})
		return srv
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, DisableLocalhostProtection: true})
}

type tools struct {
	svc *service.Service
	uid int64
}

func result(v any, err error) (*mcp.CallToolResult, any, error) {
	res := &mcp.CallToolResult{}
	if err != nil {
		res.SetError(err)
		return res, nil, nil
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	res.Content = []mcp.Content{&mcp.TextContent{Text: string(b)}}
	return res, nil, nil
}

type (
	idArg struct {
		ID int64 `json:"id" jsonschema:"task id"`
	}
	searchArgs struct {
		Query string `json:"query,omitempty" jsonschema:"filter expression; empty lists everything not deleted"`
		Limit int    `json:"limit,omitempty" jsonschema:"maximum number of tasks (default 100)"`
	}
	createArgs struct {
		Quick    string   `json:"quick,omitempty" jsonschema:"quick-add line; parsed for title, tags and dates"`
		Title    string   `json:"title,omitempty" jsonschema:"title (overrides the one parsed from quick)"`
		Notes    string   `json:"notes,omitempty" jsonschema:"Markdown notes"`
		Tags     []string `json:"tags,omitempty" jsonschema:"extra tags"`
		DueDate  string   `json:"due_date,omitempty" jsonschema:"due date YYYY-MM-DD"`
		RemindAt string   `json:"remind_at,omitempty" jsonschema:"reminder instant, RFC 3339"`
	}
	updateArgs struct {
		ID       int64   `json:"id" jsonschema:"task id"`
		Title    *string `json:"title,omitempty"`
		Notes    *string `json:"notes,omitempty" jsonschema:"Markdown notes (replaces existing)"`
		DueDate  *string `json:"due_date,omitempty" jsonschema:"YYYY-MM-DD, or empty string to clear"`
		RemindAt *string `json:"remind_at,omitempty" jsonschema:"RFC 3339 instant, or empty string to clear"`
	}
	tagArgs struct {
		ID   int64    `json:"id" jsonschema:"task id"`
		Tags []string `json:"tags"`
	}
	timerArgs struct {
		TaskID int64 `json:"task_id"`
	}
	quickArgs struct {
		Quick string `json:"quick" jsonschema:"quick-add line for the new task"`
	}
	rangeArgs struct {
		From   string `json:"from,omitempty" jsonschema:"RFC 3339 start (default 7 days ago)"`
		To     string `json:"to,omitempty" jsonschema:"RFC 3339 end (default now)"`
		TaskID int64  `json:"task_id,omitempty"`
		Query  string `json:"query,omitempty" jsonschema:"filter expression applied to the entries' tasks"`
	}
	reportArgs struct {
		rangeArgs
		Group string `json:"group,omitempty" jsonschema:"day, task, tag or a namespace name such as project (default day)"`
	}
	renameArgs struct {
		From    string `json:"from"`
		To      string `json:"to"`
		Confirm bool   `json:"confirm,omitempty" jsonschema:"apply the rename; false only previews it"`
	}
	noArgs struct{}
)

func (a rangeArgs) filter(now time.Time) (service.EntryFilter, error) {
	f := service.EntryFilter{From: now.AddDate(0, 0, -7), To: now, TaskID: a.TaskID, Query: a.Query}
	var err error
	if a.From != "" {
		if f.From, err = time.Parse(time.RFC3339, a.From); err != nil {
			return f, err
		}
	}
	if a.To != "" {
		if f.To, err = time.Parse(time.RFC3339, a.To); err != nil {
			return f, err
		}
	}
	return f, nil
}

func parseInstant(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	return &t, err
}

func ro(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true}
}

func destructive(title string) *mcp.ToolAnnotations {
	yes := true
	return &mcp.ToolAnnotations{Title: title, DestructiveHint: &yes}
}

func register(srv *mcp.Server, t *tools) {
	mcp.AddTool(srv, &mcp.Tool{Name: "search_tasks", Description: "Search tasks with the filter language.", Annotations: ro("Search tasks")},
		func(ctx context.Context, _ *mcp.CallToolRequest, a searchArgs) (*mcp.CallToolResult, any, error) {
			if a.Limit == 0 {
				a.Limit = 100
			}
			return result(t.svc.Tasks(ctx, t.uid, service.ListOptions{Query: a.Query, Limit: a.Limit}))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "get_task", Description: "Get one task by id.", Annotations: ro("Get task")},
		func(ctx context.Context, _ *mcp.CallToolRequest, a idArg) (*mcp.CallToolResult, any, error) {
			return result(t.svc.Task(ctx, t.uid, a.ID))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "create_task", Description: "Create a task, preferably from a quick-add line."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a createArgs) (*mcp.CallToolResult, any, error) {
			in := service.TaskInput{Quick: a.Quick, Title: a.Title, Notes: a.Notes, Tags: a.Tags}
			if a.DueDate != "" {
				in.DueDate = &a.DueDate
			}
			ra, err := parseInstant(a.RemindAt)
			if err != nil {
				return result(nil, err)
			}
			in.RemindAt = ra
			return result(t.svc.CreateTask(ctx, t.uid, in))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "update_task", Description: "Update a task's title, notes, due date or reminder. Use add_tags/remove_tags for tags."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a updateArgs) (*mcp.CallToolResult, any, error) {
			var p service.TaskPatch
			if a.Title != nil {
				p.Title = service.Some(*a.Title)
			}
			if a.Notes != nil {
				p.Notes = service.Some(*a.Notes)
			}
			if a.DueDate != nil {
				p.DueDate = service.Optional[string]{Set: true, Null: *a.DueDate == "", Value: *a.DueDate}
			}
			if a.RemindAt != nil {
				ra, err := parseInstant(*a.RemindAt)
				if err != nil {
					return result(nil, err)
				}
				p.RemindAt = service.Optional[time.Time]{Set: true, Null: ra == nil}
				if ra != nil {
					p.RemindAt.Value = *ra
				}
			}
			return result(t.svc.UpdateTask(ctx, t.uid, a.ID, p))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "add_tags", Description: "Add tags to a task. Exclusive namespaces (status, priority) replace the previous value."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a tagArgs) (*mcp.CallToolResult, any, error) {
			return result(t.svc.UpdateTask(ctx, t.uid, a.ID, service.TaskPatch{AddTags: a.Tags}))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "remove_tags", Description: "Remove tags from a task."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a tagArgs) (*mcp.CallToolResult, any, error) {
			return result(t.svc.UpdateTask(ctx, t.uid, a.ID, service.TaskPatch{RemoveTags: a.Tags}))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "close_task", Description: "Close a task (sets its terminal status and stops its timer)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a idArg) (*mcp.CallToolResult, any, error) {
			return result(t.svc.UpdateTask(ctx, t.uid, a.ID, service.TaskPatch{Closed: service.Some(true)}))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "reopen_task", Description: "Reopen a closed task."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a idArg) (*mcp.CallToolResult, any, error) {
			return result(t.svc.UpdateTask(ctx, t.uid, a.ID, service.TaskPatch{Closed: service.Some(false)}))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "delete_task", Description: "Move a task to the trash (restorable for 30 days).", Annotations: destructive("Delete task")},
		func(ctx context.Context, _ *mcp.CallToolRequest, a idArg) (*mcp.CallToolResult, any, error) {
			return result(map[string]bool{"deleted": true}, t.svc.DeleteTask(ctx, t.uid, a.ID))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "restore_task", Description: "Restore a task from the trash."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a idArg) (*mcp.CallToolResult, any, error) {
			return result(t.svc.RestoreTask(ctx, t.uid, a.ID))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "start_timer", Description: "Start tracking time on a task (reopens it if closed). Several timers can run at once."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a timerArgs) (*mcp.CallToolResult, any, error) {
			return result(t.svc.StartTimer(ctx, t.uid, a.TaskID))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "start_timer_new_task", Description: "Create a task from a quick-add line and start its timer."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a quickArgs) (*mcp.CallToolResult, any, error) {
			return result(t.svc.StartTimerNewTask(ctx, t.uid, service.TaskInput{Quick: a.Quick}))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "stop_timer", Description: "Stop the running timer of a task."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a timerArgs) (*mcp.CallToolResult, any, error) {
			return result(t.svc.StopTimer(ctx, t.uid, a.TaskID))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "list_active_timers", Description: "List running timers.", Annotations: ro("Active timers")},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
			return result(t.svc.ActiveTimers(ctx, t.uid))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "list_time_entries", Description: "List time entries overlapping a range.", Annotations: ro("Time entries")},
		func(ctx context.Context, _ *mcp.CallToolRequest, a rangeArgs) (*mcp.CallToolResult, any, error) {
			f, err := a.filter(t.svc.Now())
			if err != nil {
				return result(nil, err)
			}
			return result(t.svc.TimeEntries(ctx, t.uid, f))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "delete_time_entry", Description: "Permanently delete a time entry.", Annotations: destructive("Delete time entry")},
		func(ctx context.Context, _ *mcp.CallToolRequest, a idArg) (*mcp.CallToolResult, any, error) {
			return result(map[string]bool{"deleted": true}, t.svc.DeleteTimeEntry(ctx, t.uid, a.ID))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "time_report", Description: "Total tracked time grouped by day, task, tag or namespace.", Annotations: ro("Time report")},
		func(ctx context.Context, _ *mcp.CallToolRequest, a reportArgs) (*mcp.CallToolResult, any, error) {
			f, err := a.filter(t.svc.Now())
			if err != nil {
				return result(nil, err)
			}
			return result(t.svc.TimeReport(ctx, t.uid, f, a.Group))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "list_tags", Description: "List all tags with open/total task counts.", Annotations: ro("List tags")},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
			return result(t.svc.Tags(ctx, t.uid))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "rename_tag", Description: "Rename a tag and its whole subtree, merging into existing tags. Preview first, then call again with confirm=true."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a renameArgs) (*mcp.CallToolResult, any, error) {
			return result(t.svc.RenameTag(ctx, t.uid, service.RenameInput{From: a.From, To: a.To, Confirm: a.Confirm}))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "list_namespaces", Description: "List namespace definitions (values, exclusivity, defaults, terminal values).", Annotations: ro("Namespaces")},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
			return result(t.svc.Namespaces(ctx, t.uid))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "list_views", Description: "List saved views (named filters).", Annotations: ro("Views")},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
			return result(t.svc.Views(ctx, t.uid))
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "today_briefing", Description: "Today's briefing (overdue, due today, reminders) and wrap-up (open due tasks, next working day, running timers).", Annotations: ro("Today")},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
			b, err := t.svc.Briefing(ctx, t.uid)
			if err != nil {
				return result(nil, err)
			}
			w, err := t.svc.Wrapup(ctx, t.uid)
			return result(map[string]any{"briefing": b, "wrapup": w}, err)
		})
}
