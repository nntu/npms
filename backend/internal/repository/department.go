package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Department represents an organizational unit/department where devices are assigned.
type Department struct {
	ID          string    `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	DeviceCount int       `json:"device_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (d *Department) Validate() error {
	d.Code = strings.ToUpper(strings.TrimSpace(d.Code))
	d.Name = strings.TrimSpace(d.Name)
	d.Description = strings.TrimSpace(d.Description)
	if d.Code == "" {
		return errors.New("mã phòng ban không được để trống")
	}
	if d.Name == "" {
		return errors.New("tên phòng ban không được để trống")
	}
	return nil
}

func (r *SQLiteRepository) ListDepartments(ctx context.Context) ([]Department, error) {
	query := `SELECT 
		d.id, d.code, d.name, COALESCE(d.description, ''),
		(SELECT COUNT(*) FROM devices dev WHERE dev.department_id = d.id AND dev.deleted_at IS NULL) as device_count,
		d.created_at, d.updated_at
	FROM departments d
	ORDER BY d.name COLLATE NOCASE ASC, d.code ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list departments: %w", err)
	}
	defer rows.Close()

	var result []Department
	for rows.Next() {
		var dep Department
		var createdStr, updatedStr string
		if err := rows.Scan(&dep.ID, &dep.Code, &dep.Name, &dep.Description, &dep.DeviceCount, &createdStr, &updatedStr); err != nil {
			return nil, fmt.Errorf("scan department: %w", err)
		}
		dep.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdStr)
		if dep.CreatedAt.IsZero() {
			dep.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdStr)
		}
		dep.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedStr)
		if dep.UpdatedAt.IsZero() {
			dep.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedStr)
		}
		result = append(result, dep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate departments: %w", err)
	}
	if result == nil {
		result = []Department{}
	}
	return result, nil
}

func (r *SQLiteRepository) GetDepartmentByID(ctx context.Context, id string) (*Department, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("department id is required")
	}
	query := `SELECT 
		d.id, d.code, d.name, COALESCE(d.description, ''),
		(SELECT COUNT(*) FROM devices dev WHERE dev.department_id = d.id AND dev.deleted_at IS NULL) as device_count,
		d.created_at, d.updated_at
	FROM departments d
	WHERE d.id = ?`

	var dep Department
	var createdStr, updatedStr string
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&dep.ID, &dep.Code, &dep.Name, &dep.Description, &dep.DeviceCount, &createdStr, &updatedStr,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get department by id: %w", err)
	}
	dep.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdStr)
	dep.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedStr)
	return &dep, nil
}

func (r *SQLiteRepository) GetDepartmentByCode(ctx context.Context, code string) (*Department, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, errors.New("department code is required")
	}
	query := `SELECT 
		d.id, d.code, d.name, COALESCE(d.description, ''),
		(SELECT COUNT(*) FROM devices dev WHERE dev.department_id = d.id AND dev.deleted_at IS NULL) as device_count,
		d.created_at, d.updated_at
	FROM departments d
	WHERE UPPER(d.code) = ?`

	var dep Department
	var createdStr, updatedStr string
	err := r.db.QueryRowContext(ctx, query, code).Scan(
		&dep.ID, &dep.Code, &dep.Name, &dep.Description, &dep.DeviceCount, &createdStr, &updatedStr,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get department by code: %w", err)
	}
	dep.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdStr)
	dep.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedStr)
	return &dep, nil
}

func (r *SQLiteRepository) CreateDepartment(ctx context.Context, dep *Department) error {
	if dep == nil {
		return errors.New("department is required")
	}
	if err := dep.Validate(); err != nil {
		return err
	}
	if dep.ID == "" {
		dep.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if dep.CreatedAt.IsZero() {
		dep.CreatedAt = now
	}
	if dep.UpdatedAt.IsZero() {
		dep.UpdatedAt = now
	}

	query := `INSERT INTO departments (id, code, name, description, created_at, updated_at)
		VALUES (?, ?, ?, NULLIF(?, ''), ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		dep.ID, dep.Code, dep.Name, dep.Description,
		dep.CreatedAt.Format(time.RFC3339Nano), dep.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return fmt.Errorf("mã phòng ban %q đã tồn tại", dep.Code)
		}
		return fmt.Errorf("create department: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) UpdateDepartment(ctx context.Context, dep *Department) error {
	if dep == nil {
		return errors.New("department is required")
	}
	if strings.TrimSpace(dep.ID) == "" {
		return errors.New("department id is required")
	}
	if err := dep.Validate(); err != nil {
		return err
	}
	dep.UpdatedAt = time.Now().UTC()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin update department tx: %w", err)
	}
	defer tx.Rollback()

	query := `UPDATE departments 
		SET code = ?, name = ?, description = NULLIF(?, ''), updated_at = ?
		WHERE id = ?`

	res, err := tx.ExecContext(ctx, query,
		dep.Code, dep.Name, dep.Description, dep.UpdatedAt.Format(time.RFC3339Nano), dep.ID,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return fmt.Errorf("mã phòng ban %q đã tồn tại", dep.Code)
		}
		return fmt.Errorf("update department: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return errors.New("không tìm thấy phòng ban")
	}

	// Keep devices.department synchronized with updated department name
	_, err = tx.ExecContext(ctx, `UPDATE devices SET department = ?, updated_at = ? WHERE department_id = ?`,
		dep.Name, dep.UpdatedAt.Format(time.RFC3339Nano), dep.ID)
	if err != nil {
		return fmt.Errorf("sync device department name: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit update department tx: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) DeleteDepartment(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("department id is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete department tx: %w", err)
	}
	defer tx.Rollback()

	// Unlink devices from this department
	nowStr := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `UPDATE devices SET department_id = NULL, department = NULL, updated_at = ? WHERE department_id = ?`, nowStr, id); err != nil {
		return fmt.Errorf("unlink devices from department: %w", err)
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM departments WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete department: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return errors.New("không tìm thấy phòng ban")
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete department tx: %w", err)
	}
	return nil
}
