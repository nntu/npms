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

type StaleReconciliationPolicy struct {
	RunningTimeout time.Duration
	Now            time.Time
}

type StaleReconciliationStats struct {
	InterruptedRuns int64
	FailedJobs      int64
	ReleasedLeases  int64
	AuditEvents     int64
}

// ReconcileStaleRuns identifies stale/timed-out polling runs in 'running' state,
// marks them as 'interrupted' with 'poll_timeout', marks stale jobs as 'failed'
// with 'job_timeout', creates audit records in counter_events, and releases expired poll leases.
func (r *SQLiteRepository) ReconcileStaleRuns(ctx context.Context, policy StaleReconciliationPolicy) (StaleReconciliationStats, error) {
	now := policy.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	timeout := policy.RunningTimeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	threshold := now.Add(-timeout)
	nowStr := now.Format(time.RFC3339Nano)
	thresholdStr := threshold.Format(time.RFC3339Nano)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return StaleReconciliationStats{}, fmt.Errorf("begin stale reconciliation: %w", err)
	}
	defer tx.Rollback()

	var stats StaleReconciliationStats

	// 1. Reconcile stale polling runs (running past timeout or ended_at IS NULL past threshold)
	rows, err := tx.QueryContext(ctx, `SELECT id, device_id, job_kind, started_at
		FROM polling_runs
		WHERE (result = 'running' OR ended_at IS NULL) AND started_at < ?`, thresholdStr)
	if err != nil {
		return StaleReconciliationStats{}, fmt.Errorf("query stale polling runs: %w", err)
	}
	type staleRunItem struct {
		id        string
		deviceID  string
		jobKind   string
		startedAt string
	}
	var staleRuns []staleRunItem
	for rows.Next() {
		var item staleRunItem
		if err := rows.Scan(&item.id, &item.deviceID, &item.jobKind, &item.startedAt); err != nil {
			rows.Close()
			return StaleReconciliationStats{}, fmt.Errorf("scan stale polling run: %w", err)
		}
		staleRuns = append(staleRuns, item)
	}
	if err := rows.Err(); err != nil {
		return StaleReconciliationStats{}, fmt.Errorf("iterate stale polling runs: %w", err)
	}
	rows.Close()

	for _, item := range staleRuns {
		res, err := tx.ExecContext(ctx, `UPDATE polling_runs
			SET ended_at = ?, result = 'interrupted', error_code = 'poll_timeout'
			WHERE id = ? AND (result = 'running' OR ended_at IS NULL)`, nowStr, item.id)
		if err != nil {
			return StaleReconciliationStats{}, fmt.Errorf("interrupt stale polling run %s: %w", item.id, err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return StaleReconciliationStats{}, fmt.Errorf("count interrupted polling run %s: %w", item.id, err)
		}
		if affected > 0 {
			stats.InterruptedRuns += affected
			details := fmt.Sprintf(`{"poll_run_id":%q,"job_kind":%q,"started_at":%q,"reason":"run exceeded timeout threshold of %s"}`, item.id, item.jobKind, item.startedAt, timeout)
			if _, err := tx.ExecContext(ctx, `INSERT INTO counter_events(device_id, counter_definition_id, event_type, details, created_at)
				VALUES (?, NULL, 'poll_interrupted', ?, ?)`, item.deviceID, details, nowStr); err != nil {
				return StaleReconciliationStats{}, fmt.Errorf("insert audit event for run %s: %w", item.id, err)
			}
			stats.AuditEvents++
		}
	}

	// 2. Reconcile stale jobs (queued or running past threshold)
	jobRows, err := tx.QueryContext(ctx, `SELECT id, device_id, kind, created_at, started_at
		FROM jobs
		WHERE status IN ('queued', 'running') AND ended_at IS NULL
		  AND ((started_at IS NOT NULL AND started_at < ?) OR (started_at IS NULL AND created_at < ?))`, thresholdStr, thresholdStr)
	if err != nil {
		return StaleReconciliationStats{}, fmt.Errorf("query stale jobs: %w", err)
	}
	type staleJobItem struct {
		id        string
		deviceID  string
		kind      string
		createdAt string
		startedAt sql.NullString
	}
	var staleJobs []staleJobItem
	for jobRows.Next() {
		var j staleJobItem
		if err := jobRows.Scan(&j.id, &j.deviceID, &j.kind, &j.createdAt, &j.startedAt); err != nil {
			jobRows.Close()
			return StaleReconciliationStats{}, fmt.Errorf("scan stale job: %w", err)
		}
		staleJobs = append(staleJobs, j)
	}
	if err := jobRows.Err(); err != nil {
		return StaleReconciliationStats{}, fmt.Errorf("iterate stale jobs: %w", err)
	}
	jobRows.Close()

	for _, j := range staleJobs {
		res, err := tx.ExecContext(ctx, `UPDATE jobs
			SET ended_at = ?, status = 'failed', error_code = 'job_timeout'
			WHERE id = ? AND status IN ('queued', 'running')`, nowStr, j.id)
		if err != nil {
			return StaleReconciliationStats{}, fmt.Errorf("fail stale job %s: %w", j.id, err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return StaleReconciliationStats{}, fmt.Errorf("count failed job %s: %w", j.id, err)
		}
		if affected > 0 {
			stats.FailedJobs += affected
			if j.deviceID != "" {
				details := fmt.Sprintf(`{"job_id":%q,"kind":%q,"reason":"job exceeded timeout threshold of %s"}`, j.id, j.kind, timeout)
				if _, err := tx.ExecContext(ctx, `INSERT INTO counter_events(device_id, counter_definition_id, event_type, details, created_at)
					VALUES (?, NULL, 'job_timeout', ?, ?)`, j.deviceID, details, nowStr); err != nil {
					return StaleReconciliationStats{}, fmt.Errorf("insert audit event for job %s: %w", j.id, err)
				}
				stats.AuditEvents++
			}
		}
	}

	// 3. Release expired poll leases
	leaseRes, err := tx.ExecContext(ctx, `DELETE FROM poll_leases WHERE lease_until <= ?`, nowStr)
	if err != nil {
		return StaleReconciliationStats{}, fmt.Errorf("release expired poll leases: %w", err)
	}
	stats.ReleasedLeases, err = leaseRes.RowsAffected()
	if err != nil {
		return StaleReconciliationStats{}, fmt.Errorf("count released poll leases: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return StaleReconciliationStats{}, fmt.Errorf("commit stale reconciliation: %w", err)
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
	Address      string
	Port         uint16
	SNMPVersion  string
	ProfileKey   string
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
	formatted := at.UTC().Format(time.RFC3339Nano)
	_, _ = r.db.ExecContext(ctx, "UPDATE devices SET status = 'online', last_seen_at = ?, first_seen_at = COALESCE(first_seen_at, ?) WHERE id = (SELECT device_id FROM device_endpoints WHERE id = ?)", formatted, formatted, endpointID)
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

type PrinterUpdate struct {
	Device             Device
	EndpointAddress    string
	EndpointPort       uint16
	NewEndpointID      string
	Credential         *SNMPCredential
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
	var createdAt, updatedAt, address, snmpVersion, profileKey sql.NullString
	var port sql.NullInt64
	err := r.db.QueryRowContext(ctx, `SELECT
		d.id, d.asset_code, d.display_name, d.manufacturer, d.model, d.serial, d.sys_object_id, d.department, d.status, d.first_seen_at, d.last_seen_at, d.created_at, d.updated_at,
		(SELECT de.address FROM device_endpoints de WHERE de.device_id = d.id ORDER BY de.is_primary DESC, de.id LIMIT 1),
		(SELECT de.port FROM device_endpoints de WHERE de.device_id = d.id ORDER BY de.is_primary DESC, de.id LIMIT 1),
		(SELECT sc.version FROM device_endpoints de JOIN snmp_credentials sc ON sc.id = de.credential_id WHERE de.device_id = d.id ORDER BY de.is_primary DESC, de.id LIMIT 1),
		(SELECT sp.profile_key FROM device_profile_assignments dpa JOIN snmp_profiles sp ON sp.id = dpa.profile_id WHERE dpa.device_id = d.id LIMIT 1)
		FROM devices d WHERE d.id = ? AND d.deleted_at IS NULL`, id).Scan(
		&d.ID, &asset, &d.DisplayName, &manufacturer, &model, &serial, &objectID, &department, &d.Status, &firstSeen, &lastSeen, &createdAt, &updatedAt,
		&address, &port, &snmpVersion, &profileKey,
	)
	if err != nil {
		return Device{}, fmt.Errorf("get device: %w", err)
	}
	d.AssetCode, d.Manufacturer, d.Model, d.Serial, d.SysObjectID, d.Department = nullString(asset), nullString(manufacturer), nullString(model), nullString(serial), nullString(objectID), nullString(department)
	d.Address, d.SNMPVersion, d.ProfileKey = nullString(address), nullString(snmpVersion), nullString(profileKey)
	if port.Valid && port.Int64 > 0 && port.Int64 <= 65535 {
		d.Port = uint16(port.Int64)
	}
	d.FirstSeenAt = parseTime(firstSeen)
	d.LastSeenAt = parseTime(lastSeen)
	d.CreatedAt, d.UpdatedAt = parseTime(createdAt), parseTime(updatedAt)
	return d, nil
}

func (r *SQLiteRepository) ListDevices(ctx context.Context, limit, offset int) ([]Device, error) {
	if limit < 1 || limit > 1000 || offset < 0 {
		return nil, fmt.Errorf("invalid pagination")
	}
	rows, err := r.db.QueryContext(ctx, `SELECT
		d.id, d.asset_code, d.display_name, d.manufacturer, d.model, d.serial, d.sys_object_id, d.department, d.status, d.first_seen_at, d.last_seen_at, d.created_at, d.updated_at,
		(SELECT de.address FROM device_endpoints de WHERE de.device_id = d.id ORDER BY de.is_primary DESC, de.id LIMIT 1),
		(SELECT de.port FROM device_endpoints de WHERE de.device_id = d.id ORDER BY de.is_primary DESC, de.id LIMIT 1),
		(SELECT sc.version FROM device_endpoints de JOIN snmp_credentials sc ON sc.id = de.credential_id WHERE de.device_id = d.id ORDER BY de.is_primary DESC, de.id LIMIT 1),
		(SELECT sp.profile_key FROM device_profile_assignments dpa JOIN snmp_profiles sp ON sp.id = dpa.profile_id WHERE dpa.device_id = d.id LIMIT 1)
		FROM devices d WHERE d.deleted_at IS NULL ORDER BY d.created_at DESC, d.id LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()
	result := make([]Device, 0)
	for rows.Next() {
		var d Device
		var asset, manufacturer, model, serial, objectID, department, firstSeen, lastSeen, createdAt, updatedAt sql.NullString
		var address, snmpVersion, profileKey sql.NullString
		var port sql.NullInt64
		if err := rows.Scan(
			&d.ID, &asset, &d.DisplayName, &manufacturer, &model, &serial, &objectID, &department, &d.Status, &firstSeen, &lastSeen, &createdAt, &updatedAt,
			&address, &port, &snmpVersion, &profileKey,
		); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		d.AssetCode, d.Manufacturer, d.Model, d.Serial, d.SysObjectID, d.Department = nullString(asset), nullString(manufacturer), nullString(model), nullString(serial), nullString(objectID), nullString(department)
		d.Address, d.SNMPVersion, d.ProfileKey = nullString(address), nullString(snmpVersion), nullString(profileKey)
		if port.Valid && port.Int64 > 0 && port.Int64 <= 65535 {
			d.Port = uint16(port.Int64)
		}
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

func (r *SQLiteRepository) UpdatePrinter(ctx context.Context, update PrinterUpdate) error {
	if strings.TrimSpace(update.Device.ID) == "" || strings.TrimSpace(update.Device.DisplayName) == "" {
		return fmt.Errorf("device id and display name are required")
	}
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin printer update: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `UPDATE devices SET
		asset_code = NULLIF(?, ''),
		display_name = ?,
		manufacturer = NULLIF(?, ''),
		model = NULLIF(?, ''),
		serial = NULLIF(?, ''),
		sys_object_id = NULLIF(?, ''),
		department = NULLIF(?, ''),
		updated_at = ?
		WHERE id = ? AND deleted_at IS NULL`,
		update.Device.AssetCode, update.Device.DisplayName, update.Device.Manufacturer, update.Device.Model,
		update.Device.Serial, update.Device.SysObjectID, update.Device.Department, now.Format(time.RFC3339Nano), update.Device.ID)
	if err != nil {
		return fmt.Errorf("update device: %w", err)
	}
	if count, _ := res.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}

	var newCredentialID string
	if update.Credential != nil {
		if update.Credential.ID == "" || update.Credential.Version == "" || len(update.Credential.EncryptedSecretMaterial) == 0 {
			return fmt.Errorf("credential id, version and encrypted material are required")
		}
		createdAt := update.Credential.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO snmp_credentials(id, version, encrypted_secret_material, security_metadata, created_at) VALUES (?, ?, ?, ?, ?)`,
			update.Credential.ID, update.Credential.Version, update.Credential.EncryptedSecretMaterial, update.Credential.SecurityMetadata, createdAt.Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("store updated credential: %w", err)
		}
		newCredentialID = update.Credential.ID
	}

	if strings.TrimSpace(update.EndpointAddress) != "" {
		port := update.EndpointPort
		if port == 0 {
			port = 161
		}
		var existingEndpointID string
		err := tx.QueryRowContext(ctx, `SELECT id FROM device_endpoints WHERE device_id = ? ORDER BY is_primary DESC, id LIMIT 1`, update.Device.ID).Scan(&existingEndpointID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			endpointID := update.NewEndpointID
			if endpointID == "" {
				endpointID = update.Device.ID + ":primary"
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO device_endpoints(id, device_id, address, protocol, port, credential_id, is_primary) VALUES (?, ?, ?, 'snmp', ?, NULLIF(?, ''), 1)`,
				endpointID, update.Device.ID, update.EndpointAddress, port, newCredentialID); err != nil {
				return fmt.Errorf("insert primary endpoint: %w", err)
			}
		case err == nil:
			if newCredentialID != "" {
				_, err = tx.ExecContext(ctx, `UPDATE device_endpoints SET address = ?, port = ?, credential_id = ? WHERE id = ?`,
					update.EndpointAddress, port, newCredentialID, existingEndpointID)
			} else {
				_, err = tx.ExecContext(ctx, `UPDATE device_endpoints SET address = ?, port = ? WHERE id = ?`,
					update.EndpointAddress, port, existingEndpointID)
			}
			if err != nil {
				return fmt.Errorf("update primary endpoint: %w", err)
			}
		default:
			return fmt.Errorf("lookup primary endpoint: %w", err)
		}
	} else if newCredentialID != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE device_endpoints SET credential_id = ? WHERE id = (SELECT id FROM device_endpoints WHERE device_id = ? ORDER BY is_primary DESC, id LIMIT 1)`,
			newCredentialID, update.Device.ID); err != nil {
			return fmt.Errorf("update endpoint credential: %w", err)
		}
	}

	if update.Profile != nil {
		var existingID, existingChecksum string
		err = tx.QueryRowContext(ctx, `SELECT id, checksum FROM snmp_profiles WHERE profile_key = ? AND version = ?`, update.Profile.ProfileKey, update.Profile.Version).Scan(&existingID, &existingChecksum)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			_, err = tx.ExecContext(ctx, `INSERT INTO snmp_profiles(id, profile_key, version, schema_version, content, checksum, verification_status) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				update.Profile.ID, update.Profile.ProfileKey, update.Profile.Version, update.Profile.SchemaVersion, update.Profile.Content, update.Profile.Checksum, update.Profile.VerificationStatus)
		case err == nil:
			if existingID != update.Profile.ID || existingChecksum != update.Profile.Checksum {
				return fmt.Errorf("profile %q version %d conflicts with stored content", update.Profile.ProfileKey, update.Profile.Version)
			}
		default:
			return fmt.Errorf("check stored profile: %w", err)
		}
		if err != nil {
			return fmt.Errorf("store profile: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO device_profile_assignments(device_id, profile_id, assigned_at, explicit)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(device_id) DO UPDATE SET profile_id = excluded.profile_id, assigned_at = excluded.assigned_at, explicit = excluded.explicit`,
			update.Device.ID, update.Profile.ID, now.Format(time.RFC3339Nano), boolInt(update.ProfileExplicit)); err != nil {
			return fmt.Errorf("assign profile: %w", err)
		}
		for _, definition := range update.CounterDefinitions {
			if _, err := tx.ExecContext(ctx, `INSERT INTO counter_definitions(id, device_id, key, source_protocol, oid, instance, unit, semantic_type, scope, verified, mode, unit_oid, selection, aggregation, require_unit_validation)
				VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?)
				ON CONFLICT(id) DO UPDATE SET
					oid = excluded.oid,
					instance = COALESCE(excluded.instance, counter_definitions.instance),
					unit = excluded.unit,
					semantic_type = excluded.semantic_type,
					scope = excluded.scope,
					verified = excluded.verified,
					mode = excluded.mode,
					unit_oid = excluded.unit_oid,
					selection = excluded.selection,
					aggregation = excluded.aggregation,
					require_unit_validation = excluded.require_unit_validation`,
				definition.ID, definition.DeviceID, definition.Key, definition.SourceProtocol, definition.OID, definition.Instance, definition.Unit, definition.SemanticType, definition.Scope, boolInt(definition.Verified), definition.Mode, definition.UnitOID, definition.Selection, definition.Aggregation, boolInt(definition.RequireUnitValidation)); err != nil {
				return fmt.Errorf("upsert counter definition %q: %w", definition.Key, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit printer update: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) SoftDeleteDevice(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := r.db.ExecContext(ctx, "UPDATE devices SET deleted_at = ?, updated_at = ?, asset_code = NULL WHERE id = ? AND deleted_at IS NULL", now, now, id)
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

// CounterPollCompletion is the atomic terminal state of one counter poll.
// A successful completion carries exactly one immutable reading; a failed
// completion carries exactly one audit event.
type CounterPollCompletion struct {
	PollRunID           string
	DeviceID            string
	CounterDefinitionID string
	Reading             *counter.Reading
	Result              string
	ErrorCode           string
	EventType           string
	EventDetails        string
	EndedAt             time.Time
	AttemptCount        int
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

type JobWithDevice struct {
	ID         string     `json:"id"`
	DeviceID   string     `json:"device_id"`
	DeviceName string     `json:"device_name"`
	DeviceIP   string     `json:"device_ip"`
	Kind       string     `json:"kind"`
	Status     string     `json:"status"`
	ErrorCode  *string    `json:"error_code,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
	DurationMs int64      `json:"duration_ms"`
}

func (r *SQLiteRepository) ListJobsWithDevice(ctx context.Context, limit, offset int, statusFilter string) ([]JobWithDevice, int, error) {
	if limit < 1 || limit > 1000 || offset < 0 {
		return nil, 0, fmt.Errorf("invalid job pagination")
	}

	var total int
	countQuery := "SELECT COUNT(1) FROM jobs"
	var countArgs []any
	if statusFilter != "" {
		countQuery += " WHERE status = ?"
		countArgs = append(countArgs, statusFilter)
	}
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count jobs: %w", err)
	}

	query := `SELECT j.id, j.device_id, COALESCE(d.display_name, ''), COALESCE(ep.address, ''),
		j.kind, j.status, j.error_code, j.created_at, j.started_at, j.ended_at
		FROM jobs j
		LEFT JOIN devices d ON j.device_id = d.id
		LEFT JOIN device_endpoints ep ON ep.device_id = d.id AND ep.is_primary = 1`
	var args []any
	if statusFilter != "" {
		query += " WHERE j.status = ?"
		args = append(args, statusFilter)
	}
	query += " ORDER BY j.created_at DESC, j.id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list jobs with device: %w", err)
	}
	defer rows.Close()

	jobs := make([]JobWithDevice, 0)
	for rows.Next() {
		var item JobWithDevice
		var errCode, createdAt, startedAt, endedAt sql.NullString
		if err := rows.Scan(&item.ID, &item.DeviceID, &item.DeviceName, &item.DeviceIP, &item.Kind, &item.Status, &errCode, &createdAt, &startedAt, &endedAt); err != nil {
			return nil, 0, fmt.Errorf("scan job row: %w", err)
		}
		if errCode.Valid && errCode.String != "" {
			item.ErrorCode = &errCode.String
		}
		item.CreatedAt, _ = parseRequiredTime(createdAt)
		st := parseTime(startedAt)
		if !st.IsZero() {
			item.StartedAt = &st
		}
		et := parseTime(endedAt)
		if !et.IsZero() {
			item.EndedAt = &et
		}

		if item.StartedAt != nil {
			if item.EndedAt != nil {
				item.DurationMs = item.EndedAt.Sub(*item.StartedAt).Milliseconds()
			} else {
				item.DurationMs = time.Since(*item.StartedAt).Milliseconds()
			}
		}

		jobs = append(jobs, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate job rows: %w", err)
	}
	return jobs, total, nil
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

// CreateCounterPollingRuns creates the work ledger for a counter cycle as one
// transaction. Counter runs start with zero attempts and are updated to the
// scheduler's actual attempt count when completed.
func (r *SQLiteRepository) CreateCounterPollingRuns(ctx context.Context, runs []PollingRun) error {
	seen := make(map[string]struct{}, len(runs))
	for index := range runs {
		run := &runs[index]
		if run.ID == "" || run.DeviceID == "" || run.JobKind != "counter" {
			return fmt.Errorf("counter polling run id, device id and job kind are required")
		}
		if _, exists := seen[run.ID]; exists {
			return fmt.Errorf("duplicate polling run id %q", run.ID)
		}
		seen[run.ID] = struct{}{}
		if run.AttemptCount < 0 {
			return fmt.Errorf("attempt count cannot be negative")
		}
		if run.StartedAt.IsZero() {
			run.StartedAt = time.Now().UTC()
		}
		if run.Result == "" {
			run.Result = "running"
		}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin counter polling runs: %w", err)
	}
	defer tx.Rollback()
	for _, run := range runs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO polling_runs(id, device_id, job_kind, started_at, result, error_code, profile_version, attempt_count)
			VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, 0), ?)`, run.ID, run.DeviceID, run.JobKind, run.StartedAt.UTC().Format(time.RFC3339Nano), run.Result, run.ErrorCode, run.ProfileVersion, run.AttemptCount); err != nil {
			return fmt.Errorf("create counter polling run %s: %w", run.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit counter polling runs: %w", err)
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

// CompleteCounterPoll commits the polling-run terminal state and its reading
// or failure event together. Repeating an already completed run is a no-op, so
// callers can safely retry after an uncertain commit result.
func (r *SQLiteRepository) CompleteCounterPoll(ctx context.Context, completion CounterPollCompletion) error {
	if strings.TrimSpace(completion.PollRunID) == "" || strings.TrimSpace(completion.DeviceID) == "" || strings.TrimSpace(completion.CounterDefinitionID) == "" {
		return fmt.Errorf("poll run, device and counter definition are required")
	}
	if completion.AttemptCount < 0 {
		return fmt.Errorf("attempt count cannot be negative")
	}
	switch completion.Result {
	case "success":
		if completion.Reading == nil {
			return fmt.Errorf("successful counter poll requires a reading")
		}
		if completion.Reading.CollectedAt.IsZero() {
			return fmt.Errorf("reading collected_at is required")
		}
		if !validQuality(completion.Reading.Quality) {
			return fmt.Errorf("invalid reading quality %q", completion.Reading.Quality)
		}
	case "failed":
		if completion.Reading != nil || strings.TrimSpace(completion.ErrorCode) == "" || strings.TrimSpace(completion.EventType) == "" {
			return fmt.Errorf("failed counter poll requires an error code and event without a reading")
		}
	default:
		return fmt.Errorf("unsupported counter poll result %q", completion.Result)
	}
	if completion.EndedAt.IsZero() {
		completion.EndedAt = time.Now().UTC()
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin counter poll completion: %w", err)
	}
	defer tx.Rollback()
	updated, err := tx.ExecContext(ctx, `UPDATE polling_runs
		SET ended_at = ?, result = ?, error_code = NULLIF(?, ''), attempt_count = ?
		WHERE id = ? AND device_id = ? AND ended_at IS NULL`, completion.EndedAt.UTC().Format(time.RFC3339Nano), completion.Result, completion.ErrorCode, completion.AttemptCount, completion.PollRunID, completion.DeviceID)
	if err != nil {
		return fmt.Errorf("finish counter polling run: %w", err)
	}
	count, err := updated.RowsAffected()
	if err != nil {
		return fmt.Errorf("count finished counter polling run: %w", err)
	}
	if count == 0 {
		var alreadyFinished bool
		if err := tx.QueryRowContext(ctx, `SELECT ended_at IS NOT NULL FROM polling_runs WHERE id = ? AND device_id = ?`, completion.PollRunID, completion.DeviceID).Scan(&alreadyFinished); err != nil {
			return err
		}
		if alreadyFinished {
			return nil
		}
		return sql.ErrNoRows
	}

	if completion.Result == "success" {
		reading := completion.Reading
		if _, err := tx.ExecContext(ctx, `INSERT INTO counter_readings(poll_run_id, counter_definition_id, raw_value, collected_at, quality)
			VALUES (?, ?, ?, ?, ?)`, completion.PollRunID, completion.CounterDefinitionID, reading.RawValue, reading.CollectedAt.UTC().Format(time.RFC3339Nano), reading.Quality); err != nil {
			return fmt.Errorf("insert counter reading: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx, `INSERT INTO counter_events(device_id, counter_definition_id, event_type, details, created_at)
			VALUES (?, ?, ?, ?, ?)`, completion.DeviceID, completion.CounterDefinitionID, completion.EventType, completion.EventDetails, completion.EndedAt.UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("insert counter event: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit counter poll completion: %w", err)
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
