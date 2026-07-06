package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Job is a recurring task executed by the scheduler.
type Job struct {
	Name     string
	Interval time.Duration
	Fn       func(ctx context.Context) error
}

// Scheduler runs jobs on fixed intervals.
type Scheduler struct {
	logger *slog.Logger
	jobs   []Job
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// New creates a scheduler.
func New(logger *slog.Logger) *Scheduler {
	return &Scheduler{logger: logger}
}

// Register adds a job to the scheduler.
func (s *Scheduler) Register(job Job) {
	s.jobs = append(s.jobs, job)
}

// Start begins executing all registered jobs.
func (s *Scheduler) Start(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	for _, job := range s.jobs {
		s.wg.Add(1)
		go s.runJob(runCtx, job)
	}
}

func (s *Scheduler) runJob(ctx context.Context, job Job) {
	defer s.wg.Done()

	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()

	s.logger.Info("scheduler job started", "job", job.Name, "interval", job.Interval)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler job stopped", "job", job.Name)
			return
		case <-ticker.C:
			if err := job.Fn(ctx); err != nil {
				s.logger.Error("scheduler job failed", "job", job.Name, "error", err)
			}
		}
	}
}

// Stop gracefully stops all jobs.
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}
