package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
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
	return "Read the current content of a file. {\"path\": \"/path/to/file\", \"max\": 100, \"start\": 1}." +
		" max: limit number of lines, start: line number to start from (1-based)."
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
