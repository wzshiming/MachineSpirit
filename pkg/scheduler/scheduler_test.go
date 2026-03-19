package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAddCron(t *testing.T) {
	var mu sync.Mutex
	var messages []string

	sched := New(func(ctx context.Context, msg string) {
		mu.Lock()
		defer mu.Unlock()
		messages = append(messages, msg)
	})
	defer sched.Stop()

	// Schedule every second (6-field cron with seconds)
	id, err := sched.AddCron("test-cron", "* * * * * *", "cron tick")
	if err != nil {
		t.Fatalf("AddCron failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	// Wait for at least 1 tick
	time.Sleep(1500 * time.Millisecond)

	mu.Lock()
	count := len(messages)
	mu.Unlock()

	if count < 1 {
		t.Errorf("expected at least 1 cron callback, got %d", count)
	}
}

func TestAddCronValidation(t *testing.T) {
	sched := New(func(ctx context.Context, msg string) {})
	defer sched.Stop()

	_, err := sched.AddCron("", "", "test")
	if err == nil {
		t.Error("expected error for empty name")
	}

	_, err = sched.AddCron("test-job", "", "test")
	if err == nil {
		t.Error("expected error for empty schedule")
	}

	_, err = sched.AddCron("test-job", "* * * * * *", "")
	if err == nil {
		t.Error("expected error for empty message")
	}

	_, err = sched.AddCron("test-job", "invalid", "test")
	if err == nil {
		t.Error("expected error for invalid cron expression")
	}
}

func TestRemove(t *testing.T) {
	var mu sync.Mutex
	var messages []string

	sched := New(func(ctx context.Context, msg string) {
		mu.Lock()
		defer mu.Unlock()
		messages = append(messages, msg)
	})
	defer sched.Stop()

	id, err := sched.AddCron("test-tick", "* * * * * *", "tick")
	if err != nil {
		t.Fatalf("AddCron failed: %v", err)
	}

	// Wait for at least 1 tick
	time.Sleep(1500 * time.Millisecond)

	// Remove the job
	if err := sched.Remove(id); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	mu.Lock()
	countAtRemoval := len(messages)
	mu.Unlock()

	// Wait more and verify no more ticks
	time.Sleep(1500 * time.Millisecond)

	mu.Lock()
	finalCount := len(messages)
	mu.Unlock()

	if finalCount != countAtRemoval {
		t.Errorf("expected no more callbacks after removal, got %d additional", finalCount-countAtRemoval)
	}
}

func TestRemoveNotFound(t *testing.T) {
	sched := New(func(ctx context.Context, msg string) {})
	defer sched.Stop()

	err := sched.Remove("nonexistent")
	if err == nil {
		t.Error("expected error for removing nonexistent job")
	}
}

func TestList(t *testing.T) {
	sched := New(func(ctx context.Context, msg string) {})
	defer sched.Stop()

	// Initially empty
	if len(sched.List()) != 0 {
		t.Error("expected empty list initially")
	}

	_, _ = sched.AddCron("cron1", "* * * * * *", "cron1")
	_, _ = sched.AddCron("cron2", "0 0 * * * *", "cron2")

	jobs := sched.List()
	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}
}

func TestStop(t *testing.T) {
	var mu sync.Mutex
	var count int

	sched := New(func(ctx context.Context, msg string) {
		mu.Lock()
		defer mu.Unlock()
		count++
	})

	_, _ = sched.AddCron("test-job", "* * * * * *", "test")

	// Wait for a tick
	time.Sleep(1500 * time.Millisecond)

	sched.Stop()

	mu.Lock()
	countAtStop := count
	mu.Unlock()

	// Wait more and verify no more ticks
	time.Sleep(1500 * time.Millisecond)

	mu.Lock()
	finalCount := count
	mu.Unlock()

	if finalCount != countAtStop {
		t.Errorf("expected no more callbacks after stop, got %d additional", finalCount-countAtStop)
	}
}

func TestPersistToCrontab(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "CRONTAB")

	sched := New(func(ctx context.Context, msg string) {}, fp)
	defer sched.Stop()

	_, err := sched.AddCron("hourly-task", "0 0 * * * *", "hourly task")
	if err != nil {
		t.Fatalf("AddCron failed: %v", err)
	}
	_, err = sched.AddCron("daily-report", "0 0 9 * * *", "daily report")
	if err != nil {
		t.Fatalf("AddCron failed: %v", err)
	}

	// File should exist and be readable text
	data, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("Failed to read crontab file: %v", err)
	}

	content := string(data)
	// Should have comment header
	if !strings.HasPrefix(content, "#") {
		t.Error("expected crontab to start with comment")
	}
	// Should contain both jobs with @name comments
	if !strings.Contains(content, "# @name hourly-task") {
		t.Error("expected crontab to contain '@name hourly-task' comment")
	}
	if !strings.Contains(content, "0 0 * * * * hourly task") {
		t.Error("expected crontab to contain 'hourly task' job")
	}
	if !strings.Contains(content, "# @name daily-report") {
		t.Error("expected crontab to contain '@name daily-report' comment")
	}
	if !strings.Contains(content, "0 0 9 * * * daily report") {
		t.Error("expected crontab to contain 'daily report' job")
	}
}

func TestLoadFromCrontab(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "CRONTAB")

	var mu sync.Mutex
	var messages []string
	cb := func(ctx context.Context, msg string) {
		mu.Lock()
		defer mu.Unlock()
		messages = append(messages, msg)
	}

	// Create scheduler and add jobs
	sched1 := New(cb, fp)
	_, _ = sched1.AddCron("every-second", "* * * * * *", "every second")
	_, _ = sched1.AddCron("hourly-task", "0 0 * * * *", "hourly task")
	sched1.Stop()

	// Create new scheduler and load from file
	sched2 := New(cb, fp)
	defer sched2.Stop()

	if err := sched2.LoadFromFile(); err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	jobs := sched2.List()
	if len(jobs) != 2 {
		t.Errorf("expected 2 restored jobs, got %d", len(jobs))
	}

	// Wait for the every-second job to tick
	time.Sleep(1500 * time.Millisecond)

	mu.Lock()
	count := len(messages)
	mu.Unlock()

	if count < 1 {
		t.Errorf("expected at least 1 callback from restored cron, got %d", count)
	}
}

func TestLoadFromCrontabWithComments(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "CRONTAB")

	content := `# This is a comment
# Another comment
# @name check-status
* * * * * * check status
# inline comment
# @name daily-report
0 0 9 * * * daily report
`
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	sched := New(func(ctx context.Context, msg string) {}, fp)
	defer sched.Stop()

	if err := sched.LoadFromFile(); err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	jobs := sched.List()
	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}

	// Verify the jobs have the correct names
	names := make(map[string]bool)
	for _, job := range jobs {
		names[job.Name] = true
	}
	if !names["check-status"] {
		t.Error("expected job 'check-status' to be loaded")
	}
	if !names["daily-report"] {
		t.Error("expected job 'daily-report' to be loaded")
	}
}

func TestLoadFromFileNoFile(t *testing.T) {
	sched := New(func(ctx context.Context, msg string) {}, "/nonexistent/path")
	defer sched.Stop()

	if err := sched.LoadFromFile(); err != nil {
		t.Errorf("LoadFromFile should return nil for missing file, got: %v", err)
	}
}

func TestRemovePersists(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "CRONTAB")

	sched := New(func(ctx context.Context, msg string) {}, fp)
	defer sched.Stop()

	id, _ := sched.AddCron("test-job", "0 0 * * * *", "test")
	_ = sched.Remove(id)

	data, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("Failed to read crontab file: %v", err)
	}

	// The file should have no non-comment, non-empty lines
	lines := strings.Split(string(data), "\n")
	jobCount := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			jobCount++
		}
	}
	if jobCount != 0 {
		t.Errorf("expected 0 job lines after removal, got %d", jobCount)
	}
}

func TestParseCrontabLine(t *testing.T) {
	tests := []struct {
		line     string
		schedule string
		message  string
		ok       bool
	}{
		{"* * * * * * check status", "* * * * * *", "check status", true},
		{"0 0 9 * * * daily report generation", "0 0 9 * * *", "daily report generation", true},
		{"0 */5 * * * * run check", "0 */5 * * * *", "run check", true},
		{"too few fields", "", "", false},
		{"1 2 3 4 5 6", "", "", false}, // exactly 6 fields, no message
		{"", "", "", false},
	}

	for _, tc := range tests {
		schedule, message, ok := parseCrontabLine(tc.line)
		if ok != tc.ok {
			t.Errorf("parseCrontabLine(%q): ok = %v, want %v", tc.line, ok, tc.ok)
		}
		if schedule != tc.schedule {
			t.Errorf("parseCrontabLine(%q): schedule = %q, want %q", tc.line, schedule, tc.schedule)
		}
		if message != tc.message {
			t.Errorf("parseCrontabLine(%q): message = %q, want %q", tc.line, message, tc.message)
		}
	}
}
