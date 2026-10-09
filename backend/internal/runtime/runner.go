package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
}

func NewRunner(repo *repository.SQLiteRepository, poller *ingestion.StatusPoller, counterPoller *ingestion.CounterPoller, scheduler *polling.Scheduler, counterService *polling.Service, retention repository.CleanupPolicy) (*Runner, error) {
	if repo == nil || poller == nil || counterPoller == nil || scheduler == nil || counterService == nil {
		return nil, fmt.Errorf("runtime runner dependencies are required")
	}
	return &Runner{repository: repo, poller: poller, counterPoller: counterPoller, scheduler: scheduler, counterService: counterService, retention: retention}, nil
}

func (r *Runner) RunStatus(ctx context.Context) error {
	devices, err := r.repository.ListDevices(ctx, 1000, 0)
	if err != nil {
		return err
	}
	targets := make([]polling.Target, len(devices))
	for i, device := range devices {
		targets[i] = polling.Target{ID: device.ID}
	}
	results, err := r.scheduler.Run(ctx, targets, func(pollCtx context.Context, target polling.Target) error {
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
	devices, err := r.repository.ListDevices(ctx, 1000, 0)
	if err != nil {
		return err
	}
	targets := make([]polling.RunTarget, 0)
	for _, device := range devices {
		endpoints, err := r.repository.ListDeviceEndpoints(ctx, device.ID)
		if err != nil {
			return err
		}
		definitions, err := r.repository.ListCounterDefinitions(ctx, device.ID)
		if err != nil {
			return err
		}
		profileVersion, err := r.repository.GetDeviceProfileVersion(ctx, device.ID)
		if err != nil {
			return err
		}
		if len(endpoints) == 0 {
			slog.Warn("counter poll skipped", "device_id", device.ID, "reason", "no endpoint")
			continue
		}
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
			targets = append(targets, polling.RunTarget{Target: polling.Target{ID: device.ID + ":" + definition.ID}, Run: repository.PollingRun{ID: runID, DeviceID: device.ID, JobKind: "counter", ProfileVersion: profileVersion}, CounterDefinitionID: definition.ID, Read: func(readCtx context.Context) (counter.Reading, error) {
				return r.counterPoller.Read(readCtx, endpoint, definition)
			}})
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
	slog.Info("counter poll cycle complete", "targets", len(targets), "failed", failed)
	return nil
}

func (r *Runner) RunCleanup(ctx context.Context) error {
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
	loop, err := worker.NewLoop(worker.Config{StatusInterval: statusInterval, CounterInterval: counterInterval, CleanupInterval: cleanupInterval}, runner)
	if err != nil {
		return err
	}
	return loop.Run(ctx)
}
