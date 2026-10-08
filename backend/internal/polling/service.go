package polling

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"npms/backend/internal/counter"
	"npms/backend/internal/repository"
)

type Store interface {
	CreatePollingRun(context.Context, repository.PollingRun) error
	FinishPollingRun(context.Context, string, string, string, time.Time) error
	InsertCounterReading(context.Context, string, string, counter.Reading) (bool, error)
	InsertCounterEvent(context.Context, string, string, string, string, time.Time) error
}

type RunTarget struct {
	Target              Target
	Run                 repository.PollingRun
	CounterDefinitionID string
	Read                func(context.Context) (counter.Reading, error)
}

type Service struct {
	scheduler *Scheduler
	store     Store
}

func NewService(scheduler *Scheduler, store Store) (*Service, error) {
	if scheduler == nil {
		return nil, fmt.Errorf("scheduler is required")
	}
	if store == nil {
		return nil, fmt.Errorf("poll store is required")
	}
	return &Service{scheduler: scheduler, store: store}, nil
}

func (s *Service) Run(ctx context.Context, targets []RunTarget) ([]Result, error) {
	for _, target := range targets {
		if target.Read == nil {
			return nil, fmt.Errorf("reader is required for target %q", target.Target.ID)
		}
		if err := s.store.CreatePollingRun(ctx, target.Run); err != nil {
			return nil, err
		}
	}
	results, runErr := s.scheduler.Run(ctx, targetList(targets), func(pollCtx context.Context, target Target) error {
		item := findTarget(targets, target.ID)
		reading, err := item.Read(pollCtx)
		if err != nil {
			return err
		}
		_, err = s.store.InsertCounterReading(pollCtx, item.Run.ID, item.CounterDefinitionID, reading)
		return err
	})
	for _, result := range results {
		item := findTarget(targets, result.TargetID)
		status, errorCode := "success", ""
		if result.Err != nil {
			status, errorCode = "failed", "poll_failed"
		}
		if err := s.store.FinishPollingRun(ctx, item.Run.ID, status, errorCode, time.Now().UTC()); err != nil && runErr == nil {
			runErr = err
		}
		if result.Err != nil {
			details, _ := json.Marshal(map[string]string{"error": result.Err.Error(), "error_code": errorCode})
			if err := s.store.InsertCounterEvent(ctx, item.Run.DeviceID, item.CounterDefinitionID, "poll_failed", string(details), time.Now().UTC()); err != nil && runErr == nil {
				runErr = err
			}
		}
	}
	return results, runErr
}

func targetList(targets []RunTarget) []Target {
	result := make([]Target, len(targets))
	for i := range targets {
		result[i] = targets[i].Target
	}
	return result
}

func findTarget(targets []RunTarget, id string) RunTarget {
	for _, target := range targets {
		if target.Target.ID == id {
			return target
		}
	}
	return RunTarget{}
}
