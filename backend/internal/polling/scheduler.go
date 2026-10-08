package polling

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Target struct{ ID string }

type PollFunc func(context.Context, Target) error
type SleepFunc func(context.Context, time.Duration) error

type Config struct {
	Concurrency int
	MaxAttempts int
	RetryDelay  time.Duration
	Sleep       SleepFunc
}

type Result struct {
	TargetID string
	Attempts int
	Err      error
}

type Scheduler struct{ config Config }

func NewScheduler(config Config) (*Scheduler, error) {
	if config.Concurrency < 1 {
		return nil, fmt.Errorf("poll concurrency must be positive")
	}
	if config.MaxAttempts < 1 {
		return nil, fmt.Errorf("max poll attempts must be positive")
	}
	if config.RetryDelay < 0 {
		return nil, fmt.Errorf("retry delay cannot be negative")
	}
	if config.Sleep == nil {
		config.Sleep = sleepContext
	}
	return &Scheduler{config: config}, nil
}

func (s *Scheduler) Run(ctx context.Context, targets []Target, poll PollFunc) ([]Result, error) {
	if poll == nil {
		return nil, fmt.Errorf("poll function is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if target.ID == "" {
			return nil, fmt.Errorf("poll target id is required")
		}
		if _, exists := seen[target.ID]; exists {
			return nil, fmt.Errorf("duplicate poll target %q", target.ID)
		}
		seen[target.ID] = struct{}{}
	}
	results := make([]Result, len(targets))
	tasks := make(chan int)
	var workers sync.WaitGroup
	workerCount := s.config.Concurrency
	if workerCount > len(targets) {
		workerCount = len(targets)
	}
	for i := 0; i < workerCount; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range tasks {
				results[index] = s.pollOne(ctx, targets[index], poll)
			}
		}()
	}
	for index := range targets {
		select {
		case tasks <- index:
		case <-ctx.Done():
			close(tasks)
			workers.Wait()
			return results, ctx.Err()
		}
	}
	close(tasks)
	workers.Wait()
	return results, nil
}

func (s *Scheduler) pollOne(ctx context.Context, target Target, poll PollFunc) Result {
	result := Result{TargetID: target.ID}
	var lastErr error
	for attempt := 1; attempt <= s.config.MaxAttempts; attempt++ {
		result.Attempts = attempt
		if err := ctx.Err(); err != nil {
			result.Err = err
			return result
		}
		lastErr = poll(ctx, target)
		if lastErr == nil {
			return result
		}
		if attempt == s.config.MaxAttempts {
			break
		}
		if err := s.config.Sleep(ctx, retryDelay(s.config.RetryDelay, attempt)); err != nil {
			result.Err = err
			return result
		}
	}
	result.Err = fmt.Errorf("poll target %s failed after %d attempts: %w", target.ID, result.Attempts, lastErr)
	return result
}

func retryDelay(base time.Duration, attempt int) time.Duration {
	if base <= 0 {
		return 0
	}
	delay := base
	for i := 1; i < attempt && delay < time.Hour/2; i++ {
		delay *= 2
	}
	if delay > time.Hour {
		return time.Hour
	}
	return delay
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
