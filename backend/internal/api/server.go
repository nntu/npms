package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"npms/backend/internal/repository"
)

type DeviceStore interface {
	ListDevices(context.Context, int, int) ([]repository.Device, error)
	CountDevices(context.Context) (int, error)
	CreateDevice(context.Context, repository.Device) error
	GetDevice(context.Context, string) (repository.Device, error)
	ListCounterReadings(context.Context, string, int, int) ([]repository.CounterReadingRecord, error)
	CountCounterReadings(context.Context, string) (int, error)
	ListPollingRuns(context.Context, string, int, int) ([]repository.PollingRunRecord, error)
	CountPollingRuns(context.Context, string) (int, error)
	CreatePollingRun(context.Context, repository.PollingRun) error
	FinishPollingRun(context.Context, string, string, string, time.Time) error
	CreateJob(context.Context, repository.JobRecord) error
	GetJob(context.Context, string) (repository.JobRecord, error)
	StartJob(context.Context, string, time.Time) error
	FinishJob(context.Context, string, string, string, time.Time) error
}

type StatusPoller interface {
	PollDevice(context.Context, string) error
}

type Server struct {
	store         DeviceStore
	apiToken      string
	allowedOrigin string
	poller        StatusPoller
	frontendFS    fs.FS
}

func NewServer(store DeviceStore, apiToken, allowedOrigin string) (*Server, error) {
	if store == nil {
		return nil, errors.New("device store is required")
	}
	if allowedOrigin == "" {
		allowedOrigin = "http://localhost:5173"
	}
	return &Server{store: store, apiToken: apiToken, allowedOrigin: allowedOrigin}, nil
}

func (s *Server) SetStatusPoller(poller StatusPoller) { s.poller = poller }

func (s *Server) SetFrontendFS(frontendFS fs.FS) { s.frontendFS = frontendFS }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", s.health)
	mux.HandleFunc("/api/v1/health/live", s.health)
	mux.HandleFunc("/api/v1/health/ready", s.health)
	mux.HandleFunc("/api/v1/printers", s.listPrinters)
	mux.HandleFunc("/api/v1/printers/", s.getPrinter)
	mux.HandleFunc("/api/v1/jobs/", s.getJob)
	static := http.Handler(nil)
	if s.frontendFS != nil {
		static = http.FileServer(http.FS(s.frontendFS))
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.allowedOrigin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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
	device := repository.Device{ID: newJobID(), AssetCode: strings.TrimSpace(input.AssetCode), DisplayName: strings.TrimSpace(input.DisplayName), Manufacturer: strings.TrimSpace(input.Manufacturer), Model: strings.TrimSpace(input.Model), Serial: strings.TrimSpace(input.Serial), SysObjectID: strings.TrimSpace(input.SysObjectID), Status: "unknown"}
	if err := s.store.CreateDevice(r.Context(), device); err != nil {
		writeError(w, http.StatusConflict, "device_create_failed", "could not create printer")
		return
	}
	writeJSON(w, http.StatusCreated, toPrinterResponse(device))
}

func (s *Server) getPrinter(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/printers/")
	if strings.HasSuffix(id, "/poll") {
		s.startPoll(w, r, strings.TrimSuffix(id, "/poll"))
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
	if strings.HasSuffix(id, "/counters") {
		s.listCounters(w, r, strings.TrimSuffix(id, "/counters"))
		return
	}
	if strings.HasSuffix(id, "/polling-runs") {
		s.listPollingRuns(w, r, strings.TrimSuffix(id, "/polling-runs"))
		return
	}
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "not_found", "printer was not found")
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
	if s.poller == nil {
		writeError(w, http.StatusServiceUnavailable, "poller_unavailable", "polling is not configured")
		return
	}
	jobID := newJobID()
	job := repository.JobRecord{ID: jobID, DeviceID: deviceID, Kind: "status_poll", Status: "queued", CreatedAt: time.Now().UTC()}
	if err := s.store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, "job_create_failed", "could not create polling job")
		return
	}
	if err := s.store.CreatePollingRun(r.Context(), repository.PollingRun{ID: jobID, DeviceID: deviceID, JobKind: "status", Result: "running", AttemptCount: 1}); err != nil {
		_ = s.store.FinishJob(r.Context(), jobID, "failed", "polling_run_create_failed", time.Now().UTC())
		writeError(w, http.StatusInternalServerError, "polling_run_create_failed", "could not create polling run")
		return
	}
	_ = s.store.StartJob(r.Context(), jobID, time.Now().UTC())
	go func() {
		err := s.poller.PollDevice(context.Background(), deviceID)
		if err != nil {
			_ = s.store.FinishJob(context.Background(), jobID, "failed", "poll_failed", time.Now().UTC())
			_ = s.store.FinishPollingRun(context.Background(), jobID, "failed", "poll_failed", time.Now().UTC())
			return
		}
		_ = s.store.FinishJob(context.Background(), jobID, "success", "", time.Now().UTC())
		_ = s.store.FinishPollingRun(context.Background(), jobID, "success", "", time.Now().UTC())
	}()
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": job.ID, "status": job.Status, "status_url": "/api/v1/jobs/" + job.ID})
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}
	if s.apiToken != "" && r.Header.Get("Authorization") != "Bearer "+s.apiToken {
		writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/jobs/")
	stored, err := s.store.GetJob(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "job was not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "job_get_failed", "could not get job")
		return
	}
	writeJSON(w, http.StatusOK, jobResponse{ID: stored.ID, DeviceID: stored.DeviceID, Status: stored.Status, ErrorCode: nullable(stored.ErrorCode)})
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
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return fmt.Sprintf("%x", value)
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
	Status       string  `json:"status"`
	LastSeenAt   *string `json:"last_seen_at,omitempty"`
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
	return printerResponse{
		ID: device.ID, AssetCode: nullable(device.AssetCode), DisplayName: device.DisplayName,
		Manufacturer: nullable(device.Manufacturer), Model: nullable(device.Model), Serial: nullable(device.Serial),
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
