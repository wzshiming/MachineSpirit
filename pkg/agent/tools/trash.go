package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// TrashTool allows the agent to delete files and directories.
type TrashTool struct {
}

// NewTrashTool creates a new Trash tool.
func NewTrashTool() *TrashTool {
	return &TrashTool{}
}

func (t *TrashTool) Name() string {
	return "trash"
}

func (t *TrashTool) Description() string {
	return "Delete a file or directory. Use 'recursive' for directories with content. {\"path\": \"/path/to/file\", \"recursive\": false}."
}

func (t *TrashTool) Enabled() bool {
	return true
}

func (t *TrashTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}

	err := json.Unmarshal(input, &params)
	if err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if params.Path == "" {
		return nil, fmt.Errorf("path parameter is required")
	}

	// Check if path exists
	info, err := os.Stat(params.Path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("path does not exist: %s", params.Path)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	// For directories, check if recursive flag is needed
	if info.IsDir() {
		if !params.Recursive {
			// Try to remove empty directory
			err = os.Remove(params.Path)
			if err != nil {
				return nil, fmt.Errorf("directory is not empty, use recursive: true to delete: %w", err)
			}
		} else {
			// Remove directory recursively
			err = os.RemoveAll(params.Path)
			if err != nil {
				return nil, fmt.Errorf("failed to delete directory recursively: %w", err)
			}
		}
	} else {
		// Delete file
		err = os.Remove(params.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to delete file: %w", err)
		}
	}

	result, err := json.Marshal(map[string]any{
		"path":      params.Path,
		"recursive": params.Recursive,
		"status":    "success",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return json.RawMessage(result), nil
}
