package scheduler

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedulerRunsImmediatelyAndStops(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(logger)

	var runs atomic.Int32
	done := make(chan struct{})

	s.Register(Job{
		Name:     "test",
		Interval: time.Hour,
		Fn: func(ctx context.Context) error {
			if runs.Add(1) == 1 {
				close(done)
			}
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("job did not run immediately")
	}

	if got := runs.Load(); got < 1 {
		t.Fatalf("expected at least 1 run, got %d", got)
	}

	cancel()
	s.Stop()
}

func TestSchedulerRegisterMultipleJobs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(logger)

	var a, b atomic.Int32
	s.Register(Job{
		Name:     "a",
		Interval: time.Hour,
		Fn: func(ctx context.Context) error {
			a.Add(1)
			return nil
		},
	})
	s.Register(Job{
		Name:     "b",
		Interval: time.Hour,
		Fn: func(ctx context.Context) error {
			b.Add(1)
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()
	s.Stop()

	if a.Load() < 1 || b.Load() < 1 {
		t.Fatalf("expected both jobs to run immediately, got a=%d b=%d", a.Load(), b.Load())
	}
}
