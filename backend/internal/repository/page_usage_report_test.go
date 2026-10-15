package repository

import (
	"context"
	"testing"
	"time"

	"npms/backend/internal/database"
)

func TestGetPageUsageReport(t *testing.T) {
	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}

	repo, err := NewSQLiteRepository(db.DB)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339Nano)

	// Create 2 devices in "Kế toán" and 1 scanner in "Hành chính"
	p1 := Device{ID: "p1", DisplayName: "HP M501dn", Model: "HP LaserJet Enterprise M501dn", Department: "Kế toán", Status: "online"}
	p2 := Device{ID: "p2", DisplayName: "Brother L5100DN", Model: "Brother HL-L5100DN", Department: "Kế toán", Status: "online"}
	s1 := Device{ID: "s1", DisplayName: "Brother ADS-2400N", Model: "Brother ADS-2400N", Department: "Hành chính", Status: "online"}

	for _, d := range []Device{p1, p2, s1} {
		if err := repo.CreateDevice(ctx, d); err != nil {
			t.Fatal(err)
		}
	}

	// Counter definitions
	if _, err := db.DB.Exec(`
		INSERT INTO counter_definitions(id, device_id, key, source_protocol, oid, unit, semantic_type, scope, mode)
		VALUES 
			('p1:marker_life', 'p1', 'marker_life', 'snmp', '1.3.6.1.2.1.43.10.2.1.4', 'impressions', 'marker_life', 'engine', 'walk'),
			('p2:marker_life', 'p2', 'marker_life', 'snmp', '1.3.6.1.2.1.43.10.2.1.4', 'sheets', 'marker_life', 'engine', 'walk'),
			('s1:page_count', 's1', 'page_count', 'snmp', '1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.54.2.2.1.3.3', 'pages', 'marker_life', 'engine', 'get')
	`); err != nil {
		t.Fatal(err)
	}

	// Polling runs
	if _, err := db.DB.Exec(`
		INSERT INTO polling_runs(id, device_id, job_kind, started_at, ended_at, result)
		VALUES ('r1', 'p1', 'counter', ?, ?, 'success'), ('r2', 'p2', 'counter', ?, ?, 'success'), ('r3', 's1', 'counter', ?, ?, 'success')
	`, now, now, now, now, now, now); err != nil {
		t.Fatal(err)
	}

	// Counter readings
	if _, err := db.DB.Exec(`
		INSERT INTO counter_readings(poll_run_id, counter_definition_id, raw_value, collected_at, quality)
		VALUES 
			('r1', 'p1:marker_life', 50000, ?, 'valid'),
			('r2', 'p2:marker_life', 25000, ?, 'valid'),
			('r3', 's1:page_count', 12000, ?, 'valid')
	`, now, now, now); err != nil {
		t.Fatal(err)
	}

	// Daily counter usage
	if _, err := db.DB.Exec(`
		INSERT INTO daily_counter_usage(device_id, counter_definition_id, local_date, delta, quality, computed_at)
		VALUES 
			('p1', 'p1:marker_life', '2026-10-10', 350, 'valid', ?),
			('p2', 'p2:marker_life', '2026-10-10', 150, 'valid', ?),
			('s1', 's1:page_count', '2026-10-10', 500, 'valid', ?)
	`, now, now, now); err != nil {
		t.Fatal(err)
	}

	// Generate report for 2026-10-10
	report, err := repo.GetPageUsageReport(ctx, "2026-10-01", "2026-10-15", "")
	if err != nil {
		t.Fatal(err)
	}

	if report.TotalPrinters != 3 {
		t.Fatalf("expected 3 total devices, got %d", report.TotalPrinters)
	}
	if report.TotalDepartments != 2 {
		t.Fatalf("expected 2 departments, got %d", report.TotalDepartments)
	}

	// p1 (350 impressions) + p2 (150 sheets) = 500 print pages
	if report.TotalPrintPages != 500 {
		t.Fatalf("expected 500 total print pages, got %d", report.TotalPrintPages)
	}

	// s1 (500 pages) scanner pages
	if report.TotalScanPages != 500 {
		t.Fatalf("expected 500 total scan pages, got %d", report.TotalScanPages)
	}

	// Verify department "Kế toán"
	var keToan *DepartmentPageUsage
	for i := range report.Departments {
		if report.Departments[i].Department == "Kế toán" {
			keToan = &report.Departments[i]
			break
		}
	}
	if keToan == nil {
		t.Fatal("expected department Kế toán")
	}
	if keToan.PrinterCount != 2 || keToan.TotalPrintPages != 500 || keToan.TotalScanPages != 0 {
		t.Fatalf("unexpected Kế toán stats: %+v", keToan)
	}

	// Verify scanner s1
	var scannerItem *PrinterPageUsage
	for i := range report.Printers {
		if report.Printers[i].DeviceID == "s1" {
			scannerItem = &report.Printers[i]
			break
		}
	}
	if scannerItem == nil || scannerItem.Category != "scanner" || scannerItem.NormalizedPages != 500 {
		t.Fatalf("unexpected scanner item: %+v", scannerItem)
	}
}
