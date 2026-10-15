package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"npms/backend/internal/database"
)

func TestListDevicesForBackupAndFindDevice(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "test_backup_dev.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	repo, err := NewSQLiteRepository(db.DB)
	if err != nil {
		t.Fatal(err)
	}

	// Register printer
	reg := PrinterRegistration{
		Device: Device{
			ID:           "dev-backup-1",
			AssetCode:    "ASSET-001",
			DisplayName:  "Office HP Printer",
			Manufacturer: "HP",
			Model:        "LaserJet M404dn",
			Serial:       "SN123456",
			Department:   "Accounting",
			Status:       "online",
		},
		Credential: SNMPCredential{
			ID:                      "cred-backup-1",
			Version:                 "2c",
			EncryptedSecretMaterial: []byte("encrypted-secret-content"),
			SecurityMetadata:        `{"version":"2c"}`,
			CreatedAt:               time.Now().UTC(),
		},
		Endpoint: DeviceEndpoint{
			ID:           "ep-backup-1",
			DeviceID:     "dev-backup-1",
			Address:      "192.168.1.50",
			Protocol:     "snmp",
			Port:         161,
			CredentialID: "cred-backup-1",
			IsPrimary:    true,
		},
	}

	if err := repo.RegisterPrinter(ctx, reg); err != nil {
		t.Fatalf("register printer failed: %v", err)
	}

	// Test ListDevicesForBackup
	backups, err := repo.ListDevicesForBackup(ctx)
	if err != nil {
		t.Fatalf("list devices for backup: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected 1 backup item, got %d", len(backups))
	}
	item := backups[0]
	if item.Device.ID != "dev-backup-1" || item.Address != "192.168.1.50" || item.Port != 161 {
		t.Fatalf("unexpected item data: %+v", item)
	}
	if string(item.EncryptedSecretMaterial) != "encrypted-secret-content" {
		t.Fatalf("unexpected secret material: %s", string(item.EncryptedSecretMaterial))
	}

	// Test FindDeviceByAddressOrAsset by address
	foundByAddr, err := repo.FindDeviceByAddressOrAsset(ctx, "192.168.1.50", "")
	if err != nil {
		t.Fatalf("find by addr: %v", err)
	}
	if foundByAddr == nil || foundByAddr.ID != "dev-backup-1" {
		t.Fatalf("expected to find dev-backup-1 by addr, got %+v", foundByAddr)
	}

	// Test FindDeviceByAddressOrAsset by asset code
	foundByAsset, err := repo.FindDeviceByAddressOrAsset(ctx, "", "ASSET-001")
	if err != nil {
		t.Fatalf("find by asset: %v", err)
	}
	if foundByAsset == nil || foundByAsset.ID != "dev-backup-1" {
		t.Fatalf("expected to find dev-backup-1 by asset, got %+v", foundByAsset)
	}

	// Test Find not found
	notFound, err := repo.FindDeviceByAddressOrAsset(ctx, "10.0.0.99", "ASSET-NONE")
	if err != nil {
		t.Fatalf("find not found err: %v", err)
	}
	if notFound != nil {
		t.Fatalf("expected nil when not found, got %+v", notFound)
	}
}
