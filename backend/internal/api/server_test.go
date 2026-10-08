package api

import (
	"context"
	"database/sql"
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
	devices []repository.Device
	total   int
}

type fakePoller struct{}

func (fakePoller) PollDevice(context.Context, string) error { return nil }

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
func (fakeStore) RegisterPrinter(context.Context, repository.PrinterRegistration) error {
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
func (fakeStore) FinishPollingRun(context.Context, string, string, string, time.Time) error {
	return nil
}
func (fakeStore) CreateJob(context.Context, repository.JobRecord) error { return nil }
func (fakeStore) GetJob(context.Context, string) (repository.JobRecord, error) {
	return repository.JobRecord{}, sql.ErrNoRows
}
func (fakeStore) StartJob(context.Context, string, time.Time) error                  { return nil }
func (fakeStore) FinishJob(context.Context, string, string, string, time.Time) error { return nil }

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

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
