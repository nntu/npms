package repository_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"npms/backend/internal/database"
	"npms/backend/internal/repository"
)

func setupTestDB(t *testing.T) *repository.SQLiteRepository {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo, err := repository.NewSQLiteRepository(db.DB)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	return repo
}

func TestDepartmentCRUDAndSync(t *testing.T) {
	ctx := context.Background()
	repo := setupTestDB(t)

	// 1. Create department
	dept := &repository.Department{
		Code:        "kt",
		Name:        "Phòng Kế toán",
		Description: "Tầng 2, Tòa nhà A",
	}
	if err := repo.CreateDepartment(ctx, dept); err != nil {
		t.Fatalf("create department: %v", err)
	}
	if dept.ID == "" {
		t.Fatal("expected non-empty department ID")
	}
	if dept.Code != "KT" {
		t.Fatalf("expected uppercase code 'KT', got %q", dept.Code)
	}

	// 2. Duplicate code should fail
	dupDept := &repository.Department{
		Code: "kt",
		Name: "Kế toán chi nhánh",
	}
	if err := repo.CreateDepartment(ctx, dupDept); err == nil {
		t.Fatal("expected error on duplicate department code, got nil")
	}

	// 3. Get by ID and Code
	byID, err := repo.GetDepartmentByID(ctx, dept.ID)
	if err != nil || byID == nil {
		t.Fatalf("get by id: %v", err)
	}
	if byID.Name != "Phòng Kế toán" {
		t.Fatalf("expected name 'Phòng Kế toán', got %q", byID.Name)
	}

	byCode, err := repo.GetDepartmentByCode(ctx, "kt")
	if err != nil || byCode == nil {
		t.Fatalf("get by code: %v", err)
	}
	if byCode.ID != dept.ID {
		t.Fatalf("expected ID %s, got %s", dept.ID, byCode.ID)
	}

	// 4. Create device linked to department
	dev := repository.Device{
		ID:           "dev-1",
		DisplayName:  "Máy in HP Kế toán",
		DepartmentID: dept.ID,
		Status:       "online",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := repo.CreateDevice(ctx, dev); err != nil {
		t.Fatalf("create device: %v", err)
	}

	// 5. Verify device has department synced
	savedDev, err := repo.GetDevice(ctx, "dev-1")
	if err != nil {
		t.Fatalf("get device: %v", err)
	}
	if savedDev.DepartmentID != dept.ID {
		t.Fatalf("expected department ID %s, got %s", dept.ID, savedDev.DepartmentID)
	}
	if savedDev.Department != "Phòng Kế toán" {
		t.Fatalf("expected department name 'Phòng Kế toán', got %q", savedDev.Department)
	}

	// 6. List departments with device count
	list, err := repo.ListDepartments(ctx)
	if err != nil {
		t.Fatalf("list departments: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 department, got %d", len(list))
	}
	if list[0].DeviceCount != 1 {
		t.Fatalf("expected 1 device in department, got %d", list[0].DeviceCount)
	}

	// 7. Update department name and verify device sync
	dept.Name = "Phòng Kế toán - Tài chính"
	if err := repo.UpdateDepartment(ctx, dept); err != nil {
		t.Fatalf("update department: %v", err)
	}

	savedDev2, err := repo.GetDevice(ctx, "dev-1")
	if err != nil {
		t.Fatalf("get device after dept update: %v", err)
	}
	if savedDev2.Department != "Phòng Kế toán - Tài chính" {
		t.Fatalf("expected synced department name 'Phòng Kế toán - Tài chính', got %q", savedDev2.Department)
	}

	// 8. Delete department unlinks device
	if err := repo.DeleteDepartment(ctx, dept.ID); err != nil {
		t.Fatalf("delete department: %v", err)
	}
	savedDev3, err := repo.GetDevice(ctx, "dev-1")
	if err != nil {
		t.Fatalf("get device after dept delete: %v", err)
	}
	if savedDev3.DepartmentID != "" || savedDev3.Department != "" {
		t.Fatalf("expected unlinked device department, got id=%q name=%q", savedDev3.DepartmentID, savedDev3.Department)
	}
}
