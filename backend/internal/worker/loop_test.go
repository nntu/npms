package worker

import (
	"context"
	"testing"
	"time"
)

type fakeRunner struct{}

func (fakeRunner) RunStatus(context.Context) error   { return nil }
func (fakeRunner) RunCounters(context.Context) error { return nil }

func TestNewLoopValidatesIntervals(t *testing.T) {
	if _, err := NewLoop(Config{StatusInterval: time.Minute, CounterInterval: time.Minute}, fakeRunner{}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLoop(Config{StatusInterval: 0, CounterInterval: time.Minute}, fakeRunner{}); err == nil {
		t.Fatal("expected status interval error")
	}
	if _, err := NewLoop(Config{StatusInterval: time.Minute, CounterInterval: 0}, fakeRunner{}); err == nil {
		t.Fatal("expected counter interval error")
	}
}

func TestLoopStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	loop, err := NewLoop(Config{StatusInterval: time.Minute, CounterInterval: time.Minute}, fakeRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if err := loop.Run(ctx); err != context.Canceled {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
