package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestBashTool_Name(t *testing.T) {
	tool := NewBashTool()
	if tool.Name() != "bash" {
		t.Errorf("expected name 'bash', got %q", tool.Name())
	}
}

func TestBashTool_BasicCommand(t *testing.T) {
	tool := NewBashTool()
	input, _ := json.Marshal(map[string]any{
		"command": "echo hello",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res map[string]any
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if res["status"] != "success" {
		t.Errorf("expected status 'success', got %v", res["status"])
	}
	if res["output"] != "hello\n" {
		t.Errorf("expected output 'hello\\n', got %q", res["output"])
	}
	if res["exit_code"] != float64(0) {
		t.Errorf("expected exit_code 0, got %v", res["exit_code"])
	}
}

func TestBashTool_EmptyCommand(t *testing.T) {
	tool := NewBashTool()
	input, _ := json.Marshal(map[string]any{
		"command": "",
	})

	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestBashTool_InvalidInput(t *testing.T) {
	tool := NewBashTool()

	_, err := tool.Execute(context.Background(), json.RawMessage(`invalid`))
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
}

func TestBashTool_Stdin(t *testing.T) {
	tool := NewBashTool()
	input, _ := json.Marshal(map[string]any{
		"command": "cat",
		"stdin":   "hello from stdin",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res map[string]any
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if res["status"] != "success" {
		t.Errorf("expected status 'success', got %v", res["status"])
	}
	if res["output"] != "hello from stdin" {
		t.Errorf("expected output 'hello from stdin', got %q", res["output"])
	}
}

func TestBashTool_StdinMultiline(t *testing.T) {
	tool := NewBashTool()
	input, _ := json.Marshal(map[string]any{
		"command": "head -n 1",
		"stdin":   "line1\nline2\nline3\n",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res map[string]any
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if res["output"] != "line1\n" {
		t.Errorf("expected output 'line1\\n', got %q", res["output"])
	}
}

func TestBashTool_Timeout(t *testing.T) {
	tool := NewBashTool()
	input, _ := json.Marshal(map[string]any{
		"command": "sleep 10",
		"timeout": 1,
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res map[string]any
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if res["status"] != "timeout" {
		t.Errorf("expected status 'timeout', got %v", res["status"])
	}
	if res["exit_code"] != float64(-1) {
		t.Errorf("expected exit_code -1, got %v", res["exit_code"])
	}
}

func TestBashTool_TimeoutWithPartialOutput(t *testing.T) {
	tool := NewBashTool()
	input, _ := json.Marshal(map[string]any{
		"command": "echo partial && sleep 10",
		"timeout": 1,
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res map[string]any
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if res["status"] != "timeout" {
		t.Errorf("expected status 'timeout', got %v", res["status"])
	}
	if res["output"] != "partial\n" {
		t.Errorf("expected partial output, got %q", res["output"])
	}
}

func TestBashTool_NonZeroExit(t *testing.T) {
	tool := NewBashTool()
	input, _ := json.Marshal(map[string]any{
		"command": "exit 1",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res map[string]any
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if res["status"] != "failure" {
		t.Errorf("expected status 'failure', got %v", res["status"])
	}
	if res["exit_code"] != float64(1) {
		t.Errorf("expected exit_code 1, got %v", res["exit_code"])
	}
}

func TestBashTool_Stderr(t *testing.T) {
	tool := NewBashTool()
	input, _ := json.Marshal(map[string]any{
		"command": "echo error_msg >&2",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res map[string]any
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if res["stderr"] != "error_msg\n" {
		t.Errorf("expected stderr 'error_msg\\n', got %q", res["stderr"])
	}
	if res["output"] != "" {
		t.Errorf("expected empty stdout, got %q", res["output"])
	}
}

func TestBashTool_StdoutAndStderr(t *testing.T) {
	tool := NewBashTool()
	input, _ := json.Marshal(map[string]any{
		"command": "echo out && echo err >&2",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res map[string]any
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if res["output"] != "out\n" {
		t.Errorf("expected stdout 'out\\n', got %q", res["output"])
	}
	if res["stderr"] != "err\n" {
		t.Errorf("expected stderr 'err\\n', got %q", res["stderr"])
	}
}

func TestBashTool_StdinWithPassword(t *testing.T) {
	// Simulates an interactive command that reads a password from stdin
	tool := NewBashTool()
	input, _ := json.Marshal(map[string]any{
		"command": `read -r password && echo "got: $password"`,
		"stdin":   "mysecret\n",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res map[string]any
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if res["status"] != "success" {
		t.Errorf("expected status 'success', got %v", res["status"])
	}
	if res["output"] != "got: mysecret\n" {
		t.Errorf("expected output 'got: mysecret\\n', got %q", res["output"])
	}
}

func TestBashTool_NoTimeoutNoStdin(t *testing.T) {
	// Backward compatibility: calling without new params works like before
	tool := NewBashTool()
	input, _ := json.Marshal(map[string]any{
		"command": "echo backward",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res map[string]any
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if res["command"] != "echo backward" {
		t.Errorf("expected command 'echo backward', got %v", res["command"])
	}
	if res["output"] != "backward\n" {
		t.Errorf("expected output 'backward\\n', got %q", res["output"])
	}
	if res["status"] != "success" {
		t.Errorf("expected status 'success', got %v", res["status"])
	}
}

func TestBashTool_ContextCancellation(t *testing.T) {
	tool := NewBashTool()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	input, _ := json.Marshal(map[string]any{
		"command": "echo should not run",
	})

	_, err := tool.Execute(ctx, input)
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}
