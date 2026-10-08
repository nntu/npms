package worker

import (
	"context"
	"fmt"
	"time"
)

type Runner interface {
	RunStatus(context.Context) error
	RunCounters(context.Context) error
}

type CleanupRunner interface {
	RunCleanup(context.Context) error
}

type Config struct {
	StatusInterval  time.Duration
	CounterInterval time.Duration
	CleanupInterval time.Duration
}

type Loop struct {
	config Config
	runner Runner
}

func NewLoop(config Config, runner Runner) (*Loop, error) {
	if config.StatusInterval <= 0 {
		return nil, fmt.Errorf("status interval must be positive")
	}
	if config.CounterInterval <= 0 {
		return nil, fmt.Errorf("counter interval must be positive")
	}
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = 24 * time.Hour
	}
	if runner == nil {
		return nil, fmt.Errorf("worker runner is required")
	}
	return &Loop{config: config, runner: runner}, nil
}

func (l *Loop) Run(ctx context.Context) error {
	statusTicker := time.NewTicker(l.config.StatusInterval)
	counterTicker := time.NewTicker(l.config.CounterInterval)
	cleanupTicker := time.NewTicker(l.config.CleanupInterval)
	defer statusTicker.Stop()
	defer counterTicker.Stop()
	defer cleanupTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-statusTicker.C:
			if err := l.runner.RunStatus(ctx); err != nil {
				return fmt.Errorf("status polling: %w", err)
			}
		case <-counterTicker.C:
			if err := l.runner.RunCounters(ctx); err != nil {
				return fmt.Errorf("counter polling: %w", err)
			}
		case <-cleanupTicker.C:
			if cleanup, ok := l.runner.(CleanupRunner); ok {
				if err := cleanup.RunCleanup(ctx); err != nil {
					return fmt.Errorf("poller cleanup: %w", err)
				}
			}
		}
	}
}
