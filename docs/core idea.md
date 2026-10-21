# Kala

Minimalist task manager and time tracker, developed with Golang (backend) and React/TypeScript (frontend), using SQLite as the database and featuring a web-based interface. Simple email/password authentication stored in the SQL database.

## Philosophy

* A single tool to manage tasks and track time
* Organization by project, context, or label
* Clean, linear, and minimalist web-based interface

## Core Features

### Task Management

* Task entry with project or context assignment
* Task status: open, pending, delegated, closed, cancelled (closed and cancelled close the task)
* Priority implicitly tied to the due date (overdue or today: high; within 3 days: medium; otherwise: low), manually adjustable with a `priority:` tag, which takes precedence
* Properties managed as tags in the `property:value` format (e.g., `project:project1`, `status:open`)
* Support for generic category labels with fixed values, customizable by the user

### Time Tracking

* Each task can have an unlimited number of time entries
* Timer with start/stop functionality directly on the task or in the navigation bar
* Summary view of active and completed timers
* Graphical day-by-hour timeline of the hours worked in a week, with working hours shaded

### Interface

* Home dashboard: running timers, recently created or tracked tasks, and the latest open tasks
* Dense, keyboard-driven list of open tasks
* Task management via drag-and-drop (drop tasks on a project, tag or status in the sidebar to apply it)
* Calendar view (month grid) for scheduling due dates by drag-and-drop

## Also included

* Reminders, morning briefing and end-of-day wrap-up based on working hours
* Filter language, quick-add syntax and command palette
* Live sync (server-sent events), trash with 30-day retention, JSON export/import

## Development Goals

* Keep the architecture simple and extensible
* Add work statistics in a future phase (daily/weekly/monthly averages with trend charts)
* Multi-user support and integrations (REST API, MCP server, personal access tokens) already exist; extend them as needed
