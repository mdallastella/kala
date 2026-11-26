package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"slices"
	"testing"
)

// TestStatusMigration upgrades a version-2 database whose user has the old
// open/waiting/closed status set and a task in status:waiting.
func TestStatusMigration(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"migrations/0001_init.sql", "migrations/0002_drop_views.sql"} {
		body, err := migrations.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, string(body)); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
	}
	for _, q := range []string{
		"PRAGMA user_version = 2",
		"INSERT INTO users (id, email, password_hash, created_at) VALUES (1, 'a@example.com', 'x', '2026-01-01')",
		"INSERT INTO namespaces (id, user_id, name, exclusive, default_value) VALUES (1, 1, 'status', 1, 'open')",
		"INSERT INTO namespaces (id, user_id, name) VALUES (2, 1, 'Client'), (3, 1, ' #Area'), (4, 1, 'Zone'), (5, 1, 'zone')",
		"INSERT INTO namespace_values VALUES (1, 'open', 0, 0), (1, 'waiting', 1, 0), (1, 'closed', 2, 1), (1, 'someday', 3, 0)",
		"INSERT INTO tags (id, user_id, name) VALUES (1, 1, 'status:waiting')",
		"INSERT INTO tasks (id, user_id, title, created_at, updated_at) VALUES (1, 1, 'T', '2026-01-01', '2026-01-01')",
		"INSERT INTO task_tags VALUES (1, 1)",
	} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, "SELECT value, terminal FROM namespace_values WHERE namespace_id = 1 ORDER BY position")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var v string
		var terminal bool
		if err := rows.Scan(&v, &terminal); err != nil {
			t.Fatal(err)
		}
		if terminal {
			v += "†"
		}
		values = append(values, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"open", "pending", "delegated", "closed†", "cancelled†", "someday"}; !slices.Equal(values, want) {
		t.Errorf("values = %v, want %v", values, want)
	}

	var tag string
	if err := db.QueryRowContext(ctx, "SELECT g.name FROM task_tags tt JOIN tags g ON g.id = tt.tag_id WHERE tt.task_id = 1").Scan(&tag); err != nil {
		t.Fatal(err)
	}
	if tag != "status:pending" {
		t.Errorf("task tag = %q, want status:pending", tag)
	}

	// Names are normalised, except where that would collide.
	nrows, err := db.QueryContext(ctx, "SELECT name FROM namespaces ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer nrows.Close()
	var names []string
	for nrows.Next() {
		var n string
		if err := nrows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	if err := nrows.Err(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"status", "client", "area", "Zone", "zone"}; !slices.Equal(names, want) {
		t.Errorf("namespace names = %q, want %q", names, want)
	}
}

// TestEndedAtMigration upgrades a version-6 database: closed_at becomes
// ended_at, and a task carrying a terminal sub-value (status:closed:won),
// which used to stay open, ends and has its timer stopped.
func TestEndedAtMigration(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	files, err := migrations.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Name() >= "0007" {
			break
		}
		body, err := migrations.ReadFile("migrations/" + f.Name())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, string(body)); err != nil {
			t.Fatalf("%s: %v", f.Name(), err)
		}
	}
	for _, q := range []string{
		"PRAGMA user_version = 6",
		"INSERT INTO users (id, email, password_hash, created_at) VALUES (1, 'a@example.com', 'x', '2026-01-01')",
		"INSERT INTO namespaces (id, user_id, name, exclusive, default_value) VALUES (1, 1, 'status', 1, 'open')",
		"INSERT INTO namespace_values VALUES (1, 'open', 0, 0), (1, 'closed', 1, 1)",
		"INSERT INTO tags (id, user_id, name) VALUES (1, 1, 'status:closed'), (2, 1, 'status:closed:won'), (3, 1, 'status:open')",
		`INSERT INTO tasks (id, user_id, title, closed_at, created_at, updated_at) VALUES
			(1, 1, 'closed', '2026-02-01T00:00:00.000Z', '2026-01-01', '2026-01-01'),
			(2, 1, 'won', NULL, '2026-01-01', '2026-01-01'),
			(3, 1, 'open', NULL, '2026-01-01', '2026-01-01')`,
		"INSERT INTO task_tags VALUES (1, 1), (2, 2), (3, 3)",
		"INSERT INTO time_entries (user_id, task_id, started_at) VALUES (1, 2, '2026-01-02T00:00:00.000Z'), (1, 3, '2026-01-02T00:00:00.000Z')",
	} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, `SELECT t.title, COALESCE(t.ended_at, ''),
		EXISTS (SELECT 1 FROM time_entries e WHERE e.task_id = t.id AND e.stopped_at IS NULL)
		FROM tasks t ORDER BY t.id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type row struct {
		title, ended string
		running      bool
	}
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.title, &r.ended, &r.running); err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].ended != "2026-02-01T00:00:00.000Z" || got[1].ended == "" || got[1].running ||
		got[2].ended != "" || !got[2].running {
		t.Fatalf("tasks = %+v", got)
	}
}
