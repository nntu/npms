package database

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func TestSQLiteMigrateIsIdempotent(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "npms.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 11 {
		t.Fatalf("expected eleven applied migrations, got %d", count)
	}
	var columnName string
	if err := db.DB.QueryRowContext(ctx, "SELECT name FROM pragma_table_info('counter_definitions') WHERE name = 'capability_state'").Scan(&columnName); err != nil {
		t.Fatal(err)
	}
	if columnName != "capability_state" {
		t.Fatalf("unexpected capability state column %q", columnName)
	}
}

func TestSQLiteMigratesExistingCounterCapabilityState(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "existing.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.DB.ExecContext(ctx, `
		CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL);
		CREATE TABLE counter_definitions (id TEXT PRIMARY KEY NOT NULL, verified INTEGER NOT NULL DEFAULT 0);
		INSERT INTO counter_definitions(id, verified) VALUES ('verified', 1), ('unverified', 0);
	`); err != nil {
		t.Fatal(err)
	}
	for version := 1; version <= 10; version++ {
		if _, err := db.DB.ExecContext(ctx, `INSERT INTO schema_migrations(version, name, applied_at) VALUES (?, ?, '2026-01-01T00:00:00Z')`, version, fmt.Sprintf("legacy-%d", version)); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	rows, err := db.DB.QueryContext(ctx, `SELECT id, capability_state FROM counter_definitions ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	want := map[string]string{"unverified": "unverified", "verified": "valid"}
	for rows.Next() {
		var id, state string
		if err := rows.Scan(&id, &state); err != nil {
			t.Fatal(err)
		}
		if state != want[id] {
			t.Fatalf("counter %s state = %q, want %q", id, state, want[id])
		}
		delete(want, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(want) != 0 {
		t.Fatalf("missing migrated counters: %#v", want)
	}
}
