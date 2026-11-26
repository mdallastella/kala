# kala

A keyboard-driven task manager with composite tags and time tracking.
Go backend (single binary, SQLite) + React/Material UI frontend + MCP server.

- **Everything is a tag.** Projects, status and priority are tag namespaces:
  `project:kala:api`, `status:open`, `priority:high`, `client:acme`.
  Filtering by a tag matches its whole subtree (`project:kala` includes `project:kala:api`).
- **Namespaces** can be *exclusive* (one value per task: adding `status:closed` replaces `status:open`;
  the value is the first segment, so `client:acme` and `client:acme:ops` can coexist). An exclusive
  namespace with listed values only accepts those; with none it accepts any value. Namespaces can
  have a *default* for new tasks, an *order* (used for sorting), and *terminal* values (the task has
  *ended*, and `ended_at` is set, while it carries one or a sub-value of one, such as `status:closed:won`).
  New accounts get `status` (open*, pending, delegated, closed†,
  cancelled†), `priority` (high, medium, low) and `project`.
- **Priority** comes from the `priority:` tag or, for open tasks without one, from the due date:
  overdue or today → high, within 3 days → medium, later or none → low.
- **Due dates** are calendar days. **Reminders** are exact times; one left unacknowledged comes back
  once a day. Per-weekday **working hours** drive a
  morning briefing and an end-of-day wrap-up, which warns about running timers.
- **Time tracking:** any number of timers can run at once (one per task). Ending a task stops its timer,
  and starting a timer on an ended task reopens it. Reports group by day, task, tag or any namespace.
- **Live sync** over server-sent events. Deleted tasks go to the trash for 30 days. JSON export/import
  (format version 2; version 1 exports, which call `ended_at` `closed_at`, still import).

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
| `KALA_TRUST_PROXY` | `false` | take the client IP for the login rate limit from the last `X-Forwarded-For` entry (one reverse proxy in front) |

### Maintenance

```sh
./kala backup /path/to/backup.db                        # consistent copy while running
echo 'new-password' | ./kala user reset-password me@example.com   # also signs out all sessions
./kala healthcheck                                      # exit 0 if the server answers /healthz
```

### Migrating from Vikunja and Clockify

`kala convert` turns a Vikunja export and/or a Clockify CSV into a kala export. It runs offline; load the
result with **Settings → Import JSON**. Give both files in one run so the time entries land on the right tasks.

```sh
./kala convert --vikunja vikunja-export.zip --clockify clockify.csv --tz Europe/Rome > kala.json
```

- **Vikunja** (the export `.zip` or its `data.json`): each project becomes `project:<name>`, using the
  project's own name and not its parents' ("Inbox" gives no project). Archived projects are included.
  - Labels that name a project become an extra `project:` tag. Other labels become plain tags.
  - Done tasks get `status:closed`. Priority maps 1 → low, 2 → medium, 3–5 → high.
  - HTML descriptions become text. Repeats and task relations are written into the notes.
- **Clockify** (detailed time entry CSV): entries are grouped by project and task (or description).
  - A group attaches to the Vikunja task with the same title and project. Otherwise it becomes a new closed task.
  - Client and tags are ignored.
  - Times are read in `--tz` (default: the machine's timezone). DD/MM or MM/DD dates are detected from the file.

## Keyboard

Press `?` in the app for the full, context-aware list.

| | |
|---|---|
| `Ctrl/⌘ K` | command palette (actions, tags, task search) |
| `q` / `Q` | quick add / start a timer on a new task |
| `/` | edit the filter |
| `g h` `g t` `g p` `g c` `g m` `g x` `g ,` | go to Home, Tasks, project, Calendar, Time, Trash, Settings |
| `j` `k` `Enter` `Esc` | move, open details, close |
| `x` `w` `D` `c` | close/reopen, pending, delegated, cancel |
| `1`–`3` `0` | priority, clear priority (back to the due-date priority) |
| `s` `d` `r` `t` `p` `e` `Del` | timer, due, reminder, tags, project, edit title, delete |
| `Space` `V` `* a` `* n` | select, range select, all, none — actions then apply to the selection |
| `b` | open a pending briefing / wrap-up |

### Quick-add syntax

```
Write report #client:acme @kala !high due:fri remind:thu 9:00
```

`#tag`, `@project` (→ `project:…`), `!high` or `!1`, `!2`… (the priority values in their configured order, like the
`1`–`3` keys), `due:<date>`, `remind:<date> [HH:MM]` (a date alone means the start of that working day),
`remind:14:30` (today, or tomorrow if already past), `remind:+30m`, `remind:+2h`.
`spent:<duration> [HH:MM]` logs time already spent on the new task (`45m`, `2h`, `1h30m`, up to 24h), ending now
or starting at `HH:MM` today: `Call with Bob @acme spent:45m 14:00`. It can't be combined with starting a timer.
`due:`, `remind:` and `spent:` may each appear once. Tags are checked against the namespaces while you type.
Dates: `today`, `tomorrow`, `yesterday`, `mon`…`sun` (today or the next one), `+3d`, `+2w`, `nwd` (next working day),
`YYYY-MM-DD`. Reminders are not checked against the clock: `remind:today 8:00` at 10:00 fires right away.

In quick add, `Enter` adds the task and `Shift+Enter` adds it and starts a timer (`Q` opens quick add in timer
mode). `Tab` completes `#tags` (including unused namespace values such as `#status:delegated`), `@projects` and
`!priorities`.

Editing a task's title speaks the same syntax: `Fix login #bug !high due:fri` renames the task to "Fix login",
adds the tags and sets the due date. Tags are only added; remove them from the tag list. `spent:` only works
when adding a task.

### Filter language

Space-separated terms, all must match; prefix a term with `-` to negate it.

```
project:kala            tag or any descendant         project:a,b     either
project:                any project                   -project:       no project (inbox)
#urgent                 tag without namespace         "free text"     title or notes
status:pending          tasks with that status        -status:closed,cancelled   open tasks
timer:running           a timer is running            remind:pending  reminder not acknowledged yet
due:today|tomorrow|overdue|week|none|any   due:<=fri   due:2026-11-01
remind:today|due        ended:today|week|>=2026-10-01 (when the task got a terminal value)
sort:priority,due,-created   (a namespace name sorts by its value order; also ended, tracked, activity)
```

Status is a tag, so there is no `is:` term: open tasks are the ones without a terminal value. The server
builds that filter from your namespaces (`GET /api/v1/filters/open`, `-status:closed,cancelled` with the
defaults). The task list, sidebar, calendar and extension use it as their default, so they follow your
terminal values when you change them in Settings. Clicking a tag in a namespace with terminal values
(`status:…`) shows exactly that tag; other tags show their open tasks. `is:…`, `closed:` and `sort:closed`
are rejected with a hint (`closed:` is now `ended:`).

## MCP

Kala serves an MCP endpoint at `/mcp` (Streamable HTTP). Create a personal access token in
**Settings → Access tokens & MCP**, then for example:

```sh
claude mcp add --transport http kala https://kala.example.com/mcp \
  --header "Authorization: Bearer kala_pat_…"
```

Tools: `search_tasks`, `get_task`, `create_task`, `update_task`, `add_tags`, `remove_tags`, `close_task`,
`reopen_task`, `delete_task` (to trash), `restore_task`, `start_timer`, `start_timer_new_task`, `stop_timer`,
`list_active_timers`, `list_time_entries`, `delete_time_entry`, `time_report`, `list_tags`, `rename_tag`,
`list_namespaces`, `today_briefing`. `search_tasks` takes `open_only: true` to list tasks that have not ended.
Tokens also work as bearer tokens on the REST API.

## Chrome extension

`extension/` adds a toolbar popup for quick add: press `Alt+K` (or click the kala icon), type a
task in the usual quick-add syntax (with preview and `#`/`@` completion), then `Enter` to add it or
`Shift+Enter` to add it and start a timer on it. `Esc` cancels.

1. `make ext`, then load `extension/dist` unpacked from `chrome://extensions` (Developer mode).
2. In kala, create a token under **Settings → Access tokens & MCP**.
3. On the extension's settings page, enter the server URL and the token and allow access to the
   server when Chrome asks.

Change the shortcut at `chrome://extensions/shortcuts` ("Activate the extension").

### Releasing to the Chrome Web Store

Bump `version` in `extension/public/manifest.json` and `extension/package.json`, commit, then push a
matching tag: `git tag ext-vX.Y.Z && git push origin ext-vX.Y.Z`. The `extension` workflow checks the
versions, runs lint/typecheck/tests, builds, uploads the ZIP and submits it for review. It goes live
once Google approves it.

One-time setup (the API cannot create an item, and auth is keyless through Workload Identity
Federation):

1. Upload the first version by hand in the [Developer Dashboard](https://chrome.google.com/webstore/devconsole)
   (`make ext`, then `cd extension/dist && zip -r ../kala-extension.zip .`), and fill in the Store
   listing and Privacy tabs (justify `optional_host_permissions`: access to the user's own kala server).
   Note the item ID and the publisher ID (**Publisher → Settings**).
2. In Google Cloud, enable the API, create a service account, and let this repo's GitHub OIDC tokens
   impersonate it:

   ```sh
   P=my-gcp-project; REPO=mdallastella/kala
   gcloud services enable chromewebstore.googleapis.com iamcredentials.googleapis.com --project $P
   gcloud iam service-accounts create cws-publisher --project $P
   gcloud iam workload-identity-pools create github --location global --project $P
   gcloud iam workload-identity-pools providers create-oidc github --project $P \
     --location global --workload-identity-pool github \
     --issuer-uri https://token.actions.githubusercontent.com \
     --attribute-mapping google.subject=assertion.sub,attribute.repository=assertion.repository \
     --attribute-condition "assertion.repository == '$REPO'"
   POOL=$(gcloud iam workload-identity-pools describe github --location global --project $P --format 'value(name)')
   gcloud iam service-accounts add-iam-policy-binding cws-publisher@$P.iam.gserviceaccount.com --project $P \
     --role roles/iam.workloadIdentityUser --member "principalSet://iam.googleapis.com/$POOL/attribute.repository/$REPO"
   gcloud iam workload-identity-pools providers describe github --location global \
     --workload-identity-pool github --project $P --format 'value(name)'   # GCP_WIF_PROVIDER
   ```

3. In the Developer Dashboard, under **Account**, add the service account's email.
4. Set the repository variables (Settings → Secrets and variables → Actions → Variables):
   `CWS_PUBLISHER_ID`, `CWS_ITEM_ID`, `GCP_WIF_PROVIDER`,
   `GCP_SERVICE_ACCOUNT` (`cws-publisher@<project>.iam.gserviceaccount.com`).

If you later change the item's visibility in the dashboard, publish once by hand before the API can
publish again.

## REST API

JSON under `/api/v1`. `POST /tasks` and `PATCH /tasks/{id}` take a `quick` line alongside the explicit fields
(explicit fields win); clear `due_date` / `remind_at` with `null` (other fields reject it). `POST /tasks/bulk` ops: `add_tags`,
`remove_tags`, `set_due`, `set_remind`, `close`, `reopen`, `delete`, `restore`. `GET /filters/open` returns
`{"query": "…"}`, the filter for open tasks. Errors are `{"error": {"code", "message", "details?"}}`. Authenticate with
`Authorization: Bearer <access JWT | personal access token>`. Access JWTs last 15 minutes and are
renewed with `POST /auth/refresh`, using the HttpOnly refresh cookie (rotated on every use; reusing
a token more than a few seconds after its rotation signs the user out of every session). See `internal/api/routes.go` for the full route list.

## Development

```sh
make test   # go test ./... and vitest
make lint   # gofmt, golangci-lint, eslint, tsc
```

CI runs the same checks for the server and the web app (`checks` job) and for the extension; only the
Trivy gate below blocks the image.

Layout: `cmd/kala` (entry point), `internal/service` (all business rules, shared by REST and MCP),
`internal/{tags,filter,quickadd,dates}` (pure logic), `internal/{api,mcp,events,schedule,auth,store}`,
`web/` (Vite + React + MUI; `web/embed.go` embeds `web/dist`), and `extension/` (Chrome extension,
Vite + Preact).

### CI security gate

The `docker` workflow runs [Trivy](https://trivy.dev) on every build: a repo scan (dependencies,
Dockerfile misconfigurations, secrets) and a scan of the built image (OS packages and the Go binary).
Fixable CRITICAL/HIGH findings fail the job, so the image is not pushed. Full reports go to the
repository's Security tab. Accepted findings go in `.trivyignore` at the repo root, with a comment
explaining why.

Preview locally before pushing:

```sh
trivy fs --scanners vuln,misconfig,secret --severity CRITICAL,HIGH --ignore-unfixed --exit-code 1 .
docker build -t kala:scan . && trivy image --severity CRITICAL,HIGH --ignore-unfixed --exit-code 1 kala:scan
```
