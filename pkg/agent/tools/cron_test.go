package tools

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/scheduler"
)

func TestCronToolAdd(t *testing.T) {
	var mu sync.Mutex
	var messages []string

	sched := scheduler.New(func(ctx context.Context, msg string) {
		mu.Lock()
		defer mu.Unlock()
		messages = append(messages, msg)
	})
	defer sched.Stop()

	tool := NewCronTool(sched)

	if tool.Name() != "cron" {
		t.Errorf("expected name 'cron', got %q", tool.Name())
	}
	if !tool.Enabled() {
		t.Error("expected tool to be enabled")
	}

	// Add a cron job that fires every second
	input, _ := json.Marshal(map[string]any{
		"action":   "add",
		"name":     "test-cron",
		"schedule": "* * * * * *",
		"message":  "cron task",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var resultData map[string]any
	if err := json.Unmarshal(result, &resultData); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}
	if resultData["status"] != "success" {
		t.Errorf("expected status 'success', got %v", resultData["status"])
	}
	if resultData["id"] == nil || resultData["id"] == "" {
		t.Error("expected non-empty id in result")
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

func TestCronToolList(t *testing.T) {
	sched := scheduler.New(func(ctx context.Context, msg string) {})
	defer sched.Stop()

	tool := NewCronTool(sched)

	// Add a cron job
	addInput, _ := json.Marshal(map[string]any{
		"action":   "add",
		"name":     "hourly-task",
		"schedule": "0 0 * * * *",
		"message":  "hourly task",
	})
	_, err := tool.Execute(context.Background(), addInput)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// List cron jobs
	listInput, _ := json.Marshal(map[string]any{
		"action": "list",
	})
	result, err := tool.Execute(context.Background(), listInput)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	var resultData map[string]any
	if err := json.Unmarshal(result, &resultData); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}
	if resultData["count"].(float64) != 1 {
		t.Errorf("expected 1 cron job, got %v", resultData["count"])
	}
}

func TestCronToolRemove(t *testing.T) {
	sched := scheduler.New(func(ctx context.Context, msg string) {})
	defer sched.Stop()

	tool := NewCronTool(sched)

	// Add a cron job
	addInput, _ := json.Marshal(map[string]any{
		"action":   "add",
		"name":     "test-job",
		"schedule": "0 0 * * * *",
		"message":  "test",
	})
	result, err := tool.Execute(context.Background(), addInput)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	var addResult map[string]any
	json.Unmarshal(result, &addResult)
	id := addResult["id"].(string)

	// Remove it
	removeInput, _ := json.Marshal(map[string]any{
		"action": "remove",
		"id":     id,
	})
	result, err = tool.Execute(context.Background(), removeInput)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	var removeResult map[string]any
	json.Unmarshal(result, &removeResult)
	if removeResult["status"] != "success" {
		t.Errorf("expected status 'success', got %v", removeResult["status"])
	}

	// Verify list is now empty
	listInput, _ := json.Marshal(map[string]any{
		"action": "list",
	})
	result, err = tool.Execute(context.Background(), listInput)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	var listResult map[string]any
	json.Unmarshal(result, &listResult)
	if listResult["count"].(float64) != 0 {
		t.Errorf("expected 0 cron jobs after remove, got %v", listResult["count"])
	}
}

func TestCronToolValidation(t *testing.T) {
	sched := scheduler.New(func(ctx context.Context, msg string) {})
	defer sched.Stop()

	tool := NewCronTool(sched)

	tests := []struct {
		name  string
		input map[string]any
	}{
		{"missing action", map[string]any{}},
		{"add without name", map[string]any{"action": "add", "schedule": "* * * * * *", "message": "test"}},
		{"add without schedule", map[string]any{"action": "add", "name": "test", "message": "test"}},
		{"add without message", map[string]any{"action": "add", "name": "test", "schedule": "* * * * * *"}},
		{"remove without id", map[string]any{"action": "remove"}},
		{"invalid schedule", map[string]any{"action": "add", "name": "test", "schedule": "invalid", "message": "test"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input, _ := json.Marshal(tc.input)
			_, err := tool.Execute(context.Background(), input)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestCronToolDisabledWithNilScheduler(t *testing.T) {
	tool := NewCronTool(nil)
	if tool.Enabled() {
		t.Error("expected tool to be disabled with nil scheduler")
	}
}
