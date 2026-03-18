package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// ReadTool allows the agent to read  files.
type ReadTool struct {
}

// NewReadTool creates a new Read tool.
func NewReadTool() *ReadTool {
	return &ReadTool{}
}

func (t *ReadTool) Name() string {
	return "read"
}

func (t *ReadTool) Description() string {
	return "Read the current content of a file. {\"path\": \"/path/to/file\"}."
}

func (t *ReadTool) Enabled() bool {
	return true
}

func (t *ReadTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Path string `json:"path"`
	}

	err := json.Unmarshal(input, &params)
	if err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if params.Path == "" {
		return nil, fmt.Errorf("file parameter is required")
	}

	content, err := os.ReadFile(params.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	result, err := json.Marshal(map[string]any{
		"path":    params.Path,
		"content": string(content),
		"status":  "success",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return json.RawMessage(result), nil
}
