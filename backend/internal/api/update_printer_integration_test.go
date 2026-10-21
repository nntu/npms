package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"npms/backend/internal/database"
	"npms/backend/internal/profile"
	"npms/backend/internal/repository"
)

func TestUpdatePrinterIntegration(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_update_printer.db")
	db, err := database.OpenSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	repo, err := repository.NewSQLiteRepository(db.DB)
	if err != nil {
		t.Fatal(err)
	}

	dept := repository.Department{
		Code: "IT",
		Name: "Phòng Công nghệ thông tin",
	}
	if err := repo.CreateDepartment(ctx, &dept); err != nil {
		t.Fatal(err)
	}

	// Create device
	err = repo.CreateDevice(ctx, repository.Device{
		ID:          "p-test-1",
		DisplayName: "Test Printer 1",
		Address:     "192.168.1.50",
		Port:        161,
		Status:      "online",
	})
	if err != nil {
		t.Fatal(err)
	}

	server, err := NewServer(repo, "", "")
	if err != nil {
		t.Fatal(err)
	}

	// Load real profiles
	profs, err := profile.LoadDir("../../../profiles")
	if err == nil && len(profs) > 0 {
		server.SetProfiles(profs)
	}

	// 1. Update printer with department
	payload := `{"display_name":"Test Printer 1","department_id":"` + dept.ID + `","department":"Phòng Công nghệ thông tin","address":"192.168.1.50","port":161,"version":"2c"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/printers/p-test-1", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	dev, err := repo.GetDevice(ctx, "p-test-1")
	if err != nil {
		t.Fatal(err)
	}
	if dev.DepartmentID != dept.ID {
		t.Errorf("expected deptID %s, got %s", dept.ID, dev.DepartmentID)
	}
	if dev.Department != "Phòng Công nghệ thông tin" {
		t.Errorf("expected dept name, got %s", dev.Department)
	}

	// 2. Clear department
	clearPayload := `{"display_name":"Test Printer 1","department_id":"","department":"","address":"192.168.1.50","port":161,"version":"2c"}`
	clearReq := httptest.NewRequest(http.MethodPut, "/api/v1/printers/p-test-1", strings.NewReader(clearPayload))
	clearReq.Header.Set("Content-Type", "application/json")
	clearW := httptest.NewRecorder()
	server.Handler().ServeHTTP(clearW, clearReq)

	if clearW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", clearW.Code, clearW.Body.String())
	}

	devCleared, err := repo.GetDevice(ctx, "p-test-1")
	if err != nil {
		t.Fatal(err)
	}
	if devCleared.DepartmentID != "" || devCleared.Department != "" {
		t.Errorf("expected empty dept, got deptID=%q dept=%q", devCleared.DepartmentID, devCleared.Department)
	}
}
