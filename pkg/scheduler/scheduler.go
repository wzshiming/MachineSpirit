package scheduler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
)

// JobType distinguishes heartbeat from cron jobs.
type JobType string

const (
	JobTypeHeartbeat JobType = "heartbeat"
	JobTypeCron      JobType = "cron"
)

// Job describes a scheduled job visible to callers.
type Job struct {
	ID       string  `json:"id"`
	Type     JobType `json:"type"`
	Schedule string  `json:"schedule"`
	Message  string  `json:"message"`
}

// Callback is invoked each time a scheduled job fires.
type Callback func(ctx context.Context, message string)

// Scheduler manages heartbeat and cron jobs.
type Scheduler struct {
	mu       sync.RWMutex
	jobs     map[string]*managedJob
	callback Callback
	nextID   atomic.Int64
	ctx      context.Context
	cancel   context.CancelFunc
	cron     *cron.Cron
}

type managedJob struct {
	Job
	cancel  context.CancelFunc // for heartbeat goroutines
	cronID  cron.EntryID       // for cron entries
	stopped bool
}

// New creates a Scheduler that invokes cb whenever a job fires.
func New(cb Callback) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	c := cron.New(cron.WithSeconds())
	c.Start()
	return &Scheduler{
		jobs:     make(map[string]*managedJob),
		callback: cb,
		ctx:      ctx,
		cancel:   cancel,
		cron:     c,
	}
}

// AddHeartbeat starts a periodic job that fires every interval.
func (s *Scheduler) AddHeartbeat(interval time.Duration, message string) (string, error) {
	if interval <= 0 {
		return "", fmt.Errorf("interval must be positive")
	}
	if message == "" {
		return "", fmt.Errorf("message is required")
	}

	id := fmt.Sprintf("heartbeat-%d", s.nextID.Add(1))

	hbCtx, hbCancel := context.WithCancel(s.ctx)

	mj := &managedJob{
		Job: Job{
			ID:       id,
			Type:     JobTypeHeartbeat,
			Schedule: interval.String(),
			Message:  message,
		},
		cancel: hbCancel,
	}

	s.mu.Lock()
	s.jobs[id] = mj
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-ticker.C:
				s.callback(s.ctx, message)
			}
		}
	}()

	return id, nil
}

// AddCron schedules a job using a cron expression (6-field with seconds, or 5-field standard).
func (s *Scheduler) AddCron(schedule string, message string) (string, error) {
	if schedule == "" {
		return "", fmt.Errorf("schedule is required")
	}
	if message == "" {
		return "", fmt.Errorf("message is required")
	}

	id := fmt.Sprintf("cron-%d", s.nextID.Add(1))

	msg := message // capture for closure
	entryID, err := s.cron.AddFunc(schedule, func() {
		s.callback(s.ctx, msg)
	})
	if err != nil {
		return "", fmt.Errorf("invalid cron schedule %q: %w", schedule, err)
	}

	mj := &managedJob{
		Job: Job{
			ID:       id,
			Type:     JobTypeCron,
			Schedule: schedule,
			Message:  message,
		},
		cronID: entryID,
	}

	s.mu.Lock()
	s.jobs[id] = mj
	s.mu.Unlock()

	return id, nil
}

// Remove cancels and removes a job by ID.
func (s *Scheduler) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mj, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job %q not found", id)
	}

	if mj.Type == JobTypeHeartbeat && mj.cancel != nil {
		mj.cancel()
	}
	if mj.Type == JobTypeCron {
		s.cron.Remove(mj.cronID)
	}
	mj.stopped = true
	delete(s.jobs, id)
	return nil
}

// List returns all active jobs.
func (s *Scheduler) List() []Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Job, 0, len(s.jobs))
	for _, mj := range s.jobs {
		result = append(result, mj.Job)
	}
	return result
}

// Stop cancels all jobs and shuts down the scheduler.
func (s *Scheduler) Stop() {
	s.cancel()
	s.cron.Stop()

	s.mu.Lock()
	defer s.mu.Unlock()
	for id, mj := range s.jobs {
		if mj.cancel != nil {
			mj.cancel()
		}
		delete(s.jobs, id)
	}
}
