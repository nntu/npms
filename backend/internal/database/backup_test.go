package database

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSQLiteBackupAndRestore(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	db1Path := filepath.Join(dir, "origin.db")
	db1, err := OpenSQLite(db1Path)
	if err != nil {
		t.Fatal(err)
	}
	defer db1.Close()

	if err := db1.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	// Insert test device
	_, err = db1.DB.ExecContext(ctx, `INSERT INTO devices(id, display_name, status, created_at, updated_at) 
		VALUES ('dev-1', 'Printer Test 1', 'online', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}

	// Backup db1 to backup.db
	backupPath := filepath.Join(dir, "backup.db")
	if err := db1.Backup(ctx, backupPath); err != nil {
		t.Fatalf("backup failed: %v", err)
	}

	// Create db2
	db2Path := filepath.Join(dir, "dest.db")
	db2, err := OpenSQLite(db2Path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()

	if err := db2.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	// Initially db2 has 0 devices
	var count int
	if err := db2.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM devices").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 devices in dest db, got %d", count)
	}

	// Restore db2 from backup.db
	if err := db2.Restore(ctx, backupPath); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	// Check that db2 now has 1 device with display_name 'Printer Test 1'
	var name string
	if err := db2.DB.QueryRowContext(ctx, "SELECT display_name FROM devices WHERE id = 'dev-1'").Scan(&name); err != nil {
		t.Fatalf("query restored device: %v", err)
	}
	if name != "Printer Test 1" {
		t.Fatalf("expected 'Printer Test 1', got %q", name)
	}
}
