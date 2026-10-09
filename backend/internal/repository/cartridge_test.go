package repository

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

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
		LogID:       "log-1",
		CartridgeID: "cart-1",
		DeviceID:    "device-1",
		SourceType:  "new",
		Notes:       "Replaced with new toner",
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
