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
}
