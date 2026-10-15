package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Backup creates a clean, consistent snapshot of the SQLite database at destPath using VACUUM INTO.
func (s *SQLite) Backup(ctx context.Context, destPath string) error {
	if strings.TrimSpace(destPath) == "" {
		return fmt.Errorf("destination path is required")
	}
	if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove existing destination: %w", err)
	}
	if err := ensureParent(destPath); err != nil {
		return err
	}

	cleanPath := filepath.ToSlash(destPath)
	query := fmt.Sprintf("VACUUM INTO '%s'", strings.ReplaceAll(cleanPath, "'", "''"))
	if _, err := s.DB.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("vacuum into %q: %w", destPath, err)
	}
	return nil
}

// Restore restores database content from backupPath into the current database using ATTACH DATABASE.
func (s *SQLite) Restore(ctx context.Context, backupPath string) error {
	if strings.TrimSpace(backupPath) == "" {
		return fmt.Errorf("backup path is required")
	}
	if _, err := os.Stat(backupPath); err != nil {
		return fmt.Errorf("backup file does not exist: %w", err)
	}

	cleanPath := filepath.ToSlash(backupPath)
	attachQuery := fmt.Sprintf("ATTACH DATABASE '%s' AS backup_db", strings.ReplaceAll(cleanPath, "'", "''"))
	if _, err := s.DB.ExecContext(ctx, attachQuery); err != nil {
		return fmt.Errorf("attach backup database: %w", err)
	}
	defer func() {
		_, _ = s.DB.ExecContext(context.Background(), "DETACH DATABASE backup_db")
	}()

	var hasMigrations, hasDevices int
	_ = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM backup_db.sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&hasMigrations)
	_ = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM backup_db.sqlite_master WHERE type='table' AND name='devices'").Scan(&hasDevices)
	if hasMigrations == 0 && hasDevices == 0 {
		return fmt.Errorf("invalid backup database: missing schema_migrations or devices table")
	}

	rows, err := s.DB.QueryContext(ctx, "SELECT name FROM backup_db.sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return fmt.Errorf("list backup tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan table name: %w", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table names: %w", err)
	}

	if _, err := s.DB.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("disable foreign keys: %w", err)
	}
	defer func() {
		_, _ = s.DB.ExecContext(context.Background(), "PRAGMA foreign_keys = ON")
	}()

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin restore transaction: %w", err)
	}
	defer tx.Rollback()

	for _, table := range tables {
		var mainExists int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM main.sqlite_master WHERE type='table' AND name=?", table).Scan(&mainExists); err != nil {
			return fmt.Errorf("check main table %s: %w", table, err)
		}
		if mainExists == 0 {
			continue
		}

		cols, err := s.commonColumns(ctx, tx, table)
		if err != nil {
			return fmt.Errorf("find columns for %s: %w", table, err)
		}
		if len(cols) == 0 {
			continue
		}

		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM main.%s", table)); err != nil {
			return fmt.Errorf("clear main.%s: %w", table, err)
		}

		colList := strings.Join(cols, ", ")
		insertQuery := fmt.Sprintf("INSERT INTO main.%s (%s) SELECT %s FROM backup_db.%s", table, colList, colList, table)
		if _, err := tx.ExecContext(ctx, insertQuery); err != nil {
			return fmt.Errorf("restore table %s: %w", table, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit restore: %w", err)
	}

	if err := s.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate restored database: %w", err)
	}

	_, _ = s.DB.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	return nil
}

func (s *SQLite) commonColumns(ctx context.Context, tx *sql.Tx, table string) ([]string, error) {
	rowsMain, err := tx.QueryContext(ctx, fmt.Sprintf("PRAGMA main.table_info(%s)", table))
	if err != nil {
		return nil, err
	}
	defer rowsMain.Close()

	mainCols := make(map[string]bool)
	for rowsMain.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue sql.NullString
		if err := rowsMain.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return nil, err
		}
		mainCols[name] = true
	}
	if err := rowsMain.Err(); err != nil {
		return nil, err
	}

	rowsBackup, err := tx.QueryContext(ctx, fmt.Sprintf("PRAGMA backup_db.table_info(%s)", table))
	if err != nil {
		return nil, err
	}
	defer rowsBackup.Close()

	var common []string
	for rowsBackup.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue sql.NullString
		if err := rowsBackup.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return nil, err
		}
		if mainCols[name] {
			common = append(common, name)
		}
	}
	if err := rowsBackup.Err(); err != nil {
		return nil, err
	}
	return common, nil
}
