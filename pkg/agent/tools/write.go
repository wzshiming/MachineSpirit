package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/wzshiming/MachineSpirit/pkg/agent"
)

// WriteTool allows the agent to write files.
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
	return "Write content to a file. Creates the file if it does not exist."
}

func (t *WriteTool) Parameters() []agent.ToolParameter {
	return []agent.ToolParameter{
		{Name: "path", Type: "string", Required: true, Description: "Absolute path to the file to write."},
		{Name: "content", Type: "string", Required: true, Description: "The content to write to the file."},
		{Name: "append", Type: "bool", Required: false, Description: "If true, append to the file instead of overwriting. Default is false."},
	}
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

	var file *os.File
	if params.Append {
		file, err = os.OpenFile(params.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	} else {
		file, err = os.OpenFile(params.Path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString(params.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	result, err := json.Marshal(map[string]any{
		"path":   params.Path,
		"status": "success",
		"append": params.Append,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return json.RawMessage(result), nil
}
