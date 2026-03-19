package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteTool allows the agent to write content to files.
type WriteTool struct {
}

// NewWriteTool creates a new Write tool.
func NewWriteTool() *WriteTool {
	return &WriteTool{}
}

func (t *WriteTool) Name() string {
	return "write"
}

func (t *WriteTool) Description() string {
	return "Write content to a file. Creates parent directories if needed. Set 'append' to true to add to existing content. {\"path\": \"/path/to/file\", \"content\": \"new content\", \"append\": false}."
}

func (t *WriteTool) Enabled() bool {
	return true
}

func (t *WriteTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Append  bool   `json:"append"`
	}

	err := json.Unmarshal(input, &params)
	if err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if params.Path == "" {
		return nil, fmt.Errorf("path parameter is required")
	}

	// Create parent directory if it doesn't exist
	dir := filepath.Dir(params.Path)
	if dir != "" && dir != "." {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return nil, fmt.Errorf("failed to create parent directory: %w", err)
		}
	}

	// Handle append mode
	if params.Append {
		file, err := os.OpenFile(params.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open file for append: %w", err)
		}
		defer file.Close()

		_, err = file.WriteString(params.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to append to file: %w", err)
		}
	} else {
		err = os.WriteFile(params.Path, []byte(params.Content), 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}
	}

	// Get file info for response
	info, err := os.Stat(params.Path)
	bytesWritten := int64(0)
	if err == nil {
		bytesWritten = info.Size()
	}

	result, err := json.Marshal(map[string]any{
		"path":          params.Path,
		"bytes_written": bytesWritten,
		"append":        params.Append,
		"status":        "success",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return json.RawMessage(result), nil
}
