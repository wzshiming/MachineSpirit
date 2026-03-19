package tools

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/scheduler"
)

func TestHeartbeatToolStart(t *testing.T) {
	var mu sync.Mutex
	var messages []string

	sched := scheduler.New(func(ctx context.Context, msg string) {
		mu.Lock()
		defer mu.Unlock()
		messages = append(messages, msg)
	})
	defer sched.Stop()

	tool := NewHeartbeatTool(sched)

	if tool.Name() != "heartbeat" {
		t.Errorf("expected name 'heartbeat', got %q", tool.Name())
	}
	if !tool.Enabled() {
		t.Error("expected tool to be enabled")
	}

	input, _ := json.Marshal(map[string]any{
		"action":   "start",
		"interval": "100ms",
		"message":  "check status",
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

	// Wait for a tick
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	count := len(messages)
	mu.Unlock()
	if count < 1 {
		t.Errorf("expected at least 1 heartbeat callback, got %d", count)
	}
}

func TestHeartbeatToolList(t *testing.T) {
	sched := scheduler.New(func(ctx context.Context, msg string) {})
	defer sched.Stop()

	tool := NewHeartbeatTool(sched)

	// Start a heartbeat first
	startInput, _ := json.Marshal(map[string]any{
		"action":   "start",
		"interval": "1s",
		"message":  "test",
	})
	_, err := tool.Execute(context.Background(), startInput)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// List heartbeats
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
		t.Errorf("expected 1 heartbeat, got %v", resultData["count"])
	}
}

func TestHeartbeatToolStop(t *testing.T) {
	sched := scheduler.New(func(ctx context.Context, msg string) {})
	defer sched.Stop()

	tool := NewHeartbeatTool(sched)

	// Start a heartbeat
	startInput, _ := json.Marshal(map[string]any{
		"action":   "start",
		"interval": "1s",
		"message":  "test",
	})
	result, err := tool.Execute(context.Background(), startInput)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	var startResult map[string]any
	json.Unmarshal(result, &startResult)
	id := startResult["id"].(string)

	// Stop it
	stopInput, _ := json.Marshal(map[string]any{
		"action": "stop",
		"id":     id,
	})
	result, err = tool.Execute(context.Background(), stopInput)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	var stopResult map[string]any
	json.Unmarshal(result, &stopResult)
	if stopResult["status"] != "success" {
		t.Errorf("expected status 'success', got %v", stopResult["status"])
	}
}

func TestHeartbeatToolValidation(t *testing.T) {
	sched := scheduler.New(func(ctx context.Context, msg string) {})
	defer sched.Stop()

	tool := NewHeartbeatTool(sched)

	tests := []struct {
		name  string
		input map[string]any
	}{
		{"missing action", map[string]any{}},
		{"start without interval", map[string]any{"action": "start", "message": "test"}},
		{"start without message", map[string]any{"action": "start", "interval": "1s"}},
		{"stop without id", map[string]any{"action": "stop"}},
		{"invalid interval", map[string]any{"action": "start", "interval": "not-a-duration", "message": "test"}},
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

func TestHeartbeatToolDisabledWithNilScheduler(t *testing.T) {
	tool := NewHeartbeatTool(nil)
	if tool.Enabled() {
		t.Error("expected tool to be disabled with nil scheduler")
	}
}
