package polling

import (
	"context"
	"errors"
	"testing"
	"time"

	"npms/backend/internal/counter"
	"npms/backend/internal/repository"
)

type fakeStore struct {
	runs        []repository.PollingRun
	completions []repository.CounterPollCompletion
}

func (f *fakeStore) CreateCounterPollingRuns(_ context.Context, runs []repository.PollingRun) error {
	f.runs = append(f.runs, runs...)
	return nil
}
func (f *fakeStore) CompleteCounterPoll(_ context.Context, completion repository.CounterPollCompletion) error {
	f.completions = append(f.completions, completion)
	return nil
}

func TestServicePersistsSuccessAndFailurePerTarget(t *testing.T) {
	scheduler, err := NewScheduler(Config{Concurrency: 2, MaxAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeStore{}
	service, err := NewService(scheduler, store)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	results, err := service.Run(context.Background(), []RunTarget{
		{Target: Target{ID: "ok"}, Run: repository.PollingRun{ID: "run-ok", DeviceID: "device-ok", JobKind: "counter"}, CounterDefinitionID: "counter-ok", Read: func(context.Context) (counter.Reading, error) {
			return counter.Reading{RawValue: 10, CollectedAt: now, Quality: counter.QualityValid}, nil
		}},
		{Target: Target{ID: "bad"}, Run: repository.PollingRun{ID: "run-bad", DeviceID: "device-bad", JobKind: "counter"}, CounterDefinitionID: "counter-bad", Read: func(context.Context) (counter.Reading, error) { return counter.Reading{}, errors.New("timeout") }},
	})
	if err != nil || len(results) != 2 {
		t.Fatalf("unexpected service result: %#v, %v", results, err)
	}
	if len(store.runs) != 2 || len(store.completions) != 2 {
		t.Fatalf("unexpected store calls: %#v", store)
	}
	if store.completions[0].Reading == nil || store.completions[0].AttemptCount != 1 {
		t.Fatalf("unexpected successful completion: %#v", store.completions[0])
	}
	if store.completions[1].Reading != nil || store.completions[1].EventType != "poll_failed" || store.completions[1].AttemptCount != 1 {
		t.Fatalf("unexpected failed completion: %#v", store.completions[1])
	}
}

func TestServiceFinalizesEveryRunWhenCanceled(t *testing.T) {
	scheduler, err := NewScheduler(Config{Concurrency: 1, MaxAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeStore{}
	service, err := NewService(scheduler, store)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	targets := make([]RunTarget, 3)
	for index := range targets {
		id := string(rune('a' + index))
		targets[index] = RunTarget{
			Target:              Target{ID: id},
			Run:                 repository.PollingRun{ID: "run-" + id, DeviceID: "device-1", JobKind: "counter"},
			CounterDefinitionID: "counter-" + id,
			Read: func(ctx context.Context) (counter.Reading, error) {
				cancel()
				return counter.Reading{}, ctx.Err()
			},
		}
	}
	results, runErr := service.Run(ctx, targets)
	if !errors.Is(runErr, context.Canceled) {
		t.Fatalf("run error = %v, want context canceled", runErr)
	}
	if len(results) != 3 || len(store.completions) != 3 {
		t.Fatalf("canceled flow was not finalized: results=%#v store=%#v", results, store)
	}
	for index, completion := range store.completions {
		wantID := "run-" + string(rune('a'+index))
		if completion.PollRunID != wantID || completion.Result != "failed" || completion.ErrorCode != "poll_failed" || completion.EventType != "poll_failed" {
			t.Fatalf("unexpected completion %d: %#v", index, completion)
		}
	}
}

func TestServicePersistsActualRetryAttemptCount(t *testing.T) {
	scheduler, err := NewScheduler(Config{Concurrency: 1, MaxAttempts: 3, Sleep: func(context.Context, time.Duration) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeStore{}
	service, err := NewService(scheduler, store)
	if err != nil {
		t.Fatal(err)
	}
	attempts := 0
	_, err = service.Run(context.Background(), []RunTarget{{
		Target:              Target{ID: "retry"},
		Run:                 repository.PollingRun{ID: "run-retry", DeviceID: "device-retry", JobKind: "counter"},
		CounterDefinitionID: "counter-retry",
		Read: func(context.Context) (counter.Reading, error) {
			attempts++
			if attempts < 3 {
				return counter.Reading{}, errors.New("temporary timeout")
			}
			return counter.Reading{RawValue: 42, CollectedAt: time.Now().UTC(), Quality: counter.QualityValid}, nil
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.completions) != 1 || store.completions[0].AttemptCount != 3 || store.completions[0].Reading == nil {
		t.Fatalf("unexpected retry completion: %#v", store.completions)
	}
	results, err := service.Run(context.Background(), []RunTarget{{
		Target:              Target{ID: "retry-failed"},
		Run:                 repository.PollingRun{ID: "run-retry-failed", DeviceID: "device-retry", JobKind: "counter"},
		CounterDefinitionID: "counter-retry",
		Read: func(context.Context) (counter.Reading, error) {
			return counter.Reading{}, errors.New("still offline")
		},
	}})
	if err != nil || len(results) != 1 || results[0].Err == nil {
		t.Fatalf("unexpected failed retry result: results=%#v err=%v", results, err)
	}
	failed := store.completions[1]
	if failed.AttemptCount != 3 || failed.Reading != nil || failed.EventType != "poll_failed" {
		t.Fatalf("unexpected exhausted retry completion: %#v", failed)
	}
}

func TestServiceValidatesAllTargetsBeforeCreatingRuns(t *testing.T) {
	scheduler, err := NewScheduler(Config{Concurrency: 1, MaxAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeStore{}
	service, err := NewService(scheduler, store)
	if err != nil {
		t.Fatal(err)
	}
	_, runErr := service.Run(context.Background(), []RunTarget{
		{Target: Target{ID: "duplicate"}, Run: repository.PollingRun{ID: "run-a"}, CounterDefinitionID: "counter-a", Read: func(context.Context) (counter.Reading, error) { return counter.Reading{}, nil }},
		{Target: Target{ID: "duplicate"}, Run: repository.PollingRun{ID: "run-b"}, CounterDefinitionID: "counter-b", Read: func(context.Context) (counter.Reading, error) { return counter.Reading{}, nil }},
	})
	if runErr == nil {
		t.Fatal("expected duplicate target error")
	}
	if len(store.runs) != 0 {
		t.Fatalf("created %d runs before validation completed", len(store.runs))
	}
}
