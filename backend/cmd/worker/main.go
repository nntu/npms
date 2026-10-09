package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"npms/backend/internal/config"
	"npms/backend/internal/counter"
	"npms/backend/internal/database"
	"npms/backend/internal/ingestion"
	"npms/backend/internal/logging"
	"npms/backend/internal/polling"
	"npms/backend/internal/profile"
	"npms/backend/internal/repository"
	"npms/backend/internal/security"
	workerapp "npms/backend/internal/worker"
)

type registryRunner struct {
	repository     *repository.SQLiteRepository
	poller         *ingestion.StatusPoller
	counterPoller  *ingestion.CounterPoller
	scheduler      *polling.Scheduler
	counterService *polling.Service
	reportLocation *time.Location
	retention      repository.CleanupPolicy
}

func (r registryRunner) RunStatus(ctx context.Context) error {
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

func (r registryRunner) RunCounters(ctx context.Context) error {
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
			targets = append(targets, polling.RunTarget{Target: polling.Target{ID: device.ID + ":" + definition.ID}, Run: repository.PollingRun{ID: runID, DeviceID: device.ID, JobKind: "counter", ProfileVersion: 1}, CounterDefinitionID: definition.ID, Read: func(readCtx context.Context) (counter.Reading, error) {
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
	for _, device := range devices {
		if err := r.repository.RecalculateDailyUsage(ctx, device.ID, r.reportLocation); err != nil {
			return fmt.Errorf("materialize usage for device %s: %w", device.ID, err)
		}
	}
	slog.Info("counter poll cycle complete", "targets", len(targets), "failed", failed)
	return nil
}

func (r registryRunner) RunCleanup(ctx context.Context) error {
	stats, err := r.repository.CleanupPollerData(ctx, r.retention)
	if err != nil {
		return err
	}
	slog.Info("poller history cleanup complete", "polling_runs", stats.PollingRuns, "counter_events", stats.CounterEvents, "jobs", stats.Jobs)
	return nil
}

func main() {
	configPath := flag.String("config", "./config.yaml", "YAML configuration path")
	once := flag.Bool("once", false, "run one registry cycle and exit")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		fail(err)
	}
	closeLog, err := logging.Setup(cfg.Logging.ErrorFile, cfg.Logging.Daily, cfg.Logging.MaxSizeMB)
	if err != nil {
		fail(fmt.Errorf("setup error log: %w", err))
	}
	defer func() { _ = closeLog() }()
	profiles, err := profile.LoadDir(cfg.Profiles.Path)
	if err != nil {
		fail(err)
	}
	slog.Info("profiles loaded", "directory", cfg.Profiles.Path, "count", len(profiles))
	statusInterval, err := cfg.StatusInterval()
	if err != nil {
		fail(err)
	}
	counterInterval, err := cfg.CounterInterval()
	if err != nil {
		fail(err)
	}
	cleanupInterval, err := cfg.CleanupInterval()
	if err != nil {
		fail(err)
	}
	reportLocation, err := time.LoadLocation(cfg.Report.Timezone)
	if err != nil {
		fail(fmt.Errorf("load report timezone: %w", err))
	}

	db, err := database.OpenSQLite(cfg.Database.Path)
	if err != nil {
		fail(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background()); err != nil {
		fail(err)
	}
	repo, err := repository.NewSQLiteRepository(db.DB)
	if err != nil {
		fail(err)
	}
	key, err := security.ParseKey(cfg.Security.EncryptionKey)
	if err != nil {
		fail(err)
	}
	box, err := security.NewSecretBox(key)
	if err != nil {
		fail(err)
	}
	resolver := ingestion.ConfigResolver{Store: repo, Box: box}
	poller, err := ingestion.NewStatusPoller(repo, resolver, ingestion.ConnectSNMP)
	if err != nil {
		fail(err)
	}
	scheduler, err := polling.NewScheduler(polling.Config{Concurrency: cfg.Polling.Concurrency, MaxAttempts: 2, RetryDelay: time.Second})
	if err != nil {
		fail(err)
	}
	counterPoller, err := ingestion.NewCounterPoller(resolver, ingestion.ConnectSNMP)
	if err != nil {
		fail(err)
	}
	counterService, err := polling.NewService(scheduler, repo)
	if err != nil {
		fail(err)
	}
	now := time.Now().UTC()
	runner := registryRunner{repository: repo, poller: poller, counterPoller: counterPoller, scheduler: scheduler, counterService: counterService, reportLocation: reportLocation, retention: repository.CleanupPolicy{PollingRunsBefore: now.AddDate(0, 0, -cfg.Retention.PollingRunsDays), CounterEventsBefore: now.AddDate(0, 0, -cfg.Retention.CounterEventsDays), JobsBefore: now.AddDate(0, 0, -cfg.Retention.JobsDays)}}
	if *once {
		if err := runner.RunStatus(context.Background()); err != nil {
			fail(err)
		}
		return
	}
	loop, err := workerapp.NewLoop(workerapp.Config{StatusInterval: statusInterval, CounterInterval: counterInterval, CleanupInterval: cleanupInterval}, runner)
	if err != nil {
		fail(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := loop.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func newID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate run id: %w", err)
	}
	return hex.EncodeToString(value), nil
}
