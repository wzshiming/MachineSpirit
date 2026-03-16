package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// WriteTool allows the agent to read  files.
type WriteTool struct {
}

// NewWriteTool creates a new Read tool.
func NewWriteTool() *WriteTool {
	return &WriteTool{}
}

func (t *WriteTool) Name() string {
	return "write"
}

func (t *WriteTool) Description() string {
	return "Write content to a file. {\"path\": \"/path/to/file\", \"content\": \"new content\"}."
}

func (t *WriteTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}

	err := json.Unmarshal(input, &params)
	if err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if params.Path == "" {
		return nil, fmt.Errorf("path parameter is required")
	}

	err = os.WriteFile(params.Path, []byte(params.Content), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	result, err := json.Marshal(map[string]any{
		"path":   params.Path,
		"status": "success",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return json.RawMessage(result), nil
}
