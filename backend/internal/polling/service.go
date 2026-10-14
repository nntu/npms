package polling

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"npms/backend/internal/counter"
	"npms/backend/internal/repository"
)

type Store interface {
	CreateCounterPollingRuns(context.Context, []repository.PollingRun) error
	CompleteCounterPoll(context.Context, repository.CounterPollCompletion) error
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
	if err := validateRunTargets(targets); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	runs := make([]repository.PollingRun, len(targets))
	for index := range targets {
		runs[index] = targets[index].Run
	}
	if err := s.store.CreateCounterPollingRuns(ctx, runs); err != nil {
		return nil, err
	}
	indexByTargetID := make(map[string]int, len(targets))
	readings := make([]counter.Reading, len(targets))
	for index, target := range targets {
		indexByTargetID[target.Target.ID] = index
	}
	results, runErr := s.scheduler.Run(ctx, targetList(targets), func(pollCtx context.Context, target Target) error {
		index := indexByTargetID[target.ID]
		item := targets[index]
		reading, err := item.Read(pollCtx)
		if err != nil {
			return err
		}
		readings[index] = reading
		return nil
	})
	finishCtx := ctx
	finishCancel := func() {}
	if ctx.Err() != nil {
		finishCtx, finishCancel = context.WithTimeout(context.Background(), 5*time.Second)
	}
	defer finishCancel()
	for _, result := range results {
		index := indexByTargetID[result.TargetID]
		item := targets[index]
		status, errorCode := "success", ""
		completion := repository.CounterPollCompletion{
			PollRunID:           item.Run.ID,
			DeviceID:            item.Run.DeviceID,
			CounterDefinitionID: item.CounterDefinitionID,
			Result:              status,
			EndedAt:             time.Now().UTC(),
			AttemptCount:        result.Attempts,
		}
		if result.Err != nil {
			status, errorCode = "failed", "poll_failed"
			details, _ := json.Marshal(map[string]string{"error": result.Err.Error(), "error_code": errorCode})
			completion.Result = status
			completion.ErrorCode = errorCode
			completion.EventType = "poll_failed"
			completion.EventDetails = string(details)
		} else {
			completion.Reading = &readings[index]
		}
		if err := s.store.CompleteCounterPoll(finishCtx, completion); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("complete counter poll %s: %w", item.Run.ID, err))
		}
	}
	return results, runErr
}

func validateRunTargets(targets []RunTarget) error {
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if target.Target.ID == "" {
			return fmt.Errorf("poll target id is required")
		}
		if _, exists := seen[target.Target.ID]; exists {
			return fmt.Errorf("duplicate poll target %q", target.Target.ID)
		}
		seen[target.Target.ID] = struct{}{}
		if target.Run.ID == "" {
			return fmt.Errorf("poll run id is required for target %q", target.Target.ID)
		}
		if target.CounterDefinitionID == "" {
			return fmt.Errorf("counter definition id is required for target %q", target.Target.ID)
		}
		if target.Read == nil {
			return fmt.Errorf("reader is required for target %q", target.Target.ID)
		}
	}
	return nil
}

func targetList(targets []RunTarget) []Target {
	result := make([]Target, len(targets))
	for i := range targets {
		result[i] = targets[i].Target
	}
	return result
}
