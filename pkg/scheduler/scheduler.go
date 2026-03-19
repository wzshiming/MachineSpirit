package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/robfig/cron/v3"
)

// Job describes a cron-scheduled job visible to callers.
type Job struct {
	Name     string
	Schedule string
	Message  string
}

// Callback is invoked each time a scheduled job fires.
type Callback func(ctx context.Context, message string)

// Scheduler manages cron jobs with optional file persistence in crontab format.
type Scheduler struct {
	mu       sync.RWMutex
	jobs     map[string]*managedJob
	callback Callback
	ctx      context.Context
	cancel   context.CancelFunc
	cron     *cron.Cron
	filePath string // optional path for persisting jobs
}

type managedJob struct {
	Job
	cronID cron.EntryID
}

// New creates a Scheduler that invokes cb whenever a job fires.
// An optional file path may be provided for persisting jobs to disk in crontab format.
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

// LoadFromFile reads persisted cron jobs from a crontab-format file and re-activates them.
// It is safe to call even when no file exists (returns nil).
//
// File format (one job per line):
//
//	# @name <job-name>
//	<6-field-cron-schedule> <message>
func (s *Scheduler) LoadFromFile() error {
	if s.filePath == "" {
		return nil
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read crontab file: %w", err)
	}

	var pendingName string
	nameCounter := 0
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Check for @name directive
		if strings.HasPrefix(line, "# @name ") {
			pendingName = strings.TrimSpace(strings.TrimPrefix(line, "# @name "))
			continue
		}
		// Skip other comments
		if strings.HasPrefix(line, "#") {
			continue
		}
		schedule, message, ok := parseCrontabLine(line)
		if !ok {
			slog.Warn("Skipping invalid crontab line", "line", line)
			continue
		}
		// Use pending name or generate one
		name := pendingName
		if name == "" {
			nameCounter++
			name = fmt.Sprintf("cron-%d", nameCounter)
		}
		pendingName = ""
		if _, err := s.AddCron(name, schedule, message); err != nil {
			slog.Warn("Failed to restore cron job", "name", name, "schedule", schedule, "message", message, "error", err)
		}
	}

	return nil
}

// parseCrontabLine splits a crontab line into schedule and message.
// It expects at least 7 space-separated fields: a 6-field cron expression
// (with seconds) followed by one or more words forming the message.
// Returns ("", "", false) if the line does not have enough fields.
func parseCrontabLine(line string) (schedule, message string, ok bool) {
	fields := strings.Fields(line)
	if len(fields) < 7 {
		return "", "", false
	}
	schedule = strings.Join(fields[:6], " ")
	message = strings.Join(fields[6:], " ")
	return schedule, message, true
}

// save persists all current jobs to filePath in crontab format.
// Must be called while s.mu is held.
func (s *Scheduler) save() {
	if s.filePath == "" {
		return
	}

	var sb strings.Builder
	sb.WriteString("# MachineSpirit crontab\n")
	sb.WriteString("# Format: <sec> <min> <hour> <dom> <mon> <dow> <message>\n")
	sb.WriteString("#\n")
	for _, mj := range s.jobs {
		sb.WriteString("# @name ")
		sb.WriteString(mj.Name)
		sb.WriteString("\n")
		sb.WriteString(mj.Schedule)
		sb.WriteString(" ")
		sb.WriteString(mj.Message)
		sb.WriteString("\n")
	}

	if err := os.WriteFile(s.filePath, []byte(sb.String()), 0644); err != nil {
		slog.Warn("Failed to write crontab file", "path", s.filePath, "error", err)
	}
}

// AddCron schedules a job using a cron expression (6-field with seconds).
// The name identifies the job and must be unique.
func (s *Scheduler) AddCron(name, schedule, message string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	if schedule == "" {
		return "", fmt.Errorf("schedule is required")
	}
	if message == "" {
		return "", fmt.Errorf("message is required")
	}

	s.mu.RLock()
	if _, exists := s.jobs[name]; exists {
		s.mu.RUnlock()
		return "", fmt.Errorf("job %q already exists", name)
	}
	s.mu.RUnlock()

	msg := message // capture for closure
	entryID, err := s.cron.AddFunc(schedule, func() {
		s.callback(s.ctx, msg)
	})
	if err != nil {
		return "", fmt.Errorf("invalid cron schedule %q: %w", schedule, err)
	}

	mj := &managedJob{
		Job: Job{
			Name:     name,
			Schedule: schedule,
			Message:  message,
		},
		cronID: entryID,
	}

	s.mu.Lock()
	s.jobs[name] = mj
	s.save()
	s.mu.Unlock()

	return name, nil
}

// Remove cancels and removes a job by ID.
func (s *Scheduler) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	mj, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job %q not found", id)
	}

	s.cron.Remove(mj.cronID)
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
	for id := range s.jobs {
		delete(s.jobs, id)
	}
}
