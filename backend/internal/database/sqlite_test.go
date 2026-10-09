package database

import (
	"context"
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
	if count != 4 {
		t.Fatalf("expected four applied migrations, got %d", count)
	}
}
