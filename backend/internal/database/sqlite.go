package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type SQLite struct {
	DB *sql.DB
}

func OpenSQLite(path string) (*SQLite, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}
	if path != ":memory:" {
		if err := ensureParent(path); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{"PRAGMA foreign_keys = ON", "PRAGMA journal_mode = WAL", "PRAGMA busy_timeout = 5000"} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("configure sqlite: %w", err)
		}
	}
	return &SQLite{DB: db}, nil
}

func (s *SQLite) Close() error { return s.DB.Close() }

func (s *SQLite) Migrate(ctx context.Context) error {
	if _, err := s.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	entries, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(entries)
	for _, entry := range entries {
		version, name, ok := parseMigrationName(filepath.Base(entry))
		if !ok {
			return fmt.Errorf("invalid migration filename %q", entry)
		}
		var exists int
		if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&exists); err != nil {
			return fmt.Errorf("check migration %d: %w", version, err)
		}
		if exists != 0 {
			continue
		}
		contents, err := migrationFiles.ReadFile(entry)
		if err != nil {
			return fmt.Errorf("read migration %q: %w", entry, err)
		}
		tx, err := s.DB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", version, err)
		}
		if _, err := tx.ExecContext(ctx, string(contents)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", version, err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, name, applied_at) VALUES (?, ?, ?)", version, name, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %d: %w", version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", version, err)
		}
	}
	return nil
}

func parseMigrationName(name string) (int, string, bool) {
	parts := strings.SplitN(name, "_", 2)
	if len(parts) != 2 || !strings.HasSuffix(parts[1], ".sql") {
		return 0, "", false
	}
	version, err := strconv.Atoi(parts[0])
	if err != nil || version < 1 {
		return 0, "", false
	}
	return version, strings.TrimSuffix(parts[1], ".sql"), true
}

func ensureParent(path string) error {
	parent := filepath.Dir(path)
	if parent == "." || parent == "" {
		return nil
	}
	// This is intentionally limited to the configured database parent.
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return fmt.Errorf("create sqlite directory: %w", err)
	}
	return nil
}
