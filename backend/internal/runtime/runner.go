package runtime

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"npms/backend/internal/counter"
	"npms/backend/internal/ingestion"
	"npms/backend/internal/polling"
	"npms/backend/internal/repository"
	"npms/backend/internal/worker"
)

type Runner struct {
	repository     *repository.SQLiteRepository
	poller         *ingestion.StatusPoller
	counterPoller  *ingestion.CounterPoller
	scheduler      *polling.Scheduler
	counterService *polling.Service
	retention      repository.CleanupPolicy
	reportLocation *time.Location
}

func NewRunner(repo *repository.SQLiteRepository, poller *ingestion.StatusPoller, counterPoller *ingestion.CounterPoller, scheduler *polling.Scheduler, counterService *polling.Service, retention repository.CleanupPolicy, reportLocation *time.Location) (*Runner, error) {
	if repo == nil || poller == nil || counterPoller == nil || scheduler == nil || counterService == nil {
		return nil, fmt.Errorf("runtime runner dependencies are required")
	}
	if reportLocation == nil {
		reportLocation = time.UTC
	}
	return &Runner{
		repository:     repo,
		poller:         poller,
		counterPoller:  counterPoller,
		scheduler:      scheduler,
		counterService: counterService,
		retention:      retention,
		reportLocation: reportLocation,
	}, nil
}

func (r *Runner) RunStatus(ctx context.Context) error {
	cycleID, err := newID()
	if err != nil {
		return err
	}
	devices, err := r.repository.ListDevices(ctx, 1000, 0)
	if err != nil {
		return err
	}
	targets := make([]polling.Target, len(devices))
	for i, device := range devices {
		targets[i] = polling.Target{ID: device.ID}
	}
	results, err := r.scheduler.Run(ctx, targets, func(pollCtx context.Context, target polling.Target) error {
		acquiredAt := time.Now().UTC()
		ownerID := "status:" + cycleID + ":" + target.ID
		acquired, err := r.repository.AcquirePollLease(pollCtx, target.ID, "status", ownerID, acquiredAt, acquiredAt.Add(45*time.Second))
		if err != nil {
			return err
		}
		if !acquired {
			return repository.ErrPollLeaseHeld
		}
		defer func() {
			releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if releaseErr := r.repository.ReleasePollLease(releaseCtx, target.ID, "status", ownerID); releaseErr != nil && !errors.Is(releaseErr, sql.ErrNoRows) {
				slog.Error("release status poll lease failed", "device_id", target.ID, "error", releaseErr)
			}
		}()
		return r.poller.PollDevice(pollCtx, target.ID)
	})
	if err != nil {
		return err
	}
	failed := 0
	for _, result := range results {
		if result.Err != nil {
			failed++
			slog.Error("status poll failed", "device_id", result.TargetID, "attempts", result.Attempts, "error", result.Err)
		}
	}
	slog.Info("status poll cycle complete", "devices", len(devices), "failed", failed)
	return nil
}

func (r *Runner) RunCounters(ctx context.Context) error {
	cycleID, err := newID()
	if err != nil {
		return err
	}
	devices, err := r.repository.ListDevices(ctx, 1000, 0)
	if err != nil {
		return err
	}
	targets := make([]polling.RunTarget, 0)
	leases := make([]struct{ deviceID, ownerID string }, 0)
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, lease := range leases {
			if releaseErr := r.repository.ReleasePollLease(releaseCtx, lease.deviceID, "counter", lease.ownerID); releaseErr != nil && !errors.Is(releaseErr, sql.ErrNoRows) {
				slog.Error("release counter poll lease failed", "device_id", lease.deviceID, "error", releaseErr)
			}
		}
	}()
	for _, device := range devices {
		endpoints, err := r.repository.ListDeviceEndpoints(ctx, device.ID)
		if err != nil {
			return err
		}
		definitions, err := r.repository.ListCounterDefinitions(ctx, device.ID)
		if err != nil {
			return err
		}
		if len(endpoints) == 0 {
			slog.Warn("counter poll skipped", "device_id", device.ID, "reason", "no endpoint")
			continue
		}
		if len(definitions) == 0 {
			slog.Warn("counter poll skipped", "device_id", device.ID, "reason", "no counter definitions")
			continue
		}
		profileVersion, err := r.repository.GetDeviceProfileVersion(ctx, device.ID)
		if err != nil {
			return err
		}
		ownerID := "counter:" + cycleID + ":" + device.ID
		acquiredAt := time.Now().UTC()
		acquired, err := r.repository.AcquirePollLease(ctx, device.ID, "counter", ownerID, acquiredAt, acquiredAt.Add(5*time.Minute))
		if err != nil {
			return err
		}
		if !acquired {
			slog.Warn("counter poll skipped", "device_id", device.ID, "reason", "poll lease held")
			continue
		}
		leases = append(leases, struct{ deviceID, ownerID string }{device.ID, ownerID})
		endpoint := endpoints[0]
		for _, candidate := range endpoints {
			if candidate.IsPrimary {
				endpoint = candidate
				break
			}
		}
		for _, definition := range definitions {
			runID, err := newID()
			if err != nil {
				return err
			}
			definition, endpoint := definition, endpoint
			targets = append(targets, polling.RunTarget{
				Target:              polling.Target{ID: device.ID + ":" + definition.ID},
				Run:                 repository.PollingRun{ID: runID, DeviceID: device.ID, JobKind: "counter", ProfileVersion: profileVersion},
				CounterDefinitionID: definition.ID,
				Read: func(readCtx context.Context) (counter.Reading, error) {
					return r.counterPoller.Read(readCtx, endpoint, definition)
				},
			})
		}
	}
	results, err := r.counterService.Run(ctx, targets)
	if err != nil {
		return err
	}
	failed := 0
	for _, result := range results {
		if result.Err != nil {
			failed++
			slog.Error("counter poll failed", "target_id", result.TargetID, "attempts", result.Attempts, "error", result.Err)
		}
	}
	for _, device := range devices {
		if err := r.repository.RecalculateDailyUsage(ctx, device.ID, r.reportLocation); err != nil {
			slog.Error("recalculate daily usage failed", "device_id", device.ID, "error", err)
		}
	}
	slog.Info("counter poll cycle complete", "targets", len(targets), "failed", failed)
	return nil
}

func (r *Runner) PollDeviceCounters(ctx context.Context, deviceID string) error {
	endpoints, err := r.repository.ListDeviceEndpoints(ctx, deviceID)
	if err != nil {
		return fmt.Errorf("list endpoints: %w", err)
	}
	definitions, err := r.repository.ListCounterDefinitions(ctx, deviceID)
	if err != nil {
		return fmt.Errorf("list counter definitions: %w", err)
	}
	if len(endpoints) == 0 {
		return fmt.Errorf("device %s has no SNMP endpoints", deviceID)
	}
	if len(definitions) == 0 {
		slog.Warn("device has no counter definitions configured, skipping counter poll", "device_id", deviceID)
		return nil
	}
	profileVersion, err := r.repository.GetDeviceProfileVersion(ctx, deviceID)
	if err != nil {
		return fmt.Errorf("get profile version: %w", err)
	}
	endpoint := endpoints[0]
	for _, candidate := range endpoints {
		if candidate.IsPrimary {
			endpoint = candidate
			break
		}
	}

	targets := make([]polling.RunTarget, 0, len(definitions))
	for _, definition := range definitions {
		runID, err := newID()
		if err != nil {
			return err
		}
		def, ep := definition, endpoint
		targets = append(targets, polling.RunTarget{
			Target:              polling.Target{ID: deviceID + ":" + def.ID},
			Run:                 repository.PollingRun{ID: runID, DeviceID: deviceID, JobKind: "counter", ProfileVersion: profileVersion},
			CounterDefinitionID: def.ID,
			Read: func(readCtx context.Context) (counter.Reading, error) {
				return r.counterPoller.Read(readCtx, ep, def)
			},
		})
	}

	results, err := r.counterService.Run(ctx, targets)
	if err != nil {
		return fmt.Errorf("run counter poll service: %w", err)
	}

	var pollErr error
	for _, res := range results {
		if res.Err != nil {
			pollErr = res.Err
			slog.Error("single device counter poll failed", "target_id", res.TargetID, "error", res.Err)
		}
	}

	loc := r.reportLocation
	if loc == nil {
		loc = time.UTC
	}
	if err := r.repository.RecalculateDailyUsage(ctx, deviceID, loc); err != nil {
		slog.Error("recalculate daily usage failed", "device_id", deviceID, "error", err)
	}

	return pollErr
}

// PollDevice polls both device status and all counter definitions for the given device.
// This implements api.StatusPoller so manual poll jobs record live counters immediately.
func (r *Runner) PollDevice(ctx context.Context, deviceID string) error {
	statusErr := r.poller.PollDevice(ctx, deviceID)
	counterErr := r.PollDeviceCounters(ctx, deviceID)
	if statusErr != nil {
		return statusErr
	}
	return counterErr
}

func (r *Runner) RunCleanup(ctx context.Context) error {
	reconcileStats, err := r.repository.ReconcileStaleRuns(ctx, repository.StaleReconciliationPolicy{
		RunningTimeout: 10 * time.Minute,
		Now:            time.Now().UTC(),
	})
	if err != nil {
		slog.Error("stale runs reconciliation failed", "error", err)
	} else if reconcileStats.InterruptedRuns > 0 || reconcileStats.FailedJobs > 0 || reconcileStats.ReleasedLeases > 0 {
		slog.Info("stale runs reconciliation complete",
			"interrupted_runs", reconcileStats.InterruptedRuns,
			"failed_jobs", reconcileStats.FailedJobs,
			"released_leases", reconcileStats.ReleasedLeases,
			"audit_events", reconcileStats.AuditEvents,
		)
	}

	stats, err := r.repository.CleanupPollerData(ctx, r.retention)
	if err != nil {
		return err
	}
	slog.Info("poller history cleanup complete", "polling_runs", stats.PollingRuns, "counter_events", stats.CounterEvents, "jobs", stats.Jobs)
	return nil
}

func newID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate run id: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func RunLoop(ctx context.Context, runner *Runner, statusInterval, counterInterval, cleanupInterval time.Duration) error {
	if stats, err := runner.repository.ReconcileStaleRuns(ctx, repository.StaleReconciliationPolicy{
		RunningTimeout: 10 * time.Minute,
		Now:            time.Now().UTC(),
	}); err != nil {
		slog.Error("startup stale runs reconciliation failed", "error", err)
	} else if stats.InterruptedRuns > 0 || stats.FailedJobs > 0 || stats.ReleasedLeases > 0 {
		slog.Info("startup stale runs reconciliation complete",
			"interrupted_runs", stats.InterruptedRuns,
			"failed_jobs", stats.FailedJobs,
			"released_leases", stats.ReleasedLeases,
			"audit_events", stats.AuditEvents,
		)
	}

	loop, err := worker.NewLoop(worker.Config{StatusInterval: statusInterval, CounterInterval: counterInterval, CleanupInterval: cleanupInterval}, runner)
	if err != nil {
		return err
	}
	return loop.Run(ctx)
}
