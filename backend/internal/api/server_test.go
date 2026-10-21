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

	"npms/backend/internal/counter"
	"npms/backend/internal/discovery"
	"npms/backend/internal/ingestion"
	"npms/backend/internal/profile"
	"npms/backend/internal/repository"
	"npms/backend/internal/security"
	"npms/backend/internal/snmp"
)

type fakeStore struct {
	devices        []repository.Device
	total          int
	jobs           []repository.JobWithDevice
	departments    []repository.Department
	registrationCh chan repository.PrinterRegistration
	finishCh       chan string
	leaseHeld      bool
}

type fakePoller struct {
	poll func(context.Context, string) error
}

type fakeCounterSnapshotter struct{}

func TestHealthUsesTypedRuntimeContract(t *testing.T) {
	server, err := NewServer(fakeStore{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response struct {
		Status  string `json:"status"`
		Time    string `json:"time"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Status != "ok" || response.Time == "" || response.Version == "" {
		t.Fatalf("unexpected health response: %+v", response)
	}
	ready := httptest.NewRecorder()
	server.Handler().ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/api/v1/health/ready", nil))
	if ready.Code != http.StatusOK {
		t.Fatalf("readiness status = %d, want %d", ready.Code, http.StatusOK)
	}

	versionRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(versionRec, httptest.NewRequest(http.MethodGet, "/api/v1/version", nil))
	if versionRec.Code != http.StatusOK {
		t.Fatalf("version status = %d, want %d", versionRec.Code, http.StatusOK)
	}
	var versionResp struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(versionRec.Body.Bytes(), &versionResp); err != nil {
		t.Fatal(err)
	}
	if versionResp.Version == "" {
		t.Fatalf("expected non-empty version, got: %+v", versionResp)
	}
}

func (fakeCounterSnapshotter) ReadDevice(context.Context, string) (counter.Reading, repository.CounterDefinition, error) {
	return counter.Reading{RawValue: 12345, Quality: counter.QualityUnverified, CollectedAt: time.Date(2026, 10, 9, 1, 2, 3, 0, time.UTC)}, repository.CounterDefinition{Key: "marker_life"}, nil
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
func (fakeStore) GetPageUsageReport(context.Context, string, string, string) (*repository.PageUsageReport, error) {
	return &repository.PageUsageReport{
		TotalDepartments: 1,
		TotalPrinters:    1,
		TotalPrintPages:  1250,
		Departments: []repository.DepartmentPageUsage{
			{Department: "Kế toán", PrinterCount: 1, TotalPrintPages: 1250},
		},
		Printers: []repository.PrinterPageUsage{
			{DeviceID: "d1", DisplayName: "HP M501dn", Department: "Kế toán", NormalizedPages: 1250, RawUnit: "impressions", ConversionFactor: 1.0},
		},
	}, nil
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
func (f fakeStore) ListJobsWithDevice(_ context.Context, _, _ int, statusFilter string) ([]repository.JobWithDevice, int, error) {
	var filtered []repository.JobWithDevice
	for _, j := range f.jobs {
		if statusFilter == "" || j.Status == statusFilter {
			filtered = append(filtered, j)
		}
	}
	return filtered, len(filtered), nil
}
func (fakeStore) CreateCartridge(context.Context, repository.Cartridge) error { return nil }
func (fakeStore) ListCartridges(context.Context) ([]repository.Cartridge, error) {
	return []repository.Cartridge{{ID: "c1", SKUCode: "HP-26A", Name: "HP Toner", StockNew: 5, StockRefilled: 2, StockEmpty: 1}}, nil
}
func (fakeStore) GetCartridge(context.Context, string) (repository.Cartridge, error) {
	return repository.Cartridge{ID: "c1", SKUCode: "HP-26A", Name: "HP Toner", StockNew: 5, StockRefilled: 2, StockEmpty: 1}, nil
}
func (fakeStore) UpdateCartridgeStock(context.Context, string, int, int, int, string, string) error {
	return nil
}
func (fakeStore) AddRefillBottles(context.Context, string, int, string, string) error { return nil }
func (fakeStore) ReplacePrinterCartridge(context.Context, repository.ReplaceCartridgeParams) error {
	return nil
}
func (fakeStore) RefillCartridges(context.Context, string, string, int, string) error {
	return nil
}
func (fakeStore) RefillPrinterCartridge(context.Context, repository.RefillPrinterCartridgeParams) error {
	return nil
}
func (fakeStore) ListCartridgeLogs(context.Context, string, string, int, int) ([]repository.CartridgeLog, error) {
	return []repository.CartridgeLog{}, nil
}

func (f fakeStore) ListDepartments(context.Context) ([]repository.Department, error) {
	if f.departments != nil {
		return f.departments, nil
	}
	return []repository.Department{{ID: "dept-1", Code: "KT", Name: "Phòng Kế toán", DeviceCount: 2}}, nil
}
func (f fakeStore) GetDepartmentByID(_ context.Context, id string) (*repository.Department, error) {
	for _, d := range f.departments {
		if d.ID == id {
			return &d, nil
		}
	}
	if id == "dept-1" {
		return &repository.Department{ID: "dept-1", Code: "KT", Name: "Phòng Kế toán"}, nil
	}
	return nil, nil
}
func (f fakeStore) GetDepartmentByCode(_ context.Context, _ string) (*repository.Department, error) {
	return nil, nil
}
func (fakeStore) CreateDepartment(context.Context, *repository.Department) error  { return nil }
func (fakeStore) UpdateDepartment(context.Context, *repository.Department) error  { return nil }
func (fakeStore) DeleteDepartment(context.Context, string) error                  { return nil }
func (fakeStore) CountCartridgeLogs(context.Context, string, string) (int, error) { return 0, nil }
func (f fakeStore) UpdatePrinter(_ context.Context, u repository.PrinterUpdate) error {
	for _, d := range f.devices {
		if d.ID == u.Device.ID {
			return nil
		}
	}
	return sql.ErrNoRows
}
func (f fakeStore) SoftDeleteDevice(_ context.Context, id string) error {
	for _, d := range f.devices {
		if d.ID == id {
			return nil
		}
	}
	return sql.ErrNoRows
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
	server.SetCounterSnapshotter(fakeCounterSnapshotter{})

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
	if recReplace.Code != http.StatusOK || !containsAll(recReplace.Body.String(), `"counter":12345`, `"counter_quality":"unverified"`) {
		t.Fatalf("unexpected replace response: status=%d body=%s", recReplace.Code, recReplace.Body.String())
	}
}

func TestRegisterPrinterAutoProbesSNMPAndAssignsModelProfile(t *testing.T) {
	registrationCh := make(chan repository.PrinterRegistration, 1)
	server, err := NewServer(fakeStore{registrationCh: registrationCh}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	box, err := security.NewSecretBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	server.SetSecretBox(box)
	profiles, err := profile.LoadDir("../../profiles")
	if err != nil {
		t.Fatal(err)
	}
	server.SetProfiles(profiles)
	server.SetDiscoveryProbe(func(_ context.Context, cfg snmp.Config) (discovery.Result, error) {
		return discovery.Result{
			Address:      cfg.Host,
			Name:         "HP-M501DN-KT01",
			Manufacturer: "HP",
			Model:        "HP LaserJet Enterprise M501dn",
			Serial:       "REDACTED-M501DN-001",
			SysObjectID:  "1.3.6.1.4.1.11.2.3.9.4.2.1.1.3.120",
			Markers:      []snmp.MarkerRow{{Instance: "1.1", Life: 48250, Unit: 7}},
		}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/printers/register", strings.NewReader(`{"display_name":"Máy in Kế toán","address":"192.0.2.10","version":"2c","community":"public"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	reg := <-registrationCh
	if reg.Device.Manufacturer != "HP" || reg.Device.Model != "HP LaserJet Enterprise M501dn" || reg.Device.Serial != "REDACTED-M501DN-001" {
		t.Fatalf("expected auto-probed device identity, got %+v", reg.Device)
	}
	if reg.Profile == nil || reg.Profile.ProfileKey != "hp-laserjet-enterprise-m501dn" {
		t.Fatalf("expected hp-laserjet-enterprise-m501dn profile, got %+v", reg.Profile)
	}
	if len(reg.CounterDefinitions) != 1 || reg.CounterDefinitions[0].Instance != "1.1" {
		t.Fatalf("expected counter definition with discovered instance 1.1, got %+v", reg.CounterDefinitions)
	}
}

func TestUpdateAndSoftDeletePrinter(t *testing.T) {
	store := fakeStore{devices: []repository.Device{{ID: "d1", DisplayName: "Old Name", Address: "192.0.2.10", Port: 161, SNMPVersion: "2c", Status: "online"}}}
	server, err := NewServer(store, "", "")
	if err != nil {
		t.Fatal(err)
	}
	box, err := security.NewSecretBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	server.SetSecretBox(box)

	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/printers/d1", strings.NewReader(`{"display_name":"New Name","department":"IT","address":"192.0.2.11","port":161}`))
	putReq.Header.Set("Content-Type", "application/json")
	putRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", putRec.Code, putRec.Body.String())
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/printers/d1", nil)
	delRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusOK || !strings.Contains(delRec.Body.String(), `"status":"deleted"`) {
		t.Fatalf("DELETE status = %d, body = %s", delRec.Code, delRec.Body.String())
	}
}

func TestScanSubnetEndpoint(t *testing.T) {
	server, err := NewServer(fakeStore{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	server.SetSubnetScanner(func(_ context.Context, cfg discovery.SubnetScanConfig) ([]discovery.Result, error) {
		return []discovery.Result{
			{
				Address:      "192.168.1.50",
				Name:         "HP LaserJet M404dn",
				Manufacturer: "HP",
				Model:        "HP LaserJet Pro M404dn",
				SysObjectID:  "1.3.6.1.4.1.11.2.3.9.1",
				Serial:       "VNBF123456",
			},
		}, nil
	})

	validReq := httptest.NewRequest(http.MethodPost, "/api/v1/discovery/scan", strings.NewReader(`{"cidr":"192.168.1.0/24","version":"2c","community":"public"}`))
	validReq.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, validReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !containsAll(rec.Body.String(), `"scanned_count":254`, `"192.168.1.50"`, `"HP LaserJet Pro M404dn"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}

	invalidReq := httptest.NewRequest(http.MethodPost, "/api/v1/discovery/scan", strings.NewReader(`{"cidr":"8.8.8.0/24","version":"2c","community":"public"}`))
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for public IP scan, got %d", invalidRec.Code)
	}
}

func TestBulkRegisterPrintersEndpoint(t *testing.T) {
	registrationCh := make(chan repository.PrinterRegistration, 5)
	server, err := NewServer(fakeStore{registrationCh: registrationCh}, "token", "")
	if err != nil {
		t.Fatal(err)
	}
	box, err := security.NewSecretBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	server.SetSecretBox(box)

	payload := `{
		"printers": [
			{"display_name":"Printer 1","address":"192.168.1.10","version":"2c","community":"public","department":"Phòng Kế toán"},
			{"display_name":"Printer 2","address":"192.168.1.11","version":"2c","community":"public","department":"Phòng Kỹ thuật"}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/printers/bulk-register", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !containsAll(rec.Body.String(), `"registered_count":2`, `"failed_count":0`, `"192.168.1.10"`, `"192.168.1.11"`) {
		t.Fatalf("unexpected bulk register body: %s", rec.Body.String())
	}

	// Test with trailing slash: /api/v1/printers/bulk-register/
	reqSlash := httptest.NewRequest(http.MethodPost, "/api/v1/printers/bulk-register/", strings.NewReader(payload))
	reqSlash.Header.Set("Content-Type", "application/json")
	reqSlash.Header.Set("Authorization", "Bearer token")
	recSlash := httptest.NewRecorder()
	server.Handler().ServeHTTP(recSlash, reqSlash)
	if recSlash.Code != http.StatusOK {
		t.Fatalf("status with slash = %d, body = %s", recSlash.Code, recSlash.Body.String())
	}

	// Test OPTIONS preflight
	reqOpt := httptest.NewRequest(http.MethodOptions, "/api/v1/printers/bulk-register", nil)
	recOpt := httptest.NewRecorder()
	server.Handler().ServeHTTP(recOpt, reqOpt)
	if recOpt.Code != http.StatusNoContent {
		t.Fatalf("status for options = %d", recOpt.Code)
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

func TestListJobsEndpoint(t *testing.T) {
	now := time.Now().UTC()
	errCode := "poll_timeout"
	server, err := NewServer(fakeStore{
		jobs: []repository.JobWithDevice{
			{
				ID:         "job-running-1",
				DeviceID:   "dev-1",
				DeviceName: "HP M501dn",
				DeviceIP:   "192.168.1.50",
				Kind:       "status",
				Status:     "running",
				CreatedAt:  now.Add(-2 * time.Minute),
				StartedAt:  &now,
				DurationMs: 120000,
			},
			{
				ID:         "job-failed-2",
				DeviceID:   "dev-2",
				DeviceName: "Brother L5100",
				DeviceIP:   "192.168.1.51",
				Kind:       "counter",
				Status:     "failed",
				ErrorCode:  &errCode,
				CreatedAt:  now.Add(-10 * time.Minute),
				DurationMs: 3000,
			},
		},
	}, "token", "http://localhost:5173")
	if err != nil {
		t.Fatal(err)
	}

	// 1. List all jobs without trailing slash
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if !containsAll(rec.Body.String(), `"total":2`, `"job-running-1"`, `"job-failed-2"`, `"HP M501dn"`, `"poll_timeout"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}

	// 2. Filter by status 'running'
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/jobs?status=running", nil)
	reqFilter.Header.Set("Authorization", "Bearer token")
	recFilter := httptest.NewRecorder()
	server.Handler().ServeHTTP(recFilter, reqFilter)

	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recFilter.Code)
	}
	if !containsAll(recFilter.Body.String(), `"total":1`, `"job-running-1"`) {
		t.Fatalf("expected only running job, got: %s", recFilter.Body.String())
	}
}

type fakeProfileTester struct {
	report *ingestion.ProfileTestReport
	err    error
}

func (f fakeProfileTester) TestProfileForDevice(_ context.Context, _ string, _ profile.Profile) (*ingestion.ProfileTestReport, error) {
	return f.report, f.err
}

func (f fakeProfileTester) TestProfileWithConfig(_ context.Context, _ snmp.Config, _ profile.Profile) (*ingestion.ProfileTestReport, error) {
	return f.report, f.err
}

func TestTestPrinterProfileEndpoint(t *testing.T) {
	store := fakeStore{
		devices: []repository.Device{
			{ID: "d1", DisplayName: "HP M501dn", Model: "HP LaserJet Enterprise M501dn", Manufacturer: "HP", ProfileKey: "hp-laserjet-enterprise-m501dn"},
		},
	}
	server, err := NewServer(store, "token", "")
	if err != nil {
		t.Fatal(err)
	}
	server.SetProfiles([]profile.Profile{
		{ID: "hp-laserjet-enterprise-m501dn", Manufacturer: "HP"},
	})

	val := int64(45200)
	server.SetProfileTester(fakeProfileTester{
		report: &ingestion.ProfileTestReport{
			ProfileID:    "hp-laserjet-enterprise-m501dn",
			Compatible:   true,
			FitnessScore: 100,
			Message:      "Profile tương thích tốt",
			Counters: []ingestion.CounterTestResult{
				{Key: "marker_life", Status: "ok", Value: &val, ActualUnit: "impressions"},
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/printers/d1/test-profile", strings.NewReader(`{"profile_id":"hp-laserjet-enterprise-m501dn"}`))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rec.Code, rec.Body.String())
	}
	if !containsAll(rec.Body.String(), `"compatible":true`, `"fitness_score":100`, `"marker_life"`, `45200`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestTestProfileConnectionEndpoint(t *testing.T) {
	server, err := NewServer(fakeStore{}, "token", "")
	if err != nil {
		t.Fatal(err)
	}
	server.SetProfiles([]profile.Profile{
		{ID: "brother-ads", Manufacturer: "Brother"},
	})

	val := int64(8842)
	server.SetProfileTester(fakeProfileTester{
		report: &ingestion.ProfileTestReport{
			ProfileID:    "brother-ads",
			Compatible:   true,
			FitnessScore: 100,
			Message:      "Profile tương thích tốt",
			Counters: []ingestion.CounterTestResult{
				{Key: "page_count", Status: "ok", Value: &val, ActualUnit: "pages"},
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/snmp/profiles/test", strings.NewReader(`{"address":"10.141.6.40","profile_id":"brother-ads"}`))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rec.Code, rec.Body.String())
	}
	if !containsAll(rec.Body.String(), `"compatible":true`, `"brother-ads"`, `"page_count"`, `8842`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestGetPageUsageReportEndpoint(t *testing.T) {
	server, err := NewServer(fakeStore{}, "token", "")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/pages?from=2026-10-01&to=2026-10-14", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rec.Code, rec.Body.String())
	}
	if !containsAll(rec.Body.String(), `"total_departments":1`, `"Kế toán"`, `"total_print_pages":1250`, `"impressions"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestRelocatePrinterIPEndpoint(t *testing.T) {
	device := repository.Device{
		ID:          "dev-relocate",
		DisplayName: "HP LaserJet",
		Address:     "192.168.1.50",
		Port:        161,
		Status:      "online",
		Serial:      "VNC3K04185",
	}
	server, err := NewServer(fakeStore{devices: []repository.Device{device}, total: 1}, "token", "")
	if err != nil {
		t.Fatal(err)
	}

	body := strings.NewReader(`{"address":"192.168.1.75"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/printers/dev-relocate/relocate-ip", body)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", rec.Code, rec.Body.String())
	}
	if !containsAll(rec.Body.String(), `"success":true`, `"old_address":"192.168.1.50"`, `"new_address":"192.168.1.75"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestDepartmentEndpoints(t *testing.T) {
	server, err := NewServer(fakeStore{}, "token", "")
	if err != nil {
		t.Fatal(err)
	}

	// 1. List departments
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/departments", nil)
	reqList.Header.Set("Authorization", "Bearer token")
	recList := httptest.NewRecorder()
	server.Handler().ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", recList.Code, recList.Body.String())
	}
	if !containsAll(recList.Body.String(), `"code":"KT"`, `"name":"Phòng Kế toán"`) {
		t.Fatalf("unexpected list body: %s", recList.Body.String())
	}

	// 2. Create department
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/departments", strings.NewReader(`{"code":"IT","name":"Phòng Công nghệ thông tin","description":"Tầng 4"}`))
	reqCreate.Header.Set("Authorization", "Bearer token")
	reqCreate.Header.Set("Content-Type", "application/json")
	recCreate := httptest.NewRecorder()
	server.Handler().ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body: %s", recCreate.Code, recCreate.Body.String())
	}

	// 3. Get department
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/departments/dept-1", nil)
	reqGet.Header.Set("Authorization", "Bearer token")
	recGet := httptest.NewRecorder()
	server.Handler().ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", recGet.Code, recGet.Body.String())
	}

	// 4. Update department
	reqUpdate := httptest.NewRequest(http.MethodPut, "/api/v1/departments/dept-1", strings.NewReader(`{"code":"KT","name":"Phòng Kế toán tổng hợp"}`))
	reqUpdate.Header.Set("Authorization", "Bearer token")
	reqUpdate.Header.Set("Content-Type", "application/json")
	recUpdate := httptest.NewRecorder()
	server.Handler().ServeHTTP(recUpdate, reqUpdate)
	if recUpdate.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", recUpdate.Code, recUpdate.Body.String())
	}

	// 5. Delete department
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/departments/dept-1", nil)
	reqDel.Header.Set("Authorization", "Bearer token")
	recDel := httptest.NewRecorder()
	server.Handler().ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", recDel.Code, recDel.Body.String())
	}
	if !containsAll(recDel.Body.String(), `"status":"success"`) {
		t.Fatalf("unexpected delete body: %s", recDel.Body.String())
	}
}
