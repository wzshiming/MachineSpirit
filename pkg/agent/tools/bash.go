package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
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
	return "Execute a shell command. Returns the command output. {\"command\": \"cd /tmp && ls\"}"
}

func (t *BashTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Command string `json:"command"`
	}

	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if params.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// Create command with context for cancellation
	cmd := exec.CommandContext(ctx, "bash", "-c", params.Command)

	// Capture output
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	exitCode := cmd.ProcessState.ExitCode()
	status := "success"
	if exitCode != 0 {
		status = "failure"
	}
	result, err := json.Marshal(map[string]any{
		"command":   params.Command,
		"output":    string(output),
		"status":    status,
		"exit_code": exitCode,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal results: %w", err)
	}

	return json.RawMessage(result), nil
}
