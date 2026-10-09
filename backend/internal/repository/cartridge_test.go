package repository

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"npms/backend/internal/database"
)

func TestCartridgeStockAndReplacementFlow(t *testing.T) {
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "cartridge_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	repo, err := NewSQLiteRepository(db.DB)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Create Device
	device := Device{ID: "device-1", DisplayName: "HP Printer", Status: "online"}
	if err := repo.CreateDevice(ctx, device); err != nil {
		t.Fatal(err)
	}

	// 2. Create Cartridge
	cart := Cartridge{
		ID:               "cart-1",
		SKUCode:          "HP-26A",
		Name:             "HP 26A Black Toner",
		CompatibleModels: "HP M404dn, M501dn",
		StockNew:         2,
		StockRefilled:    1,
		StockEmpty:       0,
	}
	if err := repo.CreateCartridge(ctx, cart); err != nil {
		t.Fatal(err)
	}

	// 3. Replace Printer Cartridge using 'new'
	err = repo.ReplacePrinterCartridge(ctx, ReplaceCartridgeParams{
		LogID:              "log-1",
		CartridgeID:        "cart-1",
		DeviceID:           "device-1",
		SourceType:         "new",
		PageCount:          12345,
		CounterQuality:     "unverified",
		CounterCollectedAt: time.Date(2026, 10, 9, 1, 2, 3, 0, time.UTC),
		Notes:              "Replaced with new toner",
	})
	if err != nil {
		t.Fatalf("failed to replace cartridge with new toner: %v", err)
	}

	// Verify stock levels: StockNew should be 1, StockEmpty should be 1
	updatedCart, err := repo.GetCartridge(ctx, "cart-1")
	if err != nil {
		t.Fatal(err)
	}
	if updatedCart.StockNew != 1 || updatedCart.StockEmpty != 1 {
		t.Fatalf("expected StockNew=1, StockEmpty=1; got StockNew=%d, StockEmpty=%d", updatedCart.StockNew, updatedCart.StockEmpty)
	}

	// 4. Refill Cartridge
	if err := repo.RefillCartridges(ctx, "log-refill-1", "cart-1", 1, "Refilled by vendor"); err != nil {
		t.Fatalf("failed to refill cartridges: %v", err)
	}

	// Verify stock levels: StockEmpty should be 0, StockRefilled should be 2 (1 original + 1 refilled)
	updatedCart, err = repo.GetCartridge(ctx, "cart-1")
	if err != nil {
		t.Fatal(err)
	}
	if updatedCart.StockEmpty != 0 || updatedCart.StockRefilled != 2 {
		t.Fatalf("expected StockEmpty=0, StockRefilled=2; got StockEmpty=%d, StockRefilled=%d", updatedCart.StockEmpty, updatedCart.StockRefilled)
	}

	// 5. Replace Cartridge using 'refilled'
	err = repo.ReplacePrinterCartridge(ctx, ReplaceCartridgeParams{
		LogID:       "log-2",
		CartridgeID: "cart-1",
		DeviceID:    "device-1",
		SourceType:  "refilled",
	})
	if err != nil {
		t.Fatalf("failed to replace with refilled toner: %v", err)
	}

	updatedCart, err = repo.GetCartridge(ctx, "cart-1")
	if err != nil {
		t.Fatal(err)
	}
	if updatedCart.StockRefilled != 1 || updatedCart.StockEmpty != 1 {
		t.Fatalf("expected StockRefilled=1, StockEmpty=1; got StockRefilled=%d, StockEmpty=%d", updatedCart.StockRefilled, updatedCart.StockEmpty)
	}

	// 6. List Logs
	logs, err := repo.ListCartridgeLogs(ctx, "cart-1", "device-1", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs for device-1, got %d", len(logs))
	}
	if logs[1].PageCount != 12345 || logs[1].CounterQuality != "unverified" {
		t.Fatalf("expected captured counter metadata, got %+v", logs[1])
	}
}

func TestCartridgeValidationAndErrors(t *testing.T) {
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "cartridge_err_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	_ = db.Migrate(ctx)

	repo, _ := NewSQLiteRepository(db.DB)

	// Validate empty ID
	if err := repo.CreateCartridge(ctx, Cartridge{SKUCode: "HP-26A", Name: "HP Toner"}); err == nil {
		t.Fatal("expected validation error for empty ID")
	}

	// Out of stock replace error
	cart := Cartridge{ID: "c1", SKUCode: "HP-1", Name: "HP", StockNew: 0, StockRefilled: 0}
	_ = repo.CreateCartridge(ctx, cart)

	err = repo.ReplacePrinterCartridge(ctx, ReplaceCartridgeParams{LogID: "l1", CartridgeID: "c1", DeviceID: "d1", SourceType: "new"})
	if err == nil || !strings.Contains(err.Error(), "out of stock") {
		t.Fatalf("expected out of stock error, got %v", err)
	}
}

func TestPrinterRefillConsumesBottleStockAndStoresCounter(t *testing.T) {
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "printer_refill_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	repo, err := NewSQLiteRepository(db.DB)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateDevice(ctx, Device{ID: "device-refill", DisplayName: "Front", Status: "online"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateCartridge(ctx, Cartridge{ID: "cart-refill", SKUCode: "BLACK", Name: "Black", StockRefillBottles: 3}); err != nil {
		t.Fatal(err)
	}
	if err := repo.RefillPrinterCartridge(ctx, RefillPrinterCartridgeParams{
		LogID: "refill-log", CartridgeID: "cart-refill", DeviceID: "device-refill", Quantity: 2,
		PageCount: 1200, CounterQuality: "unverified", CounterCollectedAt: time.Date(2026, 10, 9, 1, 2, 3, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	cartridge, err := repo.GetCartridge(ctx, "cart-refill")
	if err != nil || cartridge.StockRefillBottles != 1 {
		t.Fatalf("bottle stock = %d, err=%v", cartridge.StockRefillBottles, err)
	}
	logs, err := repo.ListCartridgeLogs(ctx, "cart-refill", "device-refill", 10, 0)
	if err != nil || len(logs) != 1 || logs[0].PageCount != 1200 || logs[0].CounterQuality != "unverified" {
		t.Fatalf("unexpected refill logs: %+v, err=%v", logs, err)
	}
}
