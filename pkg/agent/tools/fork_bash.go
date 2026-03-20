package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"sync"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/agent"
)

// forkBashInfo tracks the state of a running or completed forked bash command.
type forkBashInfo struct {
	Name      string    `json:"name"`
	Command   string    `json:"command"`
	Status    string    `json:"status"` // "running", "completed", "failed", "terminated"
	Stdout    string    `json:"stdout,omitempty"`
	Stderr    string    `json:"stderr,omitempty"`
	ExitCode  int       `json:"exit_code,omitempty"`
	Error     string    `json:"error,omitempty"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`

	cancel context.CancelFunc
}

// ForkBashTool allows the agent to run bash commands in the background,
// list running/completed commands, and terminate them.
type ForkBashTool struct {
	mu    sync.Mutex
	forks map[string]*forkBashInfo
}

// NewForkBashTool creates a new ForkBashTool.
func NewForkBashTool() *ForkBashTool {
	return &ForkBashTool{
		forks: make(map[string]*forkBashInfo),
	}
}

func (t *ForkBashTool) Name() string {
	return "fork_bash"
}

func (t *ForkBashTool) Description() string {
	return `Run a bash command in the background (fork), list running/completed forks and their output, or terminate a running fork.
Use this tool when you need to run long-running commands without blocking the conversation, monitor background processes, or cancel them when no longer needed.`
}

func (t *ForkBashTool) Parameters() []agent.ToolParameter {
	return []agent.ToolParameter{
		{Name: "action", Type: "string", Required: true, Description: "The action to perform: 'start' to fork a new command, 'list' to list all forks, 'terminate' to stop a running fork."},
		{Name: "name", Type: "string", Required: false, Description: "Unique name for the fork (required for 'start' and 'terminate')."},
		{Name: "command", Type: "string", Required: false, Description: "The shell command to execute (required for 'start')."},
		{Name: "timeoutSecond", Type: "int", Required: false, Description: "Timeout in seconds for the forked command. 0 means no timeout (only for 'start')."},
	}
}

func (t *ForkBashTool) Enabled() bool {
	return true
}

func (t *ForkBashTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Action        string `json:"action"`
		Name          string `json:"name"`
		Command       string `json:"command"`
		TimeoutSecond int    `json:"timeoutSecond"`
	}

	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	switch params.Action {
	case "start":
		return t.startFork(params.Name, params.Command, params.TimeoutSecond)
	case "list":
		return t.listForks()
	case "terminate":
		return t.terminateFork(params.Name)
	default:
		return nil, fmt.Errorf("unknown action %q, use 'start', 'list', or 'terminate'", params.Action)
	}
}

func (t *ForkBashTool) startFork(name, command string, timeoutSecond int) (json.RawMessage, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required for 'start' action")
	}
	if command == "" {
		return nil, fmt.Errorf("command is required for 'start' action")
	}

	t.mu.Lock()
	if existing, ok := t.forks[name]; ok && existing.Status == "running" {
		t.mu.Unlock()
		return nil, fmt.Errorf("fork %q is already running", name)
	}

	ctx, cancel := context.WithCancel(context.Background())
	if timeoutSecond > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(timeoutSecond)*time.Second)
	}

	info := &forkBashInfo{
		Name:      name,
		Command:   command,
		Status:    "running",
		StartTime: time.Now(),
		cancel:    cancel,
	}
	t.forks[name] = info
	t.mu.Unlock()

	go t.runFork(ctx, info)

	result, err := json.Marshal(map[string]any{
		"status":  "started",
		"name":    name,
		"command": command,
		"message": fmt.Sprintf("Fork %q started in background. Use 'list' to check status or 'terminate' to stop it.", name),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return json.RawMessage(result), nil
}

func (t *ForkBashTool) runFork(ctx context.Context, info *forkBashInfo) {
	defer info.cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", info.Command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	t.mu.Lock()
	defer t.mu.Unlock()

	info.EndTime = time.Now()
	info.Stdout = stdout.String()
	info.Stderr = stderr.String()

	if runErr != nil {
		if ctx.Err() == context.Canceled {
			info.Status = "terminated"
			info.ExitCode = -1
			slog.Info("Fork terminated", "name", info.Name)
		} else if ctx.Err() == context.DeadlineExceeded {
			info.Status = "failed"
			info.Error = "timeout"
			info.ExitCode = -1
			slog.Warn("Fork timed out", "name", info.Name)
		} else if exitErr, ok := runErr.(*exec.ExitError); ok {
			info.Status = "failed"
			info.ExitCode = exitErr.ExitCode()
			info.Error = fmt.Sprintf("exit code %d", exitErr.ExitCode())
			slog.Warn("Fork failed", "name", info.Name, "exit_code", exitErr.ExitCode())
		} else {
			info.Status = "failed"
			info.Error = runErr.Error()
			slog.Warn("Fork failed", "name", info.Name, "error", runErr)
		}
	} else {
		info.Status = "completed"
		info.ExitCode = 0
		slog.Info("Fork completed", "name", info.Name)
	}
}

func (t *ForkBashTool) listForks() (json.RawMessage, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	forks := make([]map[string]any, 0, len(t.forks))
	for _, info := range t.forks {
		entry := map[string]any{
			"name":       info.Name,
			"command":    info.Command,
			"status":     info.Status,
			"start_time": info.StartTime.Format(time.RFC3339),
		}
		if !info.EndTime.IsZero() {
			entry["end_time"] = info.EndTime.Format(time.RFC3339)
			entry["duration"] = info.EndTime.Sub(info.StartTime).String()
		}
		if info.Stdout != "" {
			entry["stdout"] = info.Stdout
		}
		if info.Stderr != "" {
			entry["stderr"] = info.Stderr
		}
		if info.ExitCode != 0 {
			entry["exit_code"] = info.ExitCode
		}
		if info.Error != "" {
			entry["error"] = info.Error
		}
		forks = append(forks, entry)
	}

	result, err := json.Marshal(map[string]any{
		"forks": forks,
		"total": len(forks),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return json.RawMessage(result), nil
}

func (t *ForkBashTool) terminateFork(name string) (json.RawMessage, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required for 'terminate' action")
	}

	t.mu.Lock()
	info, ok := t.forks[name]
	if !ok {
		t.mu.Unlock()
		return nil, fmt.Errorf("fork %q not found", name)
	}
	if info.Status != "running" {
		t.mu.Unlock()
		return nil, fmt.Errorf("fork %q is not running (status: %s)", name, info.Status)
	}
	t.mu.Unlock()

	// Cancel the context to terminate the command.
	info.cancel()

	// Wait briefly for the goroutine to update the status.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		t.mu.Lock()
		status := info.Status
		t.mu.Unlock()
		if status != "running" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.mu.Lock()
	status := info.Status
	t.mu.Unlock()

	result, err := json.Marshal(map[string]any{
		"status":  status,
		"name":    name,
		"message": fmt.Sprintf("Fork %q has been terminated.", name),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return json.RawMessage(result), nil
}
