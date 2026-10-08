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
	runs     []repository.PollingRun
	readings int
	events   int
	finished []string
}

func (f *fakeStore) CreatePollingRun(_ context.Context, run repository.PollingRun) error {
	f.runs = append(f.runs, run)
	return nil
}
func (f *fakeStore) FinishPollingRun(_ context.Context, id, result, errorCode string, _ time.Time) error {
	f.finished = append(f.finished, id+":"+result+":"+errorCode)
	return nil
}
func (f *fakeStore) InsertCounterReading(_ context.Context, _, _ string, _ counter.Reading) (bool, error) {
	f.readings++
	return true, nil
}
func (f *fakeStore) InsertCounterEvent(_ context.Context, _, _, _, _ string, _ time.Time) error {
	f.events++
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
	if len(store.runs) != 2 || store.readings != 1 || store.events != 1 || len(store.finished) != 2 {
		t.Fatalf("unexpected store calls: %#v", store)
	}
}
