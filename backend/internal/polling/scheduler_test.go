package polling

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedulerBoundsConcurrencyAndPreservesOrder(t *testing.T) {
	scheduler, err := NewScheduler(Config{Concurrency: 2, MaxAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	var active, maxActive atomic.Int32
	results, err := scheduler.Run(context.Background(), []Target{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}, func(ctx context.Context, target Target) error {
		current := active.Add(1)
		for {
			old := maxActive.Load()
			if current <= old || maxActive.CompareAndSwap(old, current) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		active.Add(-1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if maxActive.Load() > 2 {
		t.Fatalf("concurrency exceeded limit: %d", maxActive.Load())
	}
	for i, result := range results {
		if result.TargetID != string(rune('a'+i)) || result.Attempts != 1 || result.Err != nil {
			t.Fatalf("unexpected result %d: %#v", i, result)
		}
	}
}

func TestSchedulerRetriesWithBackoff(t *testing.T) {
	scheduler, err := NewScheduler(Config{Concurrency: 1, MaxAttempts: 3, RetryDelay: time.Second, Sleep: func(context.Context, time.Duration) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	var attempts atomic.Int32
	results, err := scheduler.Run(context.Background(), []Target{{ID: "printer-1"}}, func(context.Context, Target) error {
		if attempts.Add(1) < 3 {
			return errors.New("temporary failure")
		}
		return nil
	})
	if err != nil || results[0].Err != nil || results[0].Attempts != 3 {
		t.Fatalf("unexpected retry result: %#v, %v", results, err)
	}
}

func TestSchedulerReturnsPerTargetErrorAndRejectsDuplicates(t *testing.T) {
	scheduler, err := NewScheduler(Config{Concurrency: 1, MaxAttempts: 2, Sleep: func(context.Context, time.Duration) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	results, err := scheduler.Run(context.Background(), []Target{{ID: "printer-1"}}, func(context.Context, Target) error { return errors.New("offline") })
	if err != nil || results[0].Err == nil || results[0].Attempts != 2 {
		t.Fatalf("unexpected failed result: %#v, %v", results, err)
	}
	if _, err := scheduler.Run(context.Background(), []Target{{ID: "same"}, {ID: "same"}}, func(context.Context, Target) error { return nil }); err == nil {
		t.Fatal("expected duplicate target error")
	}
}
