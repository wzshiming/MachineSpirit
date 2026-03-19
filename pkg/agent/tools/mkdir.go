package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// MkdirTool allows the agent to create directories.
type MkdirTool struct {
}

// NewMkdirTool creates a new Mkdir tool.
func NewMkdirTool() *MkdirTool {
	return &MkdirTool{}
}

func (t *MkdirTool) Name() string {
	return "mkdir"
}

func (t *MkdirTool) Description() string {
	return "Create a new directory. Set 'parents' to true to create parent directories as needed. {\"path\": \"/path/to/directory\", \"parents\": true, \"mode\": \"0755\"}."
}

func (t *MkdirTool) Enabled() bool {
	return true
}

func (t *MkdirTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Path    string `json:"path"`
		Parents bool   `json:"parents"`
		Mode    string `json:"mode"`
	}

	err := json.Unmarshal(input, &params)
	if err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if params.Path == "" {
		return nil, fmt.Errorf("path parameter is required")
	}

	// Default mode
	mode := os.FileMode(0755)
	if params.Mode != "" {
		var modeInt uint32
		_, err := fmt.Sscanf(params.Mode, "%o", &modeInt)
		if err != nil {
			return nil, fmt.Errorf("invalid mode format, use octal like '0755': %w", err)
		}
		mode = os.FileMode(modeInt)
	}

	// Create directory
	if params.Parents {
		err = os.MkdirAll(params.Path, mode)
	} else {
		err = os.Mkdir(params.Path, mode)
	}

	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("directory already exists: %s", params.Path)
		}
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	result, err := json.Marshal(map[string]any{
		"path":    params.Path,
		"parents": params.Parents,
		"mode":    params.Mode,
		"status":  "success",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return json.RawMessage(result), nil
}
