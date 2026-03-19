package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// MoveTool allows the agent to move or rename files and directories.
type MoveTool struct {
}

// NewMoveTool creates a new Move tool.
func NewMoveTool() *MoveTool {
	return &MoveTool{}
}

func (t *MoveTool) Name() string {
	return "move"
}

func (t *MoveTool) Description() string {
	return "Move or rename a file or directory. {\"source\": \"/path/to/source\", \"destination\": \"/path/to/destination\"}."
}

func (t *MoveTool) Enabled() bool {
	return true
}

func (t *MoveTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Source      string `json:"source"`
		Destination string `json:"destination"`
	}

	err := json.Unmarshal(input, &params)
	if err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if params.Source == "" {
		return nil, fmt.Errorf("source parameter is required")
	}

	if params.Destination == "" {
		return nil, fmt.Errorf("destination parameter is required")
	}

	// Check if source exists
	if _, err := os.Stat(params.Source); os.IsNotExist(err) {
		return nil, fmt.Errorf("source does not exist: %s", params.Source)
	}

	// Perform the move/rename operation
	err = os.Rename(params.Source, params.Destination)
	if err != nil {
		return nil, fmt.Errorf("failed to move: %w", err)
	}

	result, err := json.Marshal(map[string]any{
		"source":      params.Source,
		"destination": params.Destination,
		"status":      "success",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return json.RawMessage(result), nil
}
