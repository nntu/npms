package runtime

import (
	"context"
	"testing"
	"time"

	"npms/backend/internal/database"
	"npms/backend/internal/ingestion"
	"npms/backend/internal/polling"
	"npms/backend/internal/repository"
	"npms/backend/internal/security"
	"npms/backend/internal/snmp"
)

func TestRunnerPollDeviceCounters(t *testing.T) {
	ctx := context.Background()
	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo, err := repository.NewSQLiteRepository(db.DB)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	box, _ := security.NewSecretBox(make([]byte, 32))
	resolver := ingestion.ConfigResolver{Store: repo, Box: box}

	fakeClient := &fakeSNMPClient{
		getFunc: func(ctx context.Context, oids []string) ([]snmp.VarBind, error) {
			var results []snmp.VarBind
			for _, oid := range oids {
				switch oid {
				case "1.3.6.1.2.1.1.5.0":
					results = append(results, snmp.VarBind{OID: oid, Value: "hp-test-printer"})
				case "1.3.6.1.2.1.25.3.2.1.5.1":
					results = append(results, snmp.VarBind{OID: oid, Value: int64(3)})
				case "1.3.6.1.2.1.43.10.2.1.4.1.1":
					results = append(results, snmp.VarBind{OID: oid, Value: int64(54321)})
				case "1.3.6.1.2.1.43.10.2.1.3.1.1":
					results = append(results, snmp.VarBind{OID: oid, Value: int64(7)})
				}
			}
			return results, nil
		},
	}
	clientFactory := func(snmp.Config) (snmp.Client, error) {
		return fakeClient, nil
	}

	poller, err := ingestion.NewStatusPoller(repo, resolver, clientFactory)
	if err != nil {
		t.Fatalf("new status poller: %v", err)
	}
	counterPoller, err := ingestion.NewCounterPoller(resolver, clientFactory)
	if err != nil {
		t.Fatalf("new counter poller: %v", err)
	}
	scheduler, err := polling.NewScheduler(polling.Config{Concurrency: 2, MaxAttempts: 1})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}
	counterService, err := polling.NewService(scheduler, repo)
	if err != nil {
		t.Fatalf("new counter service: %v", err)
	}

	runner, err := NewRunner(repo, poller, counterPoller, scheduler, counterService, repository.CleanupPolicy{}, time.UTC)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	// Register printer with counter definitions
	secretBytes, _ := box.Encrypt([]byte(`{"version":"2c","community":"public"}`))
	credID := "cred-1"
	if err := repo.CreateSNMPCredential(ctx, repository.SNMPCredential{
		ID:                      credID,
		Version:                 "2c",
		EncryptedSecretMaterial: secretBytes,
	}); err != nil {
		t.Fatalf("create cred: %v", err)
	}

	deviceID := "printer-1"
	if err := repo.CreateDevice(ctx, repository.Device{
		ID:          deviceID,
		DisplayName: "HP Test Printer",
		Status:      "unknown",
	}); err != nil {
		t.Fatalf("create device: %v", err)
	}

	if err := repo.CreateDeviceEndpoint(ctx, repository.DeviceEndpoint{
		ID:           "ep-1",
		DeviceID:     deviceID,
		Address:      "192.168.1.100",
		Protocol:     "snmp",
		Port:         161,
		CredentialID: credID,
		IsPrimary:    true,
	}); err != nil {
		t.Fatalf("create ep: %v", err)
	}

	// Counter definition with known instance
	if err := repo.CreateCounterDefinition(ctx, repository.CounterDefinition{
		ID:             deviceID + ":marker_life",
		DeviceID:       deviceID,
		Key:            "marker_life",
		SourceProtocol: "snmp",
		OID:            "1.3.6.1.2.1.43.10.2.1.4",
		Instance:       "1.1",
		Unit:           "impressions",
		SemanticType:   "marker_life",
		Scope:          "engine",
		Mode:           "walk",
		UnitOID:        "1.3.6.1.2.1.43.10.2.1.3",
		Verified:       true,
	}); err != nil {
		t.Fatalf("create counter def: %v", err)
	}

	// PollDevice (status + counters)
	if err := runner.PollDevice(ctx, deviceID); err != nil {
		t.Fatalf("runner.PollDevice failed: %v", err)
	}

	// Verify counter reading was stored
	readings, err := repo.ListCounterReadings(ctx, deviceID, 10, 0)
	if err != nil {
		t.Fatalf("list counter readings: %v", err)
	}
	if len(readings) != 1 {
		t.Fatalf("expected 1 reading, got %d", len(readings))
	}
	if readings[0].RawValue != 54321 {
		t.Fatalf("expected raw value 54321, got %d", readings[0].RawValue)
	}
	if readings[0].Unit != "impressions" {
		t.Fatalf("expected unit impressions, got %q", readings[0].Unit)
	}

	// Check device status
	d, err := repo.GetDevice(ctx, deviceID)
	if err != nil {
		t.Fatalf("get device: %v", err)
	}
	if d.Status != "online" {
		t.Fatalf("expected status online, got %q", d.Status)
	}
}

type fakeSNMPClient struct {
	getFunc  func(context.Context, []string) ([]snmp.VarBind, error)
	walkFunc func(context.Context, string) ([]snmp.VarBind, error)
}

func (f *fakeSNMPClient) Get(ctx context.Context, oids []string) ([]snmp.VarBind, error) {
	if f.getFunc != nil {
		return f.getFunc(ctx, oids)
	}
	return nil, nil
}

func (f *fakeSNMPClient) Walk(ctx context.Context, oid string) ([]snmp.VarBind, error) {
	if f.walkFunc != nil {
		return f.walkFunc(ctx, oid)
	}
	return nil, nil
}

func (f *fakeSNMPClient) Close() error {
	return nil
}
