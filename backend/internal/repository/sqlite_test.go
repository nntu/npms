package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"npms/backend/internal/counter"
	"npms/backend/internal/database"
)

func TestDeviceValidation(t *testing.T) {
	if err := (Device{DisplayName: "Printer", Status: "unknown"}).Validate(); err == nil {
		t.Fatal("expected missing id error")
	}
	if err := (Device{ID: "device-1", Status: "unknown"}).Validate(); err == nil {
		t.Fatal("expected missing display name error")
	}
	if err := (Device{ID: "device-1", DisplayName: "Printer", Status: "unknown"}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryQualityValidation(t *testing.T) {
	for _, quality := range []counter.Quality{counter.QualityValid, counter.QualityUnverified, counter.QualityUnsupported, counter.QualityUnavailable, counter.QualitySuspicious} {
		if !validQuality(quality) {
			t.Fatalf("quality should be accepted: %q", quality)
		}
	}
}

func TestDeviceEndpointValidation(t *testing.T) {
	valid := DeviceEndpoint{ID: "endpoint-1", DeviceID: "device-1", Address: "192.168.1.20", Protocol: "snmp", Port: 161}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []DeviceEndpoint{
		{ID: "endpoint-1", DeviceID: "device-1", Address: "192.168.1.20", Protocol: "http", Port: 161},
		{ID: "endpoint-1", DeviceID: "device-1", Address: "192.168.1.20", Protocol: "snmp"},
	} {
		if err := endpoint.Validate(); err == nil {
			t.Fatalf("expected invalid endpoint: %#v", endpoint)
		}
	}
}

func TestRegisterPrinterIsAtomic(t *testing.T) {
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "npms.db"))
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
	valid := func(deviceID, credentialID, endpointID string) PrinterRegistration {
		return PrinterRegistration{
			Device:     Device{ID: deviceID, DisplayName: deviceID, Status: "unknown"},
			Credential: SNMPCredential{ID: credentialID, Version: "2c", EncryptedSecretMaterial: []byte("ciphertext")},
			Endpoint:   DeviceEndpoint{ID: endpointID, DeviceID: deviceID, Address: "192.168.1.20", Protocol: "snmp", Port: 161, CredentialID: credentialID, IsPrimary: true},
		}
	}
	if err := repo.RegisterPrinter(ctx, valid("device-1", "credential-1", "endpoint-1")); err != nil {
		t.Fatal(err)
	}
	if err := repo.RegisterPrinter(ctx, valid("device-2", "credential-2", "endpoint-1")); err == nil {
		t.Fatal("expected duplicate endpoint error")
	}
	for table, id := range map[string]string{"devices": "device-2", "snmp_credentials": "credential-2"} {
		var count int
		if err := db.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE id = ?", id).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("transaction left %s row behind", table)
		}
	}
}

func TestRegisterPrinterPersistsProfileAssignmentAndCounterMetadata(t *testing.T) {
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "npms.db"))
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
	registration := PrinterRegistration{
		Device:     Device{ID: "device-profile", DisplayName: "Profile printer", Status: "unknown"},
		Credential: SNMPCredential{ID: "credential-profile", Version: "2c", EncryptedSecretMaterial: []byte("ciphertext")},
		Endpoint:   DeviceEndpoint{ID: "endpoint-profile", DeviceID: "device-profile", Address: "192.0.2.10", Protocol: "snmp", Port: 161, CredentialID: "credential-profile", IsPrimary: true},
		Profile:    &SNMPProfile{ID: "generic-printer-mib@1", ProfileKey: "generic-printer-mib", Version: 1, SchemaVersion: 1, Content: "profile", Checksum: "checksum", VerificationStatus: "unverified"},
		CounterDefinitions: []CounterDefinition{{
			ID: "device-profile:marker_life", DeviceID: "device-profile", Key: "marker_life", SourceProtocol: "snmp", OID: "1.2.3.4", Unit: "impressions", SemanticType: "marker_life", Scope: "engine", Mode: "walk", UnitOID: "1.2.3.5", Selection: "validated_marker_rows", Aggregation: "none", RequireUnitValidation: true,
		}},
	}
	if err := repo.RegisterPrinter(ctx, registration); err != nil {
		t.Fatal(err)
	}
	var assignedProfile, mode, unitOID string
	if err := db.DB.QueryRowContext(ctx, `SELECT profile_id FROM device_profile_assignments WHERE device_id = 'device-profile'`).Scan(&assignedProfile); err != nil {
		t.Fatal(err)
	}
	if assignedProfile != "generic-printer-mib@1" {
		t.Fatalf("assigned profile = %q", assignedProfile)
	}
	if err := db.DB.QueryRowContext(ctx, `SELECT mode, unit_oid FROM counter_definitions WHERE id = 'device-profile:marker_life'`).Scan(&mode, &unitOID); err != nil {
		t.Fatal(err)
	}
	if mode != "walk" || unitOID != "1.2.3.5" {
		t.Fatalf("counter metadata = mode=%q unit_oid=%q", mode, unitOID)
	}
}

func TestPollLeasePreventsConcurrentOwnersAndAllowsExpiry(t *testing.T) {
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "npms.db"))
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
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO devices(id, display_name, status, created_at, updated_at) VALUES ('device-lease', 'Lease printer', 'unknown', ?, ?)`, time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	first, err := repo.AcquirePollLease(ctx, "device-lease", "status", "owner-1", start, start.Add(time.Minute))
	if err != nil || !first {
		t.Fatalf("first lease = %v, %v", first, err)
	}
	second, err := repo.AcquirePollLease(ctx, "device-lease", "status", "owner-2", start.Add(10*time.Second), start.Add(2*time.Minute))
	if err != nil || second {
		t.Fatalf("concurrent lease = %v, %v", second, err)
	}
	expired, err := repo.AcquirePollLease(ctx, "device-lease", "status", "owner-2", start.Add(2*time.Minute), start.Add(3*time.Minute))
	if err != nil || !expired {
		t.Fatalf("expired lease takeover = %v, %v", expired, err)
	}
	if err := repo.ReleasePollLease(ctx, "device-lease", "status", "owner-1"); err == nil {
		t.Fatal("old owner should not release the new lease")
	}
	if err := repo.ReleasePollLease(ctx, "device-lease", "status", "owner-2"); err != nil {
		t.Fatal(err)
	}
}

func TestRecalculateDailyUsageIsIdempotentAndTimezoneAware(t *testing.T) {
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "npms.db"))
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
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO devices(id, display_name, status, created_at, updated_at) VALUES ('device-usage', 'Usage printer', 'unknown', ?, ?)`, time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO counter_definitions(id, device_id, key, source_protocol, oid, unit, semantic_type, scope) VALUES ('counter-usage', 'device-usage', 'page_count', 'snmp', '1.2.3', 'pages', 'impressions', 'engine')`); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		runID string
		value int
		when  string
	}{
		{runID: "run-1", value: 100, when: "2026-01-01T16:00:00Z"},
		{runID: "run-2", value: 105, when: "2026-01-02T01:00:00Z"},
	} {
		if _, err := db.DB.ExecContext(ctx, `INSERT INTO polling_runs(id, device_id, job_kind, started_at, result) VALUES (?, 'device-usage', 'counter', ?, 'success')`, item.runID, item.when); err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB.ExecContext(ctx, `INSERT INTO counter_readings(poll_run_id, counter_definition_id, raw_value, collected_at, quality) VALUES (?, 'counter-usage', ?, ?, 'valid')`, item.runID, item.value, item.when); err != nil {
			t.Fatal(err)
		}
	}
	location, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.RecalculateDailyUsage(ctx, "device-usage", location); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecalculateDailyUsage(ctx, "device-usage", location); err != nil {
		t.Fatal(err)
	}
	var localDate string
	var delta int
	var count int
	if err := db.DB.QueryRowContext(ctx, `SELECT local_date, delta FROM daily_counter_usage WHERE device_id = 'device-usage'`).Scan(&localDate, &delta); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM daily_counter_usage WHERE device_id = 'device-usage'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if localDate != "2026-01-01" || delta != 0 || count != 2 {
		t.Fatalf("unexpected materialized usage: date=%s delta=%d count=%d", localDate, delta, count)
	}
	var nextDate string
	var nextDelta int
	if err := db.DB.QueryRowContext(ctx, `SELECT local_date, delta FROM daily_counter_usage WHERE device_id = 'device-usage' AND local_date = '2026-01-02'`).Scan(&nextDate, &nextDelta); err != nil {
		t.Fatal(err)
	}
	if nextDate != "2026-01-02" || nextDelta != 5 {
		t.Fatalf("unexpected second usage row: date=%s delta=%d", nextDate, nextDelta)
	}
}

func TestCleanupPollerDataPreservesRawReadingHistory(t *testing.T) {
	db, err := database.OpenSQLite(filepath.Join(t.TempDir(), "npms.db"))
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
	const old = "2020-01-01T00:00:00Z"
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO devices(id, display_name, status, created_at, updated_at) VALUES ('device-cleanup', 'Cleanup printer', 'unknown', ?, ?)`, old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO counter_definitions(id, device_id, key, source_protocol, oid, unit, semantic_type, scope) VALUES ('counter-cleanup', 'device-cleanup', 'page_count', 'snmp', '1.2.3', 'pages', 'impressions', 'engine')`); err != nil {
		t.Fatal(err)
	}
	for _, runID := range []string{"orphan-old", "referenced-old"} {
		if _, err := db.DB.ExecContext(ctx, `INSERT INTO polling_runs(id, device_id, job_kind, started_at, ended_at, result) VALUES (?, 'device-cleanup', 'counter', ?, ?, 'success')`, runID, old, old); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO counter_readings(poll_run_id, counter_definition_id, raw_value, collected_at, quality) VALUES ('referenced-old', 'counter-cleanup', 10, ?, 'valid')`, old); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO counter_events(device_id, event_type, details, created_at) VALUES ('device-cleanup', 'old_event', '{}', ?), ('device-cleanup', 'new_event', '{}', '2099-01-01T00:00:00Z')`, old); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO jobs(id, device_id, kind, status, created_at, ended_at) VALUES ('old-job', 'device-cleanup', 'poll', 'success', ?, ?), ('queued-job', 'device-cleanup', 'poll', 'queued', ?, NULL)`, old, old, old); err != nil {
		t.Fatal(err)
	}
	stats, err := repo.CleanupPollerData(ctx, CleanupPolicy{PollingRunsBefore: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), CounterEventsBefore: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), JobsBefore: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if stats.PollingRuns != 1 || stats.CounterEvents != 1 || stats.Jobs != 1 {
		t.Fatalf("unexpected cleanup stats: %+v", stats)
	}
	var runs, readings, queued int
	if err := db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM polling_runs WHERE device_id = 'device-cleanup'`).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM counter_readings WHERE counter_definition_id = 'counter-cleanup'`).Scan(&readings); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE id = 'queued-job'`).Scan(&queued); err != nil {
		t.Fatal(err)
	}
	if runs != 1 || readings != 1 || queued != 1 {
		t.Fatalf("cleanup removed protected history: runs=%d readings=%d queued=%d", runs, readings, queued)
	}
}
