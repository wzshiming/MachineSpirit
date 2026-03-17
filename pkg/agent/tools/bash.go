package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// BashTool executes shell commands with optional stdin input and timeout.
type BashTool struct{}

// NewBashTool creates a new Bash tool.
func NewBashTool() *BashTool {
	return &BashTool{}
}

func (t *BashTool) Name() string {
	return "bash"
}

func (t *BashTool) Description() string {
	return `Execute a shell command. Supports interactive commands via stdin and timeout. {"command": "cd /tmp && ls", "stdin": "optional input for interactive commands", "timeout": 30}`
}

func (t *BashTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Command string `json:"command"`
		Stdin   string `json:"stdin"`
		Timeout int    `json:"timeout"`
	}

	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if params.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// Apply timeout if specified
	if params.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(params.Timeout)*time.Second)
		defer cancel()
	}

	// Create command with context for cancellation
	cmd := exec.CommandContext(ctx, "bash", "-c", params.Command)

	// Provide stdin input if specified (for interactive commands like passwords)
	if params.Stdin != "" {
		cmd.Stdin = strings.NewReader(params.Stdin)
	}

	// Capture stdout and stderr separately
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
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
		"output":    stdout.String(),
		"stderr":    stderr.String(),
		"status":    status,
		"exit_code": exitCode,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal results: %w", err)
	}

	return json.RawMessage(result), nil
}
