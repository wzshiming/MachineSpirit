package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestForkBashToolName(t *testing.T) {
	tool := NewForkBashTool()
	if tool.Name() != "fork_bash" {
		t.Errorf("expected name 'fork_bash', got %q", tool.Name())
	}
}

func TestForkBashToolEnabled(t *testing.T) {
	tool := NewForkBashTool()
	if !tool.Enabled() {
		t.Error("expected tool to be enabled")
	}
}

func TestForkBashToolStartValidation(t *testing.T) {
	tool := NewForkBashTool()
	ctx := context.Background()

	// Missing name
	input, _ := json.Marshal(map[string]string{"action": "start", "command": "echo hello"})
	_, err := tool.Execute(ctx, input)
	if err == nil {
		t.Error("expected error for missing name")
	}

	// Missing command
	input, _ = json.Marshal(map[string]string{"action": "start", "name": "test"})
	_, err = tool.Execute(ctx, input)
	if err == nil {
		t.Error("expected error for missing command")
	}

	// Unknown action
	input, _ = json.Marshal(map[string]string{"action": "unknown"})
	_, err = tool.Execute(ctx, input)
	if err == nil {
		t.Error("expected error for unknown action")
	}
}

func TestForkBashToolStartAndList(t *testing.T) {
	tool := NewForkBashTool()
	ctx := context.Background()

	// Start a fork
	input, _ := json.Marshal(map[string]any{
		"action":  "start",
		"name":    "test-fork",
		"command": "echo hello",
	})
	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	var startResult map[string]any
	if err := json.Unmarshal(result, &startResult); err != nil {
		t.Fatalf("failed to unmarshal start result: %v", err)
	}
	if startResult["status"] != "started" {
		t.Errorf("expected status 'started', got %v", startResult["status"])
	}
	if startResult["name"] != "test-fork" {
		t.Errorf("expected name 'test-fork', got %v", startResult["name"])
	}

	// Wait for the fork to complete
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		listInput, _ := json.Marshal(map[string]string{"action": "list"})
		listResult, err := tool.Execute(ctx, listInput)
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}

		var lr map[string]any
		if err := json.Unmarshal(listResult, &lr); err != nil {
			t.Fatalf("failed to unmarshal list result: %v", err)
		}

		forks := lr["forks"].([]any)
		if len(forks) > 0 {
			fork := forks[0].(map[string]any)
			if fork["status"] == "completed" {
				// Verify stdout captured
				if stdout, ok := fork["stdout"].(string); ok {
					if stdout != "hello\n" {
						t.Errorf("expected stdout 'hello\\n', got %q", stdout)
					}
				} else {
					t.Error("expected stdout in completed fork")
				}
				break
			}
		}

		time.Sleep(50 * time.Millisecond)
	}

	// Final check: verify it completed
	listInput, _ := json.Marshal(map[string]string{"action": "list"})
	listResult, err := tool.Execute(ctx, listInput)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	var lr map[string]any
	if err := json.Unmarshal(listResult, &lr); err != nil {
		t.Fatalf("failed to unmarshal list result: %v", err)
	}

	total, ok := lr["total"].(float64)
	if !ok || total != 1 {
		t.Errorf("expected total 1, got %v", lr["total"])
	}
}

func TestForkBashToolTerminate(t *testing.T) {
	tool := NewForkBashTool()
	ctx := context.Background()

	// Start a long-running fork
	input, _ := json.Marshal(map[string]any{
		"action":  "start",
		"name":    "long-running",
		"command": "sleep 60",
	})
	_, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Terminate it
	termInput, _ := json.Marshal(map[string]string{"action": "terminate", "name": "long-running"})
	termResult, err := tool.Execute(ctx, termInput)
	if err != nil {
		t.Fatalf("terminate failed: %v", err)
	}

	var tr map[string]any
	if err := json.Unmarshal(termResult, &tr); err != nil {
		t.Fatalf("failed to unmarshal terminate result: %v", err)
	}
	if tr["status"] != "terminated" {
		t.Errorf("expected status 'terminated', got %v", tr["status"])
	}
}

func TestForkBashToolTerminateValidation(t *testing.T) {
	tool := NewForkBashTool()
	ctx := context.Background()

	// Terminate missing name
	input, _ := json.Marshal(map[string]string{"action": "terminate"})
	_, err := tool.Execute(ctx, input)
	if err == nil {
		t.Error("expected error for missing name")
	}

	// Terminate non-existent fork
	input, _ = json.Marshal(map[string]string{"action": "terminate", "name": "nonexistent"})
	_, err = tool.Execute(ctx, input)
	if err == nil {
		t.Error("expected error for non-existent fork")
	}
}

func TestForkBashToolDuplicateName(t *testing.T) {
	tool := NewForkBashTool()
	ctx := context.Background()

	// Start a long-running fork
	input, _ := json.Marshal(map[string]any{
		"action":  "start",
		"name":    "dup-test",
		"command": "sleep 60",
	})
	_, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("first start failed: %v", err)
	}

	// Try to start another with same name while first is running
	_, err = tool.Execute(ctx, input)
	if err == nil {
		t.Error("expected error for duplicate running fork name")
	}

	// Clean up
	termInput, _ := json.Marshal(map[string]string{"action": "terminate", "name": "dup-test"})
	_, _ = tool.Execute(ctx, termInput)
}

func TestForkBashToolListEmpty(t *testing.T) {
	tool := NewForkBashTool()
	ctx := context.Background()

	input, _ := json.Marshal(map[string]string{"action": "list"})
	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	var lr map[string]any
	if err := json.Unmarshal(result, &lr); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	total, ok := lr["total"].(float64)
	if !ok || total != 0 {
		t.Errorf("expected total 0, got %v", lr["total"])
	}
}

func TestForkBashToolFailedCommand(t *testing.T) {
	tool := NewForkBashTool()
	ctx := context.Background()

	// Start a fork that will fail
	input, _ := json.Marshal(map[string]any{
		"action":  "start",
		"name":    "fail-test",
		"command": "exit 42",
	})
	_, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// Wait for it to complete
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		listInput, _ := json.Marshal(map[string]string{"action": "list"})
		listResult, err := tool.Execute(ctx, listInput)
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}

		var lr map[string]any
		if err := json.Unmarshal(listResult, &lr); err != nil {
			t.Fatalf("failed to unmarshal list result: %v", err)
		}

		forks := lr["forks"].([]any)
		if len(forks) > 0 {
			fork := forks[0].(map[string]any)
			if fork["status"] == "failed" {
				exitCode, ok := fork["exit_code"].(float64)
				if !ok || exitCode != 42 {
					t.Errorf("expected exit_code 42, got %v", fork["exit_code"])
				}
				return
			}
		}

		time.Sleep(50 * time.Millisecond)
	}
	t.Error("fork did not complete with failure status within deadline")
}

func TestForkBashToolTerminateNonRunning(t *testing.T) {
	tool := NewForkBashTool()
	ctx := context.Background()

	// Start a quick command
	input, _ := json.Marshal(map[string]any{
		"action":  "start",
		"name":    "quick",
		"command": "echo done",
	})
	_, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// Wait for it to complete
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tool.mu.Lock()
		status := tool.forks["quick"].Status
		tool.mu.Unlock()
		if status != "running" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Try to terminate completed fork
	termInput, _ := json.Marshal(map[string]string{"action": "terminate", "name": "quick"})
	_, err = tool.Execute(ctx, termInput)
	if err == nil {
		t.Error("expected error when terminating non-running fork")
	}
}
