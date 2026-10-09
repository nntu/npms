package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"npms/backend/internal/discovery"
	"npms/backend/internal/profile"
	"npms/backend/internal/repository"
	"npms/backend/internal/security"
	"npms/backend/internal/snmp"
)

type fakeStore struct {
	devices        []repository.Device
	total          int
	registrationCh chan repository.PrinterRegistration
	finishCh       chan string
	leaseHeld      bool
}

type fakePoller struct {
	poll func(context.Context, string) error
}

func (f fakePoller) PollDevice(ctx context.Context, deviceID string) error {
	if f.poll != nil {
		return f.poll(ctx, deviceID)
	}
	return nil
}

func (f fakeStore) ListDevices(context.Context, int, int) ([]repository.Device, error) {
	return f.devices, nil
}
func (f fakeStore) CountDevices(context.Context) (int, error)           { return f.total, nil }
func (fakeStore) CreateDevice(context.Context, repository.Device) error { return nil }
func (fakeStore) CreateSNMPCredential(context.Context, repository.SNMPCredential) error {
	return nil
}
func (fakeStore) CreateDeviceEndpoint(context.Context, repository.DeviceEndpoint) error {
	return nil
}

func (f fakeStore) RegisterPrinter(ctx context.Context, registration repository.PrinterRegistration) error {
	if f.registrationCh != nil {
		select {
		case f.registrationCh <- registration:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
func (f fakeStore) GetDevice(_ context.Context, id string) (repository.Device, error) {
	for _, device := range f.devices {
		if device.ID == id {
			return device, nil
		}
	}
	return repository.Device{}, sql.ErrNoRows
}
func (fakeStore) ListCounterReadings(context.Context, string, int, int) ([]repository.CounterReadingRecord, error) {
	return []repository.CounterReadingRecord{}, nil
}
func (fakeStore) ListDailyCounterUsage(context.Context, string, string, string) ([]repository.DailyUsageRecord, error) {
	return []repository.DailyUsageRecord{}, nil
}
func (fakeStore) CountCounterReadings(context.Context, string) (int, error) { return 0, nil }
func (fakeStore) ListPollingRuns(context.Context, string, int, int) ([]repository.PollingRunRecord, error) {
	return []repository.PollingRunRecord{}, nil
}
func (fakeStore) CountPollingRuns(context.Context, string) (int, error)         { return 0, nil }
func (fakeStore) CreatePollingRun(context.Context, repository.PollingRun) error { return nil }
func (f fakeStore) FinishPollingRun(_ context.Context, _, status, errorCode string, _ time.Time) error {
	if f.finishCh != nil {
		f.finishCh <- status + ":" + errorCode
	}
	return nil
}
func (fakeStore) CreateJob(context.Context, repository.JobRecord) error { return nil }
func (fakeStore) GetJob(context.Context, string) (repository.JobRecord, error) {
	return repository.JobRecord{}, sql.ErrNoRows
}
func (fakeStore) StartJob(context.Context, string, time.Time) error { return nil }
func (f fakeStore) FinishJob(_ context.Context, _, status, errorCode string, _ time.Time) error {
	if f.finishCh != nil {
		f.finishCh <- status + ":" + errorCode
	}
	return nil
}
func (f fakeStore) AcquirePollLease(context.Context, string, string, string, time.Time, time.Time) (bool, error) {
	return !f.leaseHeld, nil
}
func (fakeStore) ReleasePollLease(context.Context, string, string, string) error { return nil }
func (fakeStore) CreateCartridge(context.Context, repository.Cartridge) error    { return nil }
func (fakeStore) ListCartridges(context.Context) ([]repository.Cartridge, error) {
	return []repository.Cartridge{{ID: "c1", SKUCode: "HP-26A", Name: "HP Toner", StockNew: 5, StockRefilled: 2, StockEmpty: 1}}, nil
}
func (fakeStore) GetCartridge(context.Context, string) (repository.Cartridge, error) {
	return repository.Cartridge{ID: "c1", SKUCode: "HP-26A", Name: "HP Toner", StockNew: 5, StockRefilled: 2, StockEmpty: 1}, nil
}
func (fakeStore) UpdateCartridgeStock(context.Context, string, int, int, int, string, string) error {
	return nil
}
func (fakeStore) ReplacePrinterCartridge(context.Context, repository.ReplaceCartridgeParams) error {
	return nil
}
func (fakeStore) RefillCartridges(context.Context, string, string, int, string) error {
	return nil
}
func (fakeStore) ListCartridgeLogs(context.Context, string, string, int, int) ([]repository.CartridgeLog, error) {
	return []repository.CartridgeLog{}, nil
}

func TestListPrintersReturnsPaginatedSafeDTO(t *testing.T) {
	server, err := NewServer(fakeStore{devices: []repository.Device{{ID: "d1", DisplayName: "Front", Status: "online", Serial: "secret", LastSeenAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}}, total: 1}, "token", "http://localhost:5173")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/printers?limit=10&offset=0", nil)
	req.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Body.String(); got == "" || !containsAll(got, `"display_name":"Front"`, `"last_seen_at":"2026-01-02T03:04:05Z"`) {
		t.Fatalf("unexpected response: %s", got)
	}
}

func TestListPrintersRejectsMissingToken(t *testing.T) {
	server, _ := NewServer(fakeStore{}, "token", "http://localhost:5173")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/printers", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestListProfilesReturnsCatalogAndRequiresToken(t *testing.T) {
	server, err := NewServer(fakeStore{}, "token", "")
	if err != nil {
		t.Fatal(err)
	}
	server.SetProfiles([]profile.Profile{{ID: "hp-test", Version: 2, Manufacturer: "HP", VerificationStatus: profile.VerificationExperimental, Counters: map[string]profile.Counter{"marker_life": {}}}})

	unauthorized := httptest.NewRecorder()
	server.Handler().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/snmp/profiles", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", unauthorized.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/snmp/profiles", nil)
	req.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if !containsAll(recorder.Body.String(), `"id":"hp-test"`, `"verification_status":"experimental"`, `"counter_keys":["marker_life"]`) {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
}

func TestRegisterPrinterResolvesProfileAndBuildsCounterDefinitions(t *testing.T) {
	registrationCh := make(chan repository.PrinterRegistration, 1)
	server, err := NewServer(fakeStore{registrationCh: registrationCh}, "token", "")
	if err != nil {
		t.Fatal(err)
	}
	box, err := security.NewSecretBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	server.SetSecretBox(box)
	server.SetProfiles([]profile.Profile{{
		SchemaVersion:      profile.CurrentSchemaVersion,
		ID:                 "test-profile",
		Version:            1,
		Manufacturer:       "Test",
		VerificationStatus: profile.VerificationUnverified,
		Counters: map[string]profile.Counter{
			"marker_life": {ValueColumn: "1.2.3.4", UnitColumn: "1.2.3.5", Mode: "walk", Selection: "validated_marker_rows", SemanticType: "marker_life", Scope: "engine", Unit: "impressions", RequireUnitValidation: true},
		},
	}})
	body, err := json.Marshal(map[string]any{"display_name": "Test printer", "address": "192.0.2.10", "version": "2c", "community": "public", "profile_id": "test-profile"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/printers/register", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var registration repository.PrinterRegistration
	select {
	case registration = <-registrationCh:
	case <-time.After(time.Second):
		t.Fatal("registration was not submitted")
	}
	if registration.Profile == nil || registration.Profile.ProfileKey != "test-profile" || !registration.ProfileExplicit {
		t.Fatalf("unexpected profile: %#v", registration.Profile)
	}
	if len(registration.CounterDefinitions) != 1 || registration.CounterDefinitions[0].Mode != "walk" || registration.CounterDefinitions[0].UnitOID != "1.2.3.5" {
		t.Fatalf("unexpected counter definitions: %#v", registration.CounterDefinitions)
	}
}

func TestCreateCredentialEncryptsAndDoesNotReturnSecret(t *testing.T) {
	server, err := NewServer(fakeStore{devices: []repository.Device{{ID: "d1", DisplayName: "Front", Status: "unknown"}}}, "token", "")
	if err != nil {
		t.Fatal(err)
	}
	box, err := security.NewSecretBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	server.SetSecretBox(box)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/printers/d1/credentials", strings.NewReader(`{"version":"2c","community":"secret-community"}`))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "secret-community") || !strings.Contains(recorder.Body.String(), `"version":"2c"`) {
		t.Fatalf("credential secret leaked or response incomplete: %s", recorder.Body.String())
	}
}

func TestCreateEndpointValidatesAndReturnsSafeDTO(t *testing.T) {
	server, err := NewServer(fakeStore{devices: []repository.Device{{ID: "d1", DisplayName: "Front", Status: "unknown"}}}, "token", "")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/printers/d1/endpoints", strings.NewReader(`{"address":"192.168.1.20","port":161,"credential_id":"cred-1","is_primary":true}`))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", recorder.Code, recorder.Body.String())
	}
	if !containsAll(recorder.Body.String(), `"address":"192.168.1.20"`, `"port":161`, `"protocol":"snmp"`) {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
}

func TestRegisterPrinterQueuesInitialPollWhenConfigured(t *testing.T) {
	server, err := NewServer(fakeStore{}, "token", "")
	if err != nil {
		t.Fatal(err)
	}
	box, err := security.NewSecretBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	server.SetSecretBox(box)
	server.SetStatusPoller(fakePoller{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/printers/register", strings.NewReader(`{"display_name":"Front","address":"192.168.1.20","version":"2c","community":"fixture"}`))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusCreated || !containsAll(recorder.Body.String(), `"poll_status":"queued"`, `"poll_job_id"`) {
		t.Fatalf("unexpected registration response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestDiscoveryProbeValidatesSingleIPAndReturnsIdentity(t *testing.T) {
	server, err := NewServer(fakeStore{}, "token", "")
	if err != nil {
		t.Fatal(err)
	}
	server.SetDiscoveryProbe(func(_ context.Context, config snmp.Config) (discovery.Result, error) {
		return discovery.Result{Address: config.Host, Name: "front-printer", SysObjectID: "1.3.6.1.4.1.11"}, nil
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discovery/probe", strings.NewReader(`{"address":"192.168.1.20","version":"2c","community":"fixture"}`))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK || !containsAll(recorder.Body.String(), `"address":"192.168.1.20"`, `"name":"front-printer"`) {
		t.Fatalf("unexpected discovery response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	invalid := httptest.NewRequest(http.MethodPost, "/api/v1/discovery/probe", strings.NewReader(`{"address":"10.0.0.0/24","version":"2c","community":"fixture"}`))
	invalid.Header.Set("Authorization", "Bearer token")
	invalidRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(invalidRecorder, invalid)
	if invalidRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", invalidRecorder.Code)
	}
}

func TestCreatePrinterValidatesAndReturnsCreatedDevice(t *testing.T) {
	server, _ := NewServer(fakeStore{}, "token", "")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/printers", strings.NewReader(`{"display_name":"Lab printer","manufacturer":"HP"}`))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", recorder.Code, recorder.Body.String())
	}
	if !containsAll(recorder.Body.String(), `"display_name":"Lab printer"`, `"status":"unknown"`) {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
}

func TestCreatePrinterAcceptsDepartment(t *testing.T) {
	server, _ := NewServer(fakeStore{}, "token", "")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/printers", strings.NewReader(`{"display_name":"IT Printer","department":"Phòng IT"}`))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", recorder.Code, recorder.Body.String())
	}
	if !containsAll(recorder.Body.String(), `"display_name":"IT Printer"`, `"department":"Phòng IT"`) {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
}

func TestCreatePrinterRejectsUnknownFields(t *testing.T) {
	server, _ := NewServer(fakeStore{}, "", "")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/printers", strings.NewReader(`{"display_name":"Lab","community":"secret"}`))
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

func TestListPrintersRejectsInvalidPagination(t *testing.T) {
	server, _ := NewServer(fakeStore{}, "", "")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/printers?limit=0", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

func TestGetPrinterReturnsNotFoundWithoutLeakingStorageDetails(t *testing.T) {
	server, _ := NewServer(fakeStore{}, "", "")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/printers/missing", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
	if got := recorder.Body.String(); !containsAll(got, `"code":"not_found"`, `"message":"printer was not found"`) {
		t.Fatalf("unexpected response: %s", got)
	}
}

func TestStartPollReturnsAcceptedAndJobStatus(t *testing.T) {
	server, _ := NewServer(fakeStore{}, "token", "")
	server.SetStatusPoller(fakePoller{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/printers/d1/poll", nil)
	req.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"job_id"`) {
		t.Fatalf("missing job id: %s", recorder.Body.String())
	}
}

func TestExecutePollJobUsesTimeoutAndPersistsTimeoutStatus(t *testing.T) {
	finishCh := make(chan string, 2)
	server, err := NewServer(fakeStore{finishCh: finishCh}, "token", "")
	if err != nil {
		t.Fatal(err)
	}
	server.SetPollTimeout(5 * time.Millisecond)
	server.SetStatusPoller(fakePoller{poll: func(ctx context.Context, _ string) error {
		<-ctx.Done()
		return ctx.Err()
	}})

	started := time.Now()
	server.executePollJob("job-timeout", "device-timeout", "job-timeout")
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("timed out poll took too long: %s", elapsed)
	}
	statuses := []string{<-finishCh, <-finishCh}
	if !containsAll(strings.Join(statuses, ","), "failed:poll_timeout") {
		t.Fatalf("unexpected persisted statuses: %v", statuses)
	}
}

func TestStartPollRejectsConcurrentPollLease(t *testing.T) {
	server, err := NewServer(fakeStore{leaseHeld: true}, "token", "")
	if err != nil {
		t.Fatal(err)
	}
	server.SetStatusPoller(fakePoller{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/printers/device-lease/poll", nil)
	req.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "poll_already_running") {
		t.Fatalf("unexpected response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestCartridgeEndpoints(t *testing.T) {
	server, _ := NewServer(fakeStore{}, "token", "")

	// GET cartridges
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cartridges", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !containsAll(rec.Body.String(), `"sku_code":"HP-26A"`) {
		t.Fatalf("unexpected list cartridges response: status=%d body=%s", rec.Code, rec.Body.String())
	}

	// Replace printer cartridge
	replaceReq := httptest.NewRequest(http.MethodPost, "/api/v1/cartridges/replace", strings.NewReader(`{"cartridge_id":"c1","device_id":"d1","source_type":"new"}`))
	replaceReq.Header.Set("Authorization", "Bearer token")
	replaceReq.Header.Set("Content-Type", "application/json")
	recReplace := httptest.NewRecorder()
	server.Handler().ServeHTTP(recReplace, replaceReq)
	if recReplace.Code != http.StatusOK {
		t.Fatalf("unexpected replace response: status=%d body=%s", recReplace.Code, recReplace.Body.String())
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
