# kala

A keyboard-driven task manager with composite tags and time tracking.
Go backend (single binary, SQLite) + React/Material UI frontend + MCP server.

- **Everything is a tag.** Projects, status and priority are tag namespaces:
  `project:kala:api`, `status:open`, `priority:high`, `client:acme`.
  Filtering by a tag matches its whole subtree (`project:kala` includes `project:kala:api`).
- **Namespaces** can be *exclusive* (one value per task: adding `status:closed` replaces `status:open`),
  have a *default* for new tasks, an *order* (used for sorting), and *terminal* values (the task is
  closed while it carries one). New accounts get `status` (open*, waiting, closed†), `priority`
  (high, medium, low) and `project`.
- **Due dates** are calendar days. **Reminders** are exact times. Per-weekday **working hours** drive a
  morning briefing and an end-of-day wrap-up, which warns about running timers.
- **Time tracking:** any number of timers can run at once (one per task). Closing a task stops its timer,
  and starting a timer on a closed task reopens it. Reports group by day, task, tag or any namespace.
- **Live sync** over server-sent events. Deleted tasks go to the trash for 30 days. JSON export/import.

## Run

```sh
make build          # npm ci + vite build + go build → ./kala (SPA embedded)
./kala              # http://localhost:8080 — the first user can always register
```

Development (two terminals):

```sh
make dev-api        # API on :8080, dev cookies, open signup, kala-dev.db
make dev-web        # Vite on :5173 with hot reload, proxying /api and /mcp
```

Docker:

```sh
docker build -t kala .
docker run -p 8080:8080 -v kala-data:/data kala
```

Put a TLS-terminating reverse proxy (Caddy, nginx) in front for anything beyond localhost.

### Configuration

| Variable | Default | |
|---|---|---|
| `KALA_ADDR` | `:8080` | listen address |
| `KALA_DB_PATH` | `kala.db` | SQLite file |
| `KALA_JWT_SECRET` | generated | ≥32 chars; if unset, a random secret is generated and stored in the DB |
| `KALA_ALLOW_SIGNUP` | `false` | allow registration after the first user |
| `KALA_DEV` | `false` | drop the `Secure` cookie flag for plain-HTTP development |
| `KALA_TRUST_PROXY` | `false` | use `X-Forwarded-For` for the login rate limit |

### Maintenance

```sh
./kala backup /path/to/backup.db                        # consistent copy while running
echo 'new-password' | ./kala user reset-password me@example.com   # also signs out all sessions
```

## Keyboard

Press `?` in the app for the full, context-aware list.

| | |
|---|---|
| `Ctrl/⌘ K` | command palette (actions, views, tags, task search) |
| `q` / `Q` | quick add / start a timer on a new task |
| `/` | edit the filter · `Ctrl/⌘ S` save it as a view |
| `g t` `g u` `g o` `g i` `g a` | Today, Upcoming, Overdue, Inbox, All open |
| `g p` `g m` `g x` `g ,` | jump to project, Time, Trash, Settings |
| `j` `k` `Enter` `Esc` | move, open details, close |
| `x` `w` `1`–`3` `0` | close/reopen, waiting, priority, clear priority |
| `s` `d` `r` `t` `p` `e` `Del` | timer, due, reminder, tags, project, edit title, delete |
| `Space` `V` `* a` `* n` | select, range select, all, none — actions then apply to the selection |
| `b` | open a pending briefing / wrap-up |

### Quick-add syntax

```
Write report #client:acme @kala !high due:fri remind:thu 9:00
```

`#tag`, `@project` (→ `project:…`), `!high` or `!1`–`!3`, `due:<date>`, `remind:<date> [HH:MM]`, `remind:14:30`, `remind:+30m`.
Dates: `today`, `tomorrow`, `mon`…`sun`, `+3d`, `+2w`, `nwd` (next working day), `YYYY-MM-DD`.

### Filter language

Space-separated terms, all must match; prefix a term with `-` to negate it.

```
project:kala            tag or any descendant         project:a,b     either
project:                any project                   -project:       no project (inbox)
#urgent                 tag without namespace         "free text"     title or notes
is:open|closed|running|reminder
due:today|tomorrow|overdue|week|none|any   due:<=fri   due:2026-11-01
remind:today|due        closed:today|week|>=2026-10-01
sort:priority,due,-created   (a namespace name sorts by its value order)
```

## MCP

Kala serves an MCP endpoint at `/mcp` (Streamable HTTP). Create a personal access token in
**Settings → Access tokens**, then for example:

```sh
claude mcp add --transport http kala https://kala.example.com/mcp \
  --header "Authorization: Bearer kala_pat_…"
```

Tools: `search_tasks`, `get_task`, `create_task`, `update_task`, `add_tags`, `remove_tags`, `close_task`,
`reopen_task`, `delete_task` (to trash), `restore_task`, `start_timer`, `start_timer_new_task`, `stop_timer`,
`list_active_timers`, `list_time_entries`, `delete_time_entry`, `time_report`, `list_tags`, `rename_tag`,
`list_namespaces`, `list_views`, `today_briefing`. Tokens also work as bearer tokens on the REST API.

## REST API

JSON under `/api/v1`. Errors are `{"error": {"code", "message", "details?"}}`. Authenticate with
`Authorization: Bearer <access JWT | personal access token>`. Access JWTs last 15 minutes and are
renewed with `POST /auth/refresh`, using the HttpOnly refresh cookie (rotated on every use; reusing
a token revokes the session family). See `internal/api/routes.go` for the full route list.

## Development

```sh
make test   # go test ./... and vitest
make lint   # gofmt, golangci-lint, eslint, tsc
```

Layout: `cmd/kala` (entry point), `internal/service` (all business rules, shared by REST and MCP),
`internal/{tags,filter,quickadd,dates}` (pure logic), `internal/{api,mcp,events,schedule,auth,store}`,
and `web/` (Vite + React + MUI; `web/embed.go` embeds `web/dist`).
