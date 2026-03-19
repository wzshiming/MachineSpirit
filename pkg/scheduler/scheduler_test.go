package scheduler

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestAddHeartbeat(t *testing.T) {
	var mu sync.Mutex
	var messages []string

	sched := New(func(ctx context.Context, msg string) {
		mu.Lock()
		defer mu.Unlock()
		messages = append(messages, msg)
	})
	defer sched.Stop()

	id, err := sched.AddHeartbeat(50*time.Millisecond, "heartbeat ping")
	if err != nil {
		t.Fatalf("AddHeartbeat failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	// Wait enough for at least 2 ticks
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	count := len(messages)
	mu.Unlock()

	if count < 2 {
		t.Errorf("expected at least 2 heartbeat callbacks, got %d", count)
	}

	// Verify the message content
	mu.Lock()
	for _, m := range messages {
		if m != "heartbeat ping" {
			t.Errorf("unexpected message: %q", m)
		}
	}
	mu.Unlock()
}

func TestAddHeartbeatValidation(t *testing.T) {
	sched := New(func(ctx context.Context, msg string) {})
	defer sched.Stop()

	_, err := sched.AddHeartbeat(0, "test")
	if err == nil {
		t.Error("expected error for zero interval")
	}

	_, err = sched.AddHeartbeat(-1*time.Second, "test")
	if err == nil {
		t.Error("expected error for negative interval")
	}

	_, err = sched.AddHeartbeat(1*time.Second, "")
	if err == nil {
		t.Error("expected error for empty message")
	}
}

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
	id, err := sched.AddCron("* * * * * *", "cron tick")
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

	_, err := sched.AddCron("", "test")
	if err == nil {
		t.Error("expected error for empty schedule")
	}

	_, err = sched.AddCron("* * * * * *", "")
	if err == nil {
		t.Error("expected error for empty message")
	}

	_, err = sched.AddCron("invalid", "test")
	if err == nil {
		t.Error("expected error for invalid cron expression")
	}
}

func TestRemove(t *testing.T) {
	var mu sync.Mutex
	var count int

	sched := New(func(ctx context.Context, msg string) {
		mu.Lock()
		defer mu.Unlock()
		count++
	})
	defer sched.Stop()

	id, err := sched.AddHeartbeat(50*time.Millisecond, "test")
	if err != nil {
		t.Fatalf("AddHeartbeat failed: %v", err)
	}

	// Wait for a couple ticks
	time.Sleep(150 * time.Millisecond)

	// Remove the job
	if err := sched.Remove(id); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	mu.Lock()
	countAtRemoval := count
	mu.Unlock()

	// Wait more and verify no more ticks
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	finalCount := count
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

	_, _ = sched.AddHeartbeat(1*time.Second, "hb")
	_, _ = sched.AddCron("* * * * * *", "cron")

	jobs := sched.List()
	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}

	hasHeartbeat := false
	hasCron := false
	for _, j := range jobs {
		if j.Type == JobTypeHeartbeat {
			hasHeartbeat = true
		}
		if j.Type == JobTypeCron {
			hasCron = true
		}
	}
	if !hasHeartbeat {
		t.Error("expected a heartbeat job in list")
	}
	if !hasCron {
		t.Error("expected a cron job in list")
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

	_, _ = sched.AddHeartbeat(50*time.Millisecond, "test")

	// Wait for a tick
	time.Sleep(100 * time.Millisecond)

	sched.Stop()

	mu.Lock()
	countAtStop := count
	mu.Unlock()

	// Wait more and verify no more ticks
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	finalCount := count
	mu.Unlock()

	if finalCount != countAtStop {
		t.Errorf("expected no more callbacks after stop, got %d additional", finalCount-countAtStop)
	}
}

func TestPersistToFile(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "SCHEDULE.json")

	sched := New(func(ctx context.Context, msg string) {}, fp)
	defer sched.Stop()

	_, err := sched.AddHeartbeat(5*time.Second, "hb check")
	if err != nil {
		t.Fatalf("AddHeartbeat failed: %v", err)
	}
	_, err = sched.AddCron("0 0 * * * *", "hourly task")
	if err != nil {
		t.Fatalf("AddCron failed: %v", err)
	}

	// File should exist
	data, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("Failed to read schedule file: %v", err)
	}

	var jobs []Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		t.Fatalf("Failed to parse schedule file: %v", err)
	}
	if len(jobs) != 2 {
		t.Errorf("expected 2 persisted jobs, got %d", len(jobs))
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "SCHEDULE.json")

	var mu sync.Mutex
	var messages []string
	cb := func(ctx context.Context, msg string) {
		mu.Lock()
		defer mu.Unlock()
		messages = append(messages, msg)
	}

	// Create scheduler and add jobs
	sched1 := New(cb, fp)
	_, _ = sched1.AddHeartbeat(50*time.Millisecond, "hb msg")
	_, _ = sched1.AddCron("0 0 * * * *", "cron msg")
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

	// Wait for heartbeat to tick
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := len(messages)
	mu.Unlock()

	if count < 1 {
		t.Errorf("expected at least 1 callback from restored heartbeat, got %d", count)
	}
}

func TestLoadFromFileNoFile(t *testing.T) {
	sched := New(func(ctx context.Context, msg string) {}, "/nonexistent/path.json")
	defer sched.Stop()

	if err := sched.LoadFromFile(); err != nil {
		t.Errorf("LoadFromFile should return nil for missing file, got: %v", err)
	}
}

func TestRemovePersists(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "SCHEDULE.json")

	sched := New(func(ctx context.Context, msg string) {}, fp)
	defer sched.Stop()

	id, _ := sched.AddHeartbeat(5*time.Second, "test")
	_ = sched.Remove(id)

	data, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("Failed to read schedule file: %v", err)
	}

	var jobs []Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		t.Fatalf("Failed to parse schedule file: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 persisted jobs after removal, got %d", len(jobs))
	}
}

func TestParseIDNumber(t *testing.T) {
	tests := []struct {
		id       string
		expected int64
	}{
		{"heartbeat-1", 1},
		{"cron-42", 42},
		{"heartbeat-0", 0},
		{"invalid", 0},
		{"", 0},
		{"no-number-", 0},
	}

	for _, tc := range tests {
		got := parseIDNumber(tc.id)
		if got != tc.expected {
			t.Errorf("parseIDNumber(%q) = %d, want %d", tc.id, got, tc.expected)
		}
	}
}
