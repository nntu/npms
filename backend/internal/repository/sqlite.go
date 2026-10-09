package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"npms/backend/internal/counter"
)

type SQLiteRepository struct{ db *sql.DB }

type CleanupPolicy struct {
	PollingRunsBefore   time.Time
	CounterEventsBefore time.Time
	JobsBefore          time.Time
}

type CleanupStats struct {
	PollingRuns   int64
	CounterEvents int64
	Jobs          int64
}

var ErrPollLeaseHeld = errors.New("poll lease is already held")

func (r *SQLiteRepository) AcquirePollLease(ctx context.Context, deviceID, jobKind, ownerID string, acquiredAt, leaseUntil time.Time) (bool, error) {
	if strings.TrimSpace(deviceID) == "" || strings.TrimSpace(jobKind) == "" || strings.TrimSpace(ownerID) == "" {
		return false, fmt.Errorf("device id, job kind and owner id are required")
	}
	if acquiredAt.IsZero() {
		acquiredAt = time.Now().UTC()
	}
	if leaseUntil.IsZero() || !leaseUntil.After(acquiredAt) {
		return false, fmt.Errorf("lease_until must be after acquired_at")
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO poll_leases(device_id, job_kind, owner_id, acquired_at, lease_until)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(device_id, job_kind) DO UPDATE SET owner_id = excluded.owner_id, acquired_at = excluded.acquired_at, lease_until = excluded.lease_until
		WHERE poll_leases.lease_until <= excluded.acquired_at OR poll_leases.owner_id = excluded.owner_id`, deviceID, jobKind, ownerID, acquiredAt.UTC().Format(time.RFC3339Nano), leaseUntil.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return false, fmt.Errorf("acquire poll lease: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count acquired poll lease: %w", err)
	}
	return count == 1, nil
}

func (r *SQLiteRepository) ReleasePollLease(ctx context.Context, deviceID, jobKind, ownerID string) error {
	if strings.TrimSpace(deviceID) == "" || strings.TrimSpace(jobKind) == "" || strings.TrimSpace(ownerID) == "" {
		return fmt.Errorf("device id, job kind and owner id are required")
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM poll_leases WHERE device_id = ? AND job_kind = ? AND owner_id = ?`, deviceID, jobKind, ownerID)
	if err != nil {
		return fmt.Errorf("release poll lease: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CleanupPollerData removes operational history only. Raw counter readings
// and derived daily usage are deliberately retained for reporting/audit.
func (r *SQLiteRepository) CleanupPollerData(ctx context.Context, policy CleanupPolicy) (CleanupStats, error) {
	if policy.PollingRunsBefore.IsZero() || policy.CounterEventsBefore.IsZero() || policy.JobsBefore.IsZero() {
		return CleanupStats{}, fmt.Errorf("cleanup policy dates are required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return CleanupStats{}, fmt.Errorf("begin poller cleanup: %w", err)
	}
	defer tx.Rollback()
	var stats CleanupStats
	result, err := tx.ExecContext(ctx, `DELETE FROM polling_runs
		WHERE ended_at IS NOT NULL AND started_at < ?
		AND NOT EXISTS (SELECT 1 FROM counter_readings WHERE counter_readings.poll_run_id = polling_runs.id)`, policy.PollingRunsBefore.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return CleanupStats{}, fmt.Errorf("delete polling runs: %w", err)
	}
	stats.PollingRuns, err = result.RowsAffected()
	if err != nil {
		return CleanupStats{}, fmt.Errorf("count deleted polling runs: %w", err)
	}
	result, err = tx.ExecContext(ctx, `DELETE FROM counter_events WHERE created_at < ?`, policy.CounterEventsBefore.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return CleanupStats{}, fmt.Errorf("delete counter events: %w", err)
	}
	stats.CounterEvents, err = result.RowsAffected()
	if err != nil {
		return CleanupStats{}, fmt.Errorf("count deleted counter events: %w", err)
	}
	result, err = tx.ExecContext(ctx, `DELETE FROM jobs WHERE ended_at IS NOT NULL AND ended_at < ?`, policy.JobsBefore.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return CleanupStats{}, fmt.Errorf("delete completed jobs: %w", err)
	}
	stats.Jobs, err = result.RowsAffected()
	if err != nil {
		return CleanupStats{}, fmt.Errorf("count deleted jobs: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return CleanupStats{}, fmt.Errorf("commit poller cleanup: %w", err)
	}
	return stats, nil
}

func NewSQLiteRepository(db *sql.DB) (*SQLiteRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}
	return &SQLiteRepository{db: db}, nil
}

type Device struct {
	ID           string
	AssetCode    string
	DisplayName  string
	Manufacturer string
	Model        string
	Serial       string
	SysObjectID  string
	Department   string
	Status       string
	FirstSeenAt  time.Time
	LastSeenAt   time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type DeviceEndpoint struct {
	ID, DeviceID, Address, Protocol, CredentialID string
	Port                                          uint16
	IsPrimary                                     bool
	LastSuccessAt                                 time.Time
}

func (e DeviceEndpoint) Validate() error {
	if strings.TrimSpace(e.ID) == "" || strings.TrimSpace(e.DeviceID) == "" {
		return fmt.Errorf("endpoint id and device id are required")
	}
	if strings.TrimSpace(e.Address) == "" {
		return fmt.Errorf("endpoint address is required")
	}
	if e.Protocol != "snmp" {
		return fmt.Errorf("unsupported endpoint protocol %q", e.Protocol)
	}
	if e.Port == 0 {
		return fmt.Errorf("endpoint port must be greater than zero")
	}
	return nil
}

func (r *SQLiteRepository) CreateDeviceEndpoint(ctx context.Context, endpoint DeviceEndpoint) error {
	if err := endpoint.Validate(); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO device_endpoints(id, device_id, address, protocol, port, credential_id, is_primary) VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?)`, endpoint.ID, endpoint.DeviceID, endpoint.Address, endpoint.Protocol, endpoint.Port, endpoint.CredentialID, boolInt(endpoint.IsPrimary))
	if err != nil {
		return fmt.Errorf("create device endpoint: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) ListDeviceEndpoints(ctx context.Context, deviceID string) ([]DeviceEndpoint, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, device_id, address, protocol, port, credential_id, is_primary, last_success_at FROM device_endpoints WHERE device_id = ? ORDER BY is_primary DESC, id`, deviceID)
	if err != nil {
		return nil, fmt.Errorf("list device endpoints: %w", err)
	}
	defer rows.Close()
	result := make([]DeviceEndpoint, 0)
	for rows.Next() {
		var endpoint DeviceEndpoint
		var credential, lastSuccess sql.NullString
		var port, primary int
		if err := rows.Scan(&endpoint.ID, &endpoint.DeviceID, &endpoint.Address, &endpoint.Protocol, &port, &credential, &primary, &lastSuccess); err != nil {
			return nil, fmt.Errorf("scan device endpoint: %w", err)
		}
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("invalid endpoint port %d", port)
		}
		endpoint.Port, endpoint.CredentialID, endpoint.IsPrimary, endpoint.LastSuccessAt = uint16(port), nullString(credential), primary == 1, parseTime(lastSuccess)
		result = append(result, endpoint)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate device endpoints: %w", err)
	}
	return result, nil
}

func (r *SQLiteRepository) MarkEndpointSuccess(ctx context.Context, endpointID string, at time.Time) error {
	if endpointID == "" {
		return fmt.Errorf("endpoint id is required")
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	result, err := r.db.ExecContext(ctx, "UPDATE device_endpoints SET last_success_at = ? WHERE id = ?", at.UTC().Format(time.RFC3339Nano), endpointID)
	if err != nil {
		return fmt.Errorf("mark endpoint success: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type SNMPCredential struct {
	ID, Version, SecurityMetadata string
	EncryptedSecretMaterial       []byte
	CreatedAt                     time.Time
}

type PrinterRegistration struct {
	Device             Device
	Credential         SNMPCredential
	Endpoint           DeviceEndpoint
	Profile            *SNMPProfile
	ProfileExplicit    bool
	CounterDefinitions []CounterDefinition
}

type SNMPProfile struct {
	ID, ProfileKey, Content, Checksum, VerificationStatus string
	Version, SchemaVersion                                int
}

func (r *SQLiteRepository) RegisterPrinter(ctx context.Context, registration PrinterRegistration) error {
	if err := registration.Device.Validate(); err != nil {
		return err
	}
	if registration.Credential.ID == "" || registration.Credential.Version == "" || len(registration.Credential.EncryptedSecretMaterial) == 0 {
		return fmt.Errorf("credential id, version and encrypted material are required")
	}
	if err := registration.Endpoint.Validate(); err != nil {
		return err
	}
	if registration.Endpoint.DeviceID != registration.Device.ID || registration.Endpoint.CredentialID != registration.Credential.ID {
		return fmt.Errorf("endpoint references do not match registration")
	}
	if registration.Profile == nil && len(registration.CounterDefinitions) > 0 {
		return fmt.Errorf("counter definitions require a profile")
	}
	if registration.Profile != nil {
		if registration.Profile.ID == "" || registration.Profile.ProfileKey == "" || registration.Profile.Version < 1 || registration.Profile.SchemaVersion < 1 || registration.Profile.Content == "" || registration.Profile.Checksum == "" || registration.Profile.VerificationStatus == "" {
			return fmt.Errorf("profile id, key, version, content, checksum and verification status are required")
		}
	}
	for _, definition := range registration.CounterDefinitions {
		if definition.ID == "" || definition.DeviceID != registration.Device.ID || definition.Key == "" || definition.SourceProtocol != "snmp" || definition.OID == "" || definition.Unit == "" || definition.SemanticType == "" || definition.Scope == "" {
			return fmt.Errorf("invalid counter definition %q", definition.Key)
		}
		if definition.Mode != "get" && definition.Mode != "walk" {
			return fmt.Errorf("counter definition %q has unsupported mode %q", definition.Key, definition.Mode)
		}
	}
	now := time.Now().UTC()
	if registration.Device.CreatedAt.IsZero() {
		registration.Device.CreatedAt = now
	}
	if registration.Device.UpdatedAt.IsZero() {
		registration.Device.UpdatedAt = now
	}
	if registration.Credential.CreatedAt.IsZero() {
		registration.Credential.CreatedAt = now
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin printer registration: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO devices
		(id, asset_code, display_name, manufacturer, model, serial, sys_object_id, department, status, first_seen_at, last_seen_at, created_at, updated_at)
		VALUES (?, NULLIF(?, ''), ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?)`,
		registration.Device.ID, registration.Device.AssetCode, registration.Device.DisplayName, registration.Device.Manufacturer, registration.Device.Model, registration.Device.Serial, registration.Device.SysObjectID, registration.Device.Department, registration.Device.Status,
		formatTime(registration.Device.FirstSeenAt), formatTime(registration.Device.LastSeenAt), registration.Device.CreatedAt.Format(time.RFC3339Nano), registration.Device.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("register device: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO snmp_credentials(id, version, encrypted_secret_material, security_metadata, created_at) VALUES (?, ?, ?, ?, ?)`, registration.Credential.ID, registration.Credential.Version, registration.Credential.EncryptedSecretMaterial, registration.Credential.SecurityMetadata, registration.Credential.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("register credential: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO device_endpoints(id, device_id, address, protocol, port, credential_id, is_primary) VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?)`, registration.Endpoint.ID, registration.Endpoint.DeviceID, registration.Endpoint.Address, registration.Endpoint.Protocol, registration.Endpoint.Port, registration.Endpoint.CredentialID, boolInt(registration.Endpoint.IsPrimary))
	if err != nil {
		return fmt.Errorf("register endpoint: %w", err)
	}
	if registration.Profile != nil {
		var existingID, existingChecksum string
		err = tx.QueryRowContext(ctx, `SELECT id, checksum FROM snmp_profiles WHERE profile_key = ? AND version = ?`, registration.Profile.ProfileKey, registration.Profile.Version).Scan(&existingID, &existingChecksum)
		switch {
		case err == sql.ErrNoRows:
			_, err = tx.ExecContext(ctx, `INSERT INTO snmp_profiles(id, profile_key, version, schema_version, content, checksum, verification_status) VALUES (?, ?, ?, ?, ?, ?, ?)`, registration.Profile.ID, registration.Profile.ProfileKey, registration.Profile.Version, registration.Profile.SchemaVersion, registration.Profile.Content, registration.Profile.Checksum, registration.Profile.VerificationStatus)
		case err == nil:
			if existingID != registration.Profile.ID || existingChecksum != registration.Profile.Checksum {
				return fmt.Errorf("profile %q version %d conflicts with stored content", registration.Profile.ProfileKey, registration.Profile.Version)
			}
		default:
			return fmt.Errorf("check stored profile: %w", err)
		}
		if err != nil {
			return fmt.Errorf("store profile: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO device_profile_assignments(device_id, profile_id, assigned_at, explicit) VALUES (?, ?, ?, ?)`, registration.Device.ID, registration.Profile.ID, now.Format(time.RFC3339Nano), boolInt(registration.ProfileExplicit)); err != nil {
			return fmt.Errorf("assign profile: %w", err)
		}
		for _, definition := range registration.CounterDefinitions {
			if _, err := tx.ExecContext(ctx, `INSERT INTO counter_definitions(id, device_id, key, source_protocol, oid, instance, unit, semantic_type, scope, verified, mode, unit_oid, selection, aggregation, require_unit_validation) VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?)`, definition.ID, definition.DeviceID, definition.Key, definition.SourceProtocol, definition.OID, definition.Instance, definition.Unit, definition.SemanticType, definition.Scope, boolInt(definition.Verified), definition.Mode, definition.UnitOID, definition.Selection, definition.Aggregation, boolInt(definition.RequireUnitValidation)); err != nil {
				return fmt.Errorf("store counter definition %q: %w", definition.Key, err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit printer registration: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) CreateSNMPCredential(ctx context.Context, credential SNMPCredential) error {
	if credential.ID == "" || credential.Version == "" || len(credential.EncryptedSecretMaterial) == 0 {
		return fmt.Errorf("credential id, version and encrypted material are required")
	}
	if credential.SecurityMetadata == "" {
		credential.SecurityMetadata = "{}"
	}
	if credential.CreatedAt.IsZero() {
		credential.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO snmp_credentials(id, version, encrypted_secret_material, security_metadata, created_at) VALUES (?, ?, ?, ?, ?)`, credential.ID, credential.Version, credential.EncryptedSecretMaterial, credential.SecurityMetadata, credential.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create SNMP credential: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) GetSNMPCredential(ctx context.Context, id string) (SNMPCredential, error) {
	var credential SNMPCredential
	var createdAt string
	err := r.db.QueryRowContext(ctx, `SELECT id, version, encrypted_secret_material, security_metadata, created_at FROM snmp_credentials WHERE id = ?`, id).Scan(&credential.ID, &credential.Version, &credential.EncryptedSecretMaterial, &credential.SecurityMetadata, &createdAt)
	if err != nil {
		return SNMPCredential{}, fmt.Errorf("get SNMP credential: %w", err)
	}
	credential.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	return credential, nil
}

func (d Device) Validate() error {
	if strings.TrimSpace(d.ID) == "" {
		return fmt.Errorf("device id is required")
	}
	if strings.TrimSpace(d.DisplayName) == "" {
		return fmt.Errorf("device display name is required")
	}
	if d.Status == "" {
		return fmt.Errorf("device status is required")
	}
	return nil
}

func (r *SQLiteRepository) CreateDevice(ctx context.Context, device Device) error {
	if err := device.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	if device.CreatedAt.IsZero() {
		device.CreatedAt = now
	}
	if device.UpdatedAt.IsZero() {
		device.UpdatedAt = now
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO devices
		(id, asset_code, display_name, manufacturer, model, serial, sys_object_id, department, status, first_seen_at, last_seen_at, created_at, updated_at)
		VALUES (?, NULLIF(?, ''), ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?)`,
		device.ID, device.AssetCode, device.DisplayName, device.Manufacturer, device.Model, device.Serial, device.SysObjectID, device.Department, device.Status,
		formatTime(device.FirstSeenAt), formatTime(device.LastSeenAt), device.CreatedAt.UTC().Format(time.RFC3339Nano), device.UpdatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create device: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) GetDevice(ctx context.Context, id string) (Device, error) {
	var d Device
	var asset, manufacturer, model, serial, objectID, department, firstSeen, lastSeen sql.NullString
	var createdAt, updatedAt sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT id, asset_code, display_name, manufacturer, model, serial, sys_object_id, department, status, first_seen_at, last_seen_at, created_at, updated_at
		FROM devices WHERE id = ? AND deleted_at IS NULL`, id).Scan(&d.ID, &asset, &d.DisplayName, &manufacturer, &model, &serial, &objectID, &department, &d.Status, &firstSeen, &lastSeen, &createdAt, &updatedAt)
	if err != nil {
		return Device{}, fmt.Errorf("get device: %w", err)
	}
	d.AssetCode, d.Manufacturer, d.Model, d.Serial, d.SysObjectID, d.Department = nullString(asset), nullString(manufacturer), nullString(model), nullString(serial), nullString(objectID), nullString(department)
	d.FirstSeenAt = parseTime(firstSeen)
	d.LastSeenAt = parseTime(lastSeen)
	d.CreatedAt, d.UpdatedAt = parseTime(createdAt), parseTime(updatedAt)
	return d, nil
}

func (r *SQLiteRepository) ListDevices(ctx context.Context, limit, offset int) ([]Device, error) {
	if limit < 1 || limit > 1000 || offset < 0 {
		return nil, fmt.Errorf("invalid pagination")
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, asset_code, display_name, manufacturer, model, serial, sys_object_id, department, status, first_seen_at, last_seen_at, created_at, updated_at
		FROM devices WHERE deleted_at IS NULL ORDER BY created_at DESC, id LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()
	result := make([]Device, 0)
	for rows.Next() {
		var d Device
		var asset, manufacturer, model, serial, objectID, department, firstSeen, lastSeen, createdAt, updatedAt sql.NullString
		if err := rows.Scan(&d.ID, &asset, &d.DisplayName, &manufacturer, &model, &serial, &objectID, &department, &d.Status, &firstSeen, &lastSeen, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		d.AssetCode, d.Manufacturer, d.Model, d.Serial, d.SysObjectID, d.Department = nullString(asset), nullString(manufacturer), nullString(model), nullString(serial), nullString(objectID), nullString(department)
		d.FirstSeenAt, d.LastSeenAt = parseTime(firstSeen), parseTime(lastSeen)
		d.CreatedAt, d.UpdatedAt = parseTime(createdAt), parseTime(updatedAt)
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate devices: %w", err)
	}
	return result, nil
}

func (r *SQLiteRepository) CountDevices(ctx context.Context) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM devices WHERE deleted_at IS NULL").Scan(&count); err != nil {
		return 0, fmt.Errorf("count devices: %w", err)
	}
	return count, nil
}

func (r *SQLiteRepository) GetDeviceProfileVersion(ctx context.Context, deviceID string) (int, error) {
	var version sql.NullInt64
	err := r.db.QueryRowContext(ctx, `SELECT sp.version
		FROM device_profile_assignments dpa
		JOIN snmp_profiles sp ON sp.id = dpa.profile_id
		WHERE dpa.device_id = ?`, deviceID).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get device profile version: %w", err)
	}
	return int(version.Int64), nil
}

func (r *SQLiteRepository) SoftDeleteDevice(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "UPDATE devices SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL", time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return fmt.Errorf("soft-delete device: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type PollingRun struct {
	ID             string
	DeviceID       string
	JobKind        string
	StartedAt      time.Time
	Result         string
	ErrorCode      string
	ProfileVersion int
	AttemptCount   int
}

type PollingRunRecord struct {
	ID             string
	DeviceID       string
	JobKind        string
	StartedAt      time.Time
	EndedAt        time.Time
	Result         string
	ErrorCode      string
	ProfileVersion int
	AttemptCount   int
}

type JobRecord struct {
	ID, DeviceID, Kind, Status, ErrorCode string
	CreatedAt, StartedAt, EndedAt         time.Time
}

func (r *SQLiteRepository) CreateJob(ctx context.Context, job JobRecord) error {
	if job.ID == "" || job.DeviceID == "" || job.Kind == "" {
		return fmt.Errorf("job id, device id and kind are required")
	}
	if job.Status == "" {
		job.Status = "queued"
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO jobs(id, device_id, kind, status, error_code, created_at, started_at, ended_at) VALUES (?, ?, ?, ?, NULLIF(?, ''), ?, NULLIF(?, ''), NULLIF(?, ''))`, job.ID, job.DeviceID, job.Kind, job.Status, job.ErrorCode, formatTime(job.CreatedAt), formatTime(job.StartedAt), formatTime(job.EndedAt))
	if err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) GetJob(ctx context.Context, id string) (JobRecord, error) {
	var job JobRecord
	var errorCode, createdAt, startedAt, endedAt sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT id, device_id, kind, status, error_code, created_at, started_at, ended_at FROM jobs WHERE id = ?`, id).Scan(&job.ID, &job.DeviceID, &job.Kind, &job.Status, &errorCode, &createdAt, &startedAt, &endedAt)
	if err != nil {
		return JobRecord{}, fmt.Errorf("get job: %w", err)
	}
	job.ErrorCode = nullString(errorCode)
	var parseErr error
	job.CreatedAt, parseErr = parseRequiredTime(createdAt)
	if parseErr != nil {
		return JobRecord{}, fmt.Errorf("parse job created time: %w", parseErr)
	}
	job.StartedAt, job.EndedAt = parseTime(startedAt), parseTime(endedAt)
	return job, nil
}

func (r *SQLiteRepository) StartJob(ctx context.Context, id string, startedAt time.Time) error {
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	result, err := r.db.ExecContext(ctx, "UPDATE jobs SET status = 'running', started_at = ? WHERE id = ? AND status = 'queued'", formatTime(startedAt), id)
	if err != nil {
		return fmt.Errorf("start job: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *SQLiteRepository) FinishJob(ctx context.Context, id, status, errorCode string, endedAt time.Time) error {
	if status != "success" && status != "failed" {
		return fmt.Errorf("invalid job finish status %q", status)
	}
	if endedAt.IsZero() {
		endedAt = time.Now().UTC()
	}
	result, err := r.db.ExecContext(ctx, "UPDATE jobs SET status = ?, error_code = NULLIF(?, ''), ended_at = ? WHERE id = ? AND status IN ('queued', 'running')", status, errorCode, formatTime(endedAt), id)
	if err != nil {
		return fmt.Errorf("finish job: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *SQLiteRepository) ListPollingRuns(ctx context.Context, deviceID string, limit, offset int) ([]PollingRunRecord, error) {
	if strings.TrimSpace(deviceID) == "" || limit < 1 || limit > 1000 || offset < 0 {
		return nil, fmt.Errorf("invalid polling run pagination")
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, device_id, job_kind, started_at, ended_at, result, error_code, profile_version, attempt_count
		FROM polling_runs WHERE device_id = ? ORDER BY started_at DESC, id DESC LIMIT ? OFFSET ?`, deviceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list polling runs: %w", err)
	}
	defer rows.Close()
	result := make([]PollingRunRecord, 0)
	for rows.Next() {
		var run PollingRunRecord
		var startedAt, endedAt, errorCode sql.NullString
		var profileVersion sql.NullInt64
		if err := rows.Scan(&run.ID, &run.DeviceID, &run.JobKind, &startedAt, &endedAt, &run.Result, &errorCode, &profileVersion, &run.AttemptCount); err != nil {
			return nil, fmt.Errorf("scan polling run: %w", err)
		}
		var parseErr error
		run.StartedAt, parseErr = parseRequiredTime(startedAt)
		if parseErr != nil {
			return nil, fmt.Errorf("parse polling start time: %w", parseErr)
		}
		run.EndedAt = parseTime(endedAt)
		run.ErrorCode, run.ProfileVersion = nullString(errorCode), int(profileVersion.Int64)
		result = append(result, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate polling runs: %w", err)
	}
	return result, nil
}

func (r *SQLiteRepository) CountPollingRuns(ctx context.Context, deviceID string) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM polling_runs WHERE device_id = ?", deviceID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count polling runs: %w", err)
	}
	return count, nil
}

func (r *SQLiteRepository) CreatePollingRun(ctx context.Context, run PollingRun) error {
	if run.ID == "" || run.DeviceID == "" || run.JobKind == "" {
		return fmt.Errorf("polling run id, device id and job kind are required")
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = time.Now().UTC()
	}
	if run.Result == "" {
		run.Result = "running"
	}
	if run.AttemptCount < 1 {
		run.AttemptCount = 1
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO polling_runs(id, device_id, job_kind, started_at, result, error_code, profile_version, attempt_count) VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, 0), ?)`, run.ID, run.DeviceID, run.JobKind, run.StartedAt.UTC().Format(time.RFC3339Nano), run.Result, run.ErrorCode, run.ProfileVersion, run.AttemptCount)
	if err != nil {
		return fmt.Errorf("create polling run: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) FinishPollingRun(ctx context.Context, id, result, errorCode string, endedAt time.Time) error {
	if result == "" {
		return fmt.Errorf("polling result is required")
	}
	if endedAt.IsZero() {
		endedAt = time.Now().UTC()
	}
	res, err := r.db.ExecContext(ctx, "UPDATE polling_runs SET ended_at = ?, result = ?, error_code = NULLIF(?, '') WHERE id = ?", endedAt.UTC().Format(time.RFC3339Nano), result, errorCode, id)
	if err != nil {
		return fmt.Errorf("finish polling run: %w", err)
	}
	if count, _ := res.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type CounterDefinition struct {
	ID, DeviceID, Key, SourceProtocol, OID, Instance, Unit, SemanticType, Scope string
	Mode, UnitOID, Selection, Aggregation                                       string
	Verified, RequireUnitValidation                                             bool
}

type CounterReadingRecord struct {
	ID            int64
	DeviceID      string
	DefinitionID  string
	DefinitionKey string
	Unit          string
	Scope         string
	RawValue      int64
	CollectedAt   time.Time
	Quality       counter.Quality
}

type DailyUsageRecord struct {
	DefinitionKey string
	Unit          string
	Scope         string
	LocalDate     string
	Delta         int64
	Quality       counter.Quality
}

func (r *SQLiteRepository) ListDailyCounterUsage(ctx context.Context, deviceID, from, to string) ([]DailyUsageRecord, error) {
	if strings.TrimSpace(deviceID) == "" {
		return nil, fmt.Errorf("device id is required")
	}
	rows, err := r.db.QueryContext(ctx, `SELECT cd.key, cd.unit, cd.scope, dcu.local_date, dcu.delta, dcu.quality
		FROM daily_counter_usage dcu JOIN counter_definitions cd ON cd.id = dcu.counter_definition_id
		WHERE dcu.device_id = ? AND (? = '' OR dcu.local_date >= ?) AND (? = '' OR dcu.local_date <= ?)
		ORDER BY dcu.local_date, cd.key`, deviceID, from, from, to, to)
	if err != nil {
		return nil, fmt.Errorf("list daily counter usage: %w", err)
	}
	defer rows.Close()
	result := make([]DailyUsageRecord, 0)
	for rows.Next() {
		var item DailyUsageRecord
		if err := rows.Scan(&item.DefinitionKey, &item.Unit, &item.Scope, &item.LocalDate, &item.Delta, &item.Quality); err != nil {
			return nil, fmt.Errorf("scan daily counter usage: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily counter usage: %w", err)
	}
	return result, nil
}

func (r *SQLiteRepository) ListCounterReadings(ctx context.Context, deviceID string, limit, offset int) ([]CounterReadingRecord, error) {
	if strings.TrimSpace(deviceID) == "" || limit < 1 || limit > 1000 || offset < 0 {
		return nil, fmt.Errorf("invalid counter reading pagination")
	}
	rows, err := r.db.QueryContext(ctx, `SELECT cr.id, cd.device_id, cr.counter_definition_id, cd.key, cd.unit, cd.scope, cr.raw_value, cr.collected_at, cr.quality
		FROM counter_readings cr JOIN counter_definitions cd ON cd.id = cr.counter_definition_id
		WHERE cd.device_id = ? ORDER BY cr.collected_at DESC, cr.id DESC LIMIT ? OFFSET ?`, deviceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list counter readings: %w", err)
	}
	defer rows.Close()
	result := make([]CounterReadingRecord, 0)
	for rows.Next() {
		var reading CounterReadingRecord
		var collectedAt string
		if err := rows.Scan(&reading.ID, &reading.DeviceID, &reading.DefinitionID, &reading.DefinitionKey, &reading.Unit, &reading.Scope, &reading.RawValue, &collectedAt, &reading.Quality); err != nil {
			return nil, fmt.Errorf("scan counter reading: %w", err)
		}
		reading.CollectedAt, err = time.Parse(time.RFC3339Nano, collectedAt)
		if err != nil {
			return nil, fmt.Errorf("parse counter reading time: %w", err)
		}
		result = append(result, reading)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate counter readings: %w", err)
	}
	return result, nil
}

// RecalculateDailyUsage replaces only the derived aggregate for one device.
// Raw counter readings remain immutable and the operation is safe to repeat.
func (r *SQLiteRepository) RecalculateDailyUsage(ctx context.Context, deviceID string, location *time.Location) error {
	if strings.TrimSpace(deviceID) == "" {
		return fmt.Errorf("device id is required")
	}
	if location == nil {
		location = time.UTC
	}
	rows, err := r.db.QueryContext(ctx, `SELECT cr.counter_definition_id, cr.raw_value, cr.collected_at, cr.quality
		FROM counter_readings cr JOIN counter_definitions cd ON cd.id = cr.counter_definition_id
		WHERE cd.device_id = ? ORDER BY cr.collected_at ASC, cr.id ASC`, deviceID)
	if err != nil {
		return fmt.Errorf("read counter history for usage: %w", err)
	}
	defer rows.Close()
	grouped := make(map[string][]counter.Reading)
	for rows.Next() {
		var definitionID, collectedAt string
		var rawValue int64
		var quality counter.Quality
		if err := rows.Scan(&definitionID, &rawValue, &collectedAt, &quality); err != nil {
			return fmt.Errorf("scan counter history for usage: %w", err)
		}
		when, err := time.Parse(time.RFC3339Nano, collectedAt)
		if err != nil {
			return fmt.Errorf("parse counter history for usage: %w", err)
		}
		grouped[definitionID] = append(grouped[definitionID], counter.Reading{RawValue: rawValue, CollectedAt: when, Quality: quality})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate counter history for usage: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin usage recalculation: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM daily_counter_usage WHERE device_id = ?`, deviceID); err != nil {
		return fmt.Errorf("clear daily usage: %w", err)
	}
	computedAt := time.Now().UTC().Format(time.RFC3339Nano)
	for definitionID, readings := range grouped {
		usage, err := counter.AggregateDaily(readings, location, counter.Policy{})
		if err != nil {
			return fmt.Errorf("calculate daily usage for %s: %w", definitionID, err)
		}
		for _, item := range usage {
			if _, err := tx.ExecContext(ctx, `INSERT INTO daily_counter_usage(device_id, counter_definition_id, local_date, delta, quality, computed_at) VALUES (?, ?, ?, ?, ?, ?)`, deviceID, definitionID, item.LocalDate, item.Delta, item.Quality, computedAt); err != nil {
				return fmt.Errorf("store daily usage: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit usage recalculation: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) CountCounterReadings(ctx context.Context, deviceID string) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM counter_readings cr JOIN counter_definitions cd ON cd.id = cr.counter_definition_id WHERE cd.device_id = ?`, deviceID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count counter readings: %w", err)
	}
	return count, nil
}

func (r *SQLiteRepository) CreateCounterDefinition(ctx context.Context, definition CounterDefinition) error {
	if definition.ID == "" || definition.DeviceID == "" || definition.Key == "" || definition.OID == "" {
		return fmt.Errorf("counter definition identity and OID are required")
	}
	if definition.Mode == "" {
		definition.Mode = "get"
	}
	if definition.Mode != "get" && definition.Mode != "walk" {
		return fmt.Errorf("unsupported counter definition mode %q", definition.Mode)
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO counter_definitions(id, device_id, key, source_protocol, oid, instance, unit, semantic_type, scope, verified, mode, unit_oid, selection, aggregation, require_unit_validation) VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?)`, definition.ID, definition.DeviceID, definition.Key, definition.SourceProtocol, definition.OID, definition.Instance, definition.Unit, definition.SemanticType, definition.Scope, boolInt(definition.Verified), definition.Mode, definition.UnitOID, definition.Selection, definition.Aggregation, boolInt(definition.RequireUnitValidation))
	if err != nil {
		return fmt.Errorf("create counter definition: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) ListCounterDefinitions(ctx context.Context, deviceID string) ([]CounterDefinition, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, device_id, key, source_protocol, oid, instance, unit, semantic_type, scope, verified, mode, unit_oid, selection, aggregation, require_unit_validation FROM counter_definitions WHERE device_id = ? ORDER BY key, id`, deviceID)
	if err != nil {
		return nil, fmt.Errorf("list counter definitions: %w", err)
	}
	defer rows.Close()
	result := make([]CounterDefinition, 0)
	for rows.Next() {
		var definition CounterDefinition
		var instance, unitOID, selection, aggregation sql.NullString
		var verified, requireUnitValidation int
		if err := rows.Scan(&definition.ID, &definition.DeviceID, &definition.Key, &definition.SourceProtocol, &definition.OID, &instance, &definition.Unit, &definition.SemanticType, &definition.Scope, &verified, &definition.Mode, &unitOID, &selection, &aggregation, &requireUnitValidation); err != nil {
			return nil, fmt.Errorf("scan counter definition: %w", err)
		}
		definition.Instance, definition.UnitOID, definition.Selection, definition.Aggregation = nullString(instance), nullString(unitOID), nullString(selection), nullString(aggregation)
		definition.Verified, definition.RequireUnitValidation = verified == 1, requireUnitValidation == 1
		if definition.Mode == "" {
			definition.Mode = "get"
		}
		result = append(result, definition)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate counter definitions: %w", err)
	}
	return result, nil
}

func (r *SQLiteRepository) InsertCounterReading(ctx context.Context, pollRunID, definitionID string, reading counter.Reading) (bool, error) {
	if pollRunID == "" || definitionID == "" {
		return false, fmt.Errorf("poll run and counter definition are required")
	}
	if reading.CollectedAt.IsZero() {
		return false, fmt.Errorf("reading collected_at is required")
	}
	if !validQuality(reading.Quality) {
		return false, fmt.Errorf("invalid reading quality %q", reading.Quality)
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO counter_readings(poll_run_id, counter_definition_id, raw_value, collected_at, quality) VALUES (?, ?, ?, ?, ?) ON CONFLICT(poll_run_id, counter_definition_id) DO NOTHING`, pollRunID, definitionID, reading.RawValue, reading.CollectedAt.UTC().Format(time.RFC3339Nano), reading.Quality)
	if err != nil {
		return false, fmt.Errorf("insert counter reading: %w", err)
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (r *SQLiteRepository) InsertCounterEvent(ctx context.Context, deviceID, definitionID, eventType, details string, createdAt time.Time) error {
	if deviceID == "" || eventType == "" {
		return fmt.Errorf("device id and event type are required")
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO counter_events(device_id, counter_definition_id, event_type, details, created_at) VALUES (?, NULLIF(?, ''), ?, ?, ?)`, deviceID, definitionID, eventType, details, createdAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("insert counter event: %w", err)
	}
	return nil
}

func validQuality(q counter.Quality) bool {
	switch q {
	case counter.QualityValid, counter.QualityUnverified, counter.QualityUnsupported, counter.QualityUnavailable, counter.QualitySuspicious:
		return true
	}
	return false
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
func parseTime(value sql.NullString) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	parsed, _ := time.Parse(time.RFC3339Nano, value.String)
	return parsed
}

func parseRequiredTime(value sql.NullString) (time.Time, error) {
	if !value.Valid {
		return time.Time{}, fmt.Errorf("timestamp is null")
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}
func nullString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
