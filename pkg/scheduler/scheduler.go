package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
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
	filePath string // optional path for persisting jobs
}

type managedJob struct {
	Job
	cancel  context.CancelFunc // for heartbeat goroutines
	cronID  cron.EntryID       // for cron entries
	stopped bool
}

// New creates a Scheduler that invokes cb whenever a job fires.
// An optional file path may be provided for persisting jobs to disk.
func New(cb Callback, filePath ...string) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	c := cron.New(cron.WithSeconds())
	c.Start()
	s := &Scheduler{
		jobs:     make(map[string]*managedJob),
		callback: cb,
		ctx:      ctx,
		cancel:   cancel,
		cron:     c,
	}
	if len(filePath) > 0 && filePath[0] != "" {
		s.filePath = filePath[0]
	}
	return s
}

// LoadFromFile reads persisted jobs and re-activates them.
// It is safe to call even when no file exists (returns nil).
func (s *Scheduler) LoadFromFile() error {
	if s.filePath == "" {
		return nil
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read schedule file: %w", err)
	}

	var jobs []Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		return fmt.Errorf("failed to parse schedule file: %w", err)
	}

	for _, j := range jobs {
		switch j.Type {
		case JobTypeHeartbeat:
			dur, err := time.ParseDuration(j.Schedule)
			if err != nil {
				slog.Warn("Skipping invalid heartbeat schedule", "id", j.ID, "schedule", j.Schedule, "error", err)
				continue
			}
			if _, err := s.AddHeartbeat(dur, j.Message); err != nil {
				slog.Warn("Failed to restore heartbeat", "id", j.ID, "error", err)
			}
		case JobTypeCron:
			if _, err := s.AddCron(j.Schedule, j.Message); err != nil {
				slog.Warn("Failed to restore cron job", "id", j.ID, "error", err)
			}
		}
	}

	// Update nextID to be at least as large as the highest loaded ID
	s.mu.RLock()
	for _, mj := range s.jobs {
		if n := parseIDNumber(mj.ID); n > 0 {
			if cur := s.nextID.Load(); n > cur {
				s.nextID.Store(n)
			}
		}
	}
	s.mu.RUnlock()

	return nil
}

// save persists all current jobs to filePath.
// Must be called while s.mu is held.
func (s *Scheduler) save() {
	if s.filePath == "" {
		return
	}

	jobs := make([]Job, 0, len(s.jobs))
	for _, mj := range s.jobs {
		jobs = append(jobs, mj.Job)
	}

	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		slog.Warn("Failed to marshal schedule", "error", err)
		return
	}
	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		slog.Warn("Failed to write schedule file", "path", s.filePath, "error", err)
	}
}

// parseIDNumber extracts the trailing number from an ID like "heartbeat-3" or "cron-5".
func parseIDNumber(id string) int64 {
	idx := strings.LastIndex(id, "-")
	if idx < 0 || idx+1 >= len(id) {
		return 0
	}
	n, err := strconv.ParseInt(id[idx+1:], 10, 64)
	if err != nil {
		return 0
	}
	return n
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
	s.save()
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
	s.save()
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
	s.save()
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
