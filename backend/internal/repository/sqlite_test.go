package repository

import (
	"context"
	"database/sql"
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

func TestCompleteCounterPollIsAtomicIdempotentAndTracksAttempts(t *testing.T) {
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
	if err := repo.CreateDevice(ctx, Device{ID: "device-completion", DisplayName: "Completion printer", Status: "unknown"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateCounterDefinition(ctx, CounterDefinition{ID: "counter-completion", DeviceID: "device-completion", Key: "pages", SourceProtocol: "snmp", OID: "1.2.3", Unit: "impressions", SemanticType: "marker_life", Scope: "engine"}); err != nil {
		t.Fatal(err)
	}
	createRun := func(id string) {
		t.Helper()
		if err := repo.CreatePollingRun(ctx, PollingRun{ID: id, DeviceID: "device-completion", JobKind: "counter"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.CreateCounterPollingRuns(ctx, []PollingRun{
		{ID: "run-batch-a", DeviceID: "device-completion", JobKind: "counter"},
		{ID: "run-batch-b", DeviceID: "missing-device", JobKind: "counter"},
	}); err == nil {
		t.Fatal("expected atomic batch creation failure")
	}
	var batchRuns int
	if err := db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM polling_runs WHERE id IN ('run-batch-a', 'run-batch-b')`).Scan(&batchRuns); err != nil {
		t.Fatal(err)
	}
	if batchRuns != 0 {
		t.Fatalf("batch creation left %d polling runs behind", batchRuns)
	}
	if err := repo.CreateCounterPollingRuns(ctx, []PollingRun{
		{ID: "run-batch-a", DeviceID: "device-completion", JobKind: "counter"},
		{ID: "run-batch-b", DeviceID: "device-completion", JobKind: "counter"},
	}); err != nil {
		t.Fatal(err)
	}
	var initialAttempts int
	if err := db.DB.QueryRowContext(ctx, `SELECT SUM(attempt_count) FROM polling_runs WHERE id IN ('run-batch-a', 'run-batch-b')`).Scan(&initialAttempts); err != nil {
		t.Fatal(err)
	}
	if initialAttempts != 0 {
		t.Fatalf("new counter runs have %d attempts, want 0", initialAttempts)
	}

	createRun("run-success")
	collectedAt := time.Date(2026, 10, 14, 1, 2, 3, 0, time.UTC)
	success := CounterPollCompletion{
		PollRunID:           "run-success",
		DeviceID:            "device-completion",
		CounterDefinitionID: "counter-completion",
		Reading:             &counter.Reading{RawValue: 123, CollectedAt: collectedAt, Quality: counter.QualityValid},
		Result:              "success",
		EndedAt:             collectedAt.Add(time.Second),
		AttemptCount:        3,
	}
	if err := repo.CompleteCounterPoll(ctx, success); err != nil {
		t.Fatal(err)
	}
	if err := repo.CompleteCounterPoll(ctx, success); err != nil {
		t.Fatalf("idempotent completion: %v", err)
	}
	var result, endedAt string
	var attempts, readings int
	if err := db.DB.QueryRowContext(ctx, `SELECT result, ended_at, attempt_count FROM polling_runs WHERE id = 'run-success'`).Scan(&result, &endedAt, &attempts); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM counter_readings WHERE poll_run_id = 'run-success'`).Scan(&readings); err != nil {
		t.Fatal(err)
	}
	if result != "success" || endedAt == "" || attempts != 3 || readings != 1 {
		t.Fatalf("unexpected success state: result=%s ended=%s attempts=%d readings=%d", result, endedAt, attempts, readings)
	}

	createRun("run-failed")
	failure := CounterPollCompletion{
		PollRunID:           "run-failed",
		DeviceID:            "device-completion",
		CounterDefinitionID: "counter-completion",
		Result:              "failed",
		ErrorCode:           "poll_failed",
		EventType:           "poll_failed",
		EventDetails:        `{"error":"timeout"}`,
		EndedAt:             collectedAt.Add(2 * time.Second),
		AttemptCount:        2,
	}
	if err := repo.CompleteCounterPoll(ctx, failure); err != nil {
		t.Fatal(err)
	}
	if err := repo.CompleteCounterPoll(ctx, failure); err != nil {
		t.Fatalf("idempotent failure completion: %v", err)
	}
	var events int
	if err := db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM counter_events WHERE device_id = 'device-completion' AND event_type = 'poll_failed'`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRowContext(ctx, `SELECT attempt_count FROM polling_runs WHERE id = 'run-failed'`).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if events != 1 || attempts != 2 {
		t.Fatalf("unexpected failure state: events=%d attempts=%d", events, attempts)
	}

	for _, runID := range []string{"run-reading-rollback", "run-event-rollback"} {
		createRun(runID)
	}
	badReading := success
	badReading.PollRunID = "run-reading-rollback"
	badReading.CounterDefinitionID = "missing-counter"
	if err := repo.CompleteCounterPoll(ctx, badReading); err == nil {
		t.Fatal("expected reading insert failure")
	}
	badEvent := failure
	badEvent.PollRunID = "run-event-rollback"
	badEvent.CounterDefinitionID = "missing-counter"
	if err := repo.CompleteCounterPoll(ctx, badEvent); err == nil {
		t.Fatal("expected event insert failure")
	}
	for _, runID := range []string{"run-reading-rollback", "run-event-rollback"} {
		var terminalAt string
		if err := db.DB.QueryRowContext(ctx, `SELECT result, COALESCE(ended_at, ''), attempt_count FROM polling_runs WHERE id = ?`, runID).Scan(&result, &terminalAt, &attempts); err != nil {
			t.Fatal(err)
		}
		if result != "running" || terminalAt != "" || attempts != 1 {
			t.Fatalf("transaction did not roll back for %s: result=%s ended=%s attempts=%d", runID, result, terminalAt, attempts)
		}
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

func TestUpdatePrinterAndSoftDeletePreservesHistory(t *testing.T) {
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
	reg := PrinterRegistration{
		Device:     Device{ID: "dev-crud", AssetCode: "ASSET-1", DisplayName: "Printer 1", Manufacturer: "HP", Model: "M402dn", Status: "online"},
		Credential: SNMPCredential{ID: "cred-1", Version: "2c", EncryptedSecretMaterial: []byte("secret-1")},
		Endpoint:   DeviceEndpoint{ID: "ep-1", DeviceID: "dev-crud", Address: "192.0.2.10", Protocol: "snmp", Port: 161, CredentialID: "cred-1", IsPrimary: true},
		Profile:    &SNMPProfile{ID: "generic-printer-mib@1", ProfileKey: "generic-printer-mib", Version: 1, SchemaVersion: 1, Content: "profile-v1", Checksum: "sum-1", VerificationStatus: "unverified"},
		CounterDefinitions: []CounterDefinition{{
			ID: "dev-crud:marker_life", DeviceID: "dev-crud", Key: "marker_life", SourceProtocol: "snmp", OID: "1.3.6.1.2.1.43.10.2.1.4", Unit: "impressions", SemanticType: "marker_life", Scope: "engine", Mode: "walk", UnitOID: "1.3.6.1.2.1.43.10.2.1.3",
		}},
	}
	if err := repo.RegisterPrinter(ctx, reg); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO polling_runs(id, device_id, job_kind, started_at, result) VALUES ('run-crud', 'dev-crud', 'counter', '2026-01-01T00:00:00Z', 'success')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO counter_readings(poll_run_id, counter_definition_id, raw_value, collected_at, quality) VALUES ('run-crud', 'dev-crud:marker_life', 500, '2026-01-01T00:00:00Z', 'valid')`); err != nil {
		t.Fatal(err)
	}

	upd := PrinterUpdate{
		Device:          Device{ID: "dev-crud", AssetCode: "ASSET-1", DisplayName: "Printer 1 Updated", Manufacturer: "HP", Model: "HP LaserJet Enterprise M501dn", Serial: "SN123", Department: "Kế toán"},
		EndpointAddress: "192.0.2.25",
		EndpointPort:    1161,
		Credential:      &SNMPCredential{ID: "cred-2", Version: "2c", EncryptedSecretMaterial: []byte("secret-2")},
		Profile:         &SNMPProfile{ID: "hp-laserjet-enterprise-m501dn@1", ProfileKey: "hp-laserjet-enterprise-m501dn", Version: 1, SchemaVersion: 1, Content: "profile-hp", Checksum: "sum-hp", VerificationStatus: "experimental"},
		ProfileExplicit: true,
		CounterDefinitions: []CounterDefinition{{
			ID: "dev-crud:marker_life", DeviceID: "dev-crud", Key: "marker_life", SourceProtocol: "snmp", OID: "1.3.6.1.2.1.43.10.2.1.4", Instance: "1.1", Unit: "impressions", SemanticType: "marker_life", Scope: "engine", Mode: "walk", UnitOID: "1.3.6.1.2.1.43.10.2.1.3",
		}},
	}
	if err := repo.UpdatePrinter(ctx, upd); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetDevice(ctx, "dev-crud")
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "Printer 1 Updated" || got.Address != "192.0.2.25" || got.Port != 1161 || got.ProfileKey != "hp-laserjet-enterprise-m501dn" || got.Department != "Kế toán" {
		t.Fatalf("unexpected updated device: %+v", got)
	}

	if err := repo.SoftDeleteDevice(ctx, "dev-crud"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetDevice(ctx, "dev-crud"); err == nil {
		t.Fatal("expected soft-deleted device to be hidden from GetDevice")
	}
	var readings int
	if err := db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM counter_readings WHERE counter_definition_id = 'dev-crud:marker_life'`).Scan(&readings); err != nil {
		t.Fatal(err)
	}
	if readings != 1 {
		t.Fatalf("expected historical counter reading to be preserved after soft delete, got %d", readings)
	}
}

func TestReconcileStaleRuns(t *testing.T) {
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

	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	twentyMinsAgo := now.Add(-20 * time.Minute).Format(time.RFC3339Nano)
	oneMinAgo := now.Add(-1 * time.Minute).Format(time.RFC3339Nano)
	fiveMinsAgo := now.Add(-5 * time.Minute).Format(time.RFC3339Nano)
	fiveMinsLater := now.Add(5 * time.Minute).Format(time.RFC3339Nano)

	// Setup device
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO devices(id, display_name, status, created_at, updated_at) VALUES ('device-stale', 'Stale test printer', 'unknown', ?, ?)`, twentyMinsAgo, twentyMinsAgo); err != nil {
		t.Fatal(err)
	}

	// 1. Polling runs: stale (20m ago, running, ended_at NULL), fresh (1m ago, running, ended_at NULL), ended (20m ago, success)
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO polling_runs(id, device_id, job_kind, started_at, ended_at, result) VALUES
		('stale-run-1', 'device-stale', 'counter', ?, NULL, 'running'),
		('fresh-run-2', 'device-stale', 'counter', ?, NULL, 'running'),
		('ended-run-3', 'device-stale', 'counter', ?, ?, 'success')`,
		twentyMinsAgo, oneMinAgo, twentyMinsAgo, twentyMinsAgo); err != nil {
		t.Fatal(err)
	}

	// 2. Jobs: stale running (20m ago), fresh running (1m ago), stale queued (20m ago)
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO jobs(id, device_id, kind, status, created_at, started_at, ended_at) VALUES
		('stale-job-1', 'device-stale', 'status_poll', 'running', ?, ?, NULL),
		('fresh-job-2', 'device-stale', 'status_poll', 'running', ?, ?, NULL),
		('stale-queued-3', 'device-stale', 'status_poll', 'queued', ?, NULL, NULL)`,
		twentyMinsAgo, twentyMinsAgo, oneMinAgo, oneMinAgo, twentyMinsAgo); err != nil {
		t.Fatal(err)
	}

	// 3. Leases: expired (5m ago), active (5m later)
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO poll_leases(device_id, job_kind, owner_id, acquired_at, lease_until) VALUES
		('device-stale', 'counter', 'owner-old', ?, ?),
		('device-stale', 'status', 'owner-live', ?, ?)`,
		twentyMinsAgo, fiveMinsAgo, oneMinAgo, fiveMinsLater); err != nil {
		t.Fatal(err)
	}

	// Run reconciliation with 10m timeout
	stats, err := repo.ReconcileStaleRuns(ctx, StaleReconciliationPolicy{
		RunningTimeout: 10 * time.Minute,
		Now:            now,
	})
	if err != nil {
		t.Fatalf("reconcile stale runs: %v", err)
	}

	if stats.InterruptedRuns != 1 {
		t.Errorf("expected 1 interrupted run, got %d", stats.InterruptedRuns)
	}
	if stats.FailedJobs != 2 {
		t.Errorf("expected 2 failed jobs, got %d", stats.FailedJobs)
	}
	if stats.ReleasedLeases != 1 {
		t.Errorf("expected 1 released lease, got %d", stats.ReleasedLeases)
	}
	if stats.AuditEvents != 3 {
		t.Errorf("expected 3 audit events, got %d", stats.AuditEvents)
	}

	// Check DB state of polling runs
	var staleRunResult, staleRunErr, staleRunEnded string
	if err := db.DB.QueryRowContext(ctx, `SELECT result, error_code, ended_at FROM polling_runs WHERE id = 'stale-run-1'`).Scan(&staleRunResult, &staleRunErr, &staleRunEnded); err != nil {
		t.Fatal(err)
	}
	if staleRunResult != "interrupted" || staleRunErr != "poll_timeout" || staleRunEnded == "" {
		t.Errorf("unexpected stale run state: result=%s, err=%s, ended=%s", staleRunResult, staleRunErr, staleRunEnded)
	}

	var freshRunResult string
	var freshRunEnded sql.NullString
	if err := db.DB.QueryRowContext(ctx, `SELECT result, ended_at FROM polling_runs WHERE id = 'fresh-run-2'`).Scan(&freshRunResult, &freshRunEnded); err != nil {
		t.Fatal(err)
	}
	if freshRunResult != "running" || freshRunEnded.Valid {
		t.Errorf("fresh run should still be running, got result=%s, ended=%v", freshRunResult, freshRunEnded)
	}

	// Check DB state of jobs
	var staleJobStatus, staleJobErr string
	if err := db.DB.QueryRowContext(ctx, `SELECT status, error_code FROM jobs WHERE id = 'stale-job-1'`).Scan(&staleJobStatus, &staleJobErr); err != nil {
		t.Fatal(err)
	}
	if staleJobStatus != "failed" || staleJobErr != "job_timeout" {
		t.Errorf("unexpected stale job state: status=%s, err=%s", staleJobStatus, staleJobErr)
	}

	var queuedJobStatus, queuedJobErr string
	if err := db.DB.QueryRowContext(ctx, `SELECT status, error_code FROM jobs WHERE id = 'stale-queued-3'`).Scan(&queuedJobStatus, &queuedJobErr); err != nil {
		t.Fatal(err)
	}
	if queuedJobStatus != "failed" || queuedJobErr != "job_timeout" {
		t.Errorf("unexpected stale queued job state: status=%s, err=%s", queuedJobStatus, queuedJobErr)
	}

	// Check leases
	var leaseCount int
	if err := db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM poll_leases WHERE device_id = 'device-stale'`).Scan(&leaseCount); err != nil {
		t.Fatal(err)
	}
	if leaseCount != 1 {
		t.Errorf("expected 1 remaining lease (fresh), got %d", leaseCount)
	}

	// Check audit events
	var auditCount int
	if err := db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM counter_events WHERE device_id = 'device-stale'`).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 3 {
		t.Errorf("expected 3 audit events, got %d", auditCount)
	}

	// Second pass should be idempotent and find nothing new
	secondStats, err := repo.ReconcileStaleRuns(ctx, StaleReconciliationPolicy{
		RunningTimeout: 10 * time.Minute,
		Now:            now,
	})
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if secondStats.InterruptedRuns != 0 || secondStats.FailedJobs != 0 || secondStats.ReleasedLeases != 0 || secondStats.AuditEvents != 0 {
		t.Errorf("expected idempotent zero stats on second pass, got %+v", secondStats)
	}
}

func TestListJobsWithDevice(t *testing.T) {
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

	devID := "dev-job-test"
	if err := repo.CreateDevice(ctx, Device{
		ID:          devID,
		DisplayName: "Office Printer 1",
		Status:      "online",
		FirstSeenAt: time.Now().UTC(),
		LastSeenAt:  time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create device: %v", err)
	}

	if err := repo.CreateDeviceEndpoint(ctx, DeviceEndpoint{
		ID:        "ep-1",
		DeviceID:  devID,
		Address:   "192.168.1.100",
		Protocol:  "snmp",
		Port:      161,
		IsPrimary: true,
	}); err != nil {
		t.Fatalf("create endpoint: %v", err)
	}

	now := time.Now().UTC()
	start1 := now.Add(-5 * time.Minute)
	end1 := now.Add(-4 * time.Minute)
	if err := repo.CreateJob(ctx, JobRecord{
		ID:        "job-1",
		DeviceID:  devID,
		Kind:      "status",
		Status:    "success",
		CreatedAt: now.Add(-6 * time.Minute),
		StartedAt: start1,
		EndedAt:   end1,
	}); err != nil {
		t.Fatalf("create job 1: %v", err)
	}

	start2 := now.Add(-2 * time.Minute)
	if err := repo.CreateJob(ctx, JobRecord{
		ID:        "job-2",
		DeviceID:  devID,
		Kind:      "counter",
		Status:    "running",
		CreatedAt: now.Add(-3 * time.Minute),
		StartedAt: start2,
	}); err != nil {
		t.Fatalf("create job 2: %v", err)
	}

	if err := repo.CreateJob(ctx, JobRecord{
		ID:        "job-3",
		DeviceID:  devID,
		Kind:      "status",
		Status:    "failed",
		ErrorCode: "snmp_timeout",
		CreatedAt: now.Add(-1 * time.Minute),
		StartedAt: now.Add(-30 * time.Second),
		EndedAt:   now,
	}); err != nil {
		t.Fatalf("create job 3: %v", err)
	}

	// 1. List all jobs
	allJobs, total, err := repo.ListJobsWithDevice(ctx, 10, 0, "")
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if total != 3 || len(allJobs) != 3 {
		t.Fatalf("expected 3 jobs, got total=%d, len=%d", total, len(allJobs))
	}
	// Verify device join
	if allJobs[0].DeviceName != "Office Printer 1" || allJobs[0].DeviceIP != "192.168.1.100" {
		t.Errorf("expected device info joined, got name=%q, ip=%q", allJobs[0].DeviceName, allJobs[0].DeviceIP)
	}

	// 2. Filter by status 'running'
	runningJobs, runningTotal, err := repo.ListJobsWithDevice(ctx, 10, 0, "running")
	if err != nil {
		t.Fatalf("list running jobs: %v", err)
	}
	if runningTotal != 1 || len(runningJobs) != 1 {
		t.Fatalf("expected 1 running job, got total=%d, len=%d", runningTotal, len(runningJobs))
	}
	if runningJobs[0].ID != "job-2" || runningJobs[0].DurationMs <= 0 {
		t.Errorf("unexpected running job: %+v", runningJobs[0])
	}

	// 3. Filter by status 'failed'
	failedJobs, failedTotal, err := repo.ListJobsWithDevice(ctx, 10, 0, "failed")
	if err != nil {
		t.Fatalf("list failed jobs: %v", err)
	}
	if failedTotal != 1 || len(failedJobs) != 1 {
		t.Fatalf("expected 1 failed job, got total=%d, len=%d", failedTotal, len(failedJobs))
	}
	if failedJobs[0].ErrorCode == nil || *failedJobs[0].ErrorCode != "snmp_timeout" {
		t.Errorf("expected error_code snmp_timeout, got: %v", failedJobs[0].ErrorCode)
	}
}


