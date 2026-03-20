package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/wzshiming/MachineSpirit/pkg/agent"
)

// ReadTool allows the agent to read files.
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
	return "Read the content of a file, optionally limiting to a range of lines."
}

func (t *ReadTool) Parameters() []agent.ToolParameter {
	return []agent.ToolParameter{
		{Name: "path", Type: "string", Required: true, Description: "Absolute path to the file to read."},
		{Name: "max", Type: "int", Required: false, Description: "Maximum number of lines to read. 0 means all lines."},
		{Name: "start", Type: "int", Required: false, Description: "Line number to start reading from (1-based). 0 means from the beginning."},
	}
}

func (t *ReadTool) Enabled() bool {
	return true
}

func (t *ReadTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Path  string `json:"path"`
		Max   int    `json:"max"`
		Start int    `json:"start"`
	}

	err := json.Unmarshal(input, &params)
	if err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if params.Path == "" {
		return nil, fmt.Errorf("path parameter is required")
	}

	file, err := os.Open(params.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		// Skip lines before start
		if params.Start > 0 && lineNum < params.Start {
			continue
		}
		// Stop if we've reached max lines
		if params.Max > 0 && len(lines) >= params.Max {
			break
		}
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	content := strings.Join(lines, "\n")

	result, err := json.Marshal(map[string]any{
		"path":    params.Path,
		"content": content,
		"status":  "success",
		"lines":   len(lines),
		"start":   params.Start,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return json.RawMessage(result), nil
}
