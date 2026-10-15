package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"path"
	"sort"

	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"gopkg.in/yaml.v3"

	"npms/backend/internal/counter"
	"npms/backend/internal/discovery"
	"npms/backend/internal/ingestion"
	"npms/backend/internal/profile"
	"npms/backend/internal/repository"
	"npms/backend/internal/security"
	"npms/backend/internal/snmp"
)

type DeviceStore interface {
	ListDevices(context.Context, int, int) ([]repository.Device, error)
	CountDevices(context.Context) (int, error)
	CreateDevice(context.Context, repository.Device) error
	GetDevice(context.Context, string) (repository.Device, error)
	ListCounterReadings(context.Context, string, int, int) ([]repository.CounterReadingRecord, error)
	CountCounterReadings(context.Context, string) (int, error)
	ListDailyCounterUsage(context.Context, string, string, string) ([]repository.DailyUsageRecord, error)
	GetPageUsageReport(context.Context, string, string, string) (*repository.PageUsageReport, error)
	ListPollingRuns(context.Context, string, int, int) ([]repository.PollingRunRecord, error)
	CountPollingRuns(context.Context, string) (int, error)
	CreatePollingRun(context.Context, repository.PollingRun) error
	FinishPollingRun(context.Context, string, string, string, time.Time) error
	CreateJob(context.Context, repository.JobRecord) error
	GetJob(context.Context, string) (repository.JobRecord, error)
	StartJob(context.Context, string, time.Time) error
	FinishJob(context.Context, string, string, string, time.Time) error
	ListJobsWithDevice(context.Context, int, int, string) ([]repository.JobWithDevice, int, error)
	AcquirePollLease(context.Context, string, string, string, time.Time, time.Time) (bool, error)
	ReleasePollLease(context.Context, string, string, string) error
}

type StatusPoller interface {
	PollDevice(context.Context, string) error
}

type CredentialStore interface {
	CreateSNMPCredential(context.Context, repository.SNMPCredential) error
}

type EndpointStore interface {
	CreateDeviceEndpoint(context.Context, repository.DeviceEndpoint) error
}

type RegistrationStore interface {
	RegisterPrinter(context.Context, repository.PrinterRegistration) error
}

type PrinterManagementStore interface {
	UpdatePrinter(context.Context, repository.PrinterUpdate) error
	SoftDeleteDevice(context.Context, string) error
}

type CartridgeStore interface {
	CreateCartridge(context.Context, repository.Cartridge) error
	ListCartridges(context.Context) ([]repository.Cartridge, error)
	GetCartridge(context.Context, string) (repository.Cartridge, error)
	UpdateCartridgeStock(context.Context, string, int, int, int, string, string) error
	AddRefillBottles(context.Context, string, int, string, string) error
	ReplacePrinterCartridge(context.Context, repository.ReplaceCartridgeParams) error
	RefillCartridges(context.Context, string, string, int, string) error
	RefillPrinterCartridge(context.Context, repository.RefillPrinterCartridgeParams) error
	ListCartridgeLogs(context.Context, string, string, int, int) ([]repository.CartridgeLog, error)
	CountCartridgeLogs(context.Context, string, string) (int, error)
}

type CounterSnapshotter interface {
	ReadDevice(context.Context, string) (counter.Reading, repository.CounterDefinition, error)
}

type ProfileTester interface {
	TestProfileForDevice(ctx context.Context, deviceID string, prof profile.Profile) (*ingestion.ProfileTestReport, error)
	TestProfileWithConfig(ctx context.Context, cfg snmp.Config, prof profile.Profile) (*ingestion.ProfileTestReport, error)
}

type DiscoveryProbe func(context.Context, snmp.Config) (discovery.Result, error)
type SubnetScanner func(context.Context, discovery.SubnetScanConfig) ([]discovery.Result, error)

type Server struct {
	store              DeviceStore
	apiToken           string
	allowedOrigin      string
	poller             StatusPoller
	frontendFS         fs.FS
	profiles           []profile.Profile
	secretBox          *security.SecretBox
	discovery          DiscoveryProbe
	subnetScanner      SubnetScanner
	counterSnapshotter CounterSnapshotter
	profileTester      ProfileTester
	backupManager      DatabaseBackupManager
	pollTimeout        time.Duration
}

func NewServer(store DeviceStore, apiToken, allowedOrigin string) (*Server, error) {
	if store == nil {
		return nil, errors.New("device store is required")
	}
	if allowedOrigin == "" {
		allowedOrigin = "http://localhost:5173"
	}
	return &Server{store: store, apiToken: apiToken, allowedOrigin: allowedOrigin, pollTimeout: 30 * time.Second}, nil
}

func (s *Server) SetStatusPoller(poller StatusPoller) { s.poller = poller }

func (s *Server) SetFrontendFS(frontendFS fs.FS) { s.frontendFS = frontendFS }

func (s *Server) SetProfiles(profiles []profile.Profile) {
	s.profiles = append([]profile.Profile(nil), profiles...)
}

func (s *Server) SetSecretBox(box *security.SecretBox) { s.secretBox = box }

func (s *Server) SetDiscoveryProbe(probe DiscoveryProbe) { s.discovery = probe }

func (s *Server) SetSubnetScanner(scanner SubnetScanner) { s.subnetScanner = scanner }

func (s *Server) SetCounterSnapshotter(snapshotter CounterSnapshotter) {
	s.counterSnapshotter = snapshotter
}

func (s *Server) SetProfileTester(tester ProfileTester) {
	s.profileTester = tester
}

func (s *Server) SetDatabaseBackupManager(mgr DatabaseBackupManager) {
	s.backupManager = mgr
}

func (s *Server) SetPollTimeout(timeout time.Duration) {
	if timeout > 0 {
		s.pollTimeout = timeout
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/v1/health", s.healthHandler())
	mux.Handle("/api/v1/health/live", s.healthHandler())
	mux.Handle("/api/v1/health/ready", s.healthHandler())
	mux.Handle("/api/v1/snmp/profiles", s.profileHandler())
	mux.HandleFunc("/api/v1/snmp/profiles/test", s.testProfileConnection)
	mux.HandleFunc("/api/v1/discovery/probe", s.probeDiscovery)
	mux.HandleFunc("/api/v1/discovery/scan", s.scanSubnet)
	mux.HandleFunc("/api/v1/printers/register", s.registerPrinter)
	mux.HandleFunc("/api/v1/printers/register/", s.registerPrinter)
	mux.HandleFunc("/api/v1/printers/bulk-register", s.bulkRegisterPrinters)
	mux.HandleFunc("/api/v1/printers/bulk-register/", s.bulkRegisterPrinters)
	mux.Handle("/api/v1/printers", s.printerCollectionHandler())
	mux.HandleFunc("/api/v1/printers/", s.getPrinter)
	mux.HandleFunc("/api/v1/jobs", s.handleJobs)
	mux.HandleFunc("/api/v1/jobs/", s.handleJobs)
	mux.HandleFunc("/api/v1/reports/pages", s.getPageUsageReport)
	mux.Handle("/api/v1/cartridges", s.cartridgeListHandler())
	mux.Handle("/api/v1/cartridges/stock", s.cartridgeStockHandler())
	mux.Handle("/api/v1/cartridges/stock/refill-bottles", s.cartridgeStockHandler())
	cartridgeActionHandler := s.cartridgeActionHandler()
	mux.Handle("/api/v1/cartridges/replace", cartridgeActionHandler)
	mux.Handle("/api/v1/cartridges/refill", cartridgeActionHandler)
	mux.Handle("/api/v1/cartridges/refill-printer", cartridgeActionHandler)
	mux.Handle("/api/v1/cartridges/logs", s.cartridgeLogsHandler())
	mux.HandleFunc("/api/v1/backup/printers", s.handlePrintersBackup)
	mux.HandleFunc("/api/v1/backup/printers/restore", s.handlePrintersRestore)
	mux.HandleFunc("/api/v1/backup/database", s.handleDatabaseBackup)
	mux.HandleFunc("/api/v1/backup/database/restore", s.handleDatabaseRestore)
	var static http.Handler
	if s.frontendFS != nil {
		fileServer := http.FileServer(http.FS(s.frontendFS))
		static = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
			if cleanPath == "" || cleanPath == "." {
				r.URL.Path = "/"
				fileServer.ServeHTTP(w, r)
				return
			}
			f, err := s.frontendFS.Open(cleanPath)
			if err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
			if path.Ext(cleanPath) == "" {
				r.URL.Path = "/"
				fileServer.ServeHTTP(w, r)
				return
			}
			fileServer.ServeHTTP(w, r)
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.allowedOrigin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || static == nil {
			mux.ServeHTTP(w, r)
			return
		}
		static.ServeHTTP(w, r)
	})
}

func (s *Server) listProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}
	if s.apiToken != "" && r.Header.Get("Authorization") != "Bearer "+s.apiToken {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	data := make([]profileResponse, 0, len(s.profiles))
	for _, item := range s.profiles {
		keys := make([]string, 0, len(item.Counters))
		for key := range item.Counters {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		data = append(data, profileResponse{ID: item.ID, Version: item.Version, Manufacturer: item.Manufacturer, VerificationStatus: string(item.VerificationStatus), CounterKeys: keys})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "total": len(data)})
}

type testProfileConnectionRequest struct {
	Address        string `json:"address"`
	Port           int    `json:"port"`
	Version        string `json:"version"`
	Community      string `json:"community"`
	Username       string `json:"username"`
	AuthProtocol   string `json:"auth_protocol"`
	AuthPassphrase string `json:"auth_passphrase"`
	PrivProtocol   string `json:"priv_protocol"`
	PrivPassphrase string `json:"priv_passphrase"`
	ProfileID      string `json:"profile_id"`
}

func (s *Server) testProfileConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", s.allowedOrigin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	if s.profileTester == nil {
		writeError(w, http.StatusServiceUnavailable, "tester_unavailable", "profile tester is not configured")
		return
	}

	var input testProfileConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON: "+err.Error())
		return
	}

	address := strings.TrimSpace(input.Address)
	if address == "" {
		writeError(w, http.StatusBadRequest, "invalid_address", "address is required")
		return
	}

	profileID := strings.TrimSpace(input.ProfileID)
	if profileID == "" {
		writeError(w, http.StatusBadRequest, "invalid_profile_id", "profile_id is required")
		return
	}

	var selectedProfile profile.Profile
	found := false
	for _, p := range s.profiles {
		if p.ID == profileID {
			selectedProfile = p
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusBadRequest, "profile_not_found", fmt.Sprintf("profile %q was not found", profileID))
		return
	}

	port := input.Port
	if port == 0 {
		port = 161
	}
	version := strings.TrimSpace(input.Version)
	if version == "" {
		version = "2c"
	}
	community := input.Community
	if version == "2c" && community == "" {
		community = "public"
	}

	cfg := snmp.Config{
		Host:           address,
		Port:           uint16(port),
		Version:        snmp.Version(version),
		Community:      community,
		Username:       strings.TrimSpace(input.Username),
		AuthProtocol:   strings.TrimSpace(input.AuthProtocol),
		AuthPassphrase: input.AuthPassphrase,
		PrivProtocol:   strings.TrimSpace(input.PrivProtocol),
		PrivPassphrase: input.PrivPassphrase,
		TimeoutSeconds: 4,
		Retries:        1,
		MaxRepetitions: 25,
	}

	report, err := s.profileTester.TestProfileWithConfig(r.Context(), cfg, selectedProfile)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "test_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, report)
}

type discoveryProbeRequest struct {
	Address        string `json:"address"`
	Port           int    `json:"port"`
	Version        string `json:"version"`
	Community      string `json:"community"`
	Username       string `json:"username"`
	AuthProtocol   string `json:"auth_protocol"`
	AuthPassphrase string `json:"auth_passphrase"`
	PrivProtocol   string `json:"priv_protocol"`
	PrivPassphrase string `json:"priv_passphrase"`
	Timeout        int    `json:"timeout"`
}

func (s *Server) probeDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	if s.discovery == nil {
		writeError(w, http.StatusServiceUnavailable, "discovery_unavailable", "discovery is not configured")
		return
	}
	var input discoveryProbeRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	address := strings.TrimSpace(input.Address)
	if net.ParseIP(address) == nil {
		writeError(w, http.StatusBadRequest, "invalid_address", "discovery address must be a single IP address")
		return
	}
	port := input.Port
	if port == 0 {
		port = 161
	}
	timeout := input.Timeout
	if timeout == 0 {
		timeout = 3
	}
	if port < 1 || port > 65535 || timeout < 1 || timeout > 30 {
		writeError(w, http.StatusBadRequest, "invalid_discovery_options", "port must be 1-65535 and timeout must be 1-30 seconds")
		return
	}
	secret := security.SNMPSecret{Version: strings.TrimSpace(input.Version), Community: input.Community, Username: strings.TrimSpace(input.Username), AuthProtocol: strings.TrimSpace(input.AuthProtocol), AuthPassphrase: input.AuthPassphrase, PrivProtocol: strings.TrimSpace(input.PrivProtocol), PrivPassphrase: input.PrivPassphrase}
	if err := secret.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_credential", err.Error())
		return
	}
	result, err := s.discovery(r.Context(), snmp.Config{Host: address, Port: uint16(port), Version: snmp.Version(secret.Version), Community: secret.Community, Username: secret.Username, AuthProtocol: secret.AuthProtocol, AuthPassphrase: secret.AuthPassphrase, PrivProtocol: secret.PrivProtocol, PrivPassphrase: secret.PrivPassphrase, TimeoutSeconds: timeout, Retries: 1, MaxRepetitions: 25})
	if err != nil {
		writeError(w, http.StatusBadGateway, "discovery_failed", "could not read the printer identity")
		return
	}
	if len(s.profiles) > 0 {
		if selected, resolveErr := (profile.Resolver{Profiles: s.profiles}).Resolve(profile.DeviceIdentity{
			SysObjectID:  result.SysObjectID,
			Manufacturer: result.Manufacturer,
			Model:        result.Model,
		}, ""); resolveErr == nil {
			result.MatchedProfileID = selected.ID
		}
	}
	writeJSON(w, http.StatusOK, result)
}

type subnetScanRequest struct {
	CIDR           string `json:"cidr"`
	StartIP        string `json:"start_ip"`
	EndIP          string `json:"end_ip"`
	Port           int    `json:"port"`
	Version        string `json:"version"`
	Community      string `json:"community"`
	Username       string `json:"username"`
	AuthProtocol   string `json:"auth_protocol"`
	AuthPassphrase string `json:"auth_passphrase"`
	PrivProtocol   string `json:"priv_protocol"`
	PrivPassphrase string `json:"priv_passphrase"`
	TimeoutMS      int    `json:"timeout_ms"`
	Concurrency    int    `json:"concurrency"`
}

type discoveredPrinterItem struct {
	Address           string                  `json:"address"`
	Port              int                     `json:"port"`
	Name              string                  `json:"name"`
	Description       string                  `json:"description"`
	SysObjectID       string                  `json:"sys_object_id"`
	Serial            string                  `json:"serial"`
	Location          string                  `json:"location,omitempty"`
	Manufacturer      string                  `json:"manufacturer,omitempty"`
	Model             string                  `json:"model,omitempty"`
	MatchedProfileID  string                  `json:"matched_profile_id,omitempty"`
	Markers           []discoveryMarkerOutput `json:"markers,omitempty"`
	AlreadyRegistered bool                    `json:"already_registered"`
}

type discoveryMarkerOutput struct {
	Instance string `json:"instance"`
	Life     int64  `json:"life_count"`
	Unit     int64  `json:"counter_unit"`
	UnitName string `json:"counter_unit_name,omitempty"`
}

func (s *Server) scanSubnet(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", s.allowedOrigin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	if s.subnetScanner == nil {
		writeError(w, http.StatusServiceUnavailable, "scan_unavailable", "subnet scanner is not configured")
		return
	}

	var input subnetScanRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON: "+err.Error())
		return
	}

	ips, err := discovery.GenerateIPList(input.CIDR, input.StartIP, input.EndIP)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_ip_range", err.Error())
		return
	}

	port := input.Port
	if port == 0 {
		port = 161
	}
	version := strings.TrimSpace(input.Version)
	if version == "" {
		version = "2c"
	}
	timeoutMS := input.TimeoutMS
	if timeoutMS <= 0 {
		timeoutMS = 1000
	}
	concurrency := input.Concurrency
	if concurrency <= 0 {
		concurrency = 25
	}

	secret := security.SNMPSecret{
		Version:        version,
		Community:      input.Community,
		Username:       strings.TrimSpace(input.Username),
		AuthProtocol:   strings.TrimSpace(input.AuthProtocol),
		AuthPassphrase: input.AuthPassphrase,
		PrivProtocol:   strings.TrimSpace(input.PrivProtocol),
		PrivPassphrase: input.PrivPassphrase,
	}
	if err := secret.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_credential", err.Error())
		return
	}

	scanCfg := discovery.SubnetScanConfig{
		IPs:            ips,
		Port:           uint16(port),
		Version:        snmp.Version(secret.Version),
		Community:      secret.Community,
		Username:       secret.Username,
		AuthProtocol:   secret.AuthProtocol,
		AuthPassphrase: secret.AuthPassphrase,
		PrivProtocol:   secret.PrivProtocol,
		PrivPassphrase: secret.PrivPassphrase,
		Timeout:        time.Duration(timeoutMS) * time.Millisecond,
		Concurrency:    concurrency,
	}

	results, err := s.subnetScanner(r.Context(), scanCfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "scan_failed", err.Error())
		return
	}

	// Identify already registered IPs to guide the user in the UI
	registeredIPs := make(map[string]bool)
	if existingDevices, err := s.store.ListDevices(r.Context(), 1000, 0); err == nil {
		for _, dev := range existingDevices {
			if dev.Address != "" {
				registeredIPs[dev.Address] = true
			}
		}
	}

	items := make([]discoveredPrinterItem, 0, len(results))
	for _, res := range results {
		matchedProfile := ""
		if len(s.profiles) > 0 {
			if selected, resolveErr := (profile.Resolver{Profiles: s.profiles}).Resolve(profile.DeviceIdentity{
				SysObjectID:  res.SysObjectID,
				Manufacturer: res.Manufacturer,
				Model:        res.Model,
			}, ""); resolveErr == nil {
				matchedProfile = selected.ID
			}
		}

		markers := make([]discoveryMarkerOutput, 0, len(res.Markers))
		for _, m := range res.Markers {
			unitName := m.UnitName
			if unitName == "" {
				unitName = snmp.CounterUnitName(m.Unit)
			}
			markers = append(markers, discoveryMarkerOutput{
				Instance: m.Instance,
				Life:     m.Life,
				Unit:     int64(m.Unit),
				UnitName: unitName,
			})
		}

		items = append(items, discoveredPrinterItem{
			Address:           res.Address,
			Port:              port,
			Name:              res.Name,
			Description:       res.Description,
			SysObjectID:       res.SysObjectID,
			Serial:            res.Serial,
			Location:          res.Location,
			Manufacturer:      res.Manufacturer,
			Model:             res.Model,
			MatchedProfileID:  matchedProfile,
			Markers:           markers,
			AlreadyRegistered: registeredIPs[res.Address],
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"scanned_count":       len(ips),
		"discovered_printers": items,
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "time": time.Now().UTC().Format(time.RFC3339Nano)})
}

func (s *Server) listPrinters(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.createPrinter(w, r)
		return
	}
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}
	if s.apiToken != "" && r.Header.Get("Authorization") != "Bearer "+s.apiToken {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	limit, offset, err := pagination(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	devices, err := s.store.ListDevices(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "device_list_failed", "could not list printers")
		return
	}
	total, err := s.store.CountDevices(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "device_count_failed", "could not count printers")
		return
	}
	data := make([]printerResponse, 0, len(devices))
	for _, device := range devices {
		data = append(data, toPrinterResponse(device))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "limit": limit, "offset": offset, "total": total})
}

type createPrinterRequest struct {
	AssetCode    string `json:"asset_code"`
	DisplayName  string `json:"display_name"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Serial       string `json:"serial"`
	SysObjectID  string `json:"sys_object_id"`
	Department   string `json:"department"`
}

func (s *Server) createPrinter(w http.ResponseWriter, r *http.Request) {
	if s.apiToken != "" && r.Header.Get("Authorization") != "Bearer "+s.apiToken {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	var input createPrinterRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if strings.TrimSpace(input.DisplayName) == "" || len(input.DisplayName) > 200 {
		writeError(w, http.StatusBadRequest, "invalid_display_name", "display_name is required and must be at most 200 characters")
		return
	}
	device := repository.Device{ID: newJobID(), AssetCode: strings.TrimSpace(input.AssetCode), DisplayName: strings.TrimSpace(input.DisplayName), Manufacturer: strings.TrimSpace(input.Manufacturer), Model: strings.TrimSpace(input.Model), Serial: strings.TrimSpace(input.Serial), SysObjectID: strings.TrimSpace(input.SysObjectID), Department: strings.TrimSpace(input.Department), Status: "unknown"}
	if err := s.store.CreateDevice(r.Context(), device); err != nil {
		writeError(w, http.StatusConflict, "device_create_failed", "could not create printer")
		return
	}
	writeJSON(w, http.StatusCreated, toPrinterResponse(device))
}

func (s *Server) getPrinter(w http.ResponseWriter, r *http.Request) {
	cleanID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/printers/"), "/")
	if cleanID == "bulk-register" {
		s.bulkRegisterPrinters(w, r)
		return
	}
	if cleanID == "register" {
		s.registerPrinter(w, r)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/printers/")
	if strings.HasSuffix(id, "/credentials") {
		s.createCredential(w, r, strings.TrimSuffix(id, "/credentials"))
		return
	}
	if strings.HasSuffix(id, "/endpoints") {
		s.createEndpoint(w, r, strings.TrimSuffix(id, "/endpoints"))
		return
	}
	if strings.HasSuffix(id, "/usage") {
		s.listUsage(w, r, strings.TrimSuffix(id, "/usage"))
		return
	}
	if strings.HasSuffix(id, "/poll") {
		s.startPoll(w, r, strings.TrimSuffix(id, "/poll"))
		return
	}
	if strings.HasSuffix(id, "/counters") {
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w)
			return
		}
		if !s.authorized(r) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
			return
		}
		s.listCounters(w, r, strings.TrimSuffix(id, "/counters"))
		return
	}
	if strings.HasSuffix(id, "/polling-runs") {
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w)
			return
		}
		if !s.authorized(r) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
			return
		}
		s.listPollingRuns(w, r, strings.TrimSuffix(id, "/polling-runs"))
		return
	}
	if strings.HasSuffix(id, "/test-profile") {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", s.allowedOrigin)
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w)
			return
		}
		if !s.authorized(r) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
			return
		}
		s.testPrinterProfile(w, r, strings.TrimSuffix(id, "/test-profile"))
		return
	}
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "not_found", "printer was not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if !s.authorized(r) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
			return
		}
		device, err := s.store.GetDevice(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "printer was not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "device_get_failed", "could not get printer")
			return
		}
		writeJSON(w, http.StatusOK, toPrinterResponse(device))
	case http.MethodPut:
		s.updatePrinter(w, r, id)
	case http.MethodDelete:
		s.deletePrinter(w, r, id)
	default:
		s.methodNotAllowed(w)
	}
}

type testProfileRequest struct {
	ProfileID string `json:"profile_id"`
}

func (s *Server) testPrinterProfile(w http.ResponseWriter, r *http.Request, printerID string) {
	if s.profileTester == nil {
		writeError(w, http.StatusServiceUnavailable, "tester_unavailable", "profile tester is not configured")
		return
	}

	var input testProfileRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&input)
	}

	var selectedProfile profile.Profile
	profileID := strings.TrimSpace(input.ProfileID)

	if profileID != "" {
		found := false
		for _, p := range s.profiles {
			if p.ID == profileID {
				selectedProfile = p
				found = true
				break
			}
		}
		if !found {
			writeError(w, http.StatusBadRequest, "profile_not_found", fmt.Sprintf("profile %q was not found", profileID))
			return
		}
	} else {
		device, err := s.store.GetDevice(r.Context(), printerID)
		if err != nil {
			writeError(w, http.StatusNotFound, "printer_not_found", "printer was not found")
			return
		}
		if device.ProfileKey != "" {
			for _, p := range s.profiles {
				if p.ID == device.ProfileKey {
					selectedProfile = p
					break
				}
			}
		}
		if selectedProfile.ID == "" && len(s.profiles) > 0 {
			res, err := (profile.Resolver{Profiles: s.profiles}).Resolve(profile.DeviceIdentity{
				SysObjectID:  device.SysObjectID,
				Manufacturer: device.Manufacturer,
				Model:        device.Model,
			}, "")
			if err == nil {
				selectedProfile = res
			}
		}
		if selectedProfile.ID == "" {
			writeError(w, http.StatusBadRequest, "profile_not_resolved", "could not determine profile for printer; please specify profile_id")
			return
		}
	}

	report, err := s.profileTester.TestProfileForDevice(r.Context(), printerID, selectedProfile)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "test_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, report)
}

type createCredentialRequest struct {
	Version        string `json:"version"`
	Community      string `json:"community"`
	Username       string `json:"username"`
	AuthProtocol   string `json:"auth_protocol"`
	AuthPassphrase string `json:"auth_passphrase"`
	PrivProtocol   string `json:"priv_protocol"`
	PrivPassphrase string `json:"priv_passphrase"`
}

func (s *Server) createCredential(w http.ResponseWriter, r *http.Request, deviceID string) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	store, ok := s.store.(CredentialStore)
	if !ok || s.secretBox == nil {
		writeError(w, http.StatusServiceUnavailable, "credential_store_unavailable", "credential storage is not configured")
		return
	}
	if deviceID == "" || strings.Contains(deviceID, "/") {
		writeError(w, http.StatusNotFound, "not_found", "printer was not found")
		return
	}
	if _, err := s.store.GetDevice(r.Context(), deviceID); errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "printer was not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "printer_get_failed", "could not get printer")
		return
	}
	var input createCredentialRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	secret := security.SNMPSecret{Version: strings.TrimSpace(input.Version), Community: input.Community, Username: strings.TrimSpace(input.Username), AuthProtocol: strings.TrimSpace(input.AuthProtocol), AuthPassphrase: input.AuthPassphrase, PrivProtocol: strings.TrimSpace(input.PrivProtocol), PrivPassphrase: input.PrivPassphrase}
	encoded, err := security.EncodeSNMPSecret(secret)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_credential", err.Error())
		return
	}
	ciphertext, err := s.secretBox.Encrypt(encoded)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "credential_encryption_failed", "could not encrypt credential")
		return
	}
	credentialID := newJobID()
	if err := store.CreateSNMPCredential(r.Context(), repository.SNMPCredential{ID: credentialID, Version: secret.Version, EncryptedSecretMaterial: ciphertext, SecurityMetadata: fmt.Sprintf(`{"version":%q}`, secret.Version)}); err != nil {
		writeError(w, http.StatusConflict, "credential_create_failed", "could not create credential")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": credentialID, "version": secret.Version})
}

type registerPrinterRequest struct {
	DisplayName    string `json:"display_name"`
	AssetCode      string `json:"asset_code"`
	Manufacturer   string `json:"manufacturer"`
	Model          string `json:"model"`
	Serial         string `json:"serial"`
	SysObjectID    string `json:"sys_object_id"`
	Department     string `json:"department"`
	ProfileID      string `json:"profile_id"`
	Address        string `json:"address"`
	Port           int    `json:"port"`
	Version        string `json:"version"`
	Community      string `json:"community"`
	Username       string `json:"username"`
	AuthProtocol   string `json:"auth_protocol"`
	AuthPassphrase string `json:"auth_passphrase"`
	PrivProtocol   string `json:"priv_protocol"`
	PrivPassphrase string `json:"priv_passphrase"`
}

type registerPrinterResult struct {
	Device                 repository.Device
	CredentialID           string
	EndpointID             string
	ProfileID              string
	CounterDefinitionCount int
	PollJobID              string
	PollStatus             string
}

func (s *Server) registerPrinterInternal(ctx context.Context, input registerPrinterRequest) (*registerPrinterResult, error) {
	store, ok := s.store.(RegistrationStore)
	if !ok || s.secretBox == nil {
		return nil, fmt.Errorf("printer registration is not configured")
	}

	address := strings.TrimSpace(input.Address)
	if address == "" {
		return nil, fmt.Errorf("address is required")
	}

	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = strings.TrimSpace(input.Manufacturer + " " + input.Model)
		if displayName == "" {
			displayName = "Máy in (" + address + ")"
		}
	}
	if len(displayName) > 200 {
		displayName = displayName[:200]
	}

	port := input.Port
	if port == 0 {
		port = 161
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("port must be between 1 and 65535")
	}

	version := strings.TrimSpace(input.Version)
	if version == "" {
		version = "2c"
	}
	community := input.Community
	if version == "2c" && community == "" {
		community = "public"
	}

	secret := security.SNMPSecret{
		Version:        version,
		Community:      community,
		Username:       strings.TrimSpace(input.Username),
		AuthProtocol:   strings.TrimSpace(input.AuthProtocol),
		AuthPassphrase: input.AuthPassphrase,
		PrivProtocol:   strings.TrimSpace(input.PrivProtocol),
		PrivPassphrase: input.PrivPassphrase,
	}
	encoded, err := security.EncodeSNMPSecret(secret)
	if err != nil {
		return nil, fmt.Errorf("invalid credential: %w", err)
	}
	ciphertext, err := s.secretBox.Encrypt(encoded)
	if err != nil {
		return nil, fmt.Errorf("could not encrypt credential: %w", err)
	}

	deviceID, credentialID, endpointID := newJobID(), newJobID(), newJobID()
	registration := repository.PrinterRegistration{
		Device: repository.Device{
			ID:           deviceID,
			AssetCode:    strings.TrimSpace(input.AssetCode),
			DisplayName:  displayName,
			Manufacturer: strings.TrimSpace(input.Manufacturer),
			Model:        strings.TrimSpace(input.Model),
			Serial:       strings.TrimSpace(input.Serial),
			SysObjectID:  strings.TrimSpace(input.SysObjectID),
			Department:   strings.TrimSpace(input.Department),
			Address:      address,
			Port:         uint16(port),
			SNMPVersion:  secret.Version,
			Status:       "unknown",
		},
		Credential: repository.SNMPCredential{
			ID:                      credentialID,
			Version:                 secret.Version,
			EncryptedSecretMaterial: ciphertext,
			SecurityMetadata:        fmt.Sprintf(`{"version":%q}`, secret.Version),
		},
		Endpoint: repository.DeviceEndpoint{
			ID:           endpointID,
			DeviceID:     deviceID,
			Address:      address,
			Protocol:     "snmp",
			Port:         uint16(port),
			CredentialID: credentialID,
			IsPrimary:    true,
		},
	}

	var discoveredInstance string
	var probeMarkers []snmp.MarkerRow
	if s.discovery != nil && (registration.Device.Manufacturer == "" || registration.Device.Model == "" || registration.Device.Serial == "" || registration.Device.SysObjectID == "" || registration.Device.Department == "") {
		probeCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		probeRes, probeErr := s.discovery(probeCtx, snmp.Config{
			Host:           address,
			Port:           uint16(port),
			Version:        snmp.Version(secret.Version),
			Community:      secret.Community,
			Username:       secret.Username,
			AuthProtocol:   secret.AuthProtocol,
			AuthPassphrase: secret.AuthPassphrase,
			PrivProtocol:   secret.PrivProtocol,
			PrivPassphrase: secret.PrivPassphrase,
			TimeoutSeconds: 3,
			Retries:        1,
			MaxRepetitions: 25,
		})
		cancel()
		if probeErr == nil {
			if registration.Device.Manufacturer == "" {
				registration.Device.Manufacturer = probeRes.Manufacturer
			}
			if registration.Device.Model == "" {
				registration.Device.Model = probeRes.Model
			}
			if registration.Device.Serial == "" {
				registration.Device.Serial = probeRes.Serial
			}
			if registration.Device.SysObjectID == "" {
				registration.Device.SysObjectID = probeRes.SysObjectID
			}
			if registration.Device.Department == "" && probeRes.Location != "" {
				registration.Device.Department = probeRes.Location
			}
			registration.Device.Status = "online"
			now := time.Now().UTC()
			registration.Device.FirstSeenAt = now
			registration.Device.LastSeenAt = now
			probeMarkers = probeRes.Markers
			if len(probeMarkers) == 1 {
				discoveredInstance = probeMarkers[0].Instance
			}
		}
	}

	if len(s.profiles) > 0 {
		selected, resolveErr := (profile.Resolver{Profiles: s.profiles}).Resolve(profile.DeviceIdentity{
			SysObjectID:  registration.Device.SysObjectID,
			Manufacturer: registration.Device.Manufacturer,
			Model:        registration.Device.Model,
		}, strings.TrimSpace(input.ProfileID))
		if resolveErr != nil {
			return nil, fmt.Errorf("profile resolution failed: %w", resolveErr)
		}
		content, marshalErr := yaml.Marshal(selected)
		if marshalErr != nil {
			return nil, fmt.Errorf("could not serialize selected profile: %w", marshalErr)
		}
		digest := sha256.Sum256(content)
		profileID := selected.ID + "@" + strconv.Itoa(selected.Version)
		registration.Profile = &repository.SNMPProfile{
			ID:                 profileID,
			ProfileKey:         selected.ID,
			Version:            selected.Version,
			SchemaVersion:      selected.SchemaVersion,
			Content:            string(content),
			Checksum:           hex.EncodeToString(digest[:]),
			VerificationStatus: string(selected.VerificationStatus),
		}
		registration.Device.ProfileKey = selected.ID
		registration.ProfileExplicit = strings.TrimSpace(input.ProfileID) != ""
		registration.CounterDefinitions = make([]repository.CounterDefinition, 0, len(selected.Counters))
		for key, definition := range selected.Counters {
			instance := ""
			if definition.Mode == "walk" {
				if discoveredInstance != "" {
					instance = discoveredInstance
				} else {
					for _, m := range probeMarkers {
						if m.UnitName == definition.Unit || snmp.CounterUnitName(m.Unit) == definition.Unit {
							instance = m.Instance
							break
						}
					}
					if instance == "" && len(probeMarkers) > 0 {
						instance = probeMarkers[0].Instance
					}
				}
			}
			registration.CounterDefinitions = append(registration.CounterDefinitions, repository.CounterDefinition{
				ID:                    deviceID + ":" + key,
				DeviceID:              deviceID,
				Key:                   key,
				SourceProtocol:        "snmp",
				OID:                   definition.ValueColumn,
				Instance:              instance,
				Unit:                  definition.Unit,
				SemanticType:          definition.SemanticType,
				Scope:                 definition.Scope,
				Mode:                  definition.Mode,
				UnitOID:               definition.UnitColumn,
				Selection:             definition.Selection,
				Aggregation:           definition.Aggregation,
				RequireUnitValidation: definition.RequireUnitValidation,
				Verified:              selected.VerificationStatus == profile.VerificationVerified,
			})
		}
	}

	if err := store.RegisterPrinter(ctx, registration); err != nil {
		return nil, fmt.Errorf("could not register printer: %w", err)
	}

	pollJobID := ""
	pollStatus := "not_started"
	if s.poller != nil {
		if pollJobID, err = s.queuePoll(ctx, deviceID); err == nil {
			pollStatus = "queued"
		} else {
			pollJobID = ""
		}
	}
	profileID := ""
	if registration.Profile != nil {
		profileID = registration.Profile.ID
	}

	return &registerPrinterResult{
		Device:                 registration.Device,
		CredentialID:           credentialID,
		EndpointID:             endpointID,
		ProfileID:              profileID,
		CounterDefinitionCount: len(registration.CounterDefinitions),
		PollJobID:              pollJobID,
		PollStatus:             pollStatus,
	}, nil
}

func (s *Server) registerPrinter(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	var input registerPrinterRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	res, err := s.registerPrinterInternal(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "registration_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"printer":                  toPrinterResponse(res.Device),
		"credential_id":            res.CredentialID,
		"endpoint_id":              res.EndpointID,
		"profile_id":               nullable(res.ProfileID),
		"counter_definition_count": res.CounterDefinitionCount,
		"poll_job_id":              nullable(res.PollJobID),
		"poll_status":              res.PollStatus,
	})
}

type bulkRegisterRequest struct {
	Printers []registerPrinterRequest `json:"printers"`
}

type bulkRegisterItemResult struct {
	Address  string `json:"address"`
	Status   string `json:"status"`
	DeviceID string `json:"device_id,omitempty"`
	Error    string `json:"error,omitempty"`
}

func (s *Server) bulkRegisterPrinters(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	var input bulkRegisterRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON: "+err.Error())
		return
	}
	if len(input.Printers) == 0 {
		writeError(w, http.StatusBadRequest, "empty_list", "printers list cannot be empty")
		return
	}

	results := make([]bulkRegisterItemResult, 0, len(input.Printers))
	registeredCount := 0
	failedCount := 0

	for _, p := range input.Printers {
		res, err := s.registerPrinterInternal(r.Context(), p)
		if err != nil {
			failedCount++
			results = append(results, bulkRegisterItemResult{
				Address: p.Address,
				Status:  "error",
				Error:   err.Error(),
			})
		} else {
			registeredCount++
			results = append(results, bulkRegisterItemResult{
				Address:  p.Address,
				Status:   "success",
				DeviceID: res.Device.ID,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"registered_count": registeredCount,
		"failed_count":     failedCount,
		"results":          results,
	})
}

type updatePrinterRequest struct {
	DisplayName    string `json:"display_name"`
	AssetCode      string `json:"asset_code"`
	Manufacturer   string `json:"manufacturer"`
	Model          string `json:"model"`
	Serial         string `json:"serial"`
	SysObjectID    string `json:"sys_object_id"`
	Department     string `json:"department"`
	ProfileID      string `json:"profile_id"`
	Address        string `json:"address"`
	Port           int    `json:"port"`
	Version        string `json:"version"`
	Community      string `json:"community"`
	Username       string `json:"username"`
	AuthProtocol   string `json:"auth_protocol"`
	AuthPassphrase string `json:"auth_passphrase"`
	PrivProtocol   string `json:"priv_protocol"`
	PrivPassphrase string `json:"priv_passphrase"`
}

func (s *Server) updatePrinter(w http.ResponseWriter, r *http.Request, deviceID string) {
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	store, ok := s.store.(PrinterManagementStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "printer_update_unavailable", "printer update is not configured")
		return
	}
	existing, err := s.store.GetDevice(r.Context(), deviceID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "printer was not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "device_get_failed", "could not get printer")
		return
	}
	var input updatePrinterRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" || len(displayName) > 200 {
		writeError(w, http.StatusBadRequest, "invalid_display_name", "display_name is required and must be at most 200 characters")
		return
	}
	address := strings.TrimSpace(input.Address)
	port := input.Port
	if port == 0 {
		if existing.Port > 0 {
			port = int(existing.Port)
		} else {
			port = 161
		}
	}
	if port < 1 || port > 65535 {
		writeError(w, http.StatusBadRequest, "invalid_endpoint", "port must be between 1 and 65535")
		return
	}

	var updatedCred *repository.SNMPCredential
	hasNewCred := strings.TrimSpace(input.Community) != "" || strings.TrimSpace(input.Username) != ""
	var secret security.SNMPSecret
	if hasNewCred {
		if s.secretBox == nil {
			writeError(w, http.StatusServiceUnavailable, "credential_store_unavailable", "credential storage is not configured")
			return
		}
		version := strings.TrimSpace(input.Version)
		if version == "" {
			if existing.SNMPVersion != "" {
				version = existing.SNMPVersion
			} else {
				version = "2c"
			}
		}
		secret = security.SNMPSecret{
			Version:        version,
			Community:      input.Community,
			Username:       strings.TrimSpace(input.Username),
			AuthProtocol:   strings.TrimSpace(input.AuthProtocol),
			AuthPassphrase: input.AuthPassphrase,
			PrivProtocol:   strings.TrimSpace(input.PrivProtocol),
			PrivPassphrase: input.PrivPassphrase,
		}
		encoded, encErr := security.EncodeSNMPSecret(secret)
		if encErr != nil {
			writeError(w, http.StatusBadRequest, "invalid_credential", encErr.Error())
			return
		}
		ciphertext, encErr := s.secretBox.Encrypt(encoded)
		if encErr != nil {
			writeError(w, http.StatusInternalServerError, "credential_encryption_failed", "could not encrypt credential")
			return
		}
		updatedCred = &repository.SNMPCredential{
			ID:                      newJobID(),
			Version:                 secret.Version,
			EncryptedSecretMaterial: ciphertext,
			SecurityMetadata:        fmt.Sprintf(`{"version":%q}`, secret.Version),
		}
	}

	update := repository.PrinterUpdate{
		Device: repository.Device{
			ID:           deviceID,
			AssetCode:    strings.TrimSpace(input.AssetCode),
			DisplayName:  displayName,
			Manufacturer: strings.TrimSpace(input.Manufacturer),
			Model:        strings.TrimSpace(input.Model),
			Serial:       strings.TrimSpace(input.Serial),
			SysObjectID:  strings.TrimSpace(input.SysObjectID),
			Department:   strings.TrimSpace(input.Department),
			Status:       existing.Status,
		},
		EndpointAddress: address,
		EndpointPort:    uint16(port),
		NewEndpointID:   newJobID(),
		Credential:      updatedCred,
	}

	var discoveredInstance string
	var probeMarkers []snmp.MarkerRow
	probeHost := address
	if probeHost == "" {
		probeHost = existing.Address
	}
	if s.discovery != nil && hasNewCred && probeHost != "" && (update.Device.Manufacturer == "" || update.Device.Model == "" || update.Device.Serial == "" || update.Device.SysObjectID == "") {
		probeCtx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		probeRes, probeErr := s.discovery(probeCtx, snmp.Config{
			Host:           probeHost,
			Port:           uint16(port),
			Version:        snmp.Version(secret.Version),
			Community:      secret.Community,
			Username:       secret.Username,
			AuthProtocol:   secret.AuthProtocol,
			AuthPassphrase: secret.AuthPassphrase,
			PrivProtocol:   secret.PrivProtocol,
			PrivPassphrase: secret.PrivPassphrase,
			TimeoutSeconds: 3,
			Retries:        1,
			MaxRepetitions: 25,
		})
		cancel()
		if probeErr == nil {
			if update.Device.Manufacturer == "" {
				update.Device.Manufacturer = probeRes.Manufacturer
			}
			if update.Device.Model == "" {
				update.Device.Model = probeRes.Model
			}
			if update.Device.Serial == "" {
				update.Device.Serial = probeRes.Serial
			}
			if update.Device.SysObjectID == "" {
				update.Device.SysObjectID = probeRes.SysObjectID
			}
			probeMarkers = probeRes.Markers
			if len(probeMarkers) == 1 {
				discoveredInstance = probeMarkers[0].Instance
			}
		}
	}

	if len(s.profiles) > 0 {
		selected, resolveErr := (profile.Resolver{Profiles: s.profiles}).Resolve(profile.DeviceIdentity{
			SysObjectID:  update.Device.SysObjectID,
			Manufacturer: update.Device.Manufacturer,
			Model:        update.Device.Model,
		}, strings.TrimSpace(input.ProfileID))
		if resolveErr != nil {
			writeError(w, http.StatusBadRequest, "profile_resolution_failed", resolveErr.Error())
			return
		}
		content, marshalErr := yaml.Marshal(selected)
		if marshalErr != nil {
			writeError(w, http.StatusInternalServerError, "profile_serialization_failed", "could not serialize selected profile")
			return
		}
		digest := sha256.Sum256(content)
		profileID := selected.ID + "@" + strconv.Itoa(selected.Version)
		update.Profile = &repository.SNMPProfile{
			ID:                 profileID,
			ProfileKey:         selected.ID,
			Version:            selected.Version,
			SchemaVersion:      selected.SchemaVersion,
			Content:            string(content),
			Checksum:           hex.EncodeToString(digest[:]),
			VerificationStatus: string(selected.VerificationStatus),
		}
		update.ProfileExplicit = strings.TrimSpace(input.ProfileID) != ""
		update.CounterDefinitions = make([]repository.CounterDefinition, 0, len(selected.Counters))
		for key, definition := range selected.Counters {
			instance := ""
			if definition.Mode == "walk" {
				if discoveredInstance != "" {
					instance = discoveredInstance
				} else {
					for _, m := range probeMarkers {
						if m.UnitName == definition.Unit || snmp.CounterUnitName(m.Unit) == definition.Unit {
							instance = m.Instance
							break
						}
					}
					if instance == "" && len(probeMarkers) > 0 {
						instance = probeMarkers[0].Instance
					}
				}
			}
			update.CounterDefinitions = append(update.CounterDefinitions, repository.CounterDefinition{
				ID:                    deviceID + ":" + key,
				DeviceID:              deviceID,
				Key:                   key,
				SourceProtocol:        "snmp",
				OID:                   definition.ValueColumn,
				Instance:              instance,
				Unit:                  definition.Unit,
				SemanticType:          definition.SemanticType,
				Scope:                 definition.Scope,
				Mode:                  definition.Mode,
				UnitOID:               definition.UnitColumn,
				Selection:             definition.Selection,
				Aggregation:           definition.Aggregation,
				RequireUnitValidation: definition.RequireUnitValidation,
				Verified:              selected.VerificationStatus == profile.VerificationVerified,
			})
		}
	}

	if err := store.UpdatePrinter(r.Context(), update); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "printer was not found")
			return
		}
		writeError(w, http.StatusConflict, "printer_update_failed", "could not update printer")
		return
	}
	updated, err := s.store.GetDevice(r.Context(), deviceID)
	if err != nil {
		updated = update.Device
	}
	writeJSON(w, http.StatusOK, toPrinterResponse(updated))
}

func (s *Server) deletePrinter(w http.ResponseWriter, r *http.Request, deviceID string) {
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	store, ok := s.store.(PrinterManagementStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "printer_delete_unavailable", "printer deletion is not configured")
		return
	}
	if err := store.SoftDeleteDevice(r.Context(), deviceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "printer was not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "printer_delete_failed", "could not delete printer")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": deviceID})
}

type createEndpointRequest struct {
	Address      string `json:"address"`
	Protocol     string `json:"protocol"`
	Port         int    `json:"port"`
	CredentialID string `json:"credential_id"`
	IsPrimary    bool   `json:"is_primary"`
}

func (s *Server) createEndpoint(w http.ResponseWriter, r *http.Request, deviceID string) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	store, ok := s.store.(EndpointStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "endpoint_store_unavailable", "endpoint storage is not configured")
		return
	}
	if deviceID == "" || strings.Contains(deviceID, "/") {
		writeError(w, http.StatusNotFound, "not_found", "printer was not found")
		return
	}
	if _, err := s.store.GetDevice(r.Context(), deviceID); errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "printer was not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "printer_get_failed", "could not get printer")
		return
	}
	var input createEndpointRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	protocol := strings.TrimSpace(input.Protocol)
	if protocol == "" {
		protocol = "snmp"
	}
	port := input.Port
	if port == 0 {
		port = 161
	}
	if port < 1 || port > 65535 {
		writeError(w, http.StatusBadRequest, "invalid_endpoint", "port must be between 1 and 65535")
		return
	}
	endpoint := repository.DeviceEndpoint{ID: newJobID(), DeviceID: deviceID, Address: strings.TrimSpace(input.Address), Protocol: protocol, Port: uint16(port), CredentialID: strings.TrimSpace(input.CredentialID), IsPrimary: input.IsPrimary}
	if err := endpoint.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_endpoint", err.Error())
		return
	}
	if err := store.CreateDeviceEndpoint(r.Context(), endpoint); err != nil {
		writeError(w, http.StatusConflict, "endpoint_create_failed", "could not create endpoint")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": endpoint.ID, "device_id": endpoint.DeviceID, "address": endpoint.Address, "protocol": endpoint.Protocol, "port": endpoint.Port, "credential_id": nullable(endpoint.CredentialID), "is_primary": endpoint.IsPrimary})
}

func (s *Server) authorized(r *http.Request) bool {
	return s.apiToken == "" || r.Header.Get("Authorization") == "Bearer "+s.apiToken
}

func (s *Server) startPoll(w http.ResponseWriter, r *http.Request, deviceID string) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if s.apiToken != "" && r.Header.Get("Authorization") != "Bearer "+s.apiToken {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	if deviceID == "" || strings.Contains(deviceID, "/") {
		writeError(w, http.StatusNotFound, "not_found", "printer was not found")
		return
	}
	jobID, err := s.queuePoll(r.Context(), deviceID)
	if err != nil {
		if errors.Is(err, repository.ErrPollLeaseHeld) {
			writeError(w, http.StatusConflict, "poll_already_running", "a poll for this printer is already running")
			return
		}
		writeError(w, http.StatusInternalServerError, "poll_start_failed", "could not start polling")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID, "status": "queued", "status_url": "/api/v1/jobs/" + jobID})
}

func (s *Server) queuePoll(ctx context.Context, deviceID string) (string, error) {
	if s.poller == nil {
		return "", errors.New("poller is not configured")
	}
	jobID := newJobID()
	acquiredAt := time.Now().UTC()
	acquired, err := s.store.AcquirePollLease(ctx, deviceID, "status", jobID, acquiredAt, acquiredAt.Add(45*time.Second))
	if err != nil {
		return "", fmt.Errorf("acquire status poll lease: %w", err)
	}
	if !acquired {
		return "", repository.ErrPollLeaseHeld
	}
	job := repository.JobRecord{ID: jobID, DeviceID: deviceID, Kind: "status_poll", Status: "queued", CreatedAt: time.Now().UTC()}
	if err := s.store.CreateJob(ctx, job); err != nil {
		_ = s.store.ReleasePollLease(context.Background(), deviceID, "status", jobID)
		return "", fmt.Errorf("create polling job: %w", err)
	}
	if err := s.store.CreatePollingRun(ctx, repository.PollingRun{ID: jobID, DeviceID: deviceID, JobKind: "status", Result: "running", AttemptCount: 1}); err != nil {
		_ = s.store.FinishJob(ctx, jobID, "failed", "polling_run_create_failed", time.Now().UTC())
		_ = s.store.ReleasePollLease(context.Background(), deviceID, "status", jobID)
		return "", fmt.Errorf("create polling run: %w", err)
	}
	if err := s.store.StartJob(ctx, jobID, time.Now().UTC()); err != nil {
		_ = s.store.FinishPollingRun(ctx, jobID, "failed", "job_start_failed", time.Now().UTC())
		_ = s.store.ReleasePollLease(context.Background(), deviceID, "status", jobID)
		return "", fmt.Errorf("start polling job: %w", err)
	}
	go func() {
		s.executePollJob(jobID, deviceID, jobID)
	}()
	return jobID, nil
}

func (s *Server) executePollJob(jobID, deviceID, leaseOwnerID string) {
	pollCtx, cancel := context.WithTimeout(context.Background(), s.pollTimeout)
	err := s.poller.PollDevice(pollCtx, deviceID)
	cancel()

	status, errorCode := "success", ""
	if err != nil {
		status, errorCode = "failed", "poll_failed"
		if errors.Is(err, context.DeadlineExceeded) {
			errorCode = "poll_timeout"
		}
		slog.Error("manual status poll failed", "job_id", jobID, "device_id", deviceID, "error_code", errorCode, "error", err)
	}

	finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer finishCancel()
	if err := s.store.FinishJob(finishCtx, jobID, status, errorCode, time.Now().UTC()); err != nil {
		slog.Error("finish manual poll job failed", "job_id", jobID, "device_id", deviceID, "error", err)
	}
	if err := s.store.FinishPollingRun(finishCtx, jobID, status, errorCode, time.Now().UTC()); err != nil {
		slog.Error("finish manual polling run failed", "job_id", jobID, "device_id", deviceID, "error", err)
	}
	if err := s.store.ReleasePollLease(finishCtx, deviceID, "status", leaseOwnerID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("release manual poll lease failed", "job_id", jobID, "device_id", deviceID, "error", err)
	}
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}
	if s.apiToken != "" && r.Header.Get("Authorization") != "Bearer "+s.apiToken {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/jobs")
	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed != "" {
		stored, err := s.store.GetJob(r.Context(), trimmed)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "job was not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "job_get_failed", "could not get job")
			return
		}
		writeJSON(w, http.StatusOK, jobResponse{ID: stored.ID, DeviceID: stored.DeviceID, Status: stored.Status, ErrorCode: nullable(stored.ErrorCode)})
		return
	}

	limit, offset, err := pagination(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	items, total, err := s.store.ListJobsWithDevice(r.Context(), limit, offset, statusFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "jobs_list_failed", "could not list jobs")
		return
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

type jobResponse struct {
	ID        string  `json:"id"`
	DeviceID  string  `json:"device_id"`
	Status    string  `json:"status"`
	ErrorCode *string `json:"error_code,omitempty"`
}

func newJobID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return fmt.Sprintf("legacy-%d", time.Now().UnixNano())
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16])
}

func (s *Server) listCounters(w http.ResponseWriter, r *http.Request, deviceID string) {
	if deviceID == "" || strings.Contains(deviceID, "/") {
		writeError(w, http.StatusNotFound, "not_found", "printer was not found")
		return
	}
	limit, offset, err := pagination(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	readings, err := s.store.ListCounterReadings(r.Context(), deviceID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "counter_list_failed", "could not list counter readings")
		return
	}
	total, err := s.store.CountCounterReadings(r.Context(), deviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "counter_count_failed", "could not count counter readings")
		return
	}
	data := make([]counterResponse, 0, len(readings))
	for _, reading := range readings {
		data = append(data, counterResponse{ID: reading.ID, DefinitionKey: reading.DefinitionKey, Unit: reading.Unit, Scope: reading.Scope, RawValue: reading.RawValue, CollectedAt: reading.CollectedAt.UTC().Format(time.RFC3339Nano), Quality: string(reading.Quality)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "limit": limit, "offset": offset, "total": total})
}

func (s *Server) listUsage(w http.ResponseWriter, r *http.Request, deviceID string) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	if deviceID == "" || strings.Contains(deviceID, "/") {
		writeError(w, http.StatusNotFound, "not_found", "printer was not found")
		return
	}
	locationName := strings.TrimSpace(r.URL.Query().Get("timezone"))
	if locationName == "" {
		locationName = "UTC"
	}
	_, err := time.LoadLocation(locationName)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_timezone", "timezone must be a valid IANA timezone")
		return
	}
	from, to, err := usageDateRange(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_date_range", err.Error())
		return
	}
	persisted, err := s.store.ListDailyCounterUsage(r.Context(), deviceID, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "usage_read_failed", "could not read daily usage")
		return
	}
	data := make([]usageResponse, 0)
	for _, item := range persisted {
		data = append(data, usageResponse{DefinitionKey: item.DefinitionKey, Unit: item.Unit, Scope: item.Scope, LocalDate: item.LocalDate, Delta: item.Delta, Quality: string(item.Quality)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "total": len(data), "timezone": locationName})
}

func usageDateRange(r *http.Request) (string, string, error) {
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	for name, value := range map[string]string{"from": from, "to": to} {
		if value == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return "", "", fmt.Errorf("%s must use YYYY-MM-DD", name)
		}
	}
	if from != "" && to != "" && from > to {
		return "", "", errors.New("from must not be after to")
	}
	return from, to, nil
}

func (s *Server) getPageUsageReport(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", s.allowedOrigin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}

	from := strings.TrimSpace(r.URL.Query().Get("from"))
	to := strings.TrimSpace(r.URL.Query().Get("to"))
	department := strings.TrimSpace(r.URL.Query().Get("department"))

	for name, value := range map[string]string{"from": from, "to": to} {
		if value == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", value); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_date_format", fmt.Sprintf("%s must be in YYYY-MM-DD format", name))
			return
		}
	}
	if from != "" && to != "" && from > to {
		writeError(w, http.StatusBadRequest, "invalid_date_range", "from must not be after to")
		return
	}

	report, err := s.store.GetPageUsageReport(r.Context(), from, to, department)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "report_generation_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func (s *Server) listPollingRuns(w http.ResponseWriter, r *http.Request, deviceID string) {
	if deviceID == "" || strings.Contains(deviceID, "/") {
		writeError(w, http.StatusNotFound, "not_found", "printer was not found")
		return
	}
	limit, offset, err := pagination(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	runs, err := s.store.ListPollingRuns(r.Context(), deviceID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "polling_run_list_failed", "could not list polling runs")
		return
	}
	total, err := s.store.CountPollingRuns(r.Context(), deviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "polling_run_count_failed", "could not count polling runs")
		return
	}
	data := make([]pollingRunResponse, 0, len(runs))
	for _, run := range runs {
		data = append(data, pollingRunResponse{ID: run.ID, JobKind: run.JobKind, StartedAt: run.StartedAt.UTC().Format(time.RFC3339Nano), EndedAt: nullableTime(run.EndedAt), Result: run.Result, ErrorCode: nullable(run.ErrorCode), ProfileVersion: run.ProfileVersion, AttemptCount: run.AttemptCount})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "limit": limit, "offset": offset, "total": total})
}

type printerResponse struct {
	ID           string  `json:"id"`
	AssetCode    *string `json:"asset_code,omitempty"`
	DisplayName  string  `json:"display_name"`
	Manufacturer *string `json:"manufacturer,omitempty"`
	Model        *string `json:"model,omitempty"`
	Serial       *string `json:"serial,omitempty"`
	SysObjectID  *string `json:"sys_object_id,omitempty"`
	Department   *string `json:"department,omitempty"`
	Address      *string `json:"address,omitempty"`
	Port         *int    `json:"port,omitempty"`
	SNMPVersion  *string `json:"snmp_version,omitempty"`
	ProfileID    *string `json:"profile_id,omitempty"`
	Status       string  `json:"status"`
	LastSeenAt   *string `json:"last_seen_at,omitempty"`
}

type profileResponse struct {
	ID                 string   `json:"id"`
	Version            int      `json:"version"`
	Manufacturer       string   `json:"manufacturer"`
	VerificationStatus string   `json:"verification_status"`
	CounterKeys        []string `json:"counter_keys"`
}

type counterResponse struct {
	ID            int64  `json:"id"`
	DefinitionKey string `json:"definition_key"`
	Unit          string `json:"unit"`
	Scope         string `json:"scope"`
	RawValue      int64  `json:"raw_value"`
	CollectedAt   string `json:"collected_at"`
	Quality       string `json:"quality"`
}

type usageResponse struct {
	DefinitionKey string `json:"definition_key"`
	Unit          string `json:"unit"`
	Scope         string `json:"scope"`
	LocalDate     string `json:"local_date"`
	Delta         int64  `json:"delta"`
	Quality       string `json:"quality"`
}

type pollingRunResponse struct {
	ID             string  `json:"id"`
	JobKind        string  `json:"job_kind"`
	StartedAt      string  `json:"started_at"`
	EndedAt        *string `json:"ended_at,omitempty"`
	Result         string  `json:"result"`
	ErrorCode      *string `json:"error_code,omitempty"`
	ProfileVersion int     `json:"profile_version,omitempty"`
	AttemptCount   int     `json:"attempt_count"`
}

func toPrinterResponse(device repository.Device) printerResponse {
	var port *int
	if device.Port > 0 {
		p := int(device.Port)
		port = &p
	}
	return printerResponse{
		ID: device.ID, AssetCode: nullable(device.AssetCode), DisplayName: device.DisplayName,
		Manufacturer: nullable(device.Manufacturer), Model: nullable(device.Model), Serial: nullable(device.Serial),
		SysObjectID: nullable(device.SysObjectID),
		Department:  nullable(device.Department),
		Address:     nullable(device.Address), Port: port, SNMPVersion: nullable(device.SNMPVersion), ProfileID: nullable(device.ProfileKey),
		Status: device.Status, LastSeenAt: nullableTime(device.LastSeenAt),
	}
}

func pagination(r *http.Request) (int, int, error) {
	limit, offset := 50, 0
	var err error
	if value := r.URL.Query().Get("limit"); value != "" {
		limit, err = strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 1000 {
			return 0, 0, errors.New("limit must be between 1 and 1000")
		}
	}
	if value := r.URL.Query().Get("offset"); value != "" {
		offset, err = strconv.Atoi(value)
		if err != nil || offset < 0 {
			return 0, 0, errors.New("offset must be zero or greater")
		}
	}
	return limit, offset, nil
}

func nullable(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func nullableTime(value time.Time) *string {
	if value.IsZero() {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339Nano)
	return &formatted
}

func (s *Server) methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", "GET, OPTIONS")
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method is not supported")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

type createCartridgeRequest struct {
	SKUCode            string `json:"sku_code"`
	Name               string `json:"name"`
	CompatibleModels   string `json:"compatible_models"`
	StockNew           int    `json:"stock_new"`
	StockRefilled      int    `json:"stock_refilled"`
	StockEmpty         int    `json:"stock_empty"`
	StockRefillBottles int    `json:"stock_refill_bottles"`
}

type updateCartridgeStockRequest struct {
	CartridgeID      string `json:"cartridge_id"`
	AddStockNew      int    `json:"add_stock_new"`
	AddStockRefilled int    `json:"add_stock_refilled"`
	AddStockEmpty    int    `json:"add_stock_empty"`
	Notes            string `json:"notes"`
}

type addRefillBottlesRequest struct {
	CartridgeID string `json:"cartridge_id"`
	Quantity    int    `json:"quantity"`
	Notes       string `json:"notes"`
}

type replaceCartridgeRequest struct {
	CartridgeID string `json:"cartridge_id"`
	DeviceID    string `json:"device_id"`
	SourceType  string `json:"source_type"`
	PageCount   int    `json:"page_count"`
	Notes       string `json:"notes"`
}

type refillCartridgesRequest struct {
	CartridgeID string `json:"cartridge_id"`
	Quantity    int    `json:"quantity"`
	Notes       string `json:"notes"`
}

type refillPrinterCartridgeRequest struct {
	CartridgeID string `json:"cartridge_id"`
	DeviceID    string `json:"device_id"`
	Quantity    int    `json:"quantity"`
	PageCount   int    `json:"page_count"`
	Notes       string `json:"notes"`
}

func (s *Server) handleCartridges(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	store, ok := s.store.(CartridgeStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "cartridge_store_unavailable", "cartridge management is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := store.ListCartridges(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "cartridge_list_failed", "could not list cartridges")
			return
		}
		if items == nil {
			items = []repository.Cartridge{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": items, "total": len(items)})
	case http.MethodPost:
		var input createCartridgeRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		c := repository.Cartridge{
			ID:                 newJobID(),
			SKUCode:            strings.TrimSpace(input.SKUCode),
			Name:               strings.TrimSpace(input.Name),
			CompatibleModels:   strings.TrimSpace(input.CompatibleModels),
			StockNew:           input.StockNew,
			StockRefilled:      input.StockRefilled,
			StockEmpty:         input.StockEmpty,
			StockRefillBottles: input.StockRefillBottles,
		}
		if err := store.CreateCartridge(r.Context(), c); err != nil {
			writeError(w, http.StatusBadRequest, "cartridge_create_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, c)
	default:
		s.methodNotAllowed(w)
	}
}

func (s *Server) updateCartridgeStock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	store, ok := s.store.(CartridgeStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "cartridge_store_unavailable", "cartridge management is not configured")
		return
	}
	var input updateCartridgeStockRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	logID := newJobID()
	if err := store.UpdateCartridgeStock(r.Context(), input.CartridgeID, input.AddStockNew, input.AddStockRefilled, input.AddStockEmpty, logID, strings.TrimSpace(input.Notes)); err != nil {
		writeError(w, http.StatusBadRequest, "stock_update_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) addRefillBottles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	store, ok := s.store.(CartridgeStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "cartridge_store_unavailable", "cartridge management is not configured")
		return
	}
	var input addRefillBottlesRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if err := store.AddRefillBottles(r.Context(), strings.TrimSpace(input.CartridgeID), input.Quantity, newJobID(), strings.TrimSpace(input.Notes)); err != nil {
		writeError(w, http.StatusBadRequest, "refill_bottle_stock_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) replaceCartridge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	store, ok := s.store.(CartridgeStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "cartridge_store_unavailable", "cartridge management is not configured")
		return
	}
	var input replaceCartridgeRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	params := repository.ReplaceCartridgeParams{
		LogID:       newJobID(),
		CartridgeID: strings.TrimSpace(input.CartridgeID),
		DeviceID:    strings.TrimSpace(input.DeviceID),
		SourceType:  strings.TrimSpace(input.SourceType),
		PageCount:   input.PageCount,
		Notes:       strings.TrimSpace(input.Notes),
	}
	counterStatus := "unavailable"
	if input.PageCount > 0 {
		params.CounterQuality = string(counter.QualityUnverified)
		counterStatus = params.CounterQuality
	} else if s.counterSnapshotter != nil {
		reading, _, snapshotErr := s.counterSnapshotter.ReadDevice(r.Context(), params.DeviceID)
		if snapshotErr == nil {
			params.PageCount = int(reading.RawValue)
			params.CounterQuality = string(reading.Quality)
			params.CounterCollectedAt = reading.CollectedAt
			counterStatus = params.CounterQuality
		}
	}
	if err := store.ReplacePrinterCartridge(r.Context(), params); err != nil {
		writeError(w, http.StatusBadRequest, "cartridge_replace_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "success", "message": "cartridge replaced successfully", "counter": params.PageCount, "counter_quality": counterStatus})
}

func (s *Server) refillCartridges(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	store, ok := s.store.(CartridgeStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "cartridge_store_unavailable", "cartridge management is not configured")
		return
	}
	var input refillCartridgesRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	logID := newJobID()
	if err := store.RefillCartridges(r.Context(), logID, strings.TrimSpace(input.CartridgeID), input.Quantity, strings.TrimSpace(input.Notes)); err != nil {
		writeError(w, http.StatusBadRequest, "cartridge_refill_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success", "message": "cartridges refilled successfully"})
}

func (s *Server) refillPrinterCartridge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	store, ok := s.store.(CartridgeStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "cartridge_store_unavailable", "cartridge management is not configured")
		return
	}
	var input refillPrinterCartridgeRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	params := repository.RefillPrinterCartridgeParams{LogID: newJobID(), CartridgeID: strings.TrimSpace(input.CartridgeID), DeviceID: strings.TrimSpace(input.DeviceID), Quantity: input.Quantity, PageCount: input.PageCount, Notes: strings.TrimSpace(input.Notes)}
	counterStatus := "unavailable"
	if input.PageCount > 0 {
		params.CounterQuality = string(counter.QualityUnverified)
		counterStatus = params.CounterQuality
	} else if s.counterSnapshotter != nil {
		reading, _, snapshotErr := s.counterSnapshotter.ReadDevice(r.Context(), params.DeviceID)
		if snapshotErr == nil {
			params.PageCount = int(reading.RawValue)
			params.CounterQuality = string(reading.Quality)
			params.CounterCollectedAt = reading.CollectedAt
			counterStatus = params.CounterQuality
		}
	}
	if err := store.RefillPrinterCartridge(r.Context(), params); err != nil {
		writeError(w, http.StatusBadRequest, "printer_refill_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "success", "message": "printer cartridge refilled successfully", "counter": params.PageCount, "counter_quality": counterStatus})
}

func (s *Server) listCartridgeLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	store, ok := s.store.(CartridgeStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "cartridge_store_unavailable", "cartridge management is not configured")
		return
	}
	limit, offset, err := pagination(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
		return
	}
	cartridgeID := strings.TrimSpace(r.URL.Query().Get("cartridge_id"))
	deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
	logs, err := store.ListCartridgeLogs(r.Context(), cartridgeID, deviceID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cartridge_logs_failed", "could not list cartridge logs")
		return
	}
	if logs == nil {
		logs = []repository.CartridgeLog{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": logs, "limit": limit, "offset": offset, "total": len(logs)})
}
