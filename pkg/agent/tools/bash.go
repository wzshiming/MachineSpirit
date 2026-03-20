package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/agent"
)

// BashTool executes shell commands.
type BashTool struct{}

// NewBashTool creates a new Bash tool.
func NewBashTool() *BashTool {
	return &BashTool{}
}

func (t *BashTool) Name() string {
	return "bash"
}

func (t *BashTool) Description() string {
	return "Execute a shell command and return its stdout, stderr, and exit code."
}

func (t *BashTool) Parameters() []agent.ToolParameter {
	return []agent.ToolParameter{
		{Name: "command", Type: "string", Required: true, Description: "The shell command to execute."},
		{Name: "timeoutSecond", Type: "int", Required: false, Description: "Timeout in seconds. 0 means no timeout."},
	}
}

func (t *BashTool) Enabled() bool {
	return true
}

func (t *BashTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Command       string `json:"command"`
		TimeoutSecond int    `json:"timeoutSecond"`
	}

	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if params.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	if params.TimeoutSecond > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(params.TimeoutSecond)*time.Second)
		defer cancel()
	}

	// Create command with context for cancellation
	cmd := exec.CommandContext(ctx, "bash", "-c", params.Command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	status := "success"
	exitCode := 0

	if runErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			status = "timeout"
			exitCode = -1
		} else if exitErr, ok := runErr.(*exec.ExitError); ok {
			status = "failure"
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("command failed: %w", runErr)
		}
	}

	result, err := json.Marshal(map[string]any{
		"command":   params.Command,
		"stdout":    stdout.String(),
		"stderr":    stderr.String(),
		"status":    status,
		"exit_code": exitCode,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal results: %w", err)
	}

	return json.RawMessage(result), nil
}
