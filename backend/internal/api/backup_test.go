package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"npms/backend/internal/database"
	"npms/backend/internal/repository"
	"npms/backend/internal/security"
)

type fakeBackupStore struct {
	fakeStore
	backupRecords []repository.DeviceBackupRecord
	registered    []repository.PrinterRegistration
	updated       []repository.PrinterUpdate
}

func (f *fakeBackupStore) ListDevicesForBackup(ctx context.Context) ([]repository.DeviceBackupRecord, error) {
	return f.backupRecords, nil
}

func (f *fakeBackupStore) FindDeviceByAddressOrAsset(ctx context.Context, address, assetCode string) (*repository.Device, error) {
	for _, rec := range f.backupRecords {
		if (address != "" && rec.Address == address) || (assetCode != "" && rec.Device.AssetCode == assetCode) {
			d := rec.Device
			return &d, nil
		}
	}
	return nil, nil
}

func (f *fakeBackupStore) RegisterPrinter(ctx context.Context, reg repository.PrinterRegistration) error {
	f.registered = append(f.registered, reg)
	return nil
}

func (f *fakeBackupStore) UpdatePrinter(ctx context.Context, update repository.PrinterUpdate) error {
	f.updated = append(f.updated, update)
	return nil
}

func (f *fakeBackupStore) SoftDeleteDevice(ctx context.Context, id string) error {
	return nil
}

type fakeBackupManager struct {
	backedUpTo   string
	restoredFrom string
	shouldFail   bool
}

func (f *fakeBackupManager) Backup(ctx context.Context, destPath string) error {
	if f.shouldFail {
		return fmt.Errorf("fake backup failure")
	}
	f.backedUpTo = destPath
	header := append([]byte("SQLite format 3\x00"), make([]byte, 100)...)
	return os.WriteFile(destPath, header, 0o600)
}

func (f *fakeBackupManager) Restore(ctx context.Context, backupPath string) error {
	if f.shouldFail {
		return fmt.Errorf("fake restore failure")
	}
	f.restoredFrom = backupPath
	return nil
}

func TestPrintersBackupAndRestoreHandlers(t *testing.T) {
	key := make([]byte, 32)
	box, _ := security.NewSecretBox(key)

	secretBytes, _ := security.EncodeSNMPSecret(security.SNMPSecret{
		Version:   "2c",
		Community: "secret-community",
	})
	encryptedSecret, _ := box.Encrypt(secretBytes)

	store := &fakeBackupStore{
		backupRecords: []repository.DeviceBackupRecord{
			{
				Device: repository.Device{
					ID:           "dev-1",
					DisplayName:  "Office HP",
					AssetCode:    "AST-001",
					Manufacturer: "HP",
					Model:        "M404dn",
				},
				Address:                 "192.168.1.100",
				Port:                    161,
				SNMPVersion:             "2c",
				ProfileKey:              "generic-printer-mib",
				EncryptedSecretMaterial: encryptedSecret,
			},
		},
	}

	server, err := NewServer(store, "", "")
	if err != nil {
		t.Fatal(err)
	}
	server.SetSecretBox(box)

	// 1. Test handlePrintersBackup
	req := httptest.NewRequest(http.MethodGet, "/api/v1/backup/printers", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var export PrintersBackupExport
	if err := json.Unmarshal(w.Body.Bytes(), &export); err != nil {
		t.Fatalf("unmarshal export: %v", err)
	}
	if export.Count != 1 || len(export.Printers) != 1 {
		t.Fatalf("expected 1 printer, got %d", export.Count)
	}
	if export.Printers[0].Community != "secret-community" {
		t.Fatalf("expected secret-community, got %q", export.Printers[0].Community)
	}

	// 2. Test handlePrintersRestore (Update existing + Create new)
	restorePayload := PrintersRestoreRequest{
		Overwrite: true,
		Printers: []PrinterBackupItem{
			{
				DisplayName: "Updated HP",
				AssetCode:   "AST-001",
				Address:     "192.168.1.100",
				Port:        161,
				Version:     "2c",
				Community:   "new-community",
			},
			{
				DisplayName:  "New Brother Printer",
				AssetCode:    "AST-002",
				Address:      "192.168.1.101",
				Port:         161,
				Version:      "2c",
				Community:    "public",
				Manufacturer: "Brother",
				Model:        "HL-L5100DN",
			},
		},
	}
	bodyBytes, _ := json.Marshal(restorePayload)
	restoreReq := httptest.NewRequest(http.MethodPost, "/api/v1/backup/printers/restore", bytes.NewReader(bodyBytes))
	restoreReq.Header.Set("Content-Type", "application/json")
	restoreW := httptest.NewRecorder()
	server.Handler().ServeHTTP(restoreW, restoreReq)

	if restoreW.Code != http.StatusOK {
		t.Fatalf("expected 200 for restore, got %d: %s", restoreW.Code, restoreW.Body.String())
	}

	var restoreResp PrintersRestoreResponse
	if err := json.Unmarshal(restoreW.Body.Bytes(), &restoreResp); err != nil {
		t.Fatalf("unmarshal restore resp: %v", err)
	}
	if restoreResp.Total != 2 {
		t.Fatalf("expected total 2, got %d", restoreResp.Total)
	}
	if restoreResp.Updated != 1 {
		t.Fatalf("expected 1 updated, got %d", restoreResp.Updated)
	}
	if restoreResp.Created != 1 {
		t.Fatalf("expected 1 created, got %d", restoreResp.Created)
	}
}

func TestDatabaseBackupAndRestoreHandlers(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := database.OpenSQLite(filepath.Join(dir, "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	// Insert sample device
	_, err = db.DB.ExecContext(ctx, "INSERT INTO devices(id, display_name, status, created_at, updated_at) VALUES ('d-test', 'Test DB Dev', 'online', '2026-01-01', '2026-01-01')")
	if err != nil {
		t.Fatal(err)
	}

	repo, err := repository.NewSQLiteRepository(db.DB)
	if err != nil {
		t.Fatal(err)
	}

	server, err := NewServer(repo, "", "")
	if err != nil {
		t.Fatal(err)
	}
	server.SetDatabaseBackupManager(db)

	// 1. Test Backup Database endpoint
	backupReq := httptest.NewRequest(http.MethodGet, "/api/v1/backup/database", nil)
	backupW := httptest.NewRecorder()
	server.Handler().ServeHTTP(backupW, backupReq)

	if backupW.Code != http.StatusOK {
		t.Fatalf("expected 200 for db backup, got %d: %s", backupW.Code, backupW.Body.String())
	}
	backupData := backupW.Body.Bytes()
	if len(backupData) < 100 {
		t.Fatalf("db backup response is too small (%d bytes)", len(backupData))
	}
	if string(backupData[:16]) != "SQLite format 3\x00" {
		t.Fatalf("invalid sqlite header in backup")
	}

	// 2. Test Restore Database endpoint via multipart upload
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, err := w.CreateFormFile("file", "backup.db")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(part, bytes.NewReader(backupData)); err != nil {
		t.Fatal(err)
	}
	w.Close()

	restoreReq := httptest.NewRequest(http.MethodPost, "/api/v1/backup/database/restore", &b)
	restoreReq.Header.Set("Content-Type", w.FormDataContentType())
	restoreW := httptest.NewRecorder()
	server.Handler().ServeHTTP(restoreW, restoreReq)

	if restoreW.Code != http.StatusOK {
		t.Fatalf("expected 200 for db restore, got %d: %s", restoreW.Code, restoreW.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(restoreW.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "success" {
		t.Fatalf("expected success status, got %+v", resp)
	}
}
